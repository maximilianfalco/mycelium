package integration

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func setupCrossResolverTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	return setupGraphTest(t)
}

func buildCrossSourceA() *indexer.BuildInput {
	return &indexer.BuildInput{
		ProjectID:  "test-cross",
		SourceID:   "source-a",
		SourcePath: "/tmp/cross-repo-a",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "@test/utils", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "formatDate",
				QualifiedName: "formatDate",
				Kind:          "function",
				Signature:     "function formatDate(date: Date): string",
				StartLine:     1,
				EndLine:       3,
				SourceCode:    "export function formatDate(date: Date): string { return date.toISOString(); }",
				BodyHash:      "fmt-hash",
			},
			{
				Name:          "parseDate",
				QualifiedName: "parseDate",
				Kind:          "function",
				Signature:     "function parseDate(str: string): Date",
				StartLine:     5,
				EndLine:       7,
				SourceCode:    "export function parseDate(str: string): Date { return new Date(str); }",
				BodyHash:      "parse-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/index.ts", Target: "formatDate", Kind: "contains", Line: 1},
			{Source: "src/index.ts", Target: "parseDate", Kind: "contains", Line: 5},
		},
		Resolved:   nil,
		Unresolved: nil,
		DependsOn:  nil,
		Embeddings: map[string][]float32{},
		FilePaths:  []string{"src/index.ts"},
	}
}

func buildCrossSourceB() *indexer.BuildInput {
	return &indexer.BuildInput{
		ProjectID:  "test-cross",
		SourceID:   "source-b",
		SourcePath: "/tmp/cross-repo-b",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "@test/app", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "displayDate",
				QualifiedName: "displayDate",
				Kind:          "function",
				Signature:     "function displayDate(): string",
				StartLine:     3,
				EndLine:       5,
				SourceCode:    "export function displayDate(): string { return formatDate(new Date()); }",
				BodyHash:      "display-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/index.ts", Target: "displayDate", Kind: "contains", Line: 3},
			{Source: "src/index.ts", Target: "@test/utils", Kind: "imports", Line: 1, Symbols: []string{"formatDate"}},
		},
		Resolved: nil,
		Unresolved: []indexer.UnresolvedRef{
			{Source: "displayDate", RawImport: "@test/utils", Kind: "imports", Line: 1},
		},
		DependsOn:  nil,
		Embeddings: map[string][]float32{},
		FilePaths:  []string{"src/index.ts"},
	}
}

