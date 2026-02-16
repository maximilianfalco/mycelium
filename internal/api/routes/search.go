package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SearchRoutes() chi.Router {
	r := chi.NewRouter()

	r.Post("/semantic", semanticSearch())
	r.Post("/structural", structuralSearch())

	return r
}

func semanticSearch() http.HandlerFunc {
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

		// Stubbed response
		writeJSON(w, http.StatusOK, []map[string]any{
			{"nodeId": "stub-1", "qualifiedName": "AuthService.login", "filePath": "src/auth/service.ts", "kind": "method", "similarity": 0.92, "signature": "login(email: string, password: string): Promise<User>"},
			{"nodeId": "stub-2", "qualifiedName": "validateToken", "filePath": "src/auth/token.ts", "kind": "function", "similarity": 0.87, "signature": "validateToken(token: string): TokenPayload"},
			{"nodeId": "stub-3", "qualifiedName": "UserRepository.findByEmail", "filePath": "src/users/repository.ts", "kind": "method", "similarity": 0.83, "signature": "findByEmail(email: string): Promise<User | null>"},
			{"nodeId": "stub-4", "qualifiedName": "hashPassword", "filePath": "src/auth/hash.ts", "kind": "function", "similarity": 0.79, "signature": "hashPassword(password: string): Promise<string>"},
			{"nodeId": "stub-5", "qualifiedName": "SessionManager", "filePath": "src/auth/session.ts", "kind": "class", "similarity": 0.75, "signature": "class SessionManager"},
		})
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

		// Stubbed response
		writeJSON(w, http.StatusOK, []map[string]any{
			{"nodeId": "stub-1", "qualifiedName": "AuthService.login", "filePath": "src/auth/service.ts", "kind": "method", "edgeKind": "calls", "target": "validateToken"},
			{"nodeId": "stub-2", "qualifiedName": "AuthController.handleLogin", "filePath": "src/auth/controller.ts", "kind": "method", "edgeKind": "calls", "target": "AuthService.login"},
			{"nodeId": "stub-3", "qualifiedName": "AuthService", "filePath": "src/auth/service.ts", "kind": "class", "edgeKind": "implements", "target": "IAuthService"},
		})
	}
}
