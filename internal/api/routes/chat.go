package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/engine"
)

func ChatRoutes(pool *pgxpool.Pool, oaiClient *openai.Client, cfg *config.Config) chi.Router {
	r := chi.NewRouter()

	r.Post("/", streamChat(pool, oaiClient, cfg))
	r.Get("/history", getChatHistory())

	return r
}

func streamChat(pool *pgxpool.Pool, oaiClient *openai.Client, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")

		var req struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Message == "" {
			writeError(w, http.StatusBadRequest, "message is required")
			return
		}
		if oaiClient == nil {
			writeError(w, http.StatusServiceUnavailable, "OpenAI API key not configured")
			return
		}

		result, err := engine.ChatStream(r.Context(), pool, oaiClient, req.Message, projectID, cfg.ChatModel, cfg.MaxContextTokens)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		// Stream deltas as SSE events
		_, streamErr := engine.ReadStreamDeltas(result.Stream, func(delta string) {
			data, _ := json.Marshal(map[string]string{"delta": delta})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		})

		if streamErr != nil {
			data, _ := json.Marshal(map[string]string{"error": streamErr.Error()})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		// Final event with sources
		done, _ := json.Marshal(map[string]any{
			"done":    true,
			"sources": result.Sources,
		})
		fmt.Fprintf(w, "data: %s\n\n", done)
		flusher.Flush()
	}
}

func getChatHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	}
}
