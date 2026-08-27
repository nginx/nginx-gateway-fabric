//! Streaming inspection logic with checkpoint buffering and content extraction.

use serde::Deserialize;

/// Unified LLM streaming chunk covering OpenAI and Ollama wire formats.
/// Using typed structs avoids allocating a full dynamic map (`serde_json::Value`)
/// for every SSE chunk received from upstream.
#[derive(Deserialize)]
struct LlmChunk {
    /// Ollama stream-completion flag.
    done: Option<bool>,
    /// Ollama chat message payload (`/api/chat`).
    message: Option<OllamaMessage>,
    /// Ollama generate output (`/api/generate`): the generated text is a
    /// top-level `response` string (streaming and non-streaming both use it).
    response: Option<String>,
    /// OpenAI choices array (streaming chat delta, non-streaming chat
    /// `message.content`, and legacy completions `text`).
    choices: Option<Vec<OpenAIChoice>>,
}

#[derive(Deserialize)]
struct OllamaMessage {
    content: Option<String>,
}

#[derive(Deserialize)]
struct OpenAIChoice {
    /// Wire choice index (OpenAI `n > 1`). Absent on single-choice responses and
    /// some provider variants → treated as 0. Used to keep the fragments of one
    /// choice contiguous even when choices interleave across streamed chunks.
    index: Option<u32>,
    /// Streaming chat completions delta (`/v1/chat/completions`, `stream:true`).
    delta: Option<OpenAIDelta>,
    /// Non-streaming chat completions message (`/v1/chat/completions`,
    /// `stream:false`): the generated text lives in `message.content`.
    message: Option<OpenAIMessage>,
    /// Legacy non-streaming completions text (`/v1/completions`).
    text: Option<String>,
}

#[derive(Deserialize)]
struct OpenAIDelta {
    content: Option<String>,
}

#[derive(Deserialize)]
struct OpenAIMessage {
    content: Option<String>,
}

/// Verdict of an asynchronous response inspection.
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum ResponseVerdict {
    /// Content cleared — release the buffered response to the client.
    Allow,
    /// Content blocked (or inspection errored under fail-closed) — send the
    /// error/termination body instead of the buffered response.
    Block,
}

/// State machine for the response path's single end-of-stream inspection.
///
/// The transitions are `Idle -> Pending -> Done -> Resumed`. `Done` carries the
/// verdict recorded by the async completion callback; the posted write-event
/// handler consumes it exactly once and moves to `Resumed`, which guarantees the
/// commit-headers/flush/finalize sequence runs **exactly once** even if the
/// connection write event fires again.
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum ResponseInspect {
    /// No async inspection has been started yet.
    Idle,
    /// A subrequest is in flight. The body filter returned `NGX_OK` without
    /// forwarding the final buffer; the request is kept alive by the in-flight
    /// subrequest (`ngx_http_subrequest` bumps `r->main->count`). No further
    /// upstream data is forwarded until the verdict resumes output.
    Pending,
    /// The verdict has been recorded by the async completion callback, and a
    /// write event has been posted to drive `guardrails_resume_write_handler`
    /// on the next clean event-loop iteration. Not yet committed to the client.
    Done(ResponseVerdict),
    /// The write handler has committed headers + body and finalized the request.
    /// Any further write events for this request are ignored.
    Resumed,
}

/// Per-request context for streaming inspection.
pub struct StreamContext {
    /// Raw chunks buffered from upstream, waiting for inspection to clear them.
    pub pending_chunks: Vec<Vec<u8>>,

    /// Partial-line accumulation used only for JSON parsing.
    /// Never sent to the client.
    pub line_buffer: Vec<u8>,

    /// Decoded text content, kept per choice index (OpenAI `n > 1`). Non-choice
    /// providers (Ollama `message`/`response`) and single-choice streams use key
    /// 0. Keying by the wire choice `index` keeps the fragments of one choice
    /// contiguous even when choices interleave across streamed chunks; distinct
    /// choices are joined with '\n' only at inspection time (see
    /// [`inspection_text`](Self::inspection_text)). Accumulating into a single
    /// flat string in arrival order would let a prohibited phrase spanning one
    /// choice's cross-chunk fragments be split by another choice's fragments and
    /// evade the scan.
    pub choice_texts: std::collections::HashMap<u32, String>,

    /// Set to true once a checkpoint is blocked; no more data is forwarded.
    pub blocked: bool,

    /// Set to true when `"done":true` is seen in the LLM stream.
    pub stream_done: bool,

    /// Running total of bytes held in `pending_chunks`.
    pub total_buffered_bytes: usize,

