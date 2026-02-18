package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/engine"
)

func SearchRoutes(pool *pgxpool.Pool, oaiClient *openai.Client) chi.Router {
	r := chi.NewRouter()

	r.Post("/semantic", semanticSearch(pool, oaiClient))
	r.Post("/structural", structuralSearch())

	return r
}

func semanticSearch(pool *pgxpool.Pool, oaiClient *openai.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string   `json:"query"`
			ProjectID string   `json:"projectId"`
			Limit     int      `json:"limit"`
			Kinds     []string `json:"kinds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Query == "" {
			writeError(w, http.StatusBadRequest, "query is required")
			return
		}
		if req.ProjectID == "" {
			writeError(w, http.StatusBadRequest, "projectId is required")
			return
		}
		if oaiClient == nil {
			writeError(w, http.StatusServiceUnavailable, "OpenAI API key not configured")
			return
		}

		results, err := engine.SemanticSearch(r.Context(), pool, oaiClient, req.Query, req.ProjectID, req.Limit, req.Kinds)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, results)
	}
}

func structuralSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string   `json:"query"`
			ProjectID string   `json:"projectId"`
			Limit     int      `json:"limit"`
			Kinds     []string `json:"kinds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Stubbed response — will be implemented in step 3.2
		writeJSON(w, http.StatusOK, []map[string]any{
			{"nodeId": "stub-1", "qualifiedName": "AuthService.login", "filePath": "src/auth/service.ts", "kind": "method", "edgeKind": "calls", "target": "validateToken"},
			{"nodeId": "stub-2", "qualifiedName": "AuthController.handleLogin", "filePath": "src/auth/controller.ts", "kind": "method", "edgeKind": "calls", "target": "AuthService.login"},
			{"nodeId": "stub-3", "qualifiedName": "AuthService", "filePath": "src/auth/service.ts", "kind": "class", "edgeKind": "implements", "target": "IAuthService"},
		})
	}
}
