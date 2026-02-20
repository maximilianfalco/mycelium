package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/engine"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func setupContextTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, pool := setupGraphTest(t)

	projectID := "test-ctx"
	createTestProject(t, ctx, pool, projectID)
	createTestSource(t, ctx, pool, projectID+"/src", projectID, "/tmp/test-ctx")

	// Build a small graph:
	//   authenticate (dim 0) --calls--> verifyPassword (dim 3)
	//   authenticate --calls--> generateToken (dim 4)
	//   queryUsers (dim 1) --calls--> authenticate
	//   Logger (dim 2) — no edges
	authVec := makeUnitVector(1536, 0)
	dbVec := makeUnitVector(1536, 1)
	logVec := makeUnitVector(1536, 2)
	verifyVec := makeUnitVector(1536, 3)
	tokenVec := makeUnitVector(1536, 4)

	input := &indexer.BuildInput{
		ProjectID:  projectID,
		SourceID:   projectID + "/src",
		SourcePath: "/tmp/test-ctx",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "app", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "authenticate",
				QualifiedName: "authenticate",
				Kind:          "function",
				Signature:     "function authenticate(token: string): User",
				StartLine:     1,
				EndLine:       5,
				SourceCode:    "function authenticate(token: string): User { return verify(token); }",
				BodyHash:      "auth-hash",
			},
			{
				Name:          "queryUsers",
				QualifiedName: "queryUsers",
				Kind:          "function",
				Signature:     "function queryUsers(filter: Filter): User[]",
				StartLine:     1,
				EndLine:       5,
				SourceCode:    "function queryUsers(filter: Filter): User[] { return db.find(filter); }",
				BodyHash:      "db-hash",
			},
			{
				Name:          "Logger",
				QualifiedName: "Logger",
				Kind:          "class",
				Signature:     "class Logger",
				StartLine:     1,
				EndLine:       10,
				SourceCode:    "class Logger { log(msg: string) { console.log(msg); } }",
				BodyHash:      "log-hash",
			},
			{
				Name:          "verifyPassword",
				QualifiedName: "verifyPassword",
				Kind:          "function",
				Signature:     "function verifyPassword(pw: string, hash: string): boolean",
				StartLine:     1,
				EndLine:       3,
				SourceCode:    "function verifyPassword(pw: string, hash: string): boolean { return bcrypt.compare(pw, hash); }",
				BodyHash:      "verify-hash",
			},
			{
				Name:          "generateToken",
				QualifiedName: "generateToken",
				Kind:          "function",
				Signature:     "function generateToken(user: User): string",
				StartLine:     1,
				EndLine:       3,
				SourceCode:    "function generateToken(user: User): string { return jwt.sign(user); }",
				BodyHash:      "token-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/auth.ts", Target: "authenticate", Kind: "contains", Line: 1},
			{Source: "src/db.ts", Target: "queryUsers", Kind: "contains", Line: 1},
			{Source: "src/logger.ts", Target: "Logger", Kind: "contains", Line: 1},
			{Source: "src/crypto.ts", Target: "verifyPassword", Kind: "contains", Line: 1},
			{Source: "src/token.ts", Target: "generateToken", Kind: "contains", Line: 1},
		},
		Resolved: []indexer.ResolvedEdge{
			{Source: "authenticate", Target: "verifyPassword", Kind: "calls", Line: 3},
			{Source: "authenticate", Target: "generateToken", Kind: "calls", Line: 4},
			{Source: "queryUsers", Target: "authenticate", Kind: "calls", Line: 3},
		},
		Embeddings: map[string][]float32{
			"authenticate":   authVec,
			"queryUsers":     dbVec,
			"Logger":         logVec,
			"verifyPassword": verifyVec,
			"generateToken":  tokenVec,
		},
		FilePaths: []string{"src/auth.ts", "src/db.ts", "src/logger.ts", "src/crypto.ts", "src/token.ts"},
	}

	_, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	return ctx, pool
}

func TestAssembleContext_BasicAssembly(t *testing.T) {
	ctx, pool := setupContextTest(t)

	// Query with a vector close to "authenticate" (dim 0)
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Fatal("expected at least one node in assembled context")
	}

	// The top result should be "authenticate" (exact match on dimension 0)
	if result.Nodes[0].QualifiedName != "authenticate" {
		t.Errorf("expected top node to be 'authenticate', got %q", result.Nodes[0].QualifiedName)
	}
	if result.Nodes[0].Score < 0.9 {
		t.Errorf("expected top score >= 0.9, got %f", result.Nodes[0].Score)
	}
}