    /// Set to `true` by the header filter when it suppresses the upstream response headers
    /// on the first pass. The body filter uses this to know it must call
    /// `call_next_header_filter(r)` before forwarding any data to the client.
    pub headers_suppressed: bool,

    /// Set to `true` by the header filter when the upstream response carries a
    /// `Content-Encoding` the module cannot inspect (anything other than
    /// `identity`). A content-encoded body is transformed on the wire, so scanning
    /// its bytes as text inspects garbage while the original compressed bytes are
    /// released to the client — a guardrail bypass. The body filter fails closed on
    /// this flag before any inspection, mirroring the request path.
    pub uninspectable_encoding: bool,

    /// State of the asynchronous end-of-stream inspection. Drives the output
    /// suspend/resume in the response-body filter.
    pub inspect_state: ResponseInspect,

    /// Configurable block message returned by the guardrails backend for this
    /// response (first failed guardrail's `flagMessage`/`message`), recorded by
    /// the async completion. `None` when the backend supplied no usable message,
    /// in which case the block emitters use their hardcoded fallback copy.
    pub block_message: Option<String>,

    /// The spawned async inspection task. Kept alive here so it is not cancelled
    /// by being dropped (dropping an `async-task` `Task` cancels it). The
    /// `StreamContext` is heap-boxed and a pool cleanup (`stream_ctx_cleanup`,
    /// registered by `alloc_stream_ctx`) runs its `Drop` at request teardown,
    /// which drops this field and cancels the task if still in flight.
    pub inspect_task: Option<ngx::async_::Task<()>>,
}

impl Default for StreamContext {
    fn default() -> Self {
        Self {
            pending_chunks: Vec::new(),
            line_buffer: Vec::with_capacity(4096),
            choice_texts: std::collections::HashMap::new(),

            blocked: false,
            stream_done: false,
            total_buffered_bytes: 0,

            headers_suppressed: false,
            uninspectable_encoding: false,

            inspect_state: ResponseInspect::Idle,
            block_message: None,
            inspect_task: None,
        }
    }
}

impl StreamContext {
    /// Append `text` to the accumulator for choice `index`. No separator is
    /// inserted here so the cross-chunk fragments of a single choice concatenate
    /// contiguously; distinct choices are separated only at inspection time
    /// (see [`inspection_text`](Self::inspection_text)).
    fn push_choice_text(&mut self, index: u32, text: &str) {
        if text.is_empty() {
            return;
        }
        self.choice_texts.entry(index).or_default().push_str(text);
    }

    /// True when no choice has accumulated any decoded text.
    pub fn has_no_decoded_text(&self) -> bool {
        self.choice_texts.values().all(String::is_empty)
    }

    /// Total decoded bytes across all choices. Used for logging only.
    pub fn decoded_len(&self) -> usize {
        self.choice_texts.values().map(String::len).sum()
    }

    /// Join the decoded text of distinct choices with '\n' to form the
    /// inspection payload, ordered by choice index for deterministic output. A
    /// single choice (the common case) yields its text with no separator, so a
    /// sentinel token cannot straddle a choice boundary and evade the scan.
    pub fn inspection_text(&self) -> String {
        let mut keys: Vec<u32> = self.choice_texts.keys().copied().collect();
        keys.sort_unstable();
        keys.iter()
            .map(|k| self.choice_texts[k].as_str())
            .filter(|s| !s.is_empty())
            .collect::<Vec<_>>()
            .join("\n")
    }

    /// Extract text content from a parsed LLM chunk into the per-choice map.
    fn extract_llm_content(&mut self, chunk: LlmChunk) {
        if chunk.done.unwrap_or(false) {
            self.stream_done = true;
        }
        if let Some(msg) = chunk.message {
            // Ollama /api/chat: text is in the top-level message.content. No
            // choice index exists, so it accumulates under key 0 (contiguous
            // across chunks, matching single-stream semantics).
            if let Some(content) = msg.content {
                self.push_choice_text(0, &content);
            }
        } else if let Some(response) = chunk.response {
            // Ollama /api/generate: text is a top-level `response` string. Without
            // this, a /api/generate response yields no decoded text and the
            // response path flushes it un-inspected. Accumulates under key 0.
            self.push_choice_text(0, &response);
        } else if let Some(choices) = chunk.choices {
            // A completion may carry multiple choices (e.g. n > 1 / best_of), and
            // every choice is forwarded to the client, so every choice must be
            // inspected — scanning only the first would let blocked content in a
            // later choice bypass the guardrail. Accumulate them all, keyed by the
            // wire `index` so a choice's fragments stay contiguous even when
            // choices interleave across streamed chunks.
            for choice in choices {
                // Missing index (single-choice responses, some providers) → 0.
                let index = choice.index.unwrap_or(0);
                // Precedence delta -> message -> text: a choice carries exactly one
                // of these in practice. `message.content` covers non-streaming
                // /v1/chat/completions, whose text would otherwise be dropped
                // (no decoded text -> un-inspected flush).
                let text = choice
                    .delta
                    .and_then(|d| d.content)
                    .or_else(|| choice.message.and_then(|m| m.content))
                    .or(choice.text);
                if let Some(text) = text {
                    self.push_choice_text(index, &text);
                }
            }
        }
    }

