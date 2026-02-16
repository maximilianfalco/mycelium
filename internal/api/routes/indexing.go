package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func IndexingRoutes(pool *pgxpool.Pool) chi.Router {
	r := chi.NewRouter()

	r.Post("/", triggerIndex(pool))
	r.Get("/status", getIndexStatus(pool))

	return r
}

func triggerIndex(_ *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":    "started",
			"projectId": projectID,
			"jobId":     "stub-job-001",
		})
	}
}

func getIndexStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")

		// Pull real counts from DB if available
		var nodeCount, edgeCount int
		_ = pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM nodes n
			 JOIN workspaces ws ON n.workspace_id = ws.id
			 WHERE ws.project_id = $1`, projectID,
		).Scan(&nodeCount)
		_ = pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM edges e
			 JOIN nodes n ON e.source_id = n.id
			 JOIN workspaces ws ON n.workspace_id = ws.id
			 WHERE ws.project_id = $1`, projectID,
		).Scan(&edgeCount)

		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "idle",
			"lastIndexedAt": nil,
			"nodeCount":     nodeCount,
			"edgeCount":     edgeCount,
		})
	}
}
