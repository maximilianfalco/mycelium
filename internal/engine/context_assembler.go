package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/indexer"
)

// ContextNode represents a node selected for inclusion in the assembled context.
type ContextNode struct {
	NodeID        string   `json:"nodeId"`
	QualifiedName string   `json:"qualifiedName"`
	FilePath      string   `json:"filePath"`
	Kind          string   `json:"kind"`
	Signature     string   `json:"signature"`
	SourceCode    string   `json:"sourceCode,omitempty"`
	Docstring     string   `json:"docstring,omitempty"`
	Similarity    float64  `json:"similarity"`
	Score         float64  `json:"score"`
	CalledBy   []string `json:"calledBy,omitempty"`
	Calls      []string `json:"calls,omitempty"`
	ImportedBy []string `json:"importedBy,omitempty"`
	Imports    []string `json:"imports,omitempty"`
	FullSource    bool     `json:"fullSource"`
	SourceAlias   string   `json:"sourceAlias,omitempty"`
}

// AssembledContext is the result of combining semantic + structural search
// into a token-budgeted context window for an LLM.
type AssembledContext struct {
	Nodes      []ContextNode `json:"nodes"`
	Text       string        `json:"text"`
	TokenCount int           `json:"tokenCount"`
	TokenLimit int           `json:"tokenLimit"`
}

