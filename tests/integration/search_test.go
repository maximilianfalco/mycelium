package integration

import (
	"context"
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/engine"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func setupSearchTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, pool := setupGraphTest(t)

	projectID := "test-search"
	createTestProject(t, ctx, pool, projectID)
	createTestSource(t, ctx, pool, projectID+"/src", projectID, "/tmp/test-search")

	// Create three nodes with distinct embeddings:
	// "auth" node gets a vector pointing mostly in dimension 0
	// "database" node gets a vector pointing mostly in dimension 1
	// "logging" node gets a vector pointing mostly in dimension 2
	authVec := makeUnitVector(1536, 0)
	dbVec := makeUnitVector(1536, 1)
	logVec := makeUnitVector(1536, 2)

	input := &indexer.BuildInput{
		ProjectID:  projectID,
		SourceID:   projectID + "/src",
		SourcePath: "/tmp/test-search",
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
				Docstring:     "Authenticates a user by token",
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
				Docstring:     "Queries users from the database",
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
				Docstring:     "Handles application logging",
				BodyHash:      "log-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/auth.ts", Target: "authenticate", Kind: "contains", Line: 1},
			{Source: "src/db.ts", Target: "queryUsers", Kind: "contains", Line: 1},
			{Source: "src/logger.ts", Target: "Logger", Kind: "contains", Line: 1},
		},
		Embeddings: map[string][]float32{
			"authenticate": authVec,
			"queryUsers":   dbVec,
			"Logger":       logVec,
		},
		FilePaths: []string{"src/auth.ts", "src/db.ts", "src/logger.ts"},
	}

	_, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	return ctx, pool
}

// makeUnitVector creates a 1536-dim vector with 1.0 at the given index, rest 0.
// This makes cosine similarity easy to reason about.
func makeUnitVector(dims, hotIndex int) []float32 {
	v := make([]float32, dims)
	v[hotIndex] = 1.0
	return v
}

func TestSemanticSearch_DirectQuery(t *testing.T) {
	ctx, pool := setupSearchTest(t)

	// Query with a vector similar to the "auth" node (dimension 0)
	// We can't call engine.SemanticSearch directly because it needs an OpenAI client
	// to embed the query. Instead, test the SQL logic by embedding a fake query vector.
	queryVec := makeUnitVector(1536, 0)

	results, err := engine.SemanticSearchWithVector(ctx, pool, queryVec, "test-search", 10, nil)
	if err != nil {
		t.Fatalf("SemanticSearchWithVector: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// The "authenticate" node should be most similar (cosine similarity = 1.0)
	if results[0].QualifiedName != "authenticate" {
		t.Errorf("expected top result to be 'authenticate', got %q", results[0].QualifiedName)
	}
	if math.Abs(results[0].Similarity-1.0) > 0.01 {
		t.Errorf("expected similarity ~1.0, got %f", results[0].Similarity)
	}
}

func TestSemanticSearch_KindFilter(t *testing.T) {
	ctx, pool := setupSearchTest(t)

	queryVec := makeUnitVector(1536, 2) // points toward Logger (class)
	results, err := engine.SemanticSearchWithVector(ctx, pool, queryVec, "test-search", 10, []string{"class"})
	if err != nil {
		t.Fatalf("SemanticSearchWithVector: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (only classes), got %d", len(results))
	}
	if results[0].QualifiedName != "Logger" {
		t.Errorf("expected 'Logger', got %q", results[0].QualifiedName)
	}
}

func TestSemanticSearch_Limit(t *testing.T) {
	ctx, pool := setupSearchTest(t)

	queryVec := makeUnitVector(1536, 0)
	results, err := engine.SemanticSearchWithVector(ctx, pool, queryVec, "test-search", 1, nil)
	if err != nil {
		t.Fatalf("SemanticSearchWithVector: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSemanticSearch_WrongProject(t *testing.T) {
	ctx, pool := setupSearchTest(t)

	queryVec := makeUnitVector(1536, 0)
	results, err := engine.SemanticSearchWithVector(ctx, pool, queryVec, "nonexistent-project", 10, nil)
	if err != nil {
		t.Fatalf("SemanticSearchWithVector: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for wrong project, got %d", len(results))
	}
}

func TestSemanticSearch_ResultFields(t *testing.T) {
	ctx, pool := setupSearchTest(t)

	queryVec := makeUnitVector(1536, 1) // points toward queryUsers
	results, err := engine.SemanticSearchWithVector(ctx, pool, queryVec, "test-search", 1, nil)
	if err != nil {
		t.Fatalf("SemanticSearchWithVector: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.NodeID == "" {
		t.Error("expected non-empty NodeID")
	}
	if r.QualifiedName != "queryUsers" {
		t.Errorf("expected 'queryUsers', got %q", r.QualifiedName)
	}
	if r.FilePath != "src/db.ts" {
		t.Errorf("expected 'src/db.ts', got %q", r.FilePath)
	}
	if r.Kind != "function" {
		t.Errorf("expected kind 'function', got %q", r.Kind)
	}
	if r.Signature != "function queryUsers(filter: Filter): User[]" {
		t.Errorf("unexpected signature: %q", r.Signature)
	}
}