func TestAssembleContext_GraphExpansion(t *testing.T) {
	ctx, pool := setupContextTest(t)

	// Query for "authenticate" — should expand to include verifyPassword and generateToken via edges
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	names := make(map[string]bool)
	for _, n := range result.Nodes {
		names[n.QualifiedName] = true
	}

	// authenticate calls verifyPassword and generateToken — they should appear via graph expansion
	if !names["verifyPassword"] {
		t.Error("expected graph expansion to include 'verifyPassword' (callee of authenticate)")
	}
	if !names["generateToken"] {
		t.Error("expected graph expansion to include 'generateToken' (callee of authenticate)")
	}
}

func TestAssembleContext_ScoreRanking(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	// Scores should be monotonically non-increasing
	for i := 1; i < len(result.Nodes); i++ {
		if result.Nodes[i].Score > result.Nodes[i-1].Score {
			t.Errorf("node %d (score %.4f) ranked higher than node %d (score %.4f)",
				i, result.Nodes[i].Score, i-1, result.Nodes[i-1].Score)
		}
	}
}

func TestAssembleContext_FullSourceTopNodes(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	// Top 5 nodes should have full source
	for i, n := range result.Nodes {
		if i < 5 {
			if !n.FullSource {
				t.Errorf("node %d (%s) should have FullSource=true", i, n.QualifiedName)
			}
		}
	}
}

func TestAssembleContext_TokenBudget(t *testing.T) {
	ctx, pool := setupContextTest(t)

	// Very small token budget — should limit output
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 100)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if result.TokenCount > 100 {
		t.Errorf("expected token count <= 100, got %d", result.TokenCount)
	}
	if result.TokenLimit != 100 {
		t.Errorf("expected token limit 100, got %d", result.TokenLimit)
	}
}

func TestAssembleContext_EmptyResults(t *testing.T) {
	ctx, pool := setupContextTest(t)

	// Query a project that doesn't exist
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "nonexistent-project", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes for nonexistent project, got %d", len(result.Nodes))
	}
	if result.Text != "No relevant code found." {
		t.Errorf("expected 'No relevant code found.', got %q", result.Text)
	}
}

func TestAssembleContext_FormatContainsHeader(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if !strings.HasPrefix(result.Text, "## Relevant Code\n\n") {
		t.Error("expected context text to start with '## Relevant Code' header")
	}
}

func TestAssembleContext_FormatContainsSignatures(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if !strings.Contains(result.Text, "Signature: function authenticate(token: string): User") {
		t.Error("expected context text to contain authenticate's signature")
	}
}

func TestAssembleContext_FormatContainsSourceCode(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	// Top-ranked node should have its source code in a code block
	if !strings.Contains(result.Text, "```\nfunction authenticate") {
		t.Error("expected context text to contain authenticate's source code in a code block")
	}
}

func TestAssembleContext_Annotations(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	// authenticate is called by queryUsers and calls verifyPassword + generateToken
	if !strings.Contains(result.Text, "Called by: queryUsers") {
		t.Error("expected annotation 'Called by: queryUsers' for authenticate")
	}
	if !strings.Contains(result.Text, "Calls: ") {
		t.Error("expected 'Calls:' annotation for authenticate")
	}
}

func TestAssembleContext_DefaultMaxTokens(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 0)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if result.TokenLimit != 8000 {
		t.Errorf("expected default token limit 8000, got %d", result.TokenLimit)
	}
}

func TestAssembleContext_NodesAreNotNil(t *testing.T) {
	ctx, pool := setupContextTest(t)

	// Even with an impossibly small budget, Nodes should be an empty slice, not nil
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 1)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if result.Nodes == nil {
		t.Error("expected Nodes to be empty slice, not nil")
	}
}

func TestAssembleContext_SimilarityInOutput(t *testing.T) {
	ctx, pool := setupContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-ctx", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if !strings.Contains(result.Text, "similarity:") {
		t.Error("expected formatted output to contain 'similarity:' in node headers")
	}
}

