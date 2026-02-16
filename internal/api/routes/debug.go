package routes

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func DebugRoutes() chi.Router {
	r := chi.NewRouter()

	r.Post("/crawl", debugCrawl())
	r.Post("/parse", debugParse())
	r.Post("/embed-text", debugEmbedText())
	r.Post("/compare", debugCompare())
	r.Post("/workspace", debugWorkspace())
	r.Post("/changes", debugChanges())

	return r
}

func debugCrawl() http.HandlerFunc {
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

		mockFiles := []map[string]any{
			{"absPath": req.Path + "/src/index.ts", "relPath": "src/index.ts", "extension": ".ts", "sizeBytes": 1240},
			{"absPath": req.Path + "/src/utils/helpers.ts", "relPath": "src/utils/helpers.ts", "extension": ".ts", "sizeBytes": 890},
			{"absPath": req.Path + "/src/components/Button.tsx", "relPath": "src/components/Button.tsx", "extension": ".tsx", "sizeBytes": 2100},
			{"absPath": req.Path + "/src/components/Card.tsx", "relPath": "src/components/Card.tsx", "extension": ".tsx", "sizeBytes": 1560},
			{"absPath": req.Path + "/src/lib/api.ts", "relPath": "src/lib/api.ts", "extension": ".ts", "sizeBytes": 3400},
			{"absPath": req.Path + "/package.json", "relPath": "package.json", "extension": ".json", "sizeBytes": 720},
			{"absPath": req.Path + "/tsconfig.json", "relPath": "tsconfig.json", "extension": ".json", "sizeBytes": 340},
			{"absPath": req.Path + "/README.md", "relPath": "README.md", "extension": ".md", "sizeBytes": 1800},
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"files": mockFiles,
			"stats": map[string]any{
				"total":   8,
				"skipped": 42,
				"byExtension": map[string]int{
					".ts":   3,
					".tsx":  2,
					".json": 2,
					".md":   1,
				},
			},
		})
	}
}

func debugParse() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FilePath string `json:"filePath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.FilePath == "" {
			writeError(w, http.StatusBadRequest, "filePath is required")
			return
		}

		fileName := filepath.Base(req.FilePath)
		ext := filepath.Ext(req.FilePath)
		baseName := strings.TrimSuffix(fileName, ext)

		mockNodes := []map[string]any{
			{
				"name":          baseName,
				"qualifiedName": "src/" + baseName,
				"kind":          "module",
				"signature":     "",
				"startLine":     1,
				"endLine":       45,
				"sourceCode":    "// module: " + baseName,
				"docstring":     "Main module for " + baseName,
				"bodyHash":      "a1b2c3d4e5f6",
			},
			{
				"name":          "calculate",
				"qualifiedName": "src/" + baseName + ".calculate",
				"kind":          "function",
				"signature":     "function calculate(a: number, b: number): number",
				"startLine":     5,
				"endLine":       12,
				"sourceCode":    "function calculate(a: number, b: number): number {\n  return a + b;\n}",
				"docstring":     "Calculates the sum of two numbers",
				"bodyHash":      "f6e5d4c3b2a1",
			},
			{
				"name":          "Config",
				"qualifiedName": "src/" + baseName + ".Config",
				"kind":          "interface",
				"signature":     "interface Config",
				"startLine":     14,
				"endLine":       20,
				"sourceCode":    "interface Config {\n  debug: boolean;\n  verbose: boolean;\n}",
				"docstring":     "",
				"bodyHash":      "1a2b3c4d5e6f",
			},
		}

		mockEdges := []map[string]any{
			{"source": "src/" + baseName + ".calculate", "target": "src/utils/helpers.add", "kind": "calls"},
			{"source": "src/" + baseName, "target": "src/utils/helpers", "kind": "imports"},
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": mockNodes,
			"edges": mockEdges,
			"stats": map[string]any{
				"nodeCount": len(mockNodes),
				"edgeCount": len(mockEdges),
				"byKind": map[string]int{
					"module":    1,
					"function":  1,
					"interface": 1,
				},
			},
		})
	}
}

func debugEmbedText() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Text == "" {
			writeError(w, http.StatusBadRequest, "text is required")
			return
		}

		dims := 1536
		vector := make([]float64, dims)
		for i := range vector {
			vector[i] = math.Round((rand.Float64()*2-1)*10000) / 10000
		}

		tokenCount := len(strings.Fields(req.Text)) + len(req.Text)/4

		writeJSON(w, http.StatusOK, map[string]any{
			"vector":     vector[:8],
			"dimensions": dims,
			"tokenCount": tokenCount,
			"model":      "text-embedding-3-small",
			"truncated":  false,
		})
	}
}

func debugCompare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text1 string `json:"text1"`
			Text2 string `json:"text2"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Text1 == "" || req.Text2 == "" {
			writeError(w, http.StatusBadRequest, "text1 and text2 are required")
			return
		}

		// Similarity based on shared words for mock realism
		words1 := strings.Fields(strings.ToLower(req.Text1))
		words2 := strings.Fields(strings.ToLower(req.Text2))
		wordSet := make(map[string]bool)
		for _, w := range words1 {
			wordSet[w] = true
		}
		shared := 0
		for _, w := range words2 {
			if wordSet[w] {
				shared++
			}
		}
		total := len(words1) + len(words2)
		similarity := 0.0
		if total > 0 {
			similarity = math.Round(float64(shared*2)/float64(total)*10000) / 10000
		}

		tokenCount1 := len(words1) + len(req.Text1)/4
		tokenCount2 := len(words2) + len(req.Text2)/4

		writeJSON(w, http.StatusOK, map[string]any{
			"similarity":  similarity,
			"tokenCount1": tokenCount1,
			"tokenCount2": tokenCount2,
			"dimensions":  1536,
		})
	}
}

func debugWorkspace() http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, map[string]any{
			"workspaceType":  "monorepo",
			"packageManager": "pnpm",
			"packages": []map[string]any{
				{"name": "@mycelium/core", "path": "packages/core", "version": "0.1.0"},
				{"name": "@mycelium/web", "path": "apps/web", "version": "0.1.0"},
				{"name": "@mycelium/cli", "path": "packages/cli", "version": "0.1.0"},
			},
			"aliasMap": map[string]string{
				"@core/*": "packages/core/src/*",
				"@web/*":  "apps/web/src/*",
			},
			"tsconfigPaths": map[string]string{
				"@/*": "src/*",
			},
		})
	}
}

func debugChanges() http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, map[string]any{
			"isGitRepo":         true,
			"currentCommit":     "a1b2c3d",
			"lastIndexedCommit": "e4f5g6h",
			"isFullIndex":       false,
			"addedFiles":        []string{"src/new-feature.ts", "src/components/Modal.tsx"},
			"modifiedFiles":     []string{"src/index.ts", "src/lib/api.ts", "package.json"},
			"deletedFiles":      []string{"src/deprecated.ts"},
			"thresholdExceeded": false,
		})
	}
}
