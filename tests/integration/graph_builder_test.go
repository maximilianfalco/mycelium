package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/db"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func setupGraphTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return ctx, pool
}

func createTestProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		"INSERT INTO projects (id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		id, "Test Project "+id,
	)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM projects WHERE id = $1", id)
	})
}

func createTestSource(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, projectID, path string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		"INSERT INTO project_sources (id, project_id, path, source_type, is_code, alias) VALUES ($1, $2, $3, 'git_repo', true, $4) ON CONFLICT DO NOTHING",
		id, projectID, path, "test-source",
	)
	if err != nil {
		t.Fatalf("creating test source: %v", err)
	}
}

func testBuildInput() *indexer.BuildInput {
	return &indexer.BuildInput{
		ProjectID:  "test-gb",
		SourceID:   "test-gb/test-source",
		SourcePath: "/tmp/test-repo",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "test-pkg", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "greet",
				QualifiedName: "greet",
				Kind:          "function",
				Signature:     "function greet(name: string): string",
				StartLine:     1,
				EndLine:       3,
				SourceCode:    "function greet(name: string): string { return `Hello ${name}`; }",
				Docstring:     "Greets a person by name",
				BodyHash:      "abc123",
			},
			{
				Name:          "farewell",
				QualifiedName: "farewell",
				Kind:          "function",
				Signature:     "function farewell(name: string): string",
				StartLine:     5,
				EndLine:       7,
				SourceCode:    "function farewell(name: string): string { return `Bye ${name}`; }",
				Docstring:     "",
				BodyHash:      "def456",
			},
			{
				Name:          "helper",
				QualifiedName: "helper",
				Kind:          "function",
				Signature:     "function helper(): void",
				StartLine:     1,
				EndLine:       2,
				SourceCode:    "function helper(): void {}",
				Docstring:     "",
				BodyHash:      "ghi789",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/greetings.ts", Target: "greet", Kind: "contains", Line: 1},
			{Source: "src/greetings.ts", Target: "farewell", Kind: "contains", Line: 5},
			{Source: "src/utils.ts", Target: "helper", Kind: "contains", Line: 1},
		},
		Resolved: []indexer.ResolvedEdge{
			{Source: "greet", Target: "helper", Kind: "calls", Line: 2},
		},
		Unresolved: []indexer.UnresolvedRef{
			{Source: "farewell", RawImport: "unknown-module", Kind: "import", Line: 1},
		},
		DependsOn:  nil,
		Embeddings: map[string][]float32{},
		FilePaths:  []string{"src/greetings.ts", "src/utils.ts"},
	}
}

func TestBuildGraph_Basic(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb")
	createTestSource(t, ctx, pool, "test-gb/test-source", "test-gb", "/tmp/test-repo")

	input := testBuildInput()
	result, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	if result.NodesUpserted != 3 {
		t.Errorf("expected 3 nodes upserted, got %d", result.NodesUpserted)
	}
	if result.EdgesUpserted < 1 {
		t.Errorf("expected at least 1 edge, got %d", result.EdgesUpserted)
	}
	if result.UnresolvedRefs != 1 {
		t.Errorf("expected 1 unresolved ref, got %d", result.UnresolvedRefs)
	}

	var nodeCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result.WorkspaceID).Scan(&nodeCount)
	if err != nil {
		t.Fatalf("counting nodes: %v", err)
	}
	if nodeCount != 3 {
		t.Errorf("expected 3 nodes in DB, got %d", nodeCount)
	}
}

func TestBuildGraph_Idempotent(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-idem")
	createTestSource(t, ctx, pool, "test-gb-idem/test-source", "test-gb-idem", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-idem"
	input.SourceID = "test-gb-idem/test-source"

	result1, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("first BuildGraph: %v", err)
	}

	result2, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("second BuildGraph: %v", err)
	}

	if result1.NodesUpserted != result2.NodesUpserted {
		t.Errorf("idempotent: node counts differ: %d vs %d", result1.NodesUpserted, result2.NodesUpserted)
	}

	var nodeCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result1.WorkspaceID).Scan(&nodeCount)
	if nodeCount != 3 {
		t.Errorf("after double upsert, expected 3 nodes, got %d", nodeCount)
	}
}

