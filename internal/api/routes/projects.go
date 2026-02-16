package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/projects"
)

func ProjectRoutes(pool *pgxpool.Pool) chi.Router {
	r := chi.NewRouter()

	r.Post("/", createProject(pool))
	r.Get("/", listProjects(pool))

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", getProject(pool))
		r.Put("/", updateProject(pool))
		r.Delete("/", deleteProject(pool))

		r.Post("/sources", addSource(pool))
		r.Get("/sources", listSources(pool))
		r.Delete("/sources/{sourceID}", removeSource(pool))

		r.Mount("/index", IndexingRoutes(pool))
		r.Mount("/chat", ChatRoutes())
	})

	return r
}

func createProject(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		p, err := projects.CreateProject(r.Context(), pool, req.Name, req.Description)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, p)
	}
}

func listProjects(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ps, err := projects.ListProjects(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ps == nil {
			ps = []projects.Project{}
		}
		writeJSON(w, http.StatusOK, ps)
	}
}

func getProject(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		p, err := projects.GetProject(r.Context(), pool, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if p == nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func updateProject(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		p, err := projects.UpdateProject(r.Context(), pool, id, req.Name, req.Description)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if p == nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func deleteProject(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := projects.DeleteProject(r.Context(), pool, id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func addSource(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		var req struct {
			Path       string `json:"path"`
			SourceType string `json:"sourceType"`
			IsCode     bool   `json:"isCode"`
			Alias      string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}
		if req.SourceType == "" {
			req.SourceType = "directory"
		}

		s, err := projects.AddSource(r.Context(), pool, projectID, req.Path, req.SourceType, req.IsCode, req.Alias)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, s)
	}
}

func listSources(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		sources, err := projects.ListSources(r.Context(), pool, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sources == nil {
			sources = []projects.ProjectSource{}
		}
		writeJSON(w, http.StatusOK, sources)
	}
}

func removeSource(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		sourceID := chi.URLParam(r, "sourceID")
		if err := projects.RemoveSource(r.Context(), pool, projectID, sourceID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
