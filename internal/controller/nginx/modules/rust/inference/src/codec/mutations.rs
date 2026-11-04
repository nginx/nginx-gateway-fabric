//! Apply EPP header mutations to a header list.
//!
//! The EPP returns `HeaderMutation` messages with `set_headers` and
//! `remove_headers`. This module applies those mutations to a plain
//! `Vec<(name, value)>` list, which the nginx glue later writes into the
//! request (`r->headers_in`) or response (`r->headers_out`) header list.
//! Mutations from consecutive replies are merged in order.

use envoy_types::pb::envoy::config::core::v3::HeaderValue;
use envoy_types::pb::envoy::service::ext_proc::v3::HeaderMutation;
use prost::Message;

use crate::error::ModuleError;

/// Cumulative limits, including mutations skipped by validation.
pub(crate) const MAX_MUTATIONS: usize = 1024;
pub(crate) const MAX_MUTATION_BYTES: usize = 64 * 1024;

/// Headers that must not be mutated — pseudo-headers and hop-by-hop headers.
const DENYLIST: &[&str] = &[
    ":method",
    ":path",
    ":authority",
    ":scheme",
    "host",
    "content-length",
    "transfer-encoding",
    "connection",
    "upgrade",
    "te",
    "keep-alive",
    "proxy-connection",
];

/// A collected mutation to apply to the upstream request headers.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct HeaderOp {
    pub name: String,
    /// Raw value bytes.
    pub value: Vec<u8>,
    pub action: MutationAction,
}

/// How to apply a header set operation.
///
/// `AddIfAbsent` and `OverwriteIfExists` are not currently produced by
/// [`MutationSet::merge`] (Envoy's ext_proc ignores `append_action` and only
/// uses the deprecated `append` wrapper), but they are part of the protocol's
/// semantic model and are exercised by unit tests via [`apply_batch`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum MutationAction {
    /// Append to existing values, or add if absent (default ext_proc behavior).
    AppendOrAdd,
    /// Add only if the header does not already exist; no-op otherwise.
    #[cfg_attr(not(test), allow(dead_code))]
    AddIfAbsent,
    /// Overwrite existing values, or add if absent.
    OverwriteOrAdd,
    /// Overwrite existing values; no-op if absent.
    #[cfg_attr(not(test), allow(dead_code))]
    OverwriteIfExists,
}

/// A set of collected mutations from one or more EPP responses.
/// Populate through `merge`; the public vectors are for inspection only.
#[derive(Debug, Default)]
pub(crate) struct MutationSet {
    /// Collected sets; use `apply` to preserve ordering with removals.
    pub sets: Vec<HeaderOp>,
    pub removes: Vec<String>,
    batches: Vec<(usize, usize)>,
    count: usize,
    bytes: usize,
}

impl MutationSet {
    /// Merge a `HeaderMutation` into this set, preserving order.
    /// Denylisted headers and headers with invalid HTTP syntax are silently skipped.
    /// Reject an entire batch if cumulative count or encoded bytes exceed the limits.
    pub fn merge(&mut self, hm: &HeaderMutation) -> Result<(), ModuleError> {
        let count = self
            .count
            .checked_add(hm.set_headers.len())
            .and_then(|n| n.checked_add(hm.remove_headers.len()))
            .filter(|&n| n <= MAX_MUTATIONS)
            .ok_or_else(|| ModuleError::InvalidResponse("too many header mutations".into()))?;
        let bytes = self
            .bytes
            .checked_add(hm.encoded_len())
            .filter(|&n| n <= MAX_MUTATION_BYTES)
            .ok_or_else(|| {
                ModuleError::InvalidResponse("header mutation byte limit exceeded".into())
            })?;

        let start = (self.sets.len(), self.removes.len());
        for hvo in &hm.set_headers {
            let Some(hv) = &hvo.header else { continue };
            let name = hv.key.to_ascii_lowercase();
            if is_denylisted(&name) || !is_valid_header_name(&name) {
                continue;
            }
            let value = header_value_bytes(hv);
            if !is_valid_header_value(value) {
                continue;
            }
            let value = value.to_vec();
            // Envoy's ext_proc filter uses only the deprecated `append` wrapper
            // (with a default of `false` → overwrite) and ignores `append_action`.
            // Match that behaviour: when the wrapper is absent, default to
            // `OverwriteOrAdd` so that EPPs written for Envoy work identically.
            #[allow(deprecated)]
            let action = if hvo.append.as_ref().is_some_and(|a| a.value) {
                MutationAction::AppendOrAdd
            } else {
                MutationAction::OverwriteOrAdd
            };
            self.sets.push(HeaderOp {
                name,
                value,
                action,
            });
        }
        for name in &hm.remove_headers {
            let lower = name.to_ascii_lowercase();
            if !is_denylisted(&lower) && is_valid_header_name(&lower) {
                self.removes.push(lower);
            }
        }
        let end = (self.sets.len(), self.removes.len());
        if start != end {
            self.batches.push(end);
        }
        self.count = count;
        self.bytes = bytes;
        Ok(())
    }
}