// AssembleContext runs semantic search, expands results via graph traversal,
// deduplicates, ranks, and produces a formatted context string within the
// given token budget.
func AssembleContext(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, maxTokens int) (*AssembledContext, error) {
	if maxTokens <= 0 {
		maxTokens = 8000
	}

	nodeCount := getProjectNodeCount(ctx, pool, projectID)
	searchLimit := dynamicSearchLimit(nodeCount)

	semanticResults, err := HybridSearch(ctx, pool, client, query, projectID, searchLimit, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	if len(semanticResults) == 0 {
		return &AssembledContext{
			Nodes:      []ContextNode{},
			Text:       "No relevant code found.",
			TokenCount: 0,
			TokenLimit: maxTokens,
		}, nil
	}

	return assembleFromResults(ctx, pool, semanticResults, maxTokens)
}

// AssembleContextWithVector is like AssembleContext but uses a pre-computed
// query vector instead of calling the OpenAI API. Useful for testing.
func AssembleContextWithVector(ctx context.Context, pool *pgxpool.Pool, queryVec []float32, projectID string, maxTokens int) (*AssembledContext, error) {
	if maxTokens <= 0 {
		maxTokens = 8000
	}

	semanticResults, err := SemanticSearchWithVector(ctx, pool, queryVec, projectID, 10, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	if len(semanticResults) == 0 {
		return &AssembledContext{
			Nodes:      []ContextNode{},
			Text:       "No relevant code found.",
			TokenCount: 0,
			TokenLimit: maxTokens,
		}, nil
	}

	return assembleFromResults(ctx, pool, semanticResults, maxTokens)
}

func getProjectNodeCount(ctx context.Context, pool *pgxpool.Pool, projectID string) int {
	var count int
	pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM nodes n JOIN workspaces w ON n.workspace_id = w.id WHERE w.project_id = $1`,
		projectID,
	).Scan(&count)
	return count
}

// dynamicSearchLimit scales the number of search results based on colony size.
func dynamicSearchLimit(nodeCount int) int {
	switch {
	case nodeCount >= 15000:
		return 25
	case nodeCount >= 5000:
		return 20
	case nodeCount >= 1000:
		return 15
	default:
		return 10
	}
}

// scoredNode tracks a discovered node with its provenance for ranking.
type scoredNode struct {
	nodeID        string
	qualifiedName string
	filePath      string
	kind          string
	signature     string
	sourceCode    string
	docstring     string
	similarity    float64
	weight        float64
	sourceAlias   string
}

// assembleFromResults is the shared core: expands semantic results via graph,
// deduplicates, ranks by combined score, and assembles the token-budgeted output.
func assembleFromResults(ctx context.Context, pool *pgxpool.Pool, semanticResults []SearchResult, maxTokens int) (*AssembledContext, error) {
	seen := make(map[string]*scoredNode)

	// Step 1: Seed with semantic hits (weight 1.0) and expand via graph
	for _, sr := range semanticResults {
		if existing, ok := seen[sr.NodeID]; ok {
			if sr.Similarity > existing.similarity {
				existing.similarity = sr.Similarity
			}
		} else {
			seen[sr.NodeID] = &scoredNode{
				nodeID:        sr.NodeID,
				qualifiedName: sr.QualifiedName,
				filePath:      sr.FilePath,
				kind:          sr.Kind,
				signature:     sr.Signature,
				sourceCode:    sr.SourceCode,
				docstring:     sr.Docstring,
				similarity:    sr.Similarity,
				weight:        1.0,
				sourceAlias:   sr.SourceAlias,
			}
		}

		// Hop 1: outgoing dependencies (calls, imports, uses_type) — top 5
		hop1, _ := GetDependencies(ctx, pool, sr.NodeID, 1, 5)
		for _, n := range hop1 {
			addOrUpdate(seen, n, sr.Similarity, 0.7)

			// Hop 2: one more hop from hop-1 nodes — top 3, reduced fan-out
			hop2, _ := GetDependencies(ctx, pool, n.NodeID, 1, 3)
			for _, n2 := range hop2 {
				addOrUpdate(seen, n2, sr.Similarity, 0.4)
			}
		}

		// Reverse hop: who imports/calls/uses this node? — top 3
		// Critical for cross-repo questions (e.g., finding consumers of a library)
		dependents, _ := GetDependents(ctx, pool, sr.NodeID, 1, 3)
		for _, n := range dependents {
			addOrUpdate(seen, n, sr.Similarity, 0.6)
		}
	}

	// Step 2: Rank by combined score (similarity × weight)
	type rankedNode struct {
		scoredNode
		score float64
	}

	ranked := make([]rankedNode, 0, len(seen))
	for _, sn := range seen {
		ranked = append(ranked, rankedNode{
			scoredNode: *sn,
			score:      sn.similarity * sn.weight,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].qualifiedName < ranked[j].qualifiedName
	})

	// Step 3: Fetch relationship annotations for the top nodes
	annotationLimit := 20
	if len(ranked) < annotationLimit {
		annotationLimit = len(ranked)
	}

	type annotations struct {
		calledBy   []string
		calls      []string
		importedBy []string
		imports    []string
	}
	nodeAnnotations := make(map[string]*annotations)

	for i := 0; i < annotationLimit; i++ {
		nodeID := ranked[i].nodeID
		callers, _ := GetCallers(ctx, pool, nodeID, 3)
		callees, _ := GetCallees(ctx, pool, nodeID, 3)
		importers, _ := GetImporters(ctx, pool, nodeID, 3)
		imported, _ := getRelated(ctx, pool, nodeID, "imports", "outgoing", 3)

		ann := &annotations{}
		for _, c := range callers {
			ann.calledBy = append(ann.calledBy, nodeLabel(c))
		}
		for _, c := range callees {
			ann.calls = append(ann.calls, nodeLabel(c))
		}
		for _, c := range importers {
			ann.importedBy = append(ann.importedBy, nodeLabel(c))
		}
		for _, c := range imported {
			ann.imports = append(ann.imports, nodeLabel(c))
		}
		nodeAnnotations[nodeID] = ann
	}

	// Step 4: Greedy token-budgeted assembly
	contextNodes := []ContextNode{}
	totalTokens := 0
	headerTokens := 20 // "## Relevant Code\n\n" overhead

	for i, rn := range ranked {
		fullSource := i < 5

		node := ContextNode{
			NodeID:        rn.nodeID,
			QualifiedName: rn.qualifiedName,
			FilePath:      rn.filePath,
			Kind:          rn.kind,
			Signature:     rn.signature,
			Docstring:     rn.docstring,
			Similarity:    rn.similarity,
			Score:         rn.score,
			FullSource:    fullSource,
			SourceAlias:   rn.sourceAlias,
		}

		if ann, ok := nodeAnnotations[rn.nodeID]; ok {
			node.CalledBy = ann.calledBy
			node.Calls = ann.calls
			node.ImportedBy = ann.importedBy
			node.Imports = ann.imports
		}

		if fullSource {
			node.SourceCode = rn.sourceCode
		}

		formatted := formatNode(node)
		nodeTokens, err := indexer.CountTokens(formatted)
		if err != nil {
			nodeTokens = len(formatted) / 4
		}

		if totalTokens+headerTokens+nodeTokens > maxTokens {
			// Try signature-only if we were planning full source
			if fullSource && rn.sourceCode != "" {
				node.SourceCode = ""
				node.FullSource = false
				formatted = formatNode(node)
				nodeTokens, err = indexer.CountTokens(formatted)
				if err != nil {
					nodeTokens = len(formatted) / 4
				}
				if totalTokens+headerTokens+nodeTokens > maxTokens {
					break
				}
			} else {
				break
			}
		}

		totalTokens += nodeTokens
		contextNodes = append(contextNodes, node)
	}

	// Step 5: Format the full context string
	text := formatContext(contextNodes)
	finalTokens, err := indexer.CountTokens(text)
	if err != nil {
		finalTokens = totalTokens + headerTokens
	}

	return &AssembledContext{
		Nodes:      contextNodes,
		Text:       text,
		TokenCount: finalTokens,
		TokenLimit: maxTokens,
	}, nil
}

func nodeLabel(n NodeResult) string {
	if n.SourceAlias != "" {
		return n.QualifiedName + " [" + n.SourceAlias + "]"
	}
	return n.QualifiedName
}

// addOrUpdate inserts a graph-discovered node into the seen map, or updates
// it if the new combined score is higher.
func addOrUpdate(seen map[string]*scoredNode, n NodeResult, similarity, weight float64) {
	combined := similarity * weight
	if existing, ok := seen[n.NodeID]; ok {
		if combined > existing.similarity*existing.weight {
			existing.similarity = similarity
			existing.weight = weight
		}
	} else {
		seen[n.NodeID] = &scoredNode{
			nodeID:        n.NodeID,
			qualifiedName: n.QualifiedName,
			filePath:      n.FilePath,
			kind:          n.Kind,
			signature:     n.Signature,
			sourceCode:    n.SourceCode,
			docstring:     n.Docstring,
			similarity:    similarity,
			weight:        weight,
			sourceAlias:   n.SourceAlias,
		}
	}
}

func formatContext(nodes []ContextNode) string {
	if len(nodes) == 0 {
		return "No relevant code found."
	}

	// Group nodes by source alias for clear repo boundaries
	groups := make(map[string][]ContextNode)
	var order []string
	for _, n := range nodes {
		alias := n.SourceAlias
		if alias == "" {
			alias = "(unknown)"
		}
		if _, exists := groups[alias]; !exists {
			order = append(order, alias)
		}
		groups[alias] = append(groups[alias], n)
	}

	var b strings.Builder
	b.WriteString("## Relevant Code\n\n")

	if len(order) == 1 && order[0] == "(unknown)" {
		// Single source or no aliases — flat list (backwards compatible)
		for _, n := range nodes {
			b.WriteString(formatNode(n))
			b.WriteString("\n")
		}
	} else {
		for _, alias := range order {
			b.WriteString(fmt.Sprintf("## Source: %s\n\n", alias))
			for _, n := range groups[alias] {
				b.WriteString(formatNode(n))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func formatNode(n ContextNode) string {
	var b strings.Builder

	if n.SourceAlias != "" {
		b.WriteString(fmt.Sprintf("### %s — %s [source: %s] (similarity: %.2f)\n", n.FilePath, n.QualifiedName, n.SourceAlias, n.Similarity))
	} else {
		b.WriteString(fmt.Sprintf("### %s — %s (similarity: %.2f)\n", n.FilePath, n.QualifiedName, n.Similarity))
	}
	b.WriteString(fmt.Sprintf("Signature: %s\n", n.Signature))

	if n.Docstring != "" {
		b.WriteString(fmt.Sprintf("Docstring: %s\n", n.Docstring))
	}

	if len(n.ImportedBy) > 0 {
		b.WriteString(fmt.Sprintf("Imported by: %s\n", strings.Join(n.ImportedBy, ", ")))
	}
	if len(n.Imports) > 0 {
		b.WriteString(fmt.Sprintf("Imports: %s\n", strings.Join(n.Imports, ", ")))
	}
	if len(n.CalledBy) > 0 {
		b.WriteString(fmt.Sprintf("Called by: %s\n", strings.Join(n.CalledBy, ", ")))
	}
	if len(n.Calls) > 0 {
		b.WriteString(fmt.Sprintf("Calls: %s\n", strings.Join(n.Calls, ", ")))
	}

	if n.FullSource && n.SourceCode != "" {
		b.WriteString(fmt.Sprintf("\n```\n%s\n```\n", n.SourceCode))
	}

	return b.String()
}
