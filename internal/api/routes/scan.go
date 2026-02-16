package routes

import (
	"encoding/json"
	"net/http"

	"github.com/maximilianfalco/mycelium/internal/projects"
)

func ScanHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}

		results, err := projects.ScanDirectory(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if results == nil {
			results = []projects.ScanResult{}
		}
		writeJSON(w, http.StatusOK, results)
	}
}