/// Apply the collected mutations to a plain header list.
///
/// `headers` is `(lowercased name, value)`. This is the unit-testable core;
/// the nginx glue calls this on a snapshot of `headers_in`.
pub(crate) fn apply(mutations: &MutationSet, headers: &mut Vec<(String, Vec<u8>)>) {
    let (mut sets_start, mut removes_start) = (0, 0);
    for &(sets_end, removes_end) in &mutations.batches {
        apply_batch(
            &mutations.sets[sets_start..sets_end],
            &mutations.removes[removes_start..removes_end],
            headers,
        );
        sets_start = sets_end;
        removes_start = removes_end;
    }
}

fn apply_batch(sets: &[HeaderOp], removes: &[String], headers: &mut Vec<(String, Vec<u8>)>) {
    for name in removes {
        headers.retain(|(k, _)| k != name);
    }

    for op in sets {
        match op.action {
            MutationAction::AppendOrAdd => {
                headers.push((op.name.clone(), op.value.clone()));
            }
            MutationAction::AddIfAbsent => {
                if !headers.iter().any(|(k, _)| k == &op.name) {
                    headers.push((op.name.clone(), op.value.clone()));
                }
            }
            MutationAction::OverwriteOrAdd => {
                // Remove all existing, then add.
                headers.retain(|(k, _)| k != &op.name);
                headers.push((op.name.clone(), op.value.clone()));
            }
            MutationAction::OverwriteIfExists => {
                if headers.iter().any(|(k, _)| k == &op.name) {
                    headers.retain(|(k, _)| k != &op.name);
                    headers.push((op.name.clone(), op.value.clone()));
                }
            }
        }
    }
}

/// Header value bytes: `raw_value` when set, otherwise the UTF-8 `value`.
pub(crate) fn header_value_bytes(hv: &HeaderValue) -> &[u8] {
    if hv.raw_value.is_empty() {
        hv.value.as_bytes()
    } else {
        &hv.raw_value
    }
}

fn is_denylisted(name: &str) -> bool {
    DENYLIST.contains(&name)
}

/// Validate an HTTP header name per RFC 7230 §3.2.6 (token).
///
/// Rejects empty names and any byte outside the token character set.
fn is_valid_header_name(name: &str) -> bool {
    if name.is_empty() {
        return false;
    }
    name.bytes()
        .all(|b| matches!(b, b'!' | b'#' | b'$' | b'%' | b'&' | b'\'' | b'*' | b'+' | b'-' | b'.' |
                               b'0'..=b'9' | b'A'..=b'Z' | b'^' | b'_' | b'`' | b'a'..=b'z' | b'|' | b'~'))
}

/// Validate an HTTP header value per RFC 9110 §5.5 (field-value).
///
/// Rejects control characters other than horizontal tab, and DEL, which
/// prevents header injection (CR/LF/NUL) and other invalid field bytes.
fn is_valid_header_value(value: &[u8]) -> bool {
    !value.iter().any(|&b| (b < 0x20 && b != b'\t') || b == 0x7f)
}

#[cfg(test)]
mod tests {
    use envoy_types::pb::envoy::config::core::v3::{
        HeaderValue, HeaderValueOption, header_value_option::HeaderAppendAction,
    };
    use envoy_types::pb::google::protobuf::BoolValue;

