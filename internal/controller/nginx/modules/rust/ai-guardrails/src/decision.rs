//! Pure, side-effect-free decision logic shared by the request and response
//! inspection paths.
//!
//! The FFI handlers in `request_path.rs` / `response_path.rs` are almost
//! impossible to unit-test directly: every branch is tangled with raw
//! `*mut ngx_http_request_t` pointer work, the NGINX filter chain, and the async
//! scheduler. This module lifts the *decisions* those handlers make — "given
//! these plain values, what should happen next?" — into small functions over
//! ordinary Rust types so they can be exhaustively unit-tested.
//!
//! Nothing here touches NGINX, allocates buffers, or spawns tasks. Each function
//! is a total mapping from inputs to a typed action; the handlers translate that
//! action back into FFI side effects. Behaviour is intentionally identical to the
//! inline branches these replaced.

use ngx::ffi::ngx_uint_t;

use crate::subrequest_client::Verdict;

// ---------------------------------------------------------------------------
// 1. Inspection outcome -> allow/block decision (shared by both paths)
// ---------------------------------------------------------------------------

/// The distilled result of one asynchronous inspection: allow or block, plus the
/// backend's optional block message.
///
/// Both paths derive their own path-specific verdict enum from this:
/// `request_path` maps it to `InspectVerdict::{Allow,Block}`, `response_path` to
/// `ResponseVerdict::{Allow,Block}`. Centralising the mapping guarantees the two
/// paths treat a cleared / flagged / errored inspection identically.
#[derive(Clone, PartialEq, Eq, Debug)]
pub(crate) struct InspectionDecision {
    /// `true` when the content cleared and may be released.
    pub allow: bool,
    /// Backend block message. Always `None` on allow; on block it is whatever the
    /// backend supplied (or `None`, meaning "use the caller's hardcoded copy").
    pub message: Option<String>,
}

impl InspectionDecision {
    /// Allow with no message.
    pub(crate) fn allow() -> Self {
        Self {
            allow: true,
            message: None,
        }
    }

    /// Block, carrying the (optional) backend message.
    pub(crate) fn block(message: Option<String>) -> Self {
        Self {
            allow: false,
            message,
        }
    }
}

/// Map the outcome of `inspect_content_async` to an [`InspectionDecision`].
///
/// This is the single source of truth for the fail-closed policy shared by both
/// paths:
///
///  * `Some(v)` with `v.cleared` -> **allow**, message dropped (an allow never
///    carries a block message).
///  * `Some(v)` with `!v.cleared` -> **block**, keeping `v.message`.
///  * `None` (the inspection itself errored) -> **block** with no message
///    (fail-closed; the caller logs the underlying error separately).
///
/// The caller passes `None` for the error case rather than the `GuardrailsError`
/// itself so this stays a pure, `Eq`-comparable mapping independent of the error
/// type (which is not `PartialEq`).
pub(crate) fn verdict_from_inspection(outcome: Option<Verdict>) -> InspectionDecision {
    match outcome {
        Some(v) if v.cleared => InspectionDecision::allow(),
        Some(v) => InspectionDecision::block(v.message),
        None => InspectionDecision::block(None),
    }
}

// ---------------------------------------------------------------------------
// 2. Request access-phase handler state machine
// ---------------------------------------------------------------------------

/// The verdict slot of the per-request inspection state, mirrored here as a
/// plain enum so the access-handler decision can be tested without the FFI
/// `RequestInspectState`.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum RequestVerdict {
    Pending,
    Allow,
    Block,
}

/// What the ACCESS-phase handler should do on the current invocation, once the
/// preliminary guards (main request, inspection enabled, internal_uri present,
/// state allocated) have passed.
///
/// This captures the verdict/`started` state machine that decides between
/// granting access, blocking, waiting, or kicking off the body read + spawn.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum AccessAction {
    /// Verdict is `Allow`: clear the ctx slot and grant access (`NGX_OK`).
    GrantAccess,
    /// Verdict is `Block`: send the 403 and finalise.
    Block,
    /// Inspection already started and still pending: yield (`NGX_AGAIN`).
    Wait,
    /// First real invocation: mark started, read the body, and spawn the task.
    StartInspection,
}

/// Decide the access-handler action from the current verdict and whether the
/// async inspection has already been kicked off.
///
/// Precedence mirrors `guardrails_access_handler` exactly: a resolved verdict
/// (`Allow`/`Block`) always wins over the `started` flag; only while still
/// `Pending` does `started` distinguish "wait for the in-flight task" from
/// "start a new one".
pub(crate) fn decide_access_action(verdict: RequestVerdict, started: bool) -> AccessAction {
    match verdict {
        RequestVerdict::Allow => AccessAction::GrantAccess,
        RequestVerdict::Block => AccessAction::Block,
        RequestVerdict::Pending => {
            if started {
                AccessAction::Wait
            } else {
                AccessAction::StartInspection
            }
        }
    }
}