    /// Ingest a new network chunk.
    ///
    /// The raw bytes are added to `pending_chunks` (held from the client).
    /// The bytes are also appended to `line_buffer`.
    /// When there is a complete JSON line, it is parsed to update the per-choice
    /// decoded-text map.
    pub fn process_chunk(&mut self, data: &[u8]) {
        if data.is_empty() {
            return;
        }

        // Hold raw bytes from the client until inspection clears them.
        self.pending_chunks.push(data.to_vec());
        self.total_buffered_bytes += data.len();

        // Append to the line buffer and process any complete lines.
        self.line_buffer.extend_from_slice(data);
        self.drain_complete_lines();
    }

    /// Parse and remove all newline-terminated lines from `line_buffer`.
    ///
    /// Uses `rposition` to find the last complete line, then drains all
    /// completed bytes in a single O(n) operation and iterates over the
    /// resulting slice.  This replaces the previous per-line `drain` loop
    /// which caused one allocation + one O(n) memmove *per line*, giving
    /// O(n²) total work for long streams.
    fn drain_complete_lines(&mut self) {
        // Nothing to process if there is no complete line yet.
        let last_newline = match self.line_buffer.iter().rposition(|&b| b == b'\n') {
            Some(p) => p,
            None => return,
        };

        // Single drain: one allocation + one O(n) memmove for all lines.
        let completed: Vec<u8> = self.line_buffer.drain(..=last_newline).collect();

        for line_bytes in completed.split(|&b| b == b'\n') {
            let line = match std::str::from_utf8(line_bytes) {
                Ok(s) => s.trim(),
                Err(_) => continue,
            };

            if line.is_empty() {
                continue;
            }

            // Strip optional SSE "data: " prefix.
            let json_str = if let Some(payload) = line.strip_prefix("data:") {
                payload.trim()
            } else {
                line
            };

            if json_str.is_empty() || json_str == "[DONE]" {
                continue;
            }

            match serde_json::from_str::<LlmChunk>(json_str) {
                Ok(chunk) => self.extract_llm_content(chunk),
                // Malformed/partial JSON lines are expected mid-stream; skip
                // silently rather than logging (the line may carry model output).
                Err(_) => continue,
            }
        }
    }

    /// Returns true when the stream is finished and there is content to inspect.
    pub fn should_inspect_final(&self, last_buf: bool) -> bool {
        !self.blocked && (last_buf || self.stream_done) && !self.has_no_decoded_text()
    }

    /// Returns true when the stream is finished with a non-empty buffered body
    /// that yielded **no** decoded text — i.e. an unrecognized response schema
    /// (e.g. Anthropic, Cohere, or any provider not covered by
    /// `extract_llm_content`). In that case the raw buffered body must be
    /// inspected (via [`raw_inspection_fallback`]) rather than flushed
    /// un-inspected, which would let un-modeled provider output bypass the
    /// guardrail. A truly empty body (no `pending_chunks`) does NOT trigger this,
    /// so genuinely body-less or empty responses still take the flush path.
    ///
    /// [`raw_inspection_fallback`]: Self::raw_inspection_fallback
    pub fn should_inspect_raw_fallback(&self, last_buf: bool) -> bool {
        !self.blocked
            && (last_buf || self.stream_done)
            && self.has_no_decoded_text()
            && !self.pending_chunks.is_empty()
    }

    /// Raw buffered response body as strict UTF-8. Used only as the inspection
    /// content when the response schema yielded no decoded text
    /// (`should_inspect_raw_fallback`), so un-modeled providers are still scanned.
    ///
    /// Returns `None` when any chunk contains invalid UTF-8 so the caller can
    /// fail closed rather than scanning replacement-character text that differs
    /// from the bytes actually released to the client.
    pub fn raw_inspection_fallback(&self) -> Option<String> {
        let mut s = String::new();
        for chunk in &self.pending_chunks {
            s.push_str(std::str::from_utf8(chunk).ok()?);
        }
        Some(s)
    }

    /// Try to parse any bytes remaining in `line_buffer` as a complete JSON
    /// object.  Called at stream end to handle non-streaming responses that
    /// arrive as a single JSON blob without a trailing newline (e.g. the
    /// OpenAI `/v1/completions` non-streaming format).
    pub fn try_drain_remaining(&mut self) {
        if !self.line_buffer.is_empty() {
            self.line_buffer.push(b'\n');
            self.drain_complete_lines();
        }
    }

