package integration

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/engine"
	"github.com/maximilianfalco/mycelium/internal/indexer"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

// setupStructuralTest creates a project with a graph that exercises all query types:
//
//	authenticate --calls--> validateToken --calls--> decodeJWT
//	authenticate --calls--> lookupUser
//	handleLogin  --calls--> authenticate
//	handleLogin  --imports--> authenticate (via import)
//	Logger (class, different file)
//
// Two packages: "api" and "auth" for cross-package queries.
func setupStructuralTest(t *testing.T) (context.Context, *pgxpool.Pool, *indexer.BuildResult) {
	t.Helper()
	ctx, pool := setupGraphTest(t)

	projectID := "test-structural"
	createTestProject(t, ctx, pool, projectID)
	createTestSource(t, ctx, pool, projectID+"/src", projectID, "/tmp/test-structural")

	input := &indexer.BuildInput{
		ProjectID:  projectID,
		SourceID:   projectID + "/src",
		SourcePath: "/tmp/test-structural",
		Workspace: &detectors.WorkspaceInfo{
			WorkspaceType:  "monorepo",
			PackageManager: "npm",
			Packages: []detectors.PackageInfo{
				{Name: "auth", Path: "packages/auth/src", Version: "1.0.0"},
				{Name: "api", Path: "packages/api/src", Version: "1.0.0"},
			},
		},
		Nodes: []parsers.NodeInfo{
			{
				Name: "authenticate", QualifiedName: "authenticate", Kind: "function",
				Signature: "function authenticate(token: string): User",
				StartLine: 1, EndLine: 5,
				SourceCode: "function authenticate(token: string): User { return validateToken(token); }",
				BodyHash:   "auth-1",
			},
			{
				Name: "validateToken", QualifiedName: "validateToken", Kind: "function",
				Signature: "function validateToken(token: string): TokenPayload",
				StartLine: 7, EndLine: 12,
				SourceCode: "function validateToken(token: string): TokenPayload { return decodeJWT(token); }",
				BodyHash:   "auth-2",
			},
			{
				Name: "decodeJWT", QualifiedName: "decodeJWT", Kind: "function",
				Signature: "function decodeJWT(token: string): any",
				StartLine: 14, EndLine: 18,
				SourceCode: "function decodeJWT(token: string): any { /* ... */ }",
				BodyHash:   "auth-3",
			},
			{
				Name: "lookupUser", QualifiedName: "lookupUser", Kind: "function",
				Signature: "function lookupUser(id: string): User",
				StartLine: 20, EndLine: 25,
				SourceCode: "function lookupUser(id: string): User { /* ... */ }",
				BodyHash:   "auth-4",
			},
			{
				Name: "handleLogin", QualifiedName: "handleLogin", Kind: "function",
				Signature: "function handleLogin(req: Request): Response",
				StartLine: 1, EndLine: 10,
				SourceCode: "function handleLogin(req: Request): Response { return authenticate(req.token); }",
				BodyHash:   "api-1",
			},
			{
				Name: "Logger", QualifiedName: "Logger", Kind: "class",
				Signature: "class Logger",
				StartLine: 1, EndLine: 15,
				SourceCode: "class Logger { log(msg: string) {} }",
				BodyHash:   "api-2",
			},
		},
		Edges: []parsers.EdgeInfo{
			// File containment
			{Source: "packages/auth/src/auth.ts", Target: "authenticate", Kind: "contains", Line: 1},
			{Source: "packages/auth/src/auth.ts", Target: "validateToken", Kind: "contains", Line: 7},
			{Source: "packages/auth/src/crypto.ts", Target: "decodeJWT", Kind: "contains", Line: 14},
			{Source: "packages/auth/src/user.ts", Target: "lookupUser", Kind: "contains", Line: 20},
			{Source: "packages/api/src/login.ts", Target: "handleLogin", Kind: "contains", Line: 1},
			{Source: "packages/api/src/logger.ts", Target: "Logger", Kind: "contains", Line: 1},
		},
		Resolved: []indexer.ResolvedEdge{
			// authenticate calls validateToken and lookupUser
			{Source: "authenticate", Target: "validateToken", Kind: "calls", Line: 3},
			{Source: "authenticate", Target: "lookupUser", Kind: "calls", Line: 4},
			// validateToken calls decodeJWT
			{Source: "validateToken", Target: "decodeJWT", Kind: "calls", Line: 9},
			// handleLogin calls authenticate (cross-package)
			{Source: "handleLogin", Target: "authenticate", Kind: "calls", Line: 5},
			// handleLogin imports authenticate (cross-package)
			{Source: "handleLogin", Target: "authenticate", Kind: "imports", Line: 1},
		},
		DependsOn:  nil,
		Embeddings: map[string][]float32{},
		FilePaths: []string{
			"packages/auth/src/auth.ts",
			"packages/auth/src/crypto.ts",
			"packages/auth/src/user.ts",
			"packages/api/src/login.ts",
			"packages/api/src/logger.ts",
		},
	}

	result, err := indexer.BuildGraph(ctx, pool, input)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	return ctx, pool, result
}

