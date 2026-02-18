# internal/indexer — Chunker

Prepares parsed node text for the OpenAI embedding API. Concatenates a node's fields into a single string, counts tokens using the model's actual tokenizer, and truncates oversized inputs while preserving the most semantically important parts.

## API

### PrepareEmbeddingInput

```go
func PrepareEmbeddingInput(signature, docstring, sourceCode string) (*ChunkResult, error)
```

Concatenates the three fields (newline-separated, empty fields skipped) and checks against the 8191 token limit for `text-embedding-3-small`.

**Truncation strategy** — when the combined text exceeds 8191 tokens:

1. Signature and docstring are always preserved (they carry the most semantic signal)
2. Source code is truncated from the end to fit within the remaining token budget
3. If signature + docstring alone exceed the limit, the entire concatenated text is hard-truncated at 8191 tokens

Returns a `ChunkResult` with the final text, accurate token count, and a truncation flag.

### CountTokens

```go
func CountTokens(text string) (int, error)
```

Returns the exact token count for arbitrary text using the `cl100k_base` encoding (same tokenizer used by `text-embedding-3-small`).

## Types

### ChunkResult

```go
type ChunkResult struct {
    Text       string `json:"text"`
    TokenCount int    `json:"tokenCount"`
    Truncated  bool   `json:"truncated"`
}
```

## Token counting

Uses `tiktoken-go` with the `cl100k_base` encoding (resolved via `EncodingForModel("text-embedding-3-small")`). The encoding is initialized once and cached for the process lifetime.

On first call, tiktoken-go downloads the BPE dictionary from the network and caches it locally. Set `TIKTOKEN_CACHE_DIR` to a local path to avoid network calls in CI/production.

## Language agnostic

The chunker operates on raw strings — it doesn't care what language produced the signature, docstring, or source code. Any language supported by the parser (currently TS/JS/Go) works without changes here.

## Text assembly

Empty fields are skipped entirely (no blank lines or trailing newlines):

| signature | docstring | sourceCode | result |
|---|---|---|---|
| `"func Add(a, b int) int"` | `"Adds two numbers"` | `"return a + b"` | `"func Add(a, b int) int\nAdds two numbers\nreturn a + b"` |
| `""` | `""` | `"x := 1"` | `"x := 1"` |
| `"func Big()"` | `""` | `"..."` | `"func Big()\n..."` |
| `""` | `""` | `""` | `""` |

## Files

| File | Purpose |
|---|---|
| `chunker.go` | `PrepareEmbeddingInput()`, `CountTokens()`, text assembly, encoding cache |
| `chunker_test.go` | Tests: concatenation, empty fields, truncation of oversized input, token counting, text assembly |
