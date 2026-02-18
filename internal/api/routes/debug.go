package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
	"github.com/sashabaranov/go-openai"
)

func DebugRoutes(cfg *config.Config) chi.Router {
	r := chi.NewRouter()

	var oaiClient *openai.Client
	if cfg.OpenAIAPIKey != "" {
		oaiClient = openai.NewClient(cfg.OpenAIAPIKey)
	}

	r.Post("/crawl", debugCrawl())
	r.Post("/parse", debugParse())
	r.Post("/resolve", debugResolve())
	r.Post("/read-file", debugReadFile())
	r.Post("/embed-text", debugEmbedText(oaiClient))
	r.Post("/compare", debugCompare(oaiClient))
	r.Post("/workspace", debugWorkspace())
	r.Post("/changes", debugChanges())

	return r
}

func debugCrawl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path          string `json:"path"`
			MaxFileSizeKB int    `json:"maxFileSizeKB"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}

		result, err := indexer.CrawlDirectory(req.Path, true, req.MaxFileSizeKB)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, result)
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

		source, err := os.ReadFile(req.FilePath)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("reading file: %v", err))
			return
		}

		result, err := parsers.ParseFile(req.FilePath, source)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parsing file: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": result.Nodes,
			"edges": result.Edges,
			"stats": result.Stats(),
		})
	}
}

func debugResolve() http.HandlerFunc {
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

		// 1. Detect workspace
		wsInfo, err := detectors.DetectWorkspace(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("workspace detection: %v", err))
			return
		}

		// 2. Crawl all code files
		crawlResult, err := indexer.CrawlDirectory(req.Path, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("crawling: %v", err))
			return
		}

		// 3. Parse all files and collect nodes + edges
		var allNodes []parsers.NodeInfo
		var allEdges []parsers.EdgeInfo
		var allFiles []string
		var parseErrors []string

		for _, f := range crawlResult.Files {
			allFiles = append(allFiles, f.RelPath)
			source, err := os.ReadFile(f.AbsPath)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", f.RelPath, err))
				continue
			}
			result, err := parsers.ParseFile(f.AbsPath, source)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", f.RelPath, err))
				continue
			}
			// Rewrite edge/node sources to use relative paths
			for _, n := range result.Nodes {
				allNodes = append(allNodes, n)
			}
			for _, e := range result.Edges {
				// Rewrite absolute file paths in edges to relative
				if e.Kind == "imports" || e.Kind == "contains" {
					if strings.HasPrefix(e.Source, "/") {
						if rel, err := filepath.Rel(req.Path, e.Source); err == nil {
							e.Source = rel
						}
					}
				}
				allEdges = append(allEdges, e)
			}
		}

		// 4. Resolve imports
		resolveResult := indexer.ResolveImports(
			allEdges,
			wsInfo.AliasMap,
			wsInfo.TSConfigPaths,
			allNodes,
			allFiles,
			req.Path,
		)

		writeJSON(w, http.StatusOK, map[string]any{
			"workspace":   wsInfo,
			"filesCount":  len(allFiles),
			"nodesCount":  len(allNodes),
			"edgesCount":  len(allEdges),
			"resolved":    resolveResult.Resolved,
			"unresolved":  resolveResult.Unresolved,
			"dependsOn":   resolveResult.DependsOn,
			"parseErrors": parseErrors,
		})
	}
}

func debugReadFile() http.HandlerFunc {
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

		content, err := os.ReadFile(req.FilePath)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("reading file: %v", err))
			return
		}

		ext := strings.TrimPrefix(filepath.Ext(req.FilePath), ".")
		lines := strings.Count(string(content), "\n") + 1

		writeJSON(w, http.StatusOK, map[string]any{
			"content":   string(content),
			"language":  ext,
			"lineCount": lines,
		})
	}
}

func debugEmbedText(client *openai.Client) http.HandlerFunc {
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
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "OPENAI_API_KEY not configured")
			return
		}

		tokenCount, err := indexer.CountTokens(req.Text)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("counting tokens: %v", err))
			return
		}

		chunk, err := indexer.PrepareEmbeddingInput("", "", req.Text)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("preparing input: %v", err))
			return
		}

		vector, err := indexer.EmbedText(r.Context(), client, chunk.Text)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("embedding: %v", err))
			return
		}

		// Return first 8 dimensions for display (same shape as before)
		preview := make([]float32, 8)
		if len(vector) >= 8 {
			copy(preview, vector[:8])
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"vector":     preview,
			"dimensions": len(vector),
			"tokenCount": tokenCount,
			"model":      "text-embedding-3-small",
			"truncated":  chunk.Truncated,
		})
	}
}

func debugCompare(client *openai.Client) http.HandlerFunc {
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
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, "OPENAI_API_KEY not configured")
			return
		}

		vectors, err := indexer.EmbedTexts(r.Context(), client, []string{req.Text1, req.Text2})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("embedding: %v", err))
			return
		}
		if len(vectors) != 2 {
			writeError(w, http.StatusInternalServerError, "expected 2 vectors from API")
			return
		}

		similarity := indexer.CosineSimilarity(vectors[0], vectors[1])

		tokenCount1, _ := indexer.CountTokens(req.Text1)
		tokenCount2, _ := indexer.CountTokens(req.Text2)

		writeJSON(w, http.StatusOK, map[string]any{
			"similarity":  similarity,
			"tokenCount1": tokenCount1,
			"tokenCount2": tokenCount2,
			"dimensions":  len(vectors[0]),
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

		result, err := detectors.DetectWorkspace(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, result)
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
