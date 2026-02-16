package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func ChatRoutes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", sendChat())
	r.Get("/history", getChatHistory())

	return r
}

func sendChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message": "Placeholder response. Indexing engine not yet implemented. Your question was: " + req.Message,
			"sources": []any{},
		})
	}
}

func getChatHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	}
}
