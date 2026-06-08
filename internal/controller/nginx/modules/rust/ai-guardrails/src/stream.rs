//! Streaming inspection logic with checkpoint buffering and content extraction.

use crate::client::{inspect_content, GuardrailsError};
use crate::config::ModuleConfig;
use serde::Deserialize;

/// Unified LLM streaming chunk covering OpenAI and Ollama wire formats.
/// Using typed structs avoids allocating a full dynamic map (`serde_json::Value`)
/// for every SSE chunk received from upstream.
#[derive(Deserialize)]
struct LlmChunk {
    /// Ollama stream-completion flag.
    done: Option<bool>,
    /// Ollama message payload.
    message: Option<OllamaMessage>,
    /// OpenAI choices array (streaming chat and non-streaming completions).
    choices: Option<Vec<OpenAIChoice>>,
}

#[derive(Deserialize)]
struct OllamaMessage {
    content: Option<String>,
}

#[derive(Deserialize)]
struct OpenAIChoice {
    /// Streaming chat completions delta.
    delta: Option<OpenAIDelta>,
    /// Non-streaming completions text.
    text: Option<String>,
}

#[derive(Deserialize)]
struct OpenAIDelta {
    content: Option<String>,
}

/// Extract text content from a parsed LLM chunk into `accumulated_text`.
fn extract_llm_content(chunk: LlmChunk, accumulated: &mut String, stream_done: &mut bool) {
    if chunk.done.unwrap_or(false) {
        *stream_done = true;
    }
    if let Some(msg) = chunk.message {
        if let Some(content) = msg.content {
            accumulated.push_str(&content);
        }
    } else if let Some(choices) = chunk.choices {
        if let Some(first) = choices.into_iter().next() {
            if let Some(delta) = first.delta {
                if let Some(content) = delta.content {
                    accumulated.push_str(&content);
                }
            } else if let Some(text) = first.text {
                accumulated.push_str(&text);
            }
        }
    }
}

/// Per-request context for streaming inspection.
pub struct StreamContext {
    /// Raw chunks buffered from upstream, waiting for inspection to clear them.
    pub pending_chunks: Vec<Vec<u8>>,

    /// Partial-line accumulation used only for JSON parsing.
    /// Never sent to the client.
    pub line_buffer: Vec<u8>,

    /// Extracted text content from all parsed JSON objects.
    pub accumulated_text: String,

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
}

impl Default for StreamContext {
    fn default() -> Self {
        Self {
            pending_chunks: Vec::new(),
            line_buffer: Vec::with_capacity(4096),
            accumulated_text: String::with_capacity(4096),

            blocked: false,
            stream_done: false,
            total_buffered_bytes: 0,

            headers_suppressed: false,
        }
    }
}

impl StreamContext {
    /// Ingest a new network chunk.
    ///
    /// The raw bytes are added to `pending_chunks` (held from the client).
    /// The bytes are also appended to `line_buffer`.
    /// When there is a complete JSON line, it is parsed to update `accumulated_text`.
    pub fn process_chunk(&mut self, data: &[u8]) {
        if data.is_empty() {
            return;
        }

        // Hold raw bytes from the client until inspection clears them.
        self.pending_chunks.push(data.to_vec());
        self.total_buffered_bytes += data.len();

        if let Ok(text) = std::str::from_utf8(data) {
            eprintln!(
                "[guardrails] Received {} bytes: {}",
                data.len(),
                if text.len() > 200 {
                    format!("{}...", &text[..200])
                } else {
                    text.to_string()
                }
            );
        }

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
                Ok(chunk) => extract_llm_content(chunk, &mut self.accumulated_text, &mut self.stream_done),
                Err(e) => eprintln!(
                    "[guardrails] Failed to parse JSON line: {} — line: {}",
                    e, json_str
                ),
            }
        }
    }

    /// Returns true when the stream is finished and there is content to inspect.
    pub fn should_inspect_final(&self, last_buf: bool) -> bool {
        !self.blocked && (last_buf || self.stream_done) && !self.accumulated_text.is_empty()
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
        eprintln!(
            "[guardrails] Releasing {} pending chunks to client",
            chunks.len()
        );
        chunks
    }

    /// Discard all buffered chunks (stream blocked).
    pub fn clear_pending_chunks(&mut self) {
        let n = self.pending_chunks.len();
        self.pending_chunks.clear();
        self.total_buffered_bytes = 0;
        eprintln!("[guardrails] Discarded {} pending chunks (blocked)", n);
    }
}

/// Synchronously inspect the new content at a checkpoint.
///
/// Returns `Ok(true)` when cleared, `Ok(false)` when the stream should be
/// terminated.
pub fn inspect_checkpoint(
    ctx: &mut StreamContext,
    conf: &ModuleConfig,
) -> Result<bool, GuardrailsError> {
    let api_url = conf
        .api_url
        .as_ref()
        .ok_or_else(|| GuardrailsError::InvalidResponse("No API URL configured".to_string()))?;

    let api_token = conf.api_token.as_deref();
    let content = ctx.accumulated_text.as_str();

    eprintln!(
        "[guardrails] Inspecting full stream: {} chars",
        content.len()
    );

    if content.is_empty() {
        eprintln!("[guardrails] No content to inspect, skipping");
        return Ok(true);
    }

    let cleared = inspect_content(content, api_url, api_token, conf.timeout_ms)?;

    if cleared {
        eprintln!("[guardrails] Full-stream inspection CLEARED");
        Ok(true)
    } else {
        eprintln!("[guardrails] Full-stream inspection BLOCKED — terminating stream");
        ctx.blocked = true;
        Ok(false)
    }
}

/// SSE termination event sent to the client when a streaming response is blocked.
pub fn termination_message() -> &'static [u8] {
    b"data: {\"error\":{\"message\":\"Stream terminated by guardrails policy.\",\"type\":\"invalid_request_error\",\"param\":null,\"code\":\"content_policy_violation\"}}\n\n"
}

/// Plain JSON error body sent to the client when a non-streaming response is blocked.
pub fn non_streaming_error_body() -> &'static [u8] {
    b"{\"error\":{\"message\":\"Response blocked by guardrails policy.\",\"type\":\"invalid_request_error\",\"param\":null,\"code\":\"content_policy_violation\"}}"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_ollama_content_extraction() {
        let mut ctx = StreamContext::default();
        let data = b"{\"model\":\"llama3\",\"message\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"done\":false}\n";
        ctx.process_chunk(data);
        assert_eq!(ctx.accumulated_text, "Hello");
    }

    #[test]
    fn test_openai_delta_extraction() {
        let mut ctx = StreamContext::default();
        let data = b"data: {\"choices\":[{\"delta\":{\"content\":\"World\"}}]}\n";
        ctx.process_chunk(data);
        assert_eq!(ctx.accumulated_text, "World");
    }

    #[test]
    fn test_partial_line_buffered_until_newline() {
        let mut ctx = StreamContext::default();
        // Send JSON without trailing newline.
        ctx.process_chunk(b"{\"message\":{\"content\":\"hi\"},\"done\":false}");
        // Complete the line.
        ctx.process_chunk(b"\n");
        assert_eq!(ctx.accumulated_text, "hi");
    }

    #[test]
    fn test_stream_done_detection() {
        let mut ctx = StreamContext::default();
        ctx.process_chunk(b"{\"message\":{\"content\":\"\"},\"done\":true}\n");
        assert!(ctx.stream_done);
    }
}
