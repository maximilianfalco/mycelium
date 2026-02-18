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

const maxTokensPerBatch = 250_000

// EmbedBatched splits texts into token-aware batches and embeds them all.
// Each batch stays under 250K tokens (OpenAI limit is 300K, this leaves headroom).
// batchSize caps the max number of items per batch as a secondary limit.
// If onProgress is non-nil, it's called after each batch with the percentage complete (0–100).
func EmbedBatched(ctx context.Context, client *openai.Client, texts []string, batchSize int, onProgress func(pct int)) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = 2048
	}

	// Pre-count tokens for each text to build token-aware batches
	tokenCounts := make([]int, len(texts))
	for i, t := range texts {
		tc, err := CountTokens(t)
		if err != nil {
			tc = len(t) / 3 // rough fallback: ~3 chars per token
		}
		tokenCounts[i] = tc
	}

	// Build batches respecting both token and item limits
	type batch struct {
		start, end int
	}
	var batches []batch
	i := 0
	for i < len(texts) {
		batchTokens := 0
		j := i
		for j < len(texts) && j-i < batchSize {
			if batchTokens+tokenCounts[j] > maxTokensPerBatch && j > i {
				break
			}
			batchTokens += tokenCounts[j]
			j++
		}
		batches = append(batches, batch{i, j})
		i = j
	}

	allVectors := make([][]float32, len(texts))
	totalBatches := len(batches)

	for batchNum, b := range batches {
		chunk := texts[b.start:b.end]

		slog.Info("embedding batch", "batch", batchNum+1, "total", totalBatches, "nodes", len(chunk))

		vectors, err := EmbedTexts(ctx, client, chunk)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d: %w", batchNum+1, totalBatches, err)
		}

		copy(allVectors[b.start:b.end], vectors)

		if onProgress != nil {
			onProgress((batchNum + 1) * 100 / totalBatches)
		}
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
