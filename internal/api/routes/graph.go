package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maximilianfalco/mycelium/internal/engine"
)

func getProjectGraph(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")

		data, err := engine.GetProjectGraph(r.Context(), pool, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

func getGraphNodeDetail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		nodeID := chi.URLParam(r, "nodeId")

		detail, err := engine.GetGraphNodeDetail(r.Context(), pool, projectID, nodeID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if detail == nil {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}

		writeJSON(w, http.StatusOK, detail)
	}
}