// ---------------------------------------------------------------------------
// 3. Response body-filter end-of-chain decision
// ---------------------------------------------------------------------------

/// Inputs to the response body-filter decision, gathered *after* the current
/// upstream chain has been ingested into the `StreamContext`.
///
/// All fields are plain snapshots of `StreamContext` flags / the parsed chain, so
/// the decision can be exercised without a live request. Field meanings match the
/// identically named `StreamContext` members.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) struct ResponseFilterInputs {
    /// `ctx.inspect_state == ResponseInspect::Pending` on entry.
    pub inspection_pending: bool,
    /// `ctx.uninspectable_encoding` — the header filter flagged an unsupported
    /// `Content-Encoding`, so the (compressed) body cannot be inspected in memory.
    pub uninspectable_encoding: bool,
    /// `ctx.blocked` on entry.
    pub already_blocked: bool,
    /// `ctx.total_buffered_bytes > MAX_RESPONSE_BYTES` after ingest.
    pub over_limit: bool,
    /// `ctx.should_inspect_final(last_buf)` — the checkpoint predicate.
    pub should_inspect: bool,
    /// A terminal buffer (`last_buf`/`last_in_chain`) was seen in this chain.
    pub last_buf: bool,
    /// `ctx.stream_done` — an explicit `"done":true` was parsed.
    pub stream_done: bool,
}

/// The action the response body filter should take for the current chain.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum ResponseAction {
    /// An async inspection is already in flight: buffer late arrivals and return
    /// `NGX_OK` without forwarding (the resume handler drives output).
    HoldForPending,
    /// The stream was already blocked on a prior chain: emit the block/termination
    /// body now.
    EmitBlock,
    /// The header filter flagged an unsupported `Content-Encoding`: the body is
    /// compressed and cannot be inspected in memory, so fail closed and emit the
    /// block body without ingesting the (compressed) chain.
    BlockUninspectable,
    /// The buffer-size ceiling was exceeded: block the stream, then emit the
    /// block/termination body.
    BlockOverLimit,
    /// The checkpoint predicate fired: suspend output and spawn the async
    /// inspection.
    SpawnInspection,
    /// End of stream with nothing to inspect: commit the suppressed headers and
    /// flush the buffered body.
    FlushBuffered,
    /// Mid-stream and below the inspection threshold: keep buffering, forward
    /// nothing.
    KeepBuffering,
}

/// Decide the response body-filter action from a snapshot of the context.
///
/// The precedence is byte-for-byte the order of the early returns in
/// `guardrails_response_body_filter`:
///
///   1. inspection pending -> hold,
///   2. already blocked    -> emit block,
///   3. uninspectable encoding -> block (fail-closed, pre-ingest),
///   4. over the byte limit -> block + emit,
///   5. checkpoint fired   -> spawn inspection,
///   6. otherwise: at end-of-stream (`last_buf || stream_done`) flush the
///      buffered body, else keep buffering.
///
/// Keeping this ordering identical is what makes the extraction behaviour-
/// preserving; the guard tests below pin each branch.
pub(crate) fn decide_response_action(inputs: ResponseFilterInputs) -> ResponseAction {
    if inputs.inspection_pending {
        return ResponseAction::HoldForPending;
    }
    if inputs.already_blocked {
        return ResponseAction::EmitBlock;
    }
    if inputs.uninspectable_encoding {
        return ResponseAction::BlockUninspectable;
    }
    if inputs.over_limit {
        return ResponseAction::BlockOverLimit;
    }
    if inputs.should_inspect {
        return ResponseAction::SpawnInspection;
    }
    if inputs.last_buf || inputs.stream_done {
        ResponseAction::FlushBuffered
    } else {
        ResponseAction::KeepBuffering
    }
}

// ---------------------------------------------------------------------------
// 4. Block-commit selector (response path)
// ---------------------------------------------------------------------------

/// Which body-commit helper the response path should use for a block.
///
/// This single predicate — "were the upstream headers suppressed?" — is repeated
/// at every response block site (over-limit, missing internal_uri, async
/// `Block`). Centralising it keeps the SSE-vs-buffered distinction in one place.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum BlockCommitKind {
    /// Headers were suppressed (non-SSE): commit a fresh 403 + JSON error body via
    /// `send_blocked_response`.
    BlockedResponse,
    /// Headers already flushed (SSE 200): inject an SSE termination frame via
    /// `send_termination`.
    Termination,
}

