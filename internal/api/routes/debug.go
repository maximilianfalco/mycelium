package routes

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func DebugRoutes() chi.Router {
	r := chi.NewRouter()

	r.Post("/crawl", debugCrawl())
	r.Post("/parse", debugParse())
	r.Post("/resolve", debugResolve())
	r.Post("/read-file", debugReadFile())
	r.Post("/embed-text", debugEmbedText())
	r.Post("/compare", debugCompare())
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
