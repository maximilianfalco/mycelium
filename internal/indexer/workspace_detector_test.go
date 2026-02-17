package indexer

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func fixturesDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "tests", "fixtures")
}

func TestDetectWorkspace_PnpmMonorepo(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "monorepo" {
		t.Errorf("expected workspace type 'monorepo', got %q", info.WorkspaceType)
	}

	if info.PackageManager != "pnpm" {
		t.Errorf("expected package manager 'pnpm', got %q", info.PackageManager)
	}

	if len(info.Packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(info.Packages))
	}

	names := packageNames(info.Packages)
	sort.Strings(names)
	expected := []string{"@test/core", "@test/utils", "@test/web"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected package %q at index %d, got %q", name, i, names[i])
		}
	}

	for _, pkg := range info.Packages {
		if pkg.EntryPoint == "" {
			t.Errorf("package %q has no entry point", pkg.Name)
		}
	}

	if _, ok := info.AliasMap["@test/core"]; !ok {
		t.Error("alias map missing @test/core")
	}
	if _, ok := info.AliasMap["@test/utils"]; !ok {
		t.Error("alias map missing @test/utils")
	}
	if _, ok := info.AliasMap["@test/web"]; !ok {
		t.Error("alias map missing @test/web")
	}
}

func TestDetectWorkspace_YarnMonorepo(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-yarn")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "monorepo" {
		t.Errorf("expected workspace type 'monorepo', got %q", info.WorkspaceType)
	}

	if info.PackageManager != "yarn" {
		t.Errorf("expected package manager 'yarn', got %q", info.PackageManager)
	}

	if len(info.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(info.Packages))
	}

	names := packageNames(info.Packages)
	sort.Strings(names)
	expected := []string{"@test/auth", "@test/shared"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected package %q at index %d, got %q", name, i, names[i])
		}
	}
}

func TestDetectWorkspace_NpmMonorepo(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-npm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "monorepo" {
		t.Errorf("expected workspace type 'monorepo', got %q", info.WorkspaceType)
	}

	if info.PackageManager != "npm" {
		t.Errorf("expected package manager 'npm', got %q", info.PackageManager)
	}

	if len(info.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(info.Packages))
	}

	names := packageNames(info.Packages)
	sort.Strings(names)
	expected := []string{"@test/server", "@test/ui"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected package %q at index %d, got %q", name, i, names[i])
		}
	}
}

func TestDetectWorkspace_Standalone(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "standalone-repo")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "standalone" {
		t.Errorf("expected workspace type 'standalone', got %q", info.WorkspaceType)
	}

	if info.PackageManager != "npm" {
		t.Errorf("expected package manager 'npm', got %q", info.PackageManager)
	}

	if len(info.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(info.Packages))
	}

	if info.Packages[0].Name != "my-app" {
		t.Errorf("expected package name 'my-app', got %q", info.Packages[0].Name)
	}

	if info.Packages[0].EntryPoint == "" {
		t.Error("standalone package should have an entry point (src/index.ts)")
	}
}

func TestDetectWorkspace_NoPackageJSON(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "no-package-json")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "standalone" {
		t.Errorf("expected workspace type 'standalone', got %q", info.WorkspaceType)
	}

	if len(info.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(info.Packages))
	}

	if info.Packages[0].Name != "no-package-json" {
		t.Errorf("expected anonymous package name from dir, got %q", info.Packages[0].Name)
	}
}

func TestDetectWorkspace_TSConfigPaths(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "standalone-repo")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.TSConfigPaths) == 0 {
		t.Fatal("expected tsconfig paths to be populated")
	}

	if _, ok := info.TSConfigPaths["@/*"]; !ok {
		t.Error("tsconfig paths missing @/*")
	}
	if _, ok := info.TSConfigPaths["@utils/*"]; !ok {
		t.Error("tsconfig paths missing @utils/*")
	}
}

func TestDetectWorkspace_TSConfigExtends(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := info.TSConfigPaths["@/*"]; !ok {
		t.Error("root tsconfig paths @/* not found")
	}

	if _, ok := info.TSConfigPaths["@components/*"]; !ok {
		t.Error("extended tsconfig paths @components/* not found (from apps/web/tsconfig.json)")
	}
}

func TestDetectWorkspace_AliasMapEntryPoints(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	coreAlias, ok := info.AliasMap["@test/core"]
	if !ok {
		t.Fatal("alias map missing @test/core")
	}
	if coreAlias != filepath.Join("packages", "core", "src", "index.ts") {
		t.Errorf("expected core alias to point to packages/core/src/index.ts, got %q", coreAlias)
	}

	webAlias, ok := info.AliasMap["@test/web"]
	if !ok {
		t.Fatal("alias map missing @test/web")
	}
	if webAlias != filepath.Join("apps", "web", "src", "index.tsx") {
		t.Errorf("expected web alias to point to apps/web/src/index.tsx, got %q", webAlias)
	}
}