/// Choose the block-commit helper based on whether headers were suppressed.
///
/// `headers_suppressed == true` means the non-SSE path where we can still write a
/// 403; `false` means SSE, whose 200 headers already reached the client, so the
/// block must be signalled inside an SSE frame.
pub(crate) fn block_commit_kind(headers_suppressed: bool) -> BlockCommitKind {
    if headers_suppressed {
        BlockCommitKind::BlockedResponse
    } else {
        BlockCommitKind::Termination
    }
}

// ---------------------------------------------------------------------------
// 5. Response status gate (shared by the header and body filters)
// ---------------------------------------------------------------------------

/// Whether a response with this HTTP status should be buffered and inspected.
///
/// Guardrails only inspects responses that carry an inspectable LLM body, i.e.
/// genuine `2xx` payloads. Everything else is passed straight through:
///   * `1xx` — informational/interim, no final body.
///   * `204 No Content` / `304 Not Modified` — defined to have no body, so
///     buffering would suppress headers that never get committed (the body
///     filter's flush is driven by a `last_buf` body buffer that never arrives),
///     hanging the client.
///   * `3xx` — redirects / flow-control responses, no inspectable content.
///   * `>= 400` — error responses (including our own injected 403) must reach the
///     client unmodified.
///
/// Returns `true` only for `200..=299` excluding `204`.
///
/// The header filter and the body filter MUST agree on this predicate: if one
/// suppresses/buffers while the other passes through, the suppressed headers are
/// never committed. Centralising the decision here keeps them in lockstep.
pub(crate) fn should_inspect_status(status: ngx_uint_t) -> bool {
    (200..300).contains(&status) && status != 204
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- verdict_from_inspection ------------------------------------------

    #[test]
    fn cleared_verdict_allows_and_drops_message() {
        // A cleared outcome allows; any message the backend attached is dropped
        // (an allow never carries a block message).
        let decision = verdict_from_inspection(Some(Verdict {
            cleared: true,
            message: Some("ignored on allow".to_string()),
        }));
        assert_eq!(decision, InspectionDecision::allow());
        assert!(decision.allow);
        assert_eq!(decision.message, None);
    }

    #[test]
    fn flagged_verdict_blocks_and_keeps_message() {
        let decision = verdict_from_inspection(Some(Verdict {
            cleared: false,
            message: Some("blocked by IP guardrail".to_string()),
        }));
        assert_eq!(
            decision,
            InspectionDecision::block(Some("blocked by IP guardrail".to_string()))
        );
    }

    #[test]
    fn flagged_verdict_without_message_blocks_with_none() {
        let decision = verdict_from_inspection(Some(Verdict {
            cleared: false,
            message: None,
        }));
        assert_eq!(decision, InspectionDecision::block(None));
    }

    #[test]
    fn errored_inspection_fails_closed_to_block() {
        // None models the Err(GuardrailsError) case: fail closed with no message.
        let decision = verdict_from_inspection(None);
        assert!(!decision.allow);
        assert_eq!(decision.message, None);
    }

    // --- decide_access_action ---------------------------------------------

    #[test]
    fn access_allow_grants_regardless_of_started() {
        // A resolved Allow verdict wins over the started flag in both states.
        assert_eq!(
            decide_access_action(RequestVerdict::Allow, false),
            AccessAction::GrantAccess
        );
        assert_eq!(
            decide_access_action(RequestVerdict::Allow, true),
            AccessAction::GrantAccess
        );
    }

    #[test]
    fn access_block_blocks_regardless_of_started() {
        assert_eq!(
            decide_access_action(RequestVerdict::Block, false),
            AccessAction::Block
        );
        assert_eq!(
            decide_access_action(RequestVerdict::Block, true),
            AccessAction::Block
        );
    }

    #[test]
    fn access_pending_not_started_starts_inspection() {
        assert_eq!(
            decide_access_action(RequestVerdict::Pending, false),
            AccessAction::StartInspection
        );
    }

    #[test]
    fn access_pending_started_waits() {
        // The re-entrancy guard: once started, a pending verdict yields NGX_AGAIN
        // rather than kicking off a second body read + spawn.
        assert_eq!(
            decide_access_action(RequestVerdict::Pending, true),
            AccessAction::Wait
        );
    }

    // --- decide_response_action -------------------------------------------

    /// Base inputs: mid-stream, nothing special. Individual tests flip one flag
    /// to pin that branch's precedence.
    fn base_inputs() -> ResponseFilterInputs {
        ResponseFilterInputs {
            inspection_pending: false,
            uninspectable_encoding: false,
            already_blocked: false,
            over_limit: false,
            should_inspect: false,
            last_buf: false,
            stream_done: false,
        }
    }

    #[test]
    fn response_pending_holds_over_everything() {
        // inspection_pending has the highest precedence — even with every other
        // flag set it still holds.
        let inputs = ResponseFilterInputs {
            inspection_pending: true,
            uninspectable_encoding: true,
            already_blocked: true,
            over_limit: true,
            should_inspect: true,
            last_buf: true,
            stream_done: true,
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::HoldForPending
        );
    }

    #[test]
    fn response_already_blocked_emits_block_before_encoding_limit_and_inspect() {
        let inputs = ResponseFilterInputs {
            already_blocked: true,
            uninspectable_encoding: true,
            over_limit: true,
            should_inspect: true,
            ..base_inputs()
        };
        assert_eq!(decide_response_action(inputs), ResponseAction::EmitBlock);
    }

    #[test]
    fn response_uninspectable_encoding_blocks_before_limit_and_inspect() {
        // An unsupported Content-Encoding fails closed and takes precedence over
        // the byte-limit and checkpoint branches (but not over pending/blocked,
        // pinned above). This is the compressed-body-cannot-be-inspected case.
        let inputs = ResponseFilterInputs {
            uninspectable_encoding: true,
            over_limit: true,
            should_inspect: true,
            last_buf: true,
            ..base_inputs()
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::BlockUninspectable
        );
    }

    #[test]
    fn response_over_limit_blocks_before_inspect() {
        let inputs = ResponseFilterInputs {
            over_limit: true,
            should_inspect: true,
            ..base_inputs()
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::BlockOverLimit
        );
    }

    #[test]
    fn response_should_inspect_spawns() {
        let inputs = ResponseFilterInputs {
            should_inspect: true,
            last_buf: true,
            ..base_inputs()
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::SpawnInspection
        );
    }

    #[test]
    fn response_end_of_stream_flushes_when_nothing_to_inspect() {
        // last_buf alone (no inspect) -> flush buffered.
        let inputs = ResponseFilterInputs {
            last_buf: true,
            ..base_inputs()
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::FlushBuffered
        );

        // stream_done alone also flushes.
        let inputs = ResponseFilterInputs {
            stream_done: true,
            ..base_inputs()
        };
        assert_eq!(
            decide_response_action(inputs),
            ResponseAction::FlushBuffered
        );
    }

    #[test]
    fn response_mid_stream_keeps_buffering() {
        assert_eq!(
            decide_response_action(base_inputs()),
            ResponseAction::KeepBuffering
        );
    }

    // --- block_commit_kind ------------------------------------------------

    #[test]
    fn block_commit_suppressed_uses_blocked_response() {
        assert_eq!(block_commit_kind(true), BlockCommitKind::BlockedResponse);
    }

    #[test]
    fn block_commit_not_suppressed_uses_termination() {
        // SSE: headers already flushed -> SSE termination frame.
        assert_eq!(block_commit_kind(false), BlockCommitKind::Termination);
    }

    // --- should_inspect_status --------------------------------------------

    #[test]
    fn status_2xx_with_body_is_inspected() {
        for status in [200, 201, 202, 206, 299] {
            assert!(
                should_inspect_status(status),
                "status {status} should be inspected"
            );
        }
    }

    #[test]
    fn status_204_no_content_is_not_inspected() {
        // No body by definition: buffering would strand the suppressed headers.
        assert!(!should_inspect_status(204));
    }

    #[test]
    fn status_304_not_modified_is_not_inspected() {
        // 304 is < 400 (the old gate missed it) but carries no body.
        assert!(!should_inspect_status(304));
    }

    #[test]
    fn status_3xx_redirects_are_not_inspected() {
        for status in [300, 301, 302, 303, 307, 308] {
            assert!(
                !should_inspect_status(status),
                "3xx status {status} should pass through"
            );
        }
    }

    #[test]
    fn status_1xx_informational_is_not_inspected() {
        for status in [100, 101, 103] {
            assert!(
                !should_inspect_status(status),
                "1xx status {status} should pass through"
            );
        }
    }

    #[test]
    fn status_4xx_5xx_errors_are_not_inspected() {
        for status in [400, 403, 404, 429, 500, 502, 503] {
            assert!(
                !should_inspect_status(status),
                "error status {status} should pass through"
            );
        }
    }

    #[test]
    fn status_boundaries() {
        assert!(!should_inspect_status(199));
        assert!(should_inspect_status(200));
        assert!(should_inspect_status(203));
        assert!(!should_inspect_status(300));
    }
}
