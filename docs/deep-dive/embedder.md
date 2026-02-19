# internal/indexer — Embedder

Calls the OpenAI embeddings API to convert text into 1536-dimensional vectors. Handles batching, retry with exponential backoff, and provides a cosine similarity function for comparing vectors.

## API

### EmbedText

```go
func EmbedText(ctx context.Context, client *openai.Client, text string) ([]float32, error)
```

Embeds a single text string. Convenience wrapper around `EmbedTexts`.

### EmbedTexts

```go
func EmbedTexts(ctx context.Context, client *openai.Client, texts []string) ([][]float32, error)
```

Sends a batch of texts to the OpenAI embeddings API in a single request. Returns vectors in the same order as the input. Handles retries internally (see below).

Returns `nil, nil` for empty input.

### EmbedBatched

```go
func EmbedBatched(ctx context.Context, client *openai.Client, texts []string, batchSize int) ([][]float32, error)
```

Splits a large set of texts into batches of `batchSize` (defaults to 2048 if <= 0) and embeds each batch sequentially. Logs progress per batch via `slog.Info`.

This is the main entry point for the indexing pipeline — pass all node texts and it handles the batching.

### CosineSimilarity

```go
func CosineSimilarity(a, b []float32) float64
```

Computes cosine similarity between two vectors. Returns a value from -1.0 (opposite) to 1.0 (identical).

**Edge cases:**
- Mismatched lengths: returns 0.0
- Empty/nil vectors: returns 0.0
- Zero-magnitude vector: returns 0.0 (no division by zero)

## Model

Uses `text-embedding-3-small` (OpenAI constant `openai.SmallEmbedding3`). Produces 1536-dimensional `float32` vectors.

## Retry logic

Retries on transient errors only — up to 5 attempts with exponential backoff and jitter.

| Error type | Retryable? |
|---|---|
| 429 Too Many Requests | Yes |
| 5xx Server Error | Yes |
| Network/transport error (`RequestError`) | Yes |
| 4xx Client Error (400, 401, 403) | No — fails immediately |

### Backoff schedule

Base: 500ms, doubles per attempt, capped at 30s. Jitter of +/-25% applied to prevent thundering herd.

| Attempt | Base backoff | With jitter range |
|---|---|---|
| 1 | 500ms | 375ms — 625ms |
| 2 | 1s | 750ms — 1.25s |
| 3 | 2s | 1.5s — 2.5s |
| 4 | 4s | 3s — 5s |
| 5 | 8s | 6s — 10s |

Context cancellation is respected between retries — if the context is cancelled during a backoff sleep, the function returns immediately with `ctx.Err()`.

## OpenAI client setup

The OpenAI client is created at server startup in `DebugRoutes(cfg)` using the `OPENAI_API_KEY` from config. If the key is empty, the client is `nil` and embed/compare endpoints return 503 Service Unavailable.

## Debug endpoints

The spore lab's embed and compare endpoints (`POST /debug/embed-text`, `POST /debug/compare`) use the real embedder. Response shapes match the original mocks:

**POST /debug/embed-text**
```
Request:  { text: string }
Response: { vector: float32[8], dimensions: 1536, tokenCount: int, model: string, truncated: bool }
```

`vector` contains only the first 8 dimensions for display. `tokenCount` is the accurate tiktoken count, not the previous rough estimate.

**POST /debug/compare**
```
Request:  { text1: string, text2: string }
Response: { similarity: float64, tokenCount1: int, tokenCount2: int, dimensions: 1536 }
```

Both texts are embedded in a single API call (batch of 2), then compared via `CosineSimilarity`.

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/sashabaranov/go-openai` | OpenAI API client |
| `github.com/pkoukk/tiktoken-go` | Token counting (used by chunker, shared encoding) |

## Files

| File | Purpose |
|---|---|
| `embedder.go` | `EmbedText()`, `EmbedTexts()`, `EmbedBatched()`, `CosineSimilarity()`, retry logic |
| `embedder_test.go` | Tests: cosine similarity edge cases, mock API server tests for single/batch embedding, retry classification, backoff bounds |
