// Command large-response-backend is a small HTTP backend used by the ProxySettingsPolicy functional tests.
//
//   - /coffee returns a 10 KB response header and a 5 MB streamed body. This exceeds NGINX's default
//     proxy_buffer_size/proxy_buffers, so the tests can verify that the buffering settings of a
//     ProxySettingsPolicy are applied.
//   - /health returns 200 and is used for the readiness and liveness probes.
//
// It is built into an image ahead of time instead of being compiled in the pod with `go run`, which
// was slow enough on busy CI runners for the liveness probe to restart the pod before it became ready.
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func coffeeHandler(w http.ResponseWriter, _ *http.Request) {
	// Returns a large response to demonstrate buffering requirements
	// This generates a response larger than NGINX's default proxy_buffer_size (4k/8k)
	// Without proper buffering configuration, this may cause errors
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Generate a large header that exceeds default proxy_buffer_size
	// Default is typically 4k-8k depending on platform
	largeHeader := strings.Repeat("X", 10000)
	w.Header().Set("X-Large-Header", largeHeader)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Generate a large response body (5MB in chunks)
	// This tests both proxy_buffer_size and proxy_buffers settings
	chunkSize := 1024   // 1KB chunks
	totalChunks := 5120 // 5MB total

	for i := 0; i < totalChunks; i++ {
		chunk := fmt.Sprintf("Coffee chunk %d: %s\n", i, strings.Repeat("x", chunkSize-20))
		fmt.Fprint(w, chunk)
		if i%10 == 0 {
			flusher.Flush()
		}
		time.Sleep(100 * time.Microsecond) // Small delay to simulate slow backend
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func main() {
	http.HandleFunc("/coffee", coffeeHandler)
	http.HandleFunc("/health", healthHandler)

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