func TestFindNodeByQualifiedName(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	node, err := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "authenticate")
	if err != nil {
		t.Fatalf("FindNodeByQualifiedName: %v", err)
	}
	if node == nil {
		t.Fatal("expected to find 'authenticate' node, got nil")
	}
	if node.QualifiedName != "authenticate" {
		t.Errorf("expected qualified_name 'authenticate', got %q", node.QualifiedName)
	}
	if node.Kind != "function" {
		t.Errorf("expected kind 'function', got %q", node.Kind)
	}
}

func TestFindNodeByQualifiedName_NotFound(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	node, err := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "nonexistent")
	if err != nil {
		t.Fatalf("FindNodeByQualifiedName: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil for nonexistent node, got %+v", node)
	}
}

func TestFindNodeByQualifiedName_WrongProject(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	node, err := engine.FindNodeByQualifiedName(ctx, pool, "wrong-project", "authenticate")
	if err != nil {
		t.Fatalf("FindNodeByQualifiedName: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil for wrong project, got %+v", node)
	}
}

func TestGetCallers(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// authenticate is called by handleLogin
	node, err := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "authenticate")
	if err != nil || node == nil {
		t.Fatalf("FindNodeByQualifiedName: err=%v, node=%v", err, node)
	}

	callers, err := engine.GetCallers(ctx, pool, node.NodeID, 10)
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("expected 1 caller of authenticate, got %d", len(callers))
	}
	if callers[0].QualifiedName != "handleLogin" {
		t.Errorf("expected caller 'handleLogin', got %q", callers[0].QualifiedName)
	}
}

func TestGetCallers_NoneExist(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// handleLogin has no callers
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "handleLogin")
	if node == nil {
		t.Fatal("expected to find handleLogin")
	}

	callers, err := engine.GetCallers(ctx, pool, node.NodeID, 10)
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	if len(callers) != 0 {
		t.Errorf("expected 0 callers, got %d", len(callers))
	}
}

func TestGetCallees(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// authenticate calls validateToken and lookupUser
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "authenticate")
	if node == nil {
		t.Fatal("expected to find authenticate")
	}

	callees, err := engine.GetCallees(ctx, pool, node.NodeID, 10)
	if err != nil {
		t.Fatalf("GetCallees: %v", err)
	}
	if len(callees) != 2 {
		t.Fatalf("expected 2 callees of authenticate, got %d", len(callees))
	}

	names := map[string]bool{}
	for _, c := range callees {
		names[c.QualifiedName] = true
	}
	if !names["validateToken"] || !names["lookupUser"] {
		t.Errorf("expected callees [validateToken, lookupUser], got %v", callees)
	}
}

func TestGetImporters(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// authenticate is imported by handleLogin
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "authenticate")
	if node == nil {
		t.Fatal("expected to find authenticate")
	}

	importers, err := engine.GetImporters(ctx, pool, node.NodeID, 10)
	if err != nil {
		t.Fatalf("GetImporters: %v", err)
	}
	if len(importers) != 1 {
		t.Fatalf("expected 1 importer of authenticate, got %d", len(importers))
	}
	if importers[0].QualifiedName != "handleLogin" {
		t.Errorf("expected importer 'handleLogin', got %q", importers[0].QualifiedName)
	}
}

func TestGetDependencies_SingleHop(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// handleLogin --calls--> authenticate, handleLogin --imports--> authenticate
	// At depth 1, should get authenticate
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "handleLogin")
	if node == nil {
		t.Fatal("expected to find handleLogin")
	}

	deps, err := engine.GetDependencies(ctx, pool, node.NodeID, 1, 50)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency at depth 1, got %d", len(deps))
	}
	if deps[0].QualifiedName != "authenticate" {
		t.Errorf("expected dependency 'authenticate', got %q", deps[0].QualifiedName)
	}
}

func TestGetDependencies_Transitive(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// handleLogin -> authenticate -> validateToken -> decodeJWT
	//                             -> lookupUser
	// At depth 5, should get all 4 transitive dependencies
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "handleLogin")
	if node == nil {
		t.Fatal("expected to find handleLogin")
	}

	deps, err := engine.GetDependencies(ctx, pool, node.NodeID, 5, 50)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}

	names := map[string]bool{}
	for _, d := range deps {
		names[d.QualifiedName] = true
	}

	expected := []string{"authenticate", "validateToken", "decodeJWT", "lookupUser"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("expected transitive dependency %q, not found in %v", e, names)
		}
	}
}

func TestGetDependencies_DepthLimit(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// handleLogin -> authenticate -> validateToken -> decodeJWT
	// At depth 1, should only get authenticate (not validateToken or decodeJWT)
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "handleLogin")
	if node == nil {
		t.Fatal("expected to find handleLogin")
	}

	deps, err := engine.GetDependencies(ctx, pool, node.NodeID, 1, 50)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}

	for _, d := range deps {
		if d.QualifiedName == "decodeJWT" {
			t.Error("decodeJWT should not appear at depth 1 (it's 3 hops away)")
		}
	}
}