func TestCrossResolve_Basic(t *testing.T) {
	ctx, pool := setupCrossResolverTest(t)
	createTestProject(t, ctx, pool, "test-cross")
	createTestSource(t, ctx, pool, "source-a", "test-cross", "/tmp/cross-repo-a")
	createTestSource(t, ctx, pool, "source-b", "test-cross", "/tmp/cross-repo-b")

	inputA := buildCrossSourceA()
	_, err := indexer.BuildGraph(ctx, pool, inputA)
	if err != nil {
		t.Fatalf("BuildGraph source A: %v", err)
	}

	inputB := buildCrossSourceB()
	_, err = indexer.BuildGraph(ctx, pool, inputB)
	if err != nil {
		t.Fatalf("BuildGraph source B: %v", err)
	}

	// Verify unresolved ref exists before cross-resolve
	var unresolvedBefore int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM unresolved_refs ur
		JOIN nodes n ON ur.source_node_id = n.id
		JOIN workspaces w ON n.workspace_id = w.id
		WHERE w.project_id = 'test-cross'`,
	).Scan(&unresolvedBefore)
	if err != nil {
		t.Fatalf("counting unresolved refs: %v", err)
	}
	if unresolvedBefore == 0 {
		t.Fatal("expected at least 1 unresolved ref before cross-resolve")
	}

	// Run cross-source resolution
	result, err := indexer.ResolveCrossSources(ctx, pool, "test-cross")
	if err != nil {
		t.Fatalf("ResolveCrossSources: %v", err)
	}

	if result.ResolvedCount == 0 {
		t.Error("expected at least 1 resolved cross-source import")
	}
	if result.EdgesCreated == 0 {
		t.Error("expected at least 1 cross-workspace edge created")
	}

	// Verify unresolved ref was removed
	var unresolvedAfter int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM unresolved_refs ur
		JOIN nodes n ON ur.source_node_id = n.id
		JOIN workspaces w ON n.workspace_id = w.id
		WHERE w.project_id = 'test-cross'`,
	).Scan(&unresolvedAfter)
	if unresolvedAfter >= unresolvedBefore {
		t.Errorf("expected unresolved count to decrease: before=%d, after=%d", unresolvedBefore, unresolvedAfter)
	}

	// Verify cross-workspace edge exists
	var edgeCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM edges e
		JOIN nodes src ON e.source_id = src.id
		JOIN nodes tgt ON e.target_id = tgt.id
		WHERE src.workspace_id != tgt.workspace_id
		AND e.kind = 'imports'`,
	).Scan(&edgeCount)
	if edgeCount == 0 {
		t.Error("expected at least 1 cross-workspace imports edge")
	}
}

func TestCrossResolve_NoFalseMatches(t *testing.T) {
	ctx, pool := setupCrossResolverTest(t)
	createTestProject(t, ctx, pool, "test-cross-nofalse")
	createTestSource(t, ctx, pool, "source-nf-a", "test-cross-nofalse", "/tmp/nf-repo-a")
	createTestSource(t, ctx, pool, "source-nf-b", "test-cross-nofalse", "/tmp/nf-repo-b")

	inputA := buildCrossSourceA()
	inputA.ProjectID = "test-cross-nofalse"
	inputA.SourceID = "source-nf-a"
	_, err := indexer.BuildGraph(ctx, pool, inputA)
	if err != nil {
		t.Fatalf("BuildGraph source A: %v", err)
	}

	// Source B has an unresolved ref to a package that does NOT exist in source A
	inputB := &indexer.BuildInput{
		ProjectID:  "test-cross-nofalse",
		SourceID:   "source-nf-b",
		SourcePath: "/tmp/nf-repo-b",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "@test/web", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "main",
				QualifiedName: "main",
				Kind:          "function",
				Signature:     "function main(): void",
				StartLine:     1,
				EndLine:       3,
				SourceCode:    "function main(): void {}",
				BodyHash:      "main-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/index.ts", Target: "main", Kind: "contains", Line: 1},
		},
		Unresolved: []indexer.UnresolvedRef{
			{Source: "main", RawImport: "@nonexistent/package", Kind: "imports", Line: 1},
		},
		Embeddings: map[string][]float32{},
		FilePaths:  []string{"src/index.ts"},
	}
	_, err = indexer.BuildGraph(ctx, pool, inputB)
	if err != nil {
		t.Fatalf("BuildGraph source B: %v", err)
	}

	result, err := indexer.ResolveCrossSources(ctx, pool, "test-cross-nofalse")
	if err != nil {
		t.Fatalf("ResolveCrossSources: %v", err)
	}

	if result.ResolvedCount != 0 {
		t.Errorf("expected 0 resolved (no matching package), got %d", result.ResolvedCount)
	}
	if result.StillUnresolvedCount != 1 {
		t.Errorf("expected 1 still unresolved, got %d", result.StillUnresolvedCount)
	}
}

func TestCrossResolve_Idempotent(t *testing.T) {
	ctx, pool := setupCrossResolverTest(t)
	createTestProject(t, ctx, pool, "test-cross-idem")
	createTestSource(t, ctx, pool, "source-idem-a", "test-cross-idem", "/tmp/idem-repo-a")
	createTestSource(t, ctx, pool, "source-idem-b", "test-cross-idem", "/tmp/idem-repo-b")

	inputA := buildCrossSourceA()
	inputA.ProjectID = "test-cross-idem"
	inputA.SourceID = "source-idem-a"
	_, err := indexer.BuildGraph(ctx, pool, inputA)
	if err != nil {
		t.Fatalf("BuildGraph source A: %v", err)
	}

	inputB := buildCrossSourceB()
	inputB.ProjectID = "test-cross-idem"
	inputB.SourceID = "source-idem-b"
	_, err = indexer.BuildGraph(ctx, pool, inputB)
	if err != nil {
		t.Fatalf("BuildGraph source B: %v", err)
	}

	result1, err := indexer.ResolveCrossSources(ctx, pool, "test-cross-idem")
	if err != nil {
		t.Fatalf("first ResolveCrossSources: %v", err)
	}

	// Second run should find nothing to resolve (refs already deleted)
	result2, err := indexer.ResolveCrossSources(ctx, pool, "test-cross-idem")
	if err != nil {
		t.Fatalf("second ResolveCrossSources: %v", err)
	}

	if result2.ResolvedCount != 0 {
		t.Errorf("expected 0 resolved on second run, got %d", result2.ResolvedCount)
	}

	// Edge count should not increase
	var edgeCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM edges e
		JOIN nodes src ON e.source_id = src.id
		JOIN nodes tgt ON e.target_id = tgt.id
		WHERE src.workspace_id != tgt.workspace_id`,
	).Scan(&edgeCount)
	if edgeCount != result1.EdgesCreated {
		t.Errorf("expected edge count unchanged at %d, got %d", result1.EdgesCreated, edgeCount)
	}
}

