package indexer

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("expected 1.0 for identical vectors, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 1e-6 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	sim := CosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for zero vector, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for mismatched lengths, got %f", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for nil vectors, got %f", sim)
	}
}

func TestEmbedText_MockAPI(t *testing.T) {
	fakeVector := make([]float32, 1536)
	for i := range fakeVector {
		fakeVector[i] = float32(i) * 0.001
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.EmbeddingResponse{
			Data: []openai.Embedding{
				{
					Object:    "embedding",
					Embedding: fakeVector,
					Index:     0,
				},
			},
			Model: openai.SmallEmbedding3,
			Usage: openai.Usage{TotalTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	vector, err := EmbedText(context.Background(), client, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vector) != 1536 {
		t.Errorf("expected 1536 dimensions, got %d", len(vector))
	}
	if vector[0] != 0.0 {
		t.Errorf("expected first element 0.0, got %f", vector[0])
	}
}

func TestEmbedTexts_MockAPI_Batch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.EmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		inputs := req.Input.([]any)
		data := make([]openai.Embedding, len(inputs))
		for i := range inputs {
			vec := make([]float32, 4)
			vec[0] = float32(i)
			data[i] = openai.Embedding{
				Object:    "embedding",
				Embedding: vec,
				Index:     i,
			}
		}

		resp := openai.EmbeddingResponse{
			Data:  data,
			Model: openai.SmallEmbedding3,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	vectors, err := EmbedTexts(context.Background(), client, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vectors) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(vectors))
	}
	for i, v := range vectors {
		if v[0] != float32(i) {
			t.Errorf("vector %d: expected first element %f, got %f", i, float32(i), v[0])
		}
	}
}

func TestEmbedTexts_Empty(t *testing.T) {
	vectors, err := EmbedTexts(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vectors != nil {
		t.Errorf("expected nil for empty input, got %v", vectors)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "429 rate limit",
			err:  &openai.APIError{HTTPStatusCode: 429},
			want: true,
		},
		{
			name: "500 server error",
			err:  &openai.APIError{HTTPStatusCode: 500},
			want: true,
		},
		{
			name: "503 service unavailable",
			err:  &openai.APIError{HTTPStatusCode: 503},
			want: true,
		},
		{
			name: "400 bad request",
			err:  &openai.APIError{HTTPStatusCode: 400},
			want: false,
		},
		{
			name: "401 unauthorized",
			err:  &openai.APIError{HTTPStatusCode: 401},
			want: false,
		},
		{
			name: "network error",
			err:  &openai.RequestError{HTTPStatusCode: 0},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalcBackoff(t *testing.T) {
	for attempt := range 5 {
		backoff := calcBackoff(attempt)
		if backoff <= 0 {
			t.Errorf("attempt %d: backoff should be positive, got %v", attempt, backoff)
		}
		if backoff > maxBackoff*2 {
			t.Errorf("attempt %d: backoff %v exceeds reasonable max", attempt, backoff)
		}
	}
}
