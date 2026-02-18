package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/engine"
)

func ChatRoutes(pool *pgxpool.Pool, oaiClient *openai.Client, cfg *config.Config) chi.Router {
	r := chi.NewRouter()

	r.Post("/", sendChat(pool, oaiClient, cfg))
	r.Get("/history", getChatHistory())

	return r
}

func sendChat(pool *pgxpool.Pool, oaiClient *openai.Client, cfg *config.Config) http.HandlerFunc {
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

		resp, err := engine.Chat(r.Context(), pool, oaiClient, req.Message, projectID, cfg.ChatModel, cfg.MaxContextTokens)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func getChatHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	}
}
