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
	r.Post("/structural", structuralSearch(pool))

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

func structuralSearch(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string   `json:"query"`
			ProjectID string   `json:"projectId"`
			Limit     int      `json:"limit"`
			Kinds     []string `json:"kinds"`
			QueryType string   `json:"queryType"`
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

		// Look up the node by qualified name
		node, err := engine.FindNodeByQualifiedName(r.Context(), pool, req.ProjectID, req.Query)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if node == nil {
			writeJSON(w, http.StatusOK, []engine.NodeResult{})
			return
		}

		queryType := req.QueryType
		if queryType == "" {
			queryType = "callers"
		}

		var results []engine.NodeResult
		switch queryType {
		case "callers":
			results, err = engine.GetCallers(r.Context(), pool, node.NodeID, req.Limit)
		case "callees":
			results, err = engine.GetCallees(r.Context(), pool, node.NodeID, req.Limit)
		case "importers":
			results, err = engine.GetImporters(r.Context(), pool, node.NodeID, req.Limit)
		case "dependencies":
			results, err = engine.GetDependencies(r.Context(), pool, node.NodeID, 5, req.Limit)
		case "dependents":
			results, err = engine.GetDependents(r.Context(), pool, node.NodeID, 5, req.Limit)
		case "file":
			results, err = engine.GetFileContext(r.Context(), pool, node.FilePath, req.ProjectID)
		default:
			writeError(w, http.StatusBadRequest, "unknown queryType: "+queryType)
			return
		}

		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Apply kind filter if specified
		if len(req.Kinds) > 0 {
			kindSet := make(map[string]bool, len(req.Kinds))
			for _, k := range req.Kinds {
				kindSet[k] = true
			}
			filtered := make([]engine.NodeResult, 0, len(results))
			for _, nr := range results {
				if kindSet[nr.Kind] {
					filtered = append(filtered, nr)
				}
			}
			results = filtered
		}

		writeJSON(w, http.StatusOK, results)
	}
}