    use super::*;

    fn hvo(key: &str, value: &[u8], action: HeaderAppendAction) -> HeaderValueOption {
        HeaderValueOption {
            header: Some(HeaderValue {
                key: key.into(),
                raw_value: value.to_vec(),
                ..Default::default()
            }),
            append_action: action as i32,
            ..Default::default()
        }
    }

    fn make_mutation(sets: Vec<HeaderValueOption>, removes: Vec<&str>) -> HeaderMutation {
        HeaderMutation {
            set_headers: sets,
            remove_headers: removes.into_iter().map(String::from).collect(),
        }
    }

    fn h(name: &str, val: &[u8]) -> (String, Vec<u8>) {
        (name.into(), val.to_vec())
    }

    #[test]
    #[allow(deprecated)]
    fn append_wrapper_overrides_append_action() {
        // When the deprecated `append` wrapper is present, it takes precedence
        // over any `append_action` value.
        for action in [0, 1, 2, 3, i32::MAX] {
            for append_val in [false, true] {
                let mut option = hvo("x-test", b"new", HeaderAppendAction::AppendIfExistsOrAdd);
                option.append = Some(BoolValue { value: append_val });
                option.append_action = action;
                let mut ms = MutationSet::default();
                ms.merge(&make_mutation(vec![option], vec![])).unwrap();
                let mut headers = vec![h("x-test", b"old"), h("x-test", b"duplicate")];
                apply(&ms, &mut headers);
                let expected = if append_val {
                    vec![
                        h("x-test", b"old"),
                        h("x-test", b"duplicate"),
                        h("x-test", b"new"),
                    ]
                } else {
                    vec![h("x-test", b"new")]
                };
                assert_eq!(headers, expected, "append={append_val}, action={action}");

                let mut headers = vec![];
                apply(&ms, &mut headers);
                assert_eq!(headers, vec![h("x-test", b"new")]);
            }
        }
    }

    #[test]
    fn absent_append_wrapper_defaults_to_overwrite() {
        // When the deprecated `append` wrapper is absent, we match Envoy's
        // ext_proc behaviour: default to overwrite (OverwriteOrAdd),
        // regardless of the `append_action` field value.
        for action in [
            HeaderAppendAction::AppendIfExistsOrAdd,
            HeaderAppendAction::AddIfAbsent,
            HeaderAppendAction::OverwriteIfExistsOrAdd,
            HeaderAppendAction::OverwriteIfExists,
        ] {
            let mut ms = MutationSet::default();
            ms.merge(&make_mutation(vec![hvo("x-test", b"new", action)], vec![]))
                .unwrap();
            assert_eq!(
                ms.sets[0].action,
                MutationAction::OverwriteOrAdd,
                "action={action:?} should map to OverwriteOrAdd"
            );
        }
    }

