package indexer

import (
	"testing"

	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

func TestResolveImports_AliasMap(t *testing.T) {
	aliasMap := map[string]string{
		"@test/utils": "packages/utils/src/index.ts",
		"@test/core":  "packages/core/src/index.ts",
	}
	allFiles := []string{
		"packages/utils/src/index.ts",
		"packages/core/src/index.ts",
		"packages/core/src/validator.ts",
		"apps/web/src/index.tsx",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "packages/core/src/index.ts", Target: "@test/utils", Kind: "imports", Line: 1, Symbols: []string{"formatName"}},
		{Source: "apps/web/src/index.tsx", Target: "@test/core", Kind: "imports", Line: 1, Symbols: []string{"createUser", "greet"}},
		{Source: "apps/web/src/index.tsx", Target: "@test/utils", Kind: "imports", Line: 2, Symbols: []string{"add"}},
	}

	result := ResolveImports(rawEdges, aliasMap, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 3 {
		t.Fatalf("expected 3 resolved edges, got %d", len(result.Resolved))
	}

	assertResolved(t, result.Resolved[0], "@test/utils", "packages/utils/src/index.ts")
	assertResolved(t, result.Resolved[1], "@test/core", "packages/core/src/index.ts")
	assertResolved(t, result.Resolved[2], "@test/utils", "packages/utils/src/index.ts")

	if len(result.Unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d: %+v", len(result.Unresolved), result.Unresolved)
	}
}

func TestResolveImports_RelativePaths(t *testing.T) {
	allFiles := []string{
		"packages/core/src/index.ts",
		"packages/core/src/validator.ts",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "packages/core/src/validator.ts", Target: "./index", Kind: "imports", Line: 1, Symbols: []string{"User"}},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 1 {
		t.Fatalf("expected 1 resolved edge, got %d", len(result.Resolved))
	}
	assertResolved(t, result.Resolved[0], "./index", "packages/core/src/index.ts")
}

func TestResolveImports_RelativePathWithExtension(t *testing.T) {
	allFiles := []string{
		"src/index.ts",
		"src/utils.ts",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "./utils.js", Kind: "imports", Line: 1},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 1 {
		t.Fatalf("expected 1 resolved (ESM .js → .ts), got %d", len(result.Resolved))
	}
	assertResolved(t, result.Resolved[0], "./utils.js", "src/utils.ts")
}

func TestResolveImports_IndexFile(t *testing.T) {
	allFiles := []string{
		"src/index.ts",
		"src/components/index.tsx",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "./components", Kind: "imports", Line: 1},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 1 {
		t.Fatalf("expected 1 resolved (directory → index.tsx), got %d", len(result.Resolved))
	}
	assertResolved(t, result.Resolved[0], "./components", "src/components/index.tsx")
}

func TestResolveImports_TSConfigPaths(t *testing.T) {
	tsconfigPaths := map[string]string{
		"@/*":           "src/*",
		"@components/*": "src/components/*",
	}
	allFiles := []string{
		"src/utils/format.ts",
		"src/components/Button.tsx",
		"src/index.ts",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "@/utils/format", Kind: "imports", Line: 1},
		{Source: "src/index.ts", Target: "@components/Button", Kind: "imports", Line: 2},
	}

	result := ResolveImports(rawEdges, nil, tsconfigPaths, nil, allFiles, "/root")

	if len(result.Resolved) != 2 {
		t.Fatalf("expected 2 resolved tsconfig paths, got %d", len(result.Resolved))
	}
	assertResolved(t, result.Resolved[0], "@/utils/format", "src/utils/format.ts")
	assertResolved(t, result.Resolved[1], "@components/Button", "src/components/Button.tsx")
}

func TestResolveImports_NodeBuiltins(t *testing.T) {
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "fs", Kind: "imports", Line: 1},
		{Source: "src/index.ts", Target: "node:path", Kind: "imports", Line: 2},
		{Source: "src/index.ts", Target: "crypto", Kind: "imports", Line: 3},
		{Source: "src/index.ts", Target: "fs/promises", Kind: "imports", Line: 4},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, []string{"src/index.ts"}, "/root")

	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved (all builtins), got %d", len(result.Resolved))
	}
	if len(result.Unresolved) != 0 {
		t.Errorf("expected 0 unresolved (builtins are skipped, not unresolved), got %d", len(result.Unresolved))
	}
}

func TestResolveImports_ExternalPackagesAreUnresolved(t *testing.T) {
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "react", Kind: "imports", Line: 1, Symbols: []string{"React"}},
		{Source: "src/index.ts", Target: "lodash/debounce", Kind: "imports", Line: 2},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, []string{"src/index.ts"}, "/root")

	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(result.Resolved))
	}
	if len(result.Unresolved) != 2 {
		t.Fatalf("expected 2 unresolved (external packages), got %d", len(result.Unresolved))
	}
	if result.Unresolved[0].RawImport != "react" {
		t.Errorf("expected unresolved[0] to be 'react', got %q", result.Unresolved[0].RawImport)
	}
}

