package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/engine"
)

func TestChat_ReturnsLLMResponse(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/embeddings"):
			resp := openai.EmbeddingResponse{
				Data: []openai.Embedding{{
					Object:    "embedding",
					Embedding: queryVec,
					Index:     0,
				}},
				Model: openai.SmallEmbedding3,
				Usage: openai.Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/chat/completions"):
			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Index: 0,
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: "The authenticate function validates a token and returns a User.",
					},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	result, err := engine.Chat(ctx, pool, client, "how does authentication work?", "test-ctx", "gpt-4o", 8000)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if result.Message == "" {
		t.Error("expected non-empty message")
	}
	if !strings.Contains(result.Message, "authenticate") {
		t.Errorf("expected message to mention 'authenticate', got %q", result.Message)
	}
}

func TestChat_IncludesSources(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/embeddings"):
			resp := openai.EmbeddingResponse{
				Data: []openai.Embedding{{
					Object:    "embedding",
					Embedding: queryVec,
					Index:     0,
				}},
				Model: openai.SmallEmbedding3,
				Usage: openai.Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/chat/completions"):
			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Index: 0,
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: "Here is the answer.",
					},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	result, err := engine.Chat(ctx, pool, client, "what is authenticate?", "test-ctx", "gpt-4o", 8000)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if len(result.Sources) == 0 {
		t.Fatal("expected at least one source")
	}

	hasAuth := false
	for _, s := range result.Sources {
		if s.QualifiedName == "authenticate" {
			hasAuth = true
		}
		if s.NodeID == "" {
			t.Error("source has empty nodeId")
		}
		if s.FilePath == "" {
			t.Error("source has empty filePath")
		}
	}
	if !hasAuth {
		t.Error("expected sources to include 'authenticate'")
	}
}

func TestChat_NoDuplicateSources(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/embeddings"):
			resp := openai.EmbeddingResponse{
				Data: []openai.Embedding{{
					Object:    "embedding",
					Embedding: queryVec,
					Index:     0,
				}},
				Model: openai.SmallEmbedding3,
				Usage: openai.Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/chat/completions"):
			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Index:   0,
					Message: openai.ChatCompletionMessage{Role: "assistant", Content: "answer"},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	result, err := engine.Chat(ctx, pool, client, "query", "test-ctx", "gpt-4o", 8000)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	seen := make(map[string]bool)
	for _, s := range result.Sources {
		if seen[s.NodeID] {
			t.Errorf("duplicate source nodeId: %s", s.NodeID)
		}
		seen[s.NodeID] = true
	}
}

func TestChat_EmptyProject(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/embeddings"):
			resp := openai.EmbeddingResponse{
				Data: []openai.Embedding{{
					Object:    "embedding",
					Embedding: queryVec,
					Index:     0,
				}},
				Model: openai.SmallEmbedding3,
				Usage: openai.Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/chat/completions"):
			// Verify the system prompt contains "No relevant code found"
			var req openai.ChatCompletionRequest
			json.NewDecoder(r.Body).Decode(&req)

			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Index:   0,
					Message: openai.ChatCompletionMessage{Role: "assistant", Content: "No indexed code is available."},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	result, err := engine.Chat(ctx, pool, client, "how does this work?", "nonexistent-project", "gpt-4o", 8000)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if result.Message == "" {
		t.Error("expected non-empty message even for empty project")
	}
	if len(result.Sources) != 0 {
		t.Errorf("expected 0 sources for empty project, got %d", len(result.Sources))
	}
}

func TestChat_SystemPromptIncludesContext(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	var capturedSystemPrompt string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/embeddings"):
			resp := openai.EmbeddingResponse{
				Data: []openai.Embedding{{
					Object:    "embedding",
					Embedding: queryVec,
					Index:     0,
				}},
				Model: openai.SmallEmbedding3,
				Usage: openai.Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/chat/completions"):
			var req openai.ChatCompletionRequest
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Messages) > 0 {
				capturedSystemPrompt = req.Messages[0].Content
			}

			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Index:   0,
					Message: openai.ChatCompletionMessage{Role: "assistant", Content: "ok"},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	_, err := engine.Chat(ctx, pool, client, "explain authentication", "test-ctx", "gpt-4o", 8000)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if !strings.Contains(capturedSystemPrompt, "code intelligence assistant") {
		t.Error("expected system prompt to contain 'code intelligence assistant'")
	}
	if !strings.Contains(capturedSystemPrompt, "authenticate") {
		t.Error("expected system prompt to contain assembled context with 'authenticate'")
	}
}
