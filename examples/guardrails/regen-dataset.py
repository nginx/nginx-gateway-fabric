#!/usr/bin/env python3
"""Regenerate the llm-d-inference-sim dataset using the simulator's built-in
SimpleTokenizer.

Why this exists
---------------
`llm-d-inference-sim` only loads the *real* HuggingFace tokenizer for `--model`
when it can reach `https://huggingface.co/api/models/<model>`. In an offline /
air-gapped cluster that probe fails and the simulator silently falls back to its
built-in `SimpleTokenizer` (a regex splitter + FNV-32a hash), logging:

    "Model is not a real HF model, using simulated tokenizer"

The prompt hash that keys the dataset is computed from the tokenizer's token IDs.
If the dataset was built with the real Qwen tokenizer but the running pod uses the
SimpleTokenizer, the hashes never match, so every request falls through to the
"random response" path (round-robin over the whole dataset).

This script builds the dataset with the SAME SimpleTokenizer the offline pod uses,
so prompt hashes match and each seeded prompt deterministically returns its
response. It is a faithful re-implementation of the relevant upstream logic:

  - SimpleTokenizer regex + FNV-32a:  pkg/tokenizer/tokenizer.go
  - prompt hash (LE uint32 -> sha256): pkg/dataset/utils.go getInputHash
  - two records per turn (/completions raw + /chat flattened):
                                       pkg/dataset/ds_tool.go
  - sqlite schema + gen_tokens JSON:   pkg/dataset/sqlite_helper.go

Usage:
    python3 regen-dataset.py
Outputs (overwrites):
    inference-sim-dataset.sqlite3
    inference-sim-dataset.json
"""

import json
import re
import sqlite3
import struct
import hashlib
from pathlib import Path

HERE = Path(__file__).parent
SRC = HERE / "test-data.json"
SQLITE_OUT = HERE / "inference-sim-dataset.sqlite3"
JSON_OUT = HERE / "inference-sim-dataset.json"
TABLE = "llmd"

# --- SimpleTokenizer (pkg/tokenizer/tokenizer.go) ---------------------------
# Go regex: (\{|\}|:|,|-|\.|\?|\!|;|@|#|\$|%|\^|&|\*|\(|\)|\+|\-|_|~|/|\\|>|<|\[|\]|=|"|\w+)(\s*)
# Go's FindAllString returns each full match (token + trailing whitespace) and
# skips zero-length matches.
_TOKEN_RE = re.compile(
    r'(\{|\}|:|,|-|\.|\?|\!|;|@|#|\$|%|\^|&|\*|\(|\)|\+|\-|_|~|/|\\|>|<|\[|\]|=|"|\w+)(\s*)'
)


def simple_tokenize(text: str):
    """Return the list of token strings, matching Go's FindAllString output."""
    return [m.group(0) for m in _TOKEN_RE.finditer(text) if m.group(0) != ""]


def fnv32a(s: str) -> int:
    """32-bit FNV-1a hash over UTF-8 bytes (Go hash/fnv New32a)."""
    h = 0x811C9DC5
    for b in s.encode("utf-8"):
        h ^= b
        h = (h * 0x01000193) & 0xFFFFFFFF
    return h


def tokenize(text: str):
    """Return (token_ids, token_strings) exactly as SimpleTokenizer.RenderText."""
    strings = simple_tokenize(text)
    ids = [fnv32a(s) for s in strings]
    return ids, strings


def flatten_chat(user_text: str) -> str:
    """Replicate FlattenChatRequest for a single user message.

    Upstream builds "### <role>:\n<content>\n" per message. The /chat record in
    ds-tool contains only the user message at that turn (single-turn dataset).
    """
    return f"### user:\n{user_text}\n"


def input_hash(token_ids) -> bytes:
    """SHA-256 over little-endian uint32 token IDs (utils.go getInputHash)."""
    hasher = hashlib.sha256()
    for tid in token_ids:
        hasher.update(struct.pack("<I", tid))
    return hasher.digest()


def build_records():
    data = json.loads(SRC.read_text())
    records = []  # for the human-readable JSON dump
    rows = []     # (prompt_hash_bytes, gen_tokens_json, n_gen_tokens) for sqlite

    for item in data:
        convs = item["conversations"]
        # single-turn: one human prompt + one gpt response
        user_text = next(c["value"] for c in convs if c["from"] == "human")
        gpt_text = next(c["value"] for c in convs if c["from"] == "gpt")

        gen_ids, gen_strs = tokenize(gpt_text)
        n_gen = len(gen_ids)
        gen_tokens_obj = {"Tokens": gen_ids, "Strings": gen_strs}
        gen_tokens_json = json.dumps(gen_tokens_obj, separators=(",", ":"))

        # sanity: response text must round-trip
        assert "".join(gen_strs) == gpt_text, f"response mismatch: {gpt_text!r}"

        # --- /completions variant: raw prompt ---
        comp_ids, _ = tokenize(user_text)
        comp_hash = input_hash(comp_ids)

        # --- /chat/completions variant: flattened chat template ---
        chat_ids, _ = tokenize(flatten_chat(user_text))
        chat_hash = input_hash(chat_ids)

        for input_text, h in ((user_text, comp_hash), (flatten_chat(user_text), chat_hash)):
            rows.append((h, gen_tokens_json, n_gen))
            import base64
            records.append(
                {
                    "prompt_hash": base64.b64encode(h).decode("ascii"),
                    "n_gen_tokens": n_gen,
                    "gen_tokens": gen_tokens_obj,
                    "input_text": input_text,
                    "generated": gpt_text,
                }
            )

    return records, rows


def write_sqlite(rows):
    if SQLITE_OUT.exists():
        SQLITE_OUT.unlink()
    conn = sqlite3.connect(str(SQLITE_OUT))
    try:
        conn.execute(
            f"""CREATE TABLE {TABLE} (
                id INTEGER PRIMARY KEY,
                prompt_hash BLOB NOT NULL,
                gen_tokens JSON NOT NULL,
                n_gen_tokens INTEGER NOT NULL
            )"""
        )
        conn.executemany(
            f"INSERT INTO {TABLE} (prompt_hash, gen_tokens, n_gen_tokens) VALUES (?, ?, ?)",
            [(sqlite3.Binary(h), g, n) for (h, g, n) in rows],
        )
        conn.commit()
    finally:
        conn.close()


def main():
    records, rows = build_records()
    write_sqlite(rows)
    JSON_OUT.write_text(json.dumps(records, indent=2) + "\n")
    print(f"Wrote {len(rows)} rows to {SQLITE_OUT.name} and {JSON_OUT.name}")
    for r in records:
        print(f"  {r['n_gen_tokens']:3d} tok  {r['input_text']!r} -> {r['generated'][:40]!r}")


if __name__ == "__main__":
    main()