    #[test]
    fn later_removal_does_not_resurrect_earlier_set() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![hvo(
                "x-test",
                b"first",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();
        ms.merge(&make_mutation(vec![], vec!["x-test"])).unwrap();
        let mut headers = vec![h("x-test", b"original"), h("x-keep", b"keep")];
        apply(&ms, &mut headers);
        assert_eq!(headers, vec![h("x-keep", b"keep")]);
    }

    #[test]
    #[allow(deprecated)]
    fn each_batch_removes_before_setting_and_preserves_set_order() {
        let mut ms = MutationSet::default();
        for value in [b"first".as_slice(), b"second".as_slice()] {
            let mut first = hvo("x-test", value, HeaderAppendAction::OverwriteIfExistsOrAdd);
            first.append = Some(BoolValue { value: true });
            let mut last = first.clone();
            last.header.as_mut().unwrap().raw_value = b"last".to_vec();
            ms.merge(&make_mutation(vec![first, last], vec!["x-test"]))
                .unwrap();
        }
        let mut headers = vec![h("x-test", b"original")];
        apply(&ms, &mut headers);
        assert_eq!(headers, vec![h("x-test", b"second"), h("x-test", b"last")]);
    }

    #[test]
    fn mutation_count_limit_is_cumulative_and_atomic() {
        let mut ms = MutationSet::default();
        let batch = make_mutation(
            vec![
                hvo(
                    "x-test",
                    b"value",
                    HeaderAppendAction::OverwriteIfExistsOrAdd
                );
                MAX_MUTATIONS / 2
            ],
            vec![],
        );
        ms.merge(&batch).unwrap();
        ms.merge(&batch).unwrap();
        assert_eq!(ms.count, MAX_MUTATIONS);
        let bytes = ms.bytes;
        assert!(ms.merge(&make_mutation(vec![], vec!["x-test"])).is_err());
        assert_eq!(ms.sets.len(), MAX_MUTATIONS);
        assert!(ms.removes.is_empty());
        assert_eq!(ms.batches.len(), 2);
        assert_eq!(ms.bytes, bytes);
    }

    #[test]
    fn mutation_byte_limit_is_cumulative_and_atomic() {
        let mut ms = MutationSet::default();
        let mut batch = make_mutation(
            vec![hvo(
                "x-test",
                &vec![b'a'; MAX_MUTATION_BYTES / 2],
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        );
        ms.merge(&batch).unwrap();
        let excess = 2 * batch.encoded_len() - MAX_MUTATION_BYTES;
        batch.set_headers[0]
            .header
            .as_mut()
            .unwrap()
            .raw_value
            .truncate(MAX_MUTATION_BYTES / 2 - excess);
        ms.merge(&batch).unwrap();
        assert_eq!(ms.bytes, MAX_MUTATION_BYTES);
        assert!(ms.merge(&make_mutation(vec![], vec!["x-test"])).is_err());
        assert_eq!(ms.sets.len(), 2);
        assert_eq!(ms.count, 2);
        assert!(ms.removes.is_empty());
    }

    #[test]
    fn rejected_and_missing_headers_still_consume_budget() {
        let mut ms = MutationSet::default();
        let batch = make_mutation(
            vec![HeaderValueOption::default(); MAX_MUTATIONS / 2],
            vec!["host"; MAX_MUTATIONS / 2],
        );
        ms.merge(&batch).unwrap();
        assert!(ms.sets.is_empty());
        assert!(ms.removes.is_empty());
        assert!(ms.batches.is_empty());
        assert!(ms.merge(&make_mutation(vec![], vec!["x-test"])).is_err());

        let oversized = make_mutation(
            vec![hvo(
                "host",
                &vec![b'a'; MAX_MUTATION_BYTES],
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        );
        let mut ms = MutationSet::default();
        assert!(ms.merge(&oversized).is_err());
        assert_eq!((ms.count, ms.bytes), (0, 0));
    }

    #[test]
    fn overwrite_or_add_replaces_existing() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![hvo(
                "x-worker-id",
                b"w1",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();

        let mut headers = vec![h("x-worker-id", b"old"), h("other", b"keep")];
        apply(&ms, &mut headers);
        assert_eq!(headers.len(), 2);
        assert_eq!(headers[0], h("other", b"keep"));
        assert_eq!(headers[1], h("x-worker-id", b"w1"));
    }

    #[test]
    fn overwrite_or_add_adds_when_absent() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![hvo(
                "x-new",
                b"val",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();

        let mut headers = vec![h("existing", b"e")];
        apply(&ms, &mut headers);
        assert_eq!(headers.len(), 2);
        assert_eq!(headers[1], h("x-new", b"val"));
    }

    #[test]
    fn absent_append_wrapper_overwrites() {
        // Without the deprecated `append` wrapper, even if `append_action`
        // is AppendIfExistsOrAdd, Envoy-compatible behaviour overwrites.
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![hvo(
                "x-multi",
                b"second",
                HeaderAppendAction::AppendIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();

        let mut headers = vec![h("x-multi", b"first")];
        apply(&ms, &mut headers);
        // OverwriteOrAdd: replaces existing.
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0], h("x-multi", b"second"));
    }

    #[test]
    #[allow(deprecated)]
    fn true_append_wrapper_appends() {
        let mut option = hvo(
            "x-multi",
            b"second",
            HeaderAppendAction::OverwriteIfExistsOrAdd,
        );
        option.append = Some(BoolValue { value: true });
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(vec![option], vec![])).unwrap();

        let mut headers = vec![h("x-multi", b"first")];
        apply(&ms, &mut headers);
        assert_eq!(headers.len(), 2);
        assert_eq!(headers[0], h("x-multi", b"first"));
        assert_eq!(headers[1], h("x-multi", b"second"));
    }

    #[test]
    fn apply_add_if_absent_skips_when_present() {
        // Test apply_batch directly for AddIfAbsent action.
        let ops = [HeaderOp {
            name: "x-existing".into(),
            value: b"new".to_vec(),
            action: MutationAction::AddIfAbsent,
        }];

        let mut headers = vec![h("x-existing", b"old")];
        apply_batch(&ops, &[], &mut headers);
        // AddIfAbsent: no-op when header already exists.
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0], h("x-existing", b"old"));
    }

    #[test]
    fn apply_add_if_absent_adds_when_missing() {
        let ops = [HeaderOp {
            name: "x-new".into(),
            value: b"val".to_vec(),
            action: MutationAction::AddIfAbsent,
        }];

        let mut headers = vec![];
        apply_batch(&ops, &[], &mut headers);
        assert_eq!(headers.len(), 1);
    }

    #[test]
    fn apply_overwrite_if_exists_skips_when_missing() {
        let ops = [HeaderOp {
            name: "x-gone".into(),
            value: b"val".to_vec(),
            action: MutationAction::OverwriteIfExists,
        }];

        let mut headers = vec![h("other", b"x")];
        apply_batch(&ops, &[], &mut headers);
        // OverwriteIfExists: no-op when header is absent.
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0], h("other", b"x"));
    }

    #[test]
    fn apply_overwrite_if_exists_replaces() {
        let ops = [HeaderOp {
            name: "x-target".into(),
            value: b"new".to_vec(),
            action: MutationAction::OverwriteIfExists,
        }];

        let mut headers = vec![h("x-target", b"old"), h("x-target", b"dup")];
        apply_batch(&ops, &[], &mut headers);
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0], h("x-target", b"new"));
    }