func TestGetDependencies_IncludesDepth(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "handleLogin")
	if node == nil {
		t.Fatal("expected to find handleLogin")
	}

	deps, err := engine.GetDependencies(ctx, pool, node.NodeID, 5, 50)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}

	for _, d := range deps {
		if d.QualifiedName == "authenticate" && d.Depth != 1 {
			t.Errorf("expected authenticate at depth 1, got %d", d.Depth)
		}
		if d.QualifiedName == "decodeJWT" && d.Depth != 3 {
			t.Errorf("expected decodeJWT at depth 3, got %d", d.Depth)
		}
	}
}

func TestGetDependents(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// decodeJWT is called by validateToken, which is called by authenticate, which is called by handleLogin
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "decodeJWT")
	if node == nil {
		t.Fatal("expected to find decodeJWT")
	}

	dependents, err := engine.GetDependents(ctx, pool, node.NodeID, 5, 50)
	if err != nil {
		t.Fatalf("GetDependents: %v", err)
	}

	names := map[string]bool{}
	for _, d := range dependents {
		names[d.QualifiedName] = true
	}

	if !names["validateToken"] {
		t.Error("expected validateToken as dependent of decodeJWT")
	}
	if !names["authenticate"] {
		t.Error("expected authenticate as transitive dependent of decodeJWT")
	}
	if !names["handleLogin"] {
		t.Error("expected handleLogin as transitive dependent of decodeJWT")
	}
}

func TestGetCrossPackageDeps(t *testing.T) {
	ctx, pool, result := setupStructuralTest(t)

	// Find the package IDs — they follow the pattern: {workspaceID}/{packageName}
	wsID := result.WorkspaceID
	apiPkgID := wsID + "/api"
	authPkgID := wsID + "/auth"

	edges, err := engine.GetCrossPackageDeps(ctx, pool, apiPkgID, authPkgID, 50)
	if err != nil {
		t.Fatalf("GetCrossPackageDeps: %v", err)
	}

	if len(edges) == 0 {
		t.Fatal("expected cross-package edges between api and auth packages")
	}

	// handleLogin (api) -> authenticate (auth) should exist as calls and imports
	foundCalls := false
	foundImports := false
	for _, e := range edges {
		if e.Kind == "calls" {
			foundCalls = true
		}
		if e.Kind == "imports" {
			foundImports = true
		}
	}
	if !foundCalls {
		t.Error("expected a 'calls' edge between api and auth packages")
	}
	if !foundImports {
		t.Error("expected an 'imports' edge between api and auth packages")
	}
}

func TestGetCrossPackageDeps_NoEdges(t *testing.T) {
	ctx, pool, result := setupStructuralTest(t)

	wsID := result.WorkspaceID
	authPkgID := wsID + "/auth"
	apiPkgID := wsID + "/api"

	// auth -> api direction should have no edges (auth doesn't import from api)
	edges, err := engine.GetCrossPackageDeps(ctx, pool, authPkgID, apiPkgID, 50)
	if err != nil {
		t.Fatalf("GetCrossPackageDeps: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from auth to api, got %d", len(edges))
	}
}

func TestGetFileContext(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// packages/auth/src/auth.ts contains authenticate and validateToken
	nodes, err := engine.GetFileContext(ctx, pool, "packages/auth/src/auth.ts", "test-structural")
	if err != nil {
		t.Fatalf("GetFileContext: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes in auth.ts, got %d", len(nodes))
	}

	names := map[string]bool{}
	for _, n := range nodes {
		names[n.QualifiedName] = true
	}
	if !names["authenticate"] || !names["validateToken"] {
		t.Errorf("expected [authenticate, validateToken] in auth.ts, got %v", names)
	}
}

func TestGetFileContext_EmptyFile(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	nodes, err := engine.GetFileContext(ctx, pool, "nonexistent.ts", "test-structural")
	if err != nil {
		t.Fatalf("GetFileContext: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for nonexistent file, got %d", len(nodes))
	}
}

func TestGetCallers_Limit(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	// validateToken is called by authenticate (only 1 caller)
	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "validateToken")
	if node == nil {
		t.Fatal("expected to find validateToken")
	}

	callers, err := engine.GetCallers(ctx, pool, node.NodeID, 1)
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("expected 1 caller with limit 1, got %d", len(callers))
	}
}

func TestGetCallees_ResultFields(t *testing.T) {
	ctx, pool, _ := setupStructuralTest(t)

	node, _ := engine.FindNodeByQualifiedName(ctx, pool, "test-structural", "authenticate")
	if node == nil {
		t.Fatal("expected to find authenticate")
	}

	callees, err := engine.GetCallees(ctx, pool, node.NodeID, 10)
	if err != nil {
		t.Fatalf("GetCallees: %v", err)
	}

	for _, c := range callees {
		if c.NodeID == "" {
			t.Error("expected non-empty NodeID")
		}
		if c.QualifiedName == "" {
			t.Error("expected non-empty QualifiedName")
		}
		if c.FilePath == "" {
			t.Error("expected non-empty FilePath")
		}
		if c.Kind == "" {
			t.Error("expected non-empty Kind")
		}
	}
}