func TestCrossResolve_NoUnresolvedRefs(t *testing.T) {
	ctx, pool := setupCrossResolverTest(t)
	createTestProject(t, ctx, pool, "test-cross-empty")

	result, err := indexer.ResolveCrossSources(ctx, pool, "test-cross-empty")
	if err != nil {
		t.Fatalf("ResolveCrossSources: %v", err)
	}

	if result.ResolvedCount != 0 {
		t.Errorf("expected 0 resolved for empty project, got %d", result.ResolvedCount)
	}
	if result.EdgesCreated != 0 {
		t.Errorf("expected 0 edges for empty project, got %d", result.EdgesCreated)
	}
}

func TestCrossResolve_SameWorkspaceSkipped(t *testing.T) {
	ctx, pool := setupCrossResolverTest(t)
	createTestProject(t, ctx, pool, "test-cross-same")
	createTestSource(t, ctx, pool, "source-same", "test-cross-same", "/tmp/same-repo")

	// Single source that has both the package and the unresolved ref
	input := &indexer.BuildInput{
		ProjectID:  "test-cross-same",
		SourceID:   "source-same",
		SourcePath: "/tmp/same-repo",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "@test/utils", Path: "src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name:          "helper",
				QualifiedName: "helper",
				Kind:          "function",
				Signature:     "function helper(): void",
				StartLine:     1,
				EndLine:       2,
				SourceCode:    "function helper(): void {}",
				BodyHash:      "helper-hash",
			},
		},
		Edges: []parsers.EdgeInfo{
			{Source: "src/index.ts", Target: "helper", Kind: "contains", Line: 1},
		},
		Unresolved: []indexer.UnresolvedRef{
			{Source: "helper", RawImport: "@test/utils", Kind: "imports", Line: 1},
		},
		Embeddings: map[string][]float32{},
		FilePaths:  []string{"src/index.ts"},
	}

	_, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	result, err := indexer.ResolveCrossSources(ctx, pool, "test-cross-same")
	if err != nil {
		t.Fatalf("ResolveCrossSources: %v", err)
	}

	if result.ResolvedCount != 0 {
		t.Errorf("expected 0 resolved (same workspace should be skipped), got %d", result.ResolvedCount)
	}
}