// setupMultiSourceContextTest creates a project with TWO sources (repo-a, repo-b).
// repo-a has: exportedFunc (dim 0)
// repo-b has: consumerFunc (dim 1) which calls exportedFunc (cross-source edge)
func setupMultiSourceContextTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, pool := setupGraphTest(t)

	projectID := "test-multi-src"
	createTestProject(t, ctx, pool, projectID)

	// Create two separate sources with distinct aliases
	_, err := pool.Exec(ctx,
		"INSERT INTO project_sources (id, project_id, path, source_type, is_code, alias) VALUES ($1, $2, $3, 'git_repo', true, $4) ON CONFLICT DO NOTHING",
		"source-a", projectID, "/tmp/repo-a", "repo-a",
	)
	if err != nil {
		t.Fatalf("creating source-a: %v", err)
	}
	_, err = pool.Exec(ctx,
		"INSERT INTO project_sources (id, project_id, path, source_type, is_code, alias) VALUES ($1, $2, $3, 'git_repo', true, $4) ON CONFLICT DO NOTHING",
		"source-b", projectID, "/tmp/repo-b", "repo-b",
	)
	if err != nil {
		t.Fatalf("creating source-b: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM projects WHERE id = $1", projectID)
	})

	exportedVec := makeUnitVector(1536, 0)
	consumerVec := makeUnitVector(1536, 1)

	// Build graph for source-a
	inputA := &indexer.BuildInput{
		ProjectID:  projectID,
		SourceID:   "source-a",
		SourcePath: "/tmp/repo-a",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "repo-a", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name: "exportedFunc", QualifiedName: "exportedFunc", Kind: "function",
				Signature: "function exportedFunc(): string",
				StartLine: 1, EndLine: 3,
				SourceCode: "function exportedFunc(): string { return 'hello'; }",
				BodyHash:   "exp-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/index.ts", Target: "exportedFunc", Kind: "contains", Line: 1},
		},
		Embeddings: map[string][]float32{
			"exportedFunc": exportedVec,
		},
		FilePaths: []string{"src/index.ts"},
	}
	_, err = indexer.BuildGraph(ctx, pool, inputA)
	if err != nil {
		t.Fatalf("BuildGraph source-a: %v", err)
	}

	// Build graph for source-b
	inputB := &indexer.BuildInput{
		ProjectID:  projectID,
		SourceID:   "source-b",
		SourcePath: "/tmp/repo-b",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "repo-b", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name: "consumerFunc", QualifiedName: "consumerFunc", Kind: "function",
				Signature: "function consumerFunc(): void",
				StartLine: 1, EndLine: 5,
				SourceCode: "function consumerFunc(): void { exportedFunc(); }",
				BodyHash:   "con-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/main.ts", Target: "consumerFunc", Kind: "contains", Line: 1},
		},
		Embeddings: map[string][]float32{
			"consumerFunc": consumerVec,
		},
		FilePaths: []string{"src/main.ts"},
	}
	_, err = indexer.BuildGraph(ctx, pool, inputB)
	if err != nil {
		t.Fatalf("BuildGraph source-b: %v", err)
	}

	// Manually insert a cross-source "calls" edge: consumerFunc -> exportedFunc
	var consumerID, exportedID string
	pool.QueryRow(ctx, "SELECT id FROM nodes WHERE qualified_name = 'consumerFunc'").Scan(&consumerID)
	pool.QueryRow(ctx, "SELECT id FROM nodes WHERE qualified_name = 'exportedFunc'").Scan(&exportedID)

	if consumerID == "" || exportedID == "" {
		t.Fatal("could not find consumerFunc or exportedFunc node IDs")
	}

	_, err = pool.Exec(ctx,
		"INSERT INTO edges (source_id, target_id, kind, weight) VALUES ($1, $2, 'calls', 1.0) ON CONFLICT DO NOTHING",
		consumerID, exportedID,
	)
	if err != nil {
		t.Fatalf("inserting cross-source edge: %v", err)
	}

	return ctx, pool
}

func TestAssembleContext_SourceAlias(t *testing.T) {
	ctx, pool := setupMultiSourceContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-multi-src", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Fatal("expected at least one node")
	}

	foundAlias := false
	for _, n := range result.Nodes {
		if n.SourceAlias != "" {
			foundAlias = true
			break
		}
	}
	if !foundAlias {
		t.Error("expected at least one node to have a non-empty SourceAlias")
	}
}

func TestAssembleContext_BidirectionalExpansion(t *testing.T) {
	ctx, pool := setupMultiSourceContextTest(t)

	// Query for exportedFunc (dim 0) — should find consumerFunc via reverse edge
	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-multi-src", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	names := make(map[string]bool)
	for _, n := range result.Nodes {
		names[n.QualifiedName] = true
	}

	if !names["consumerFunc"] {
		t.Error("expected bidirectional expansion to include 'consumerFunc' (dependent of exportedFunc)")
	}
}

func TestAssembleContext_SourceGrouping(t *testing.T) {
	ctx, pool := setupMultiSourceContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-multi-src", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if !strings.Contains(result.Text, "## Source: repo-a") {
		t.Error("expected formatted context to contain '## Source: repo-a' section header")
	}
}

func TestAssembleContext_SourceLabelInNodeHeader(t *testing.T) {
	ctx, pool := setupMultiSourceContextTest(t)

	queryVec := makeUnitVector(1536, 0)
	result, err := engine.AssembleContextWithVector(ctx, pool, queryVec, "test-multi-src", 8000)
	if err != nil {
		t.Fatalf("AssembleContextWithVector: %v", err)
	}

	if !strings.Contains(result.Text, "[source: repo-a]") {
		t.Error("expected node header to contain '[source: repo-a]'")
	}
}