func TestResolveImports_GoStdlib(t *testing.T) {
	rawEdges := []parsers.EdgeInfo{
		{Source: "main.go", Target: "fmt", Kind: "imports", Line: 3},
		{Source: "main.go", Target: "net/http", Kind: "imports", Line: 4},
		{Source: "main.go", Target: "context", Kind: "imports", Line: 5},
		{Source: "main.go", Target: "encoding/json", Kind: "imports", Line: 6},
	}

	result := ResolveImports(rawEdges, nil, nil, nil, []string{"main.go"}, "/root")

	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved (Go stdlib), got %d", len(result.Resolved))
	}
	if len(result.Unresolved) != 0 {
		t.Errorf("expected 0 unresolved (Go stdlib is skipped), got %d", len(result.Unresolved))
	}
}

func TestResolveImports_GoModuleImport(t *testing.T) {
	aliasMap := map[string]string{
		"github.com/test/standalone":               ".",
		"github.com/test/standalone/internal/auth": "internal/auth",
		"github.com/test/standalone/internal/db":   "internal/db",
		"github.com/test/standalone/pkg/utils":     "pkg/utils",
	}
	allFiles := []string{
		"main.go",
		"internal/auth/auth.go",
		"internal/db/db.go",
		"pkg/utils/utils.go",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "main.go", Target: "github.com/test/standalone/internal/auth", Kind: "imports", Line: 4},
		{Source: "main.go", Target: "github.com/test/standalone/pkg/utils", Kind: "imports", Line: 5},
	}

	result := ResolveImports(rawEdges, aliasMap, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 2 {
		t.Fatalf("expected 2 resolved Go module imports, got %d", len(result.Resolved))
	}
	assertResolved(t, result.Resolved[0], "github.com/test/standalone/internal/auth", "internal/auth")
	assertResolved(t, result.Resolved[1], "github.com/test/standalone/pkg/utils", "pkg/utils")
}

func TestResolveImports_DependsOnEdges(t *testing.T) {
	aliasMap := map[string]string{
		"@test/utils": "packages/utils/src/index.ts",
		"@test/core":  "packages/core/src/index.ts",
	}
	allFiles := []string{
		"packages/utils/src/index.ts",
		"packages/core/src/index.ts",
		"apps/web/src/index.tsx",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "packages/core/src/index.ts", Target: "@test/utils", Kind: "imports", Line: 1},
		{Source: "apps/web/src/index.tsx", Target: "@test/core", Kind: "imports", Line: 1},
		{Source: "apps/web/src/index.tsx", Target: "@test/utils", Kind: "imports", Line: 2},
	}

	result := ResolveImports(rawEdges, aliasMap, nil, nil, allFiles, "/root")

	if len(result.DependsOn) < 2 {
		t.Fatalf("expected at least 2 depends_on edges, got %d", len(result.DependsOn))
	}

	foundCoreToUtils := false
	foundWebToCore := false
	for _, dep := range result.DependsOn {
		if dep.Kind != "depends_on" {
			t.Errorf("expected kind 'depends_on', got %q", dep.Kind)
		}
		if dep.Source == "packages/core" && dep.Target == "packages/utils" {
			foundCoreToUtils = true
		}
		if dep.Source == "apps/web" && dep.Target == "packages/core" {
			foundWebToCore = true
		}
	}
	if !foundCoreToUtils {
		t.Error("missing depends_on edge: packages/core → packages/utils")
	}
	if !foundWebToCore {
		t.Error("missing depends_on edge: apps/web → packages/core")
	}
}

func TestResolveImports_CallResolution_SameFile(t *testing.T) {
	nodes := []parsers.NodeInfo{
		{Name: "createUser", QualifiedName: "createUser", Kind: "function"},
		{Name: "greet", QualifiedName: "greet", Kind: "function"},
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "createUser", Kind: "contains", Line: 1},
		{Source: "src/index.ts", Target: "greet", Kind: "contains", Line: 5},
		{Source: "greet", Target: "createUser", Kind: "calls", Line: 6},
	}

	result := ResolveImports(rawEdges, nil, nil, nodes, []string{"src/index.ts"}, "/root")

	foundCall := false
	for _, r := range result.Resolved {
		if r.Kind == "calls" && r.Source == "greet" && r.Target == "createUser" {
			foundCall = true
			if r.ResolvedPath != "src/index.ts" {
				t.Errorf("expected call resolved to src/index.ts, got %q", r.ResolvedPath)
			}
		}
	}
	if !foundCall {
		t.Error("expected resolved call edge greet → createUser")
	}
}

