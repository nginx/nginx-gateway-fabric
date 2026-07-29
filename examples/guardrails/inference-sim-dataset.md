
# Dataset Card

## Overview

This dataset is derived from conversational data and has been processed into a tokenized format suitable for LLM inference simulation.
The dataset contains pre-tokenized prompts and responses, enabling efficient testing of inference systems without requiring live model execution.

## Tokenization Model

Built with the `llm-d-inference-sim` **`SimpleTokenizer`** (regex split + FNV-32a hash) via
`regen-dataset.py`. This matches the tokenizer the simulator falls back to when it cannot reach
HuggingFace to load the real `Qwen/Qwen3-32B` tokenizer (the offline / air-gapped default). The
dataset's `prompt_hash` values must be produced with the same tokenizer the running pod uses, or
prompt lookups miss and responses become random. See the example README's "Regenerating the dataset"
section.

## Source Dataset

The original dataset consists of multi-turn conversations between humans and AI assistants. <br>
Dataset: local file test-data.json


### Dataset Formats

This dataset is available in two formats:

- **JSON:** Human-readable format ideal for debugging and reference.
- **SQLite:** Optimized for efficient querying, used by the simulator.

### Data Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `prompt_hash` | string | SHA-256 hash uniquely identifying the input prompt |
| `input_text` | string | The prompt text (raw or chat-templated) |
| `generated` | string | The response text from the assistant |
| `n_gen_tokens` | integer | Total count of tokens in the generated response |
| `gen_tokens` | object | Tokenized response containing `strings` (token text) and `numbers` (token IDs) |

### Data Example

```json
{
  "prompt_hash": "OZ5Edy+9rw0CsSMabW2TwSxR78jJGYRVRWtz8SXRm6U=",
  "n_gen_tokens": 4,
  "gen_tokens": {
    "strings": ["g", "pt", " a", "1"],
    "numbers": [70, 417, 264, 16]
  },
  "input_text": "human q1",
  "generated": "gpt a1"
}
```

## SQLite Database Schema

The SQLite version provides efficient querying capabilities and used by the simulator. <br>
The data is stored in table called `llmd`.<br>
The table has the following schema:

| Column | Data Type | Description |
| :--- | :--- | :--- |
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | Auto-incrementing primary key |
| `prompt_hash` | BLOB NOT NULL | Binary hash identifier for the input prompt |
| `gen_tokens` | JSON NOT NULL | JSON object containing tokenized response data |
| `n_gen_tokens` | INTEGER NOT NULL | Count of generated tokens |

### Example Query

Calculate the average response length:

```sql
SELECT AVG(n_gen_tokens) FROM llmd;
```

## Dataset Characteristics

- **Tokenization**: All responses are pre-tokenized using a specified language model tokenizer
- **Dual Format**: Each conversation generates both completion and chat-completion variants
- **Hash-Based Indexing**: Prompts are indexed by SHA-256 hash for efficient lookup
- **Token Details**: Both string representations and numerical token IDs are preserved
- **Scalable**: SQLite format supports efficient querying of large datasets


## Dataset Statistics

- **Source Dataset Record Count**: 10
- **Generated Dataset Record Count**: 20