func TestBuildGraph_UpdateChanged(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-upd")
	createTestSource(t, ctx, pool, "test-gb-upd/test-source", "test-gb-upd", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-upd"
	input.SourceID = "test-gb-upd/test-source"

	_, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("first BuildGraph: %v", err)
	}

	input.Nodes[0].BodyHash = "updated-hash"
	input.Nodes[0].SourceCode = "function greet(name: string): string { return `Hi ${name}`; }"

	_, err = indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("second BuildGraph: %v", err)
	}

	wsID := input.ProjectID + "/" + input.SourceID
	var bodyHash string
	nodeID := wsID + "/test-gb-upd/test-source/test-pkg/src/greetings.ts::greet"
	err = pool.QueryRow(ctx, "SELECT body_hash FROM nodes WHERE id = $1", nodeID).Scan(&bodyHash)
	if err != nil {
		// Try without the double workspace prefix — ID generation varies
		// Just check that exactly 3 nodes exist and one has the updated hash
		var updatedCount int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1 AND body_hash = 'updated-hash'", wsID).Scan(&updatedCount)
		if updatedCount != 1 {
			t.Errorf("expected 1 node with updated hash, got %d", updatedCount)
		}
	} else if bodyHash != "updated-hash" {
		t.Errorf("expected updated body_hash, got %q", bodyHash)
	}
}

func TestBuildGraph_StaleCleanup(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-stale")
	createTestSource(t, ctx, pool, "test-gb-stale/test-source", "test-gb-stale", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-stale"
	input.SourceID = "test-gb-stale/test-source"

	result1, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("first BuildGraph: %v", err)
	}

	var nodesBefore int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result1.WorkspaceID).Scan(&nodesBefore)

	// Remove utils.ts (contains 'helper' node) from file list
	input.FilePaths = []string{"src/greetings.ts"}
	// Remove the helper node and its edges too
	input.Nodes = input.Nodes[:2]
	input.Edges = input.Edges[:2]
	input.Resolved = nil

	result2, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("second BuildGraph: %v", err)
	}

	if result2.NodesDeleted == 0 {
		t.Error("expected stale nodes to be deleted")
	}

	var nodesAfter int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result2.WorkspaceID).Scan(&nodesAfter)
	if nodesAfter != 2 {
		t.Errorf("expected 2 nodes after cleanup, got %d", nodesAfter)
	}
}

func TestBuildGraph_CascadeDelete(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-cascade")
	createTestSource(t, ctx, pool, "test-gb-cascade/test-source", "test-gb-cascade", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-cascade"
	input.SourceID = "test-gb-cascade/test-source"

	result, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Delete the project — should cascade to workspace, packages, nodes, edges
	_, err = pool.Exec(ctx, "DELETE FROM projects WHERE id = 'test-gb-cascade'")
	if err != nil {
		t.Fatalf("deleting project: %v", err)
	}

	var wsCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM workspaces WHERE id = $1", result.WorkspaceID).Scan(&wsCount)
	if wsCount != 0 {
		t.Error("expected workspace to be cascade deleted")
	}

	var nodeCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result.WorkspaceID).Scan(&nodeCount)
	if nodeCount != 0 {
		t.Error("expected nodes to be cascade deleted")
	}
}

func TestBuildGraph_WithEmbedding(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-embed")
	createTestSource(t, ctx, pool, "test-gb-embed/test-source", "test-gb-embed", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-embed"
	input.SourceID = "test-gb-embed/test-source"

	// Add a fake embedding for one node
	fakeVec := make([]float32, 1536)
	for i := range fakeVec {
		fakeVec[i] = float32(i) * 0.001
	}
	input.Embeddings = map[string][]float32{
		"greet": fakeVec,
	}

	result, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Verify the embedding was stored
	var hasEmbedding bool
	pool.QueryRow(ctx,
		"SELECT embedding IS NOT NULL FROM nodes WHERE workspace_id = $1 AND qualified_name = 'greet'",
		result.WorkspaceID,
	).Scan(&hasEmbedding)
	if !hasEmbedding {
		t.Error("expected greet node to have an embedding")
	}

	// Verify other node has no embedding
	var noEmbedding bool
	pool.QueryRow(ctx,
		"SELECT embedding IS NULL FROM nodes WHERE workspace_id = $1 AND qualified_name = 'farewell'",
		result.WorkspaceID,
	).Scan(&noEmbedding)
	if !noEmbedding {
		t.Error("expected farewell node to have no embedding")
	}
}

func TestCleanupStale_Standalone(t *testing.T) {
	ctx, pool := setupGraphTest(t)
	createTestProject(t, ctx, pool, "test-gb-cleanup")
	createTestSource(t, ctx, pool, "test-gb-cleanup/test-source", "test-gb-cleanup", "/tmp/test-repo")

	input := testBuildInput()
	input.ProjectID = "test-gb-cleanup"
	input.SourceID = "test-gb-cleanup/test-source"

	result, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Use standalone CleanupStale to remove all but greetings.ts
	deleted, err := indexer.CleanupStale(ctx, pool, result.WorkspaceID, []string{"src/greetings.ts"})
	if err != nil {
		t.Fatalf("CleanupStale: %v", err)
	}
	if deleted == 0 {
		t.Error("expected at least 1 node deleted")
	}

	var remaining int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM nodes WHERE workspace_id = $1", result.WorkspaceID).Scan(&remaining)
	if remaining != 2 {
		t.Errorf("expected 2 remaining nodes (from greetings.ts), got %d", remaining)
	}
}