func TestResolveImports_CallResolution_GlobalsSkipped(t *testing.T) {
	rawEdges := []parsers.EdgeInfo{
		{Source: "src/index.ts", Target: "myFunc", Kind: "contains", Line: 1},
		{Source: "myFunc", Target: "console.log", Kind: "calls", Line: 2},
		{Source: "myFunc", Target: "JSON.stringify", Kind: "calls", Line: 3},
		{Source: "myFunc", Target: "fmt.Sprintf", Kind: "calls", Line: 4},
	}
	nodes := []parsers.NodeInfo{
		{Name: "myFunc", QualifiedName: "myFunc", Kind: "function"},
	}

	result := ResolveImports(rawEdges, nil, nil, nodes, []string{"src/index.ts"}, "/root")

	for _, r := range result.Resolved {
		if r.Kind == "calls" {
			t.Errorf("expected no resolved call edges (all globals), got %+v", r)
		}
	}
}

func TestResolveImports_MixedEdgeKinds(t *testing.T) {
	aliasMap := map[string]string{
		"@test/utils": "packages/utils/src/index.ts",
	}
	allFiles := []string{
		"packages/utils/src/index.ts",
		"src/index.ts",
	}
	nodes := []parsers.NodeInfo{
		{Name: "add", QualifiedName: "add", Kind: "function"},
		{Name: "multiply", QualifiedName: "multiply", Kind: "function"},
	}
	rawEdges := []parsers.EdgeInfo{
		// Non-import/non-call edges should be passed through untouched
		{Source: "src/index.ts", Target: "add", Kind: "contains", Line: 1},
		{Source: "src/index.ts", Target: "@test/utils", Kind: "imports", Line: 1, Symbols: []string{"add"}},
		{Source: "add", Target: "multiply", Kind: "calls", Line: 3},
	}

	result := ResolveImports(rawEdges, aliasMap, nil, nodes, allFiles, "/root")

	importResolved := false
	for _, r := range result.Resolved {
		if r.Kind == "imports" {
			importResolved = true
		}
	}
	if !importResolved {
		t.Error("expected imports edge to be resolved")
	}
}

func TestIsNodeBuiltin(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"fs", true},
		{"path", true},
		{"node:path", true},
		{"node:fs", true},
		{"crypto", true},
		{"fs/promises", true},
		{"react", false},
		{"lodash", false},
		{"@company/auth", false},
	}

	for _, tt := range tests {
		got := isNodeBuiltin(tt.input)
		if got != tt.want {
			t.Errorf("isNodeBuiltin(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsGoStdlib(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"fmt", true},
		{"net/http", true},
		{"encoding/json", true},
		{"context", true},
		{"github.com/company/pkg", false},
		{"golang.org/x/sync", false},
	}

	for _, tt := range tests {
		got := isGoStdlib(tt.input)
		if got != tt.want {
			t.Errorf("isGoStdlib(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPackageForFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"packages/auth/src/validators.ts", "packages/auth"},
		{"apps/web/src/index.tsx", "apps/web"},
		{"internal/db/pool.go", "internal/db"},
		{"cmd/api/main.go", "cmd/api"},
		{"pkg/utils/format.go", "pkg/utils"},
		{"src/index.ts", ""},
		{"index.ts", ""},
	}

	for _, tt := range tests {
		got := packageForFile(tt.input)
		if got != tt.want {
			t.Errorf("packageForFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveImports_SubpathImport(t *testing.T) {
	aliasMap := map[string]string{
		"@test/core": "packages/core/src/index.ts",
	}
	allFiles := []string{
		"packages/core/src/index.ts",
		"packages/core/src/validator.ts",
		"apps/web/src/index.tsx",
	}
	rawEdges := []parsers.EdgeInfo{
		{Source: "apps/web/src/index.tsx", Target: "@test/core/src/validator", Kind: "imports", Line: 3},
	}

	result := ResolveImports(rawEdges, aliasMap, nil, nil, allFiles, "/root")

	if len(result.Resolved) != 1 {
		t.Fatalf("expected 1 resolved subpath import, got %d; unresolved: %+v", len(result.Resolved), result.Unresolved)
	}
	assertResolved(t, result.Resolved[0], "@test/core/src/validator", "packages/core/src/validator.ts")
}

// --- Helpers ---

func assertResolved(t *testing.T, edge ResolvedEdge, expectedTarget, expectedResolvedPath string) {
	t.Helper()
	if edge.Target != expectedTarget {
		t.Errorf("expected target %q, got %q", expectedTarget, edge.Target)
	}
	if edge.ResolvedPath != expectedResolvedPath {
		t.Errorf("expected resolvedPath %q, got %q (target: %q)", expectedResolvedPath, edge.ResolvedPath, edge.Target)
	}
}