    /// Take all buffered chunks (clears `pending_chunks`).
    pub fn take_pending_chunks(&mut self) -> Vec<Vec<u8>> {
        let chunks = std::mem::take(&mut self.pending_chunks);
        self.total_buffered_bytes = 0;
        chunks
    }

    /// Discard all buffered chunks (stream blocked).
    pub fn clear_pending_chunks(&mut self) {
        self.pending_chunks.clear();
        self.total_buffered_bytes = 0;
    }
}

/// Default block message used when the guardrails backend supplies none.
pub const DEFAULT_SSE_BLOCK_MESSAGE: &str = "Stream terminated by guardrails policy.";
/// Default block message used when the guardrails backend supplies none.
pub const DEFAULT_NON_STREAMING_BLOCK_MESSAGE: &str = "Response blocked by guardrails policy.";

/// Compose the client-facing block message.
///
/// When the guardrails backend supplies no message (`None`, or empty after the
/// upstream trims it), the `default` policy text is returned verbatim. When a
/// backend message is present, it is appended as `"<default> Message: <m>"` so
/// the operator-configured text augments (rather than replaces) the policy
/// notice. The result is later JSON-escaped by [`output_error_json`] /
/// `request_path::request_block_body`, so backend-supplied text cannot break the JSON.
pub fn compose_block_message(default: &str, message: Option<&str>) -> String {
    match message {
        Some(m) => format!("{default} Message: {m}"),
        None => default.to_owned(),
    }
}

/// Build an OpenAI-style error JSON object with the given (already-untrusted)
/// message, using `type: "api_error"` and `code: "content_policy_violation"`.
///
/// `message` is JSON-escaped via `serde_json`, so backend-supplied text cannot
/// break out of the JSON structure.
fn output_error_json(message: &str) -> String {
    let obj = serde_json::json!({
        "error": {
            "message": message,
            "type": "api_error",
            "param": null,
            "code": "content_policy_violation",
        }
    });
    // Serializing a plain object of string/null values cannot fail.
    obj.to_string()
}

/// SSE termination event sent to the client when a streaming response is blocked.
///
/// `message` is the guardrails backend's configurable block text; when present
/// it is appended to the default as `"<default> Message: <m>"` (see
/// [`compose_block_message`]), otherwise the plain [`DEFAULT_SSE_BLOCK_MESSAGE`]
/// is used.
///
/// This is an *output-side* block: the client's request was valid, but the model's
/// generated response tripped the guardrails policy. The error `type` is therefore
/// `api_error` (a server-side failure in OpenAI's error taxonomy), NOT
/// `invalid_request_error` (which denotes a bad client request). The `code` remains
/// `content_policy_violation` to convey the policy reason. See the status/type matrix
/// documented on `send_termination` / `send_blocked_response` in `response_path.rs`.
pub fn termination_message(message: Option<&str>) -> Vec<u8> {
    let msg = compose_block_message(DEFAULT_SSE_BLOCK_MESSAGE, message);
    let mut body = Vec::new();
    body.extend_from_slice(b"data: ");
    body.extend_from_slice(output_error_json(&msg).as_bytes());
    body.extend_from_slice(b"\n\n");
    body
}

