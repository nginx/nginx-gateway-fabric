// Command guardrails-mock serves two mock roles used by the ai-guardrails functional tests:
//
//  1. Mock Guardrails API — POST /backend/v1/scans. Returns a deterministic verdict driven by the
//     inspected content:
//
//     request:  {"input":"<text>","configOverrides":{},"forceEnabled":[],"disabled":[],
//     "scanDirection":"request"|"response","verbose":false}
//     response: {"result":{"outcome":"cleared"}}                         (allow)
//     {"result":{"outcome":"flagged","scannerResults":[         (block)
//     {"outcome":"failed","message":"<msg>"}]}}
//
//     Verdict rule: the request-path scan flags input containing the request sentinel (default
//     "BLOCKME"); the response-path scan flags input containing either the request sentinel or the
//     response sentinel (default "BLOCKRESP"). Distinct sentinels let request-path and response-path
//     blocking be exercised independently: a prompt carrying only the response sentinel passes the
//     request scan and is flagged only after being echoed into the model output.
//
//  2. Mock LLM — POST /v1/completions. Returns a non-streaming OpenAI-shaped completion whose
//     choices[].text echoes the request "prompt". This lets a prompt (which the response path then
//     inspects as model output) carry the sentinel so response-path blocking can be triggered.
//
// A single binary serves both roles so only one image is built; the two roles are deployed as
// separate Services (guardrails-api and the mock LLM) pointing at the same Deployment image.
//
// TLS: when both TLS_CERT_FILE and TLS_KEY_FILE env vars are set the server listens over HTTPS
// (ListenAndServeTLS) instead of HTTP. This backs the in-cluster HTTPS guardrails backend used by
// the BackendTLSPolicy functional test; when unset the server stays HTTP-only (default behavior).
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

// Protocol constants mirroring the Guardrails wire contract. These values must
// stay in sync with that module's ScanDirection and parse_outcome; a mismatch silently breaks the
// mock (the module fails closed) and surfaces as confusing test failures.
const (
	// scanDirectionResponse is the scanDirection the module sends on the response path.
	scanDirectionResponse = "response"
	// outcomeFlagged / outcomeCleared are the two result.outcome values the module recognizes.
	outcomeFlagged = "flagged"
	outcomeCleared = "cleared"
	// scannerOutcomeFailed is the scannerResults[].outcome the module reads the block message from.
	scannerOutcomeFailed = "failed"
)

// scanRequest is the subset of the module's scan request we care about.
type scanRequest struct {
	Input         string `json:"input"`
	ScanDirection string `json:"scanDirection"`
}

// scannerResult mirrors one entry in result.scannerResults.
type scannerResult struct {
	Outcome string `json:"outcome"`
	Message string `json:"message,omitempty"`
}

// scanResult is the result object nested in the scan response.
type scanResult struct {
	Outcome        string          `json:"outcome"`
	ScannerResults []scannerResult `json:"scannerResults,omitempty"`
}

// scanResponse is the top-level scan response body.
type scanResponse struct {
	Result scanResult `json:"result"`
}

// completionRequest is the subset of an OpenAI /v1/completions request we echo.
type completionRequest struct {
	Prompt string `json:"prompt"`
}

// completionChoice mirrors one OpenAI non-streaming completion choice.
type completionChoice struct {
	Text  string `json:"text"`
	Index int    `json:"index"`
}

// completionResponse is a minimal non-streaming OpenAI completion response.
type completionResponse struct {
	Object  string             `json:"object"`
	Choices []completionChoice `json:"choices"`
}

func main() {
	addr := envOr("LISTEN_ADDR", ":8080")
	// requestSentinel flags the request-path scan; responseSentinel flags the response-path scan.
	// Using distinct sentinels lets each direction be exercised independently: a prompt carrying
	// only responseSentinel passes the request scan, reaches the mock LLM, is echoed into the
	// completion, and is then flagged by the response scan (surfacing as an api_error 403).
	requestSentinel := envOr("BLOCK_SENTINEL", "BLOCKME")
	responseSentinel := envOr("RESPONSE_BLOCK_SENTINEL", "BLOCKRESP")
	blockMessage := envOr("BLOCK_MESSAGE", "blocked by test guardrail")

	mux := http.NewServeMux()
	mux.HandleFunc("/backend/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req scanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Fail the scan loudly so the module fails closed rather than silently allowing.
			log.Printf("failed to decode scan request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// The response direction flags on either sentinel so a request-side block sentinel that
		// survives into the echoed completion is still caught; the request direction flags only on
		// the request sentinel so a response-only sentinel passes the request scan untouched.
		var blocked bool
		if req.ScanDirection == scanDirectionResponse {
			blocked = strings.Contains(req.Input, requestSentinel) || strings.Contains(req.Input, responseSentinel)
		} else {
			blocked = strings.Contains(req.Input, requestSentinel)
		}
		log.Printf("scan direction=%q blocked=%v inputLen=%d", req.ScanDirection, blocked, len(req.Input))

		var resp scanResponse
		if blocked {
			resp.Result = scanResult{
				Outcome: outcomeFlagged,
				ScannerResults: []scannerResult{
					{Outcome: scannerOutcomeFailed, Message: blockMessage},
				},
			}
		} else {
			resp.Result = scanResult{Outcome: outcomeCleared}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode scan response: %v", err)
		}
	})

	// Mock LLM: echo the request prompt back as an OpenAI-shaped non-streaming completion so the
	// response path inspects text that can carry the sentinel.
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req completionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("failed to decode completion request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		resp := completionResponse{
			Object: "text_completion",
			Choices: []completionChoice{
				{Text: "echo: " + req.Prompt, Index: 0},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode completion response: %v", err)
		}
	})

	// Liveness/readiness endpoint.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Optional TLS: when both TLS_CERT_FILE and TLS_KEY_FILE are set the mock serves HTTPS instead
	// of HTTP. This lets the same image back an in-cluster HTTPS guardrails Service (verified by NGINX
	// via a BackendTLSPolicy) without a separate TLS-terminating sidecar. When either is unset the
	// mock stays HTTP-only, preserving the default (plaintext) behavior.
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	server := &http.Server{Addr: addr, Handler: mux}
	if certFile != "" && keyFile != "" {
		log.Printf(
			"guardrails-mock listening on %s over TLS (requestSentinel=%q responseSentinel=%q)",
			addr, requestSentinel, responseSentinel,
		)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	log.Printf(
		"guardrails-mock listening on %s (requestSentinel=%q responseSentinel=%q)",
		addr, requestSentinel, responseSentinel,
	)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