    #[test]
    fn remove_headers() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(vec![], vec!["x-remove-me"]))
            .unwrap();

        let mut headers = vec![h("x-remove-me", b"v"), h("keep", b"v")];
        apply(&ms, &mut headers);
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0].0, "keep");
    }

    #[test]
    fn denylist_blocks_pseudo_headers() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![
                hvo(
                    ":method",
                    b"DELETE",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo("host", b"evil", HeaderAppendAction::OverwriteIfExistsOrAdd),
                hvo("x-ok", b"val", HeaderAppendAction::OverwriteIfExistsOrAdd),
            ],
            vec![":path", "content-length", "x-remove-ok"],
        ))
        .unwrap();

        assert_eq!(ms.sets.len(), 1);
        assert_eq!(ms.sets[0].name, "x-ok");
        assert_eq!(ms.removes.len(), 1);
        assert_eq!(ms.removes[0], "x-remove-ok");
    }

    #[test]
    fn merge_preserves_order() {
        let mut ms = MutationSet::default();
        // First response.
        ms.merge(&make_mutation(
            vec![hvo(
                "x-first",
                b"1",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();
        // Second response.
        ms.merge(&make_mutation(
            vec![hvo(
                "x-second",
                b"2",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec![],
        ))
        .unwrap();

        assert_eq!(ms.sets.len(), 2);
        assert_eq!(ms.sets[0].name, "x-first");
        assert_eq!(ms.sets[1].name, "x-second");
    }

    #[test]
    fn raw_value_preferred_over_value() {
        let mut ms = MutationSet::default();
        let hm = HeaderMutation {
            set_headers: vec![HeaderValueOption {
                header: Some(HeaderValue {
                    key: "x-test".into(),
                    value: "string-val".into(),
                    raw_value: b"raw-val".to_vec(),
                }),
                append_action: HeaderAppendAction::OverwriteIfExistsOrAdd as i32,
                ..Default::default()
            }],
            remove_headers: vec![],
        };
        ms.merge(&hm).unwrap();
        assert_eq!(ms.sets[0].value, b"raw-val");
    }

    #[test]
    fn falls_back_to_string_value() {
        let mut ms = MutationSet::default();
        let hm = HeaderMutation {
            set_headers: vec![HeaderValueOption {
                header: Some(HeaderValue {
                    key: "x-test".into(),
                    value: "string-val".into(),
                    raw_value: vec![],
                }),
                append_action: HeaderAppendAction::OverwriteIfExistsOrAdd as i32,
                ..Default::default()
            }],
            remove_headers: vec![],
        };
        ms.merge(&hm).unwrap();
        assert_eq!(ms.sets[0].value, b"string-val");
    }

    #[test]
    fn case_insensitive_names() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![hvo(
                "X-Mixed-Case",
                b"v",
                HeaderAppendAction::OverwriteIfExistsOrAdd,
            )],
            vec!["X-Remove"],
        ))
        .unwrap();
        assert_eq!(ms.sets[0].name, "x-mixed-case");
        assert_eq!(ms.removes[0], "x-remove");
    }

    #[test]
    fn rejects_header_name_with_invalid_chars() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![
                hvo(
                    "valid-name",
                    b"ok",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo(
                    "bad name",
                    b"space",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo(
                    "bad\rname",
                    b"cr",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo("", b"empty", HeaderAppendAction::OverwriteIfExistsOrAdd),
            ],
            vec!["ok-remove", "bad remove"],
        ))
        .unwrap();
        assert_eq!(ms.sets.len(), 1);
        assert_eq!(ms.sets[0].name, "valid-name");
        assert_eq!(ms.removes.len(), 1);
        assert_eq!(ms.removes[0], "ok-remove");
    }

    #[test]
    fn unset_append_wrapper_with_default_action_overwrites() {
        // Envoy's ext_proc ignores `append_action` and defaults the absent
        // deprecated `append` wrapper to `false` (overwrite).  Our module
        // mirrors that behaviour, so an absent wrapper + default action 0
        // must produce OverwriteOrAdd, not AppendIfExistsOrAdd.
        let mut ms = MutationSet::default();
        let hm = HeaderMutation {
            set_headers: vec![HeaderValueOption {
                header: Some(HeaderValue {
                    key: "x-served".into(),
                    raw_value: b"10.0.0.1:3000".to_vec(),
                    ..Default::default()
                }),
                // append_action 0 = AppendIfExistsOrAdd (proto default),
                // but we ignore it when the append wrapper is absent.
                append_action: 0,
                ..Default::default()
            }],
            remove_headers: vec![],
        };
        ms.merge(&hm).unwrap();

        let mut headers = vec![h("x-served", b"10.0.0.1:3000")];
        apply(&ms, &mut headers);
        // OverwriteOrAdd replaces the existing header in-place.
        assert_eq!(headers.len(), 1);
        assert_eq!(headers[0], h("x-served", b"10.0.0.1:3000"));
    }

    #[test]
    fn rejects_header_value_with_crlf() {
        let mut ms = MutationSet::default();
        ms.merge(&make_mutation(
            vec![
                hvo("x-ok", b"clean", HeaderAppendAction::OverwriteIfExistsOrAdd),
                hvo(
                    "x-inject-cr",
                    b"val\rHTTP/1.1",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo(
                    "x-inject-lf",
                    b"val\nEvil: header",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
                hvo(
                    "x-inject-nul",
                    b"val\0rest",
                    HeaderAppendAction::OverwriteIfExistsOrAdd,
                ),
            ],
            vec![],
        ))
        .unwrap();
        assert_eq!(ms.sets.len(), 1);
        assert_eq!(ms.sets[0].name, "x-ok");
    }

    #[test]
    fn header_value_validation_matches_field_value_grammar() {
        for b in (0x00..0x20).filter(|&b| b != b'\t').chain([0x7f]) {
            assert!(!is_valid_header_value(&[b'a', b, b'z']), "byte {b:#04x}");
        }
        assert!(is_valid_header_value(b"a\tb c~"));
        assert!(is_valid_header_value(&[0x80, 0xff]));
        assert!(is_valid_header_value(b""));
    }
}