func TestDetectWorkspace_PackagePaths(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pathMap := make(map[string]string)
	for _, pkg := range info.Packages {
		pathMap[pkg.Name] = pkg.Path
	}

	if pathMap["@test/core"] != filepath.Join("packages", "core") {
		t.Errorf("expected @test/core path packages/core, got %q", pathMap["@test/core"])
	}
	if pathMap["@test/web"] != filepath.Join("apps", "web") {
		t.Errorf("expected @test/web path apps/web, got %q", pathMap["@test/web"])
	}
}

func TestDetectWorkspace_PackageVersions(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	info, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	versionMap := make(map[string]string)
	for _, pkg := range info.Packages {
		versionMap[pkg.Name] = pkg.Version
	}

	if versionMap["@test/core"] != "0.1.0" {
		t.Errorf("expected @test/core version 0.1.0, got %q", versionMap["@test/core"])
	}
	if versionMap["@test/utils"] != "0.2.0" {
		t.Errorf("expected @test/utils version 0.2.0, got %q", versionMap["@test/utils"])
	}
}

func TestDetectWorkspace_NonexistentPath(t *testing.T) {
	_, err := DetectWorkspace("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestParsePnpmWorkspace(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-pnpm")
	globs, err := parsePnpmWorkspace(filepath.Join(dir, "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(globs) != 2 {
		t.Fatalf("expected 2 globs, got %d: %v", len(globs), globs)
	}

	if globs[0] != "packages/*" {
		t.Errorf("expected first glob 'packages/*', got %q", globs[0])
	}
	if globs[1] != "apps/*" {
		t.Errorf("expected second glob 'apps/*', got %q", globs[1])
	}
}

func TestParsePackageJSONWorkspaces(t *testing.T) {
	dir := filepath.Join(fixturesDir(), "monorepo-yarn")
	globs, err := parsePackageJSONWorkspaces(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(globs) != 1 {
		t.Fatalf("expected 1 glob, got %d: %v", len(globs), globs)
	}

	if globs[0] != "packages/*" {
		t.Errorf("expected glob 'packages/*', got %q", globs[0])
	}
}

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		expected string
	}{
		{"pnpm monorepo", "monorepo-pnpm", "pnpm"},
		{"yarn monorepo", "monorepo-yarn", "yarn"},
		{"npm standalone", "standalone-repo", "npm"},
		{"no lockfile", "no-package-json", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(fixturesDir(), tt.fixture)
			result := detectPackageManager(dir)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStripJSONComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"no comments",
			`{"key": "value"}`,
			`{"key": "value"}`,
		},
		{
			"single-line comment",
			"{\n  // this is a comment\n  \"key\": \"value\"\n}",
			"{\n  \n  \"key\": \"value\"\n}",
		},
		{
			"comment with slashes in string",
			`{"url": "https://example.com"}`,
			`{"url": "https://example.com"}`,
		},
		{
			"multi-line comment",
			"{\n  /* multi\n  line */\n  \"key\": \"value\"\n}",
			"{\n  \n  \"key\": \"value\"\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(stripJSONComments([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNegationPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a monorepo with negation patterns
	os.MkdirAll(filepath.Join(tmpDir, "packages", "keep", "src"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "packages", "deprecated-old", "src"), 0o755)

	os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte("packages:\n  - 'packages/*'\n  - '!packages/deprecated-*'\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "pnpm-lock.yaml"), nil, 0o644)
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "negation-test"}`), 0o644)

	os.WriteFile(filepath.Join(tmpDir, "packages", "keep", "package.json"), []byte(`{"name": "@test/keep", "version": "1.0.0"}`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "packages", "deprecated-old", "package.json"), []byte(`{"name": "@test/deprecated-old", "version": "0.1.0"}`), 0o644)

	info, err := DetectWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.Packages) != 1 {
		t.Fatalf("expected 1 package (deprecated should be excluded), got %d: %v", len(info.Packages), packageNames(info.Packages))
	}

	if info.Packages[0].Name != "@test/keep" {
		t.Errorf("expected package @test/keep, got %q", info.Packages[0].Name)
	}
}

func TestYarnWorkspacesObjectForm(t *testing.T) {
	tmpDir := t.TempDir()

	// Yarn workspace with object form: { packages: [...] }
	os.MkdirAll(filepath.Join(tmpDir, "packages", "lib"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "yarn.lock"), nil, 0o644)
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "yarn-obj", "workspaces": {"packages": ["packages/*"]}}`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "packages", "lib", "package.json"), []byte(`{"name": "@test/lib", "version": "1.0.0"}`), 0o644)

	info, err := DetectWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.WorkspaceType != "monorepo" {
		t.Errorf("expected monorepo, got %q", info.WorkspaceType)
	}
	if len(info.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(info.Packages))
	}
	if info.Packages[0].Name != "@test/lib" {
		t.Errorf("expected @test/lib, got %q", info.Packages[0].Name)
	}
}

func packageNames(packages []PackageInfo) []string {
	names := make([]string, len(packages))
	for i, p := range packages {
		names[i] = p.Name
	}
	return names
}
