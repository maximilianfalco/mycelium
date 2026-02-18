package indexer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	maxRetries     = 5
	baseBackoff    = 500 * time.Millisecond
	maxBackoff     = 30 * time.Second
	embeddingModel = openai.SmallEmbedding3
)

// EmbedTexts calls the OpenAI embeddings API for a batch of texts.
// Handles rate limiting (429) and server errors (5xx) with exponential backoff.
func EmbedTexts(ctx context.Context, client *openai.Client, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var resp openai.EmbeddingResponse
	var err error

	for attempt := range maxRetries {
		resp, err = client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: texts,
			Model: embeddingModel,
		})
		if err == nil {
			break
		}
		if !isRetryable(err) {
			return nil, fmt.Errorf("embedding API: %w", err)
		}

		backoff := calcBackoff(attempt)
		slog.Warn("embedding API retrying", "attempt", attempt+1, "backoff", backoff, "err", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("embedding API after %d retries: %w", maxRetries, err)
	}

	vectors := make([][]float32, len(resp.Data))
	for _, d := range resp.Data {
		vectors[d.Index] = d.Embedding
	}
	return vectors, nil
}

// EmbedText embeds a single text string and returns the vector.
func EmbedText(ctx context.Context, client *openai.Client, text string) ([]float32, error) {
	vectors, err := EmbedTexts(ctx, client, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embedding API returned no vectors")
	}
	return vectors[0], nil
}

// EmbedBatched splits texts into batches of batchSize and embeds them all.
// Logs progress per batch.
func EmbedBatched(ctx context.Context, client *openai.Client, texts []string, batchSize int) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = 2048
	}

	totalBatches := (len(texts) + batchSize - 1) / batchSize
	allVectors := make([][]float32, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		batchNum := i/batchSize + 1

		slog.Info("embedding batch", "batch", batchNum, "total", totalBatches, "nodes", len(batch))

		vectors, err := EmbedTexts(ctx, client, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d: %w", batchNum, totalBatches, err)
		}

		copy(allVectors[i:end], vectors)
	}

	return allVectors, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0.0 if either vector has zero magnitude.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dot, magA, magB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		magA += ai * ai
		magB += bi * bi
	}

	mag := math.Sqrt(magA) * math.Sqrt(magB)
	if mag == 0 {
		return 0.0
	}
	return dot / mag
}

func isRetryable(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == 429 || apiErr.HTTPStatusCode >= 500
	}
	// Network errors are retryable
	var reqErr *openai.RequestError
	return errors.As(err, &reqErr)
}

func calcBackoff(attempt int) time.Duration {
	backoff := baseBackoff * time.Duration(1<<uint(attempt))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	// Add jitter: ±25%
	jitter := time.Duration(float64(backoff) * (0.75 + rand.Float64()*0.5))
	return jitter
}
