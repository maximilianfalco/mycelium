package routes

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/indexer"
)

var statusStore = indexer.NewStatusStore()

func IndexingRoutes(pool *pgxpool.Pool, cfg *config.Config) chi.Router {
	r := chi.NewRouter()

	var oaiClient *openai.Client
	if cfg.OpenAIAPIKey != "" {
		oaiClient = openai.NewClient(cfg.OpenAIAPIKey)
	}

	r.Post("/", triggerIndex(pool, cfg, oaiClient))
	r.Get("/status", getIndexStatus(pool))

	return r
}

func triggerIndex(pool *pgxpool.Pool, cfg *config.Config, oaiClient *openai.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")

		// Check if already running
		existing := statusStore.GetByProject(projectID)
		if existing != nil && existing.Status == "running" {
			writeError(w, http.StatusConflict, "indexing already in progress for this project")
			return
		}

		jobID := fmt.Sprintf("idx-%s-%d", projectID, time.Now().UnixMilli())
		status := &indexer.IndexStatus{
			JobID:     jobID,
			ProjectID: projectID,
			Status:    "running",
			Stage:     "starting",
			StartedAt: time.Now(),
		}
		statusStore.Set(jobID, status)

		// Run indexing in background — use a detached context so the job
		// isn't cancelled when the HTTP response is sent.
		go func() {
			result := indexer.IndexProject(context.Background(), pool, cfg, oaiClient, projectID, status)
			now := time.Now()
			status.DoneAt = &now
			status.Result = result
			if len(result.Errors) > 0 {
				status.Status = "failed"
				status.Error = result.Errors[0]
			} else {
				status.Status = "completed"
			}
			status.Stage = "done"
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":    "started",
			"projectId": projectID,
			"jobId":     jobID,
		})
	}
}

func getIndexStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")

		// Check in-memory status first
		job := statusStore.GetByProject(projectID)

		// Pull real counts from DB
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

		// Get last indexed time from sources
		var lastIndexedAt *time.Time
		_ = pool.QueryRow(r.Context(),
			`SELECT MAX(last_indexed_at) FROM project_sources WHERE project_id = $1`, projectID,
		).Scan(&lastIndexedAt)

		resp := map[string]any{
			"nodeCount":     nodeCount,
			"edgeCount":     edgeCount,
			"lastIndexedAt": lastIndexedAt,
		}

		if job != nil {
			resp["status"] = job.Status
			resp["jobId"] = job.JobID
			resp["stage"] = job.Stage
			resp["progress"] = job.Progress
			resp["startedAt"] = job.StartedAt
			resp["doneAt"] = job.DoneAt
			if job.Result != nil {
				resp["result"] = job.Result
			}
			if job.Error != "" {
				resp["error"] = job.Error
			}
		} else {
			resp["status"] = "idle"
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