/// Plain JSON error body sent to the client when a non-streaming response is blocked.
///
/// `message` is the guardrails backend's configurable block text; when present
/// it is appended to the default as `"<default> Message: <m>"` (see
/// [`compose_block_message`]), otherwise the plain
/// [`DEFAULT_NON_STREAMING_BLOCK_MESSAGE`] is used.
///
/// Like `termination_message`, this represents an *output-side* block, so the error
/// `type` is `api_error` (server-side), not `invalid_request_error` (client request).
/// The `code` stays `content_policy_violation` to convey the policy reason.
pub fn non_streaming_error_body(message: Option<&str>) -> Vec<u8> {
    let msg = compose_block_message(DEFAULT_NON_STREAMING_BLOCK_MESSAGE, message);
    output_error_json(&msg).into_bytes()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_ollama_content_extraction() {
        let mut ctx = StreamContext::default();
        let data = b"{\"model\":\"llama3\",\"message\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"done\":false}\n";
        ctx.process_chunk(data);
        assert_eq!(ctx.inspection_text(), "Hello");
    }

    #[test]
    fn test_openai_delta_extraction() {
        let mut ctx = StreamContext::default();
        let data = b"data: {\"choices\":[{\"delta\":{\"content\":\"World\"}}]}\n";
        ctx.process_chunk(data);
        assert_eq!(ctx.inspection_text(), "World");
    }

    #[test]
    fn test_ollama_generate_response_extracted() {
        // Ollama /api/generate: text is a top-level `response` string.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"model\":\"llama3\",\"response\":\"Hello\",\"done\":true}\n");
        assert_eq!(ctx.inspection_text(), "Hello");
    }

    #[test]
    fn test_ollama_generate_response_streaming() {
        // Streaming /api/generate delivers `response` in fragments across chunks;
        // they concatenate with no separator (cross-chunk, like other streams).
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"response\":\"foo\",\"done\":false}\n");
        ctx.process_chunk(b"{\"response\":\"bar\",\"done\":true}\n");
        assert_eq!(ctx.inspection_text(), "foobar");
    }

    #[test]
    fn test_openai_nonstreaming_chat_message_content_extracted() {
        // Non-streaming /v1/chat/completions: text is in choices[].message.content.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n",
        );
        assert_eq!(ctx.inspection_text(), "hi");
    }

    #[test]
    fn test_openai_multiple_message_choices_all_extracted() {
        // n > 1 non-streaming chat: every choice's message.content must be
        // accumulated under its own index and joined with a newline for inspection.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"{\"choices\":[{\"index\":0,\"message\":{\"content\":\"a\"}},{\"index\":1,\"message\":{\"content\":\"b\"}}]}\n",
        );
        assert_eq!(ctx.inspection_text(), "a\nb");
    }

    #[test]
    fn test_nonstreaming_chat_sentinel_is_inspected() {
        // Regression: blocked content in a non-streaming chat message.content
        // must reach the decoded text so should_inspect_final fires (rather than
        // the response being flushed un-inspected).
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"choices\":[{\"message\":{\"content\":\"SENTINEL\"}}]}\n");
        assert!(
            ctx.inspection_text().contains("SENTINEL"),
            "non-streaming chat message.content must be accumulated for inspection"
        );
        assert!(
            ctx.should_inspect_final(true),
            "a non-empty extracted body at end-of-stream must trigger inspection"
        );
    }

    #[test]
    fn test_partial_line_buffered_until_newline() {
        let mut ctx = StreamContext::default();
        // Send JSON without trailing newline.
        ctx.process_chunk(b"{\"message\":{\"content\":\"hi\"},\"done\":false}");
        // Complete the line.
        ctx.process_chunk(b"\n");
        assert_eq!(ctx.inspection_text(), "hi");
    }

    #[test]
    fn test_stream_done_detection() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"message\":{\"content\":\"\"},\"done\":true}\n");
        assert!(ctx.stream_done);
    }

    #[test]
    fn test_openai_completions_text_extraction() {
        // Non-streaming completions use choices[].text rather than a delta.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"data: {\"choices\":[{\"text\":\"answer\"}]}\n");
        assert_eq!(ctx.inspection_text(), "answer");
    }

    #[test]
    fn test_openai_multiple_choice_deltas_all_extracted() {
        // n > 1 chat completion: every choice's delta content must be
        // accumulated under its own index (not just the first) and joined with a
        // newline for inspection.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"foo\"}},{\"index\":1,\"delta\":{\"content\":\"bar\"}}]}\n",
        );
        assert_eq!(ctx.inspection_text(), "foo\nbar");
    }

    #[test]
    fn test_openai_multiple_completion_texts_all_extracted() {
        // n > 1 non-streaming completion: every choice's text must be accumulated
        // under its own index and joined with a newline for inspection.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"data: {\"choices\":[{\"index\":0,\"text\":\"a\"},{\"index\":1,\"text\":\"b\"}]}\n",
        );
        assert_eq!(ctx.inspection_text(), "a\nb");
    }

    #[test]
    fn test_later_choice_content_is_inspected() {
        // Regression: blocked content in a non-first choice must reach the decoded
        // text so it is scanned before the response is released.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"clean\"}},{\"index\":1,\"delta\":{\"content\":\"SENTINEL\"}}]}\n",
        );
        assert!(
            ctx.inspection_text().contains("SENTINEL"),
            "content in a later choice must be accumulated for inspection"
        );
    }

    #[test]
    fn test_interleaved_choices_across_chunks_are_not_merged() {
        // The core regression: two choices streamed with their fragments
        // interleaved across chunks. Choice 1's fragments ("Safe " then
        // "SENTINEL") must stay contiguous so the sentinel token is intact for
        // inspection, despite choice 0's fragment arriving between them. With the
        // old flat arrival-order accumulation the text became
        // "How Safe to XSENTINEL" (roughly), splitting "Safe SENTINEL".
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"How \"}},{\"index\":1,\"delta\":{\"content\":\"Safe \"}}]}\n",
        );
        ctx.process_chunk(
            b"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"to X\"}},{\"index\":1,\"delta\":{\"content\":\"SENTINEL\"}}]}\n",
        );

        // Choice 0 = "How to X", choice 1 = "Safe SENTINEL", joined by '\n'.
        assert_eq!(ctx.inspection_text(), "How to X\nSafe SENTINEL");
        assert!(
            ctx.inspection_text().contains("Safe SENTINEL"),
            "a choice's cross-chunk fragments must remain contiguous for inspection"
        );
    }

    #[test]
    fn test_openai_choice_without_index_defaults_to_bucket_zero() {
        // A choice with no wire `index` (single-choice responses, some providers)
        // accumulates under key 0, so its cross-chunk fragments stay contiguous
        // with no separator.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"data: {\"choices\":[{\"delta\":{\"content\":\"foo\"}}]}\n");
        ctx.process_chunk(b"data: {\"choices\":[{\"delta\":{\"content\":\"bar\"}}]}\n");
        assert_eq!(ctx.inspection_text(), "foobar");
    }

    #[test]
    fn test_sse_done_marker_ignored() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n");
        ctx.process_chunk(b"data: [DONE]\n");
        assert_eq!(ctx.inspection_text(), "hi");
    }

    #[test]
    fn test_multiple_lines_in_single_chunk() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(
            b"data: {\"choices\":[{\"delta\":{\"content\":\"foo\"}}]}\n\
              data: {\"choices\":[{\"delta\":{\"content\":\"bar\"}}]}\n",
        );
        assert_eq!(ctx.inspection_text(), "foobar");
    }

    #[test]
    fn test_malformed_json_is_skipped() {
        let mut ctx = StreamContext::default();
        // A malformed line must not panic and must not block later valid lines.
        ctx.process_chunk(b"data: {not valid json}\n");
        ctx.process_chunk(b"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n");
        assert_eq!(ctx.inspection_text(), "ok");
    }

    #[test]
    fn test_try_drain_remaining_no_trailing_newline() {
        // Non-streaming /v1/completions arrives as a single blob without a
        // trailing newline; try_drain_remaining must still parse it.
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"choices\":[{\"text\":\"blob\"}]}");
        assert_eq!(ctx.inspection_text(), "", "not parsed before drain");
        ctx.try_drain_remaining();
        assert_eq!(ctx.inspection_text(), "blob");
    }

    #[test]
    fn test_try_drain_remaining_empty_is_noop() {
        let mut ctx = StreamContext::default();
        ctx.try_drain_remaining();
        assert_eq!(ctx.inspection_text(), "");
    }

    /// Build a `choice_texts` map for tests: empty `text` yields an empty map
    /// (so `has_no_decoded_text()` is true), otherwise a single bucket at index 0.
    fn choice_texts_for(text: &str) -> std::collections::HashMap<u32, String> {
        let mut m = std::collections::HashMap::new();
        if !text.is_empty() {
            m.insert(0, text.to_string());
        }
        m
    }

    #[test]
    fn test_should_inspect_final() {
        // (blocked, last_buf, stream_done, text, expected)
        let cases = [
            (false, true, false, "x", true),   // last_buf with content
            (false, false, true, "x", true),   // stream_done with content
            (false, false, false, "x", false), // not final yet
            (false, true, false, "", false),   // final but no content
            (true, true, true, "x", false),    // blocked short-circuits
        ];
        for (blocked, last_buf, stream_done, text, expected) in cases {
            let ctx = StreamContext {
                blocked,
                stream_done,
                choice_texts: choice_texts_for(text),
                ..StreamContext::default()
            };
            assert_eq!(
                ctx.should_inspect_final(last_buf),
                expected,
                "blocked={blocked} last_buf={last_buf} stream_done={stream_done} text={text:?}"
            );
        }
    }

    #[test]
    fn test_non_llm_body_buffers_but_yields_no_text() {
        // A `/v1/models` response is valid JSON but has no LLM content fields
        // (message/choices/done), so it buffers bytes yet accumulates no
        // decoded text. `should_inspect_final` is therefore false. Because the
        // buffer is non-empty, the raw-body fallback (`should_inspect_raw_fallback`)
        // fires instead so the body is inspected rather than flushed un-inspected.
        let mut ctx = StreamContext::default();
        let body = br#"{"object":"list","data":[{"id":"Qwen/Qwen3-32B","object":"model"}]}"#;
        ctx.process_chunk(body);
        ctx.try_drain_remaining();

        assert!(
            ctx.total_buffered_bytes > 0,
            "raw bytes must be buffered ({} expected > 0)",
            ctx.total_buffered_bytes
        );
        assert_eq!(ctx.inspection_text(), "", "no LLM text should be extracted");
        assert!(
            !ctx.should_inspect_final(true),
            "no decoded text to inspect at end-of-stream"
        );
        assert!(
            ctx.should_inspect_raw_fallback(true),
            "non-empty undecoded body must take the raw-inspection fallback"
        );
        // The buffered chunk is still present and available (to inspect / flush).
        let chunks = ctx.take_pending_chunks();
        assert_eq!(chunks.len(), 1, "buffered body must be available");
    }

    #[test]
    fn test_anthropic_shaped_body_yields_no_decoded_text() {
        // An Anthropic-style response is well-formed JSON but its schema is not
        // covered by extract_llm_content (text lives in content[].text, not
        // message/choices/response). It must decode to no text so the raw-body
        // fallback path engages instead of a bare flush.
        let mut ctx = StreamContext::default();
        let body =
            br#"{"type":"message","role":"assistant","content":[{"type":"text","text":"secret"}]}"#;
        ctx.process_chunk(body);
        ctx.try_drain_remaining();

        assert_eq!(
            ctx.inspection_text(),
            "",
            "Anthropic schema is undecoded (documents the extraction gap)"
        );
        assert!(
            ctx.should_inspect_raw_fallback(true),
            "undecoded Anthropic body must take the raw-inspection fallback"
        );
        // The raw fallback content carries the model output so it can be scanned.
        assert!(
            ctx.raw_inspection_fallback()
                .expect("valid UTF-8 body must return Some")
                .contains("secret"),
            "raw fallback must include the undecoded model output"
        );
    }

    #[test]
    fn test_should_inspect_raw_fallback() {
        // (blocked, last_buf, stream_done, text, has_buffer, expected)
        let cases = [
            (false, true, false, "", true, true), // final, no text, buffered => fallback
            (false, false, true, "", true, true), // stream_done variant
            (false, false, false, "", true, false), // not final yet
            (false, true, false, "x", true, false), // decoded text => primary path, not fallback
            (false, true, false, "", false, false), // truly empty body => flush, not fallback
            (true, true, true, "", true, false),  // blocked short-circuits
        ];
        for (blocked, last_buf, stream_done, text, has_buffer, expected) in cases {
            let mut ctx = StreamContext {
                blocked,
                stream_done,
                choice_texts: choice_texts_for(text),
                ..StreamContext::default()
            };
            if has_buffer {
                ctx.pending_chunks.push(b"{}".to_vec());
            }
            assert_eq!(
                ctx.should_inspect_raw_fallback(last_buf),
                expected,
                "blocked={blocked} last_buf={last_buf} stream_done={stream_done} \
                 text={text:?} has_buffer={has_buffer}"
            );
        }
    }

    #[test]
    fn test_raw_inspection_fallback_joins_chunks() {
        let mut ctx = StreamContext::default();
        assert_eq!(
            ctx.raw_inspection_fallback(),
            Some(String::new()),
            "no chunks => empty fallback"
        );
        ctx.pending_chunks.push(b"foo".to_vec());
        ctx.pending_chunks.push(b"bar".to_vec());
        assert_eq!(
            ctx.raw_inspection_fallback(),
            Some("foobar".to_string()),
            "fallback concatenates all buffered chunks"
        );
    }

    #[test]
    fn test_raw_inspection_fallback_rejects_invalid_utf8() {
        let mut ctx = StreamContext::default();
        // 0xFF is never valid in UTF-8.
        ctx.pending_chunks.push(b"valid".to_vec());
        ctx.pending_chunks.push(vec![0xFF, 0xFE]);
        assert_eq!(
            ctx.raw_inspection_fallback(),
            None,
            "invalid UTF-8 must return None (fail closed)"
        );
    }

    #[test]
    fn test_take_pending_chunks_resets_state() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"message\":{\"content\":\"a\"},\"done\":false}\n");
        assert!(ctx.total_buffered_bytes > 0);
        let chunks = ctx.take_pending_chunks();
        assert_eq!(chunks.len(), 1);
        assert!(ctx.pending_chunks.is_empty());
        assert_eq!(ctx.total_buffered_bytes, 0);
    }

    #[test]
    fn test_clear_pending_chunks_resets_state() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"message\":{\"content\":\"a\"},\"done\":false}\n");
        assert!(ctx.total_buffered_bytes > 0);
        ctx.clear_pending_chunks();
        assert!(ctx.pending_chunks.is_empty());
        assert_eq!(ctx.total_buffered_bytes, 0);
    }

    /// Strip the optional SSE `data:` prefix/trailing blank line and parse the
    /// remaining bytes as an OpenAI error JSON object.
    fn parse_error_body(body: &[u8]) -> serde_json::Value {
        let text = std::str::from_utf8(body).expect("utf8");
        let json_str = text
            .trim()
            .strip_prefix("data:")
            .map(str::trim)
            .unwrap_or(text.trim());
        serde_json::from_str(json_str).expect("error body must be valid JSON")
    }

    #[test]
    fn test_error_bodies_are_valid_json_with_policy_code() {
        // Fallback (None) path: uses the default block copy.
        for body in [termination_message(None), non_streaming_error_body(None)] {
            let value = parse_error_body(&body);
            assert_eq!(
                value["error"]["code"], "content_policy_violation",
                "unexpected code in {value}"
            );
            // Output-side (response/stream) blocks are server-side failures, so the
            // OpenAI error `type` must be `api_error`, NOT `invalid_request_error`
            // (which is reserved for the request-side block in `send_403_and_finalize`).
            assert_eq!(
                value["error"]["type"], "api_error",
                "response-side error body must use api_error, got {value}"
            );
        }
        assert_eq!(
            parse_error_body(&termination_message(None))["error"]["message"],
            DEFAULT_SSE_BLOCK_MESSAGE
        );
        assert_eq!(
            parse_error_body(&non_streaming_error_body(None))["error"]["message"],
            DEFAULT_NON_STREAMING_BLOCK_MESSAGE
        );
    }

    #[test]
    fn test_compose_block_message() {
        // None -> default verbatim (no "Message:" suffix).
        assert_eq!(compose_block_message("Blocked.", None), "Blocked.");
        // Some -> default with the backend message appended.
        assert_eq!(
            compose_block_message("Blocked.", Some("PII detected")),
            "Blocked. Message: PII detected"
        );
    }

    #[test]
    fn test_error_bodies_carry_backend_message() {
        // Backend-supplied message is APPENDED to the default as
        // "<default> Message: <m>", type stays api_error.
        let msg = "Prompt contains PII (email).";
        let expected = [
            (
                termination_message(Some(msg)),
                format!("{DEFAULT_SSE_BLOCK_MESSAGE} Message: {msg}"),
            ),
            (
                non_streaming_error_body(Some(msg)),
                format!("{DEFAULT_NON_STREAMING_BLOCK_MESSAGE} Message: {msg}"),
            ),
        ];
        for (body, want) in expected {
            let value = parse_error_body(&body);
            assert_eq!(value["error"]["message"], want);
            assert_eq!(value["error"]["type"], "api_error");
            assert_eq!(value["error"]["code"], "content_policy_violation");
        }
    }

    #[test]
    fn test_error_bodies_omit_message_suffix_when_none() {
        // No backend message -> plain default, no "Message:" segment.
        let value = parse_error_body(&non_streaming_error_body(None));
        assert_eq!(
            value["error"]["message"],
            DEFAULT_NON_STREAMING_BLOCK_MESSAGE
        );
        assert!(
            !value["error"]["message"]
                .as_str()
                .unwrap()
                .contains("Message:"),
            "default body must not contain a Message: suffix"
        );
    }

    #[test]
    fn test_error_body_escapes_untrusted_message() {
        // A message containing JSON metacharacters must not break the envelope,
        // even after being appended to the default.
        let msg = r#"bad "quote" and \backslash and }brace"#;
        let value = parse_error_body(&non_streaming_error_body(Some(msg)));
        assert_eq!(
            value["error"]["message"],
            format!("{DEFAULT_NON_STREAMING_BLOCK_MESSAGE} Message: {msg}")
        );
        assert_eq!(value["error"]["type"], "api_error");
    }

    #[test]
    fn test_sse_termination_has_data_prefix_and_blank_line() {
        let body = termination_message(Some("blocked"));
        let text = std::str::from_utf8(&body).unwrap();
        assert!(text.starts_with("data: "));
        assert!(text.ends_with("\n\n"));
    }

    #[test]
    fn test_request_block_body_uses_invalid_request_error() {
        // Mirror of the request-blocked fallback body produced by
        // `request_path::request_block_body(None)`. The request-side block DOES represent a bad
        // client request, so it must keep `type: invalid_request_error` (distinct from the
        // output-side `api_error`). Keep this literal in sync with `request_block_body`'s
        // envelope + `DEFAULT_REQUEST_BLOCK_MESSAGE`.
        let request_block_body = br#"{"error":{"message":"Request blocked by guardrails policy.","type":"invalid_request_error","param":null,"code":"content_policy_violation"}}"#;
        let value: serde_json::Value = serde_json::from_slice(request_block_body)
            .expect("request block body must be valid JSON");
        assert_eq!(value["error"]["type"], "invalid_request_error");
        assert_eq!(value["error"]["code"], "content_policy_violation");
    }
}
