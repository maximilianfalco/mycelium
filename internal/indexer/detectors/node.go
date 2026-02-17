package detectors

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// NodeDetector detects JS/TS workspaces (pnpm, yarn, npm, lerna).
type NodeDetector struct{}

// Detect checks for JS/TS workspace config files and package.json.
// Returns nil, nil if no JS/TS project indicators are found.
func (d *NodeDetector) Detect(sourcePath string) (*WorkspaceInfo, error) {
	globs, err := detectWorkspaceGlobs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("detecting workspace: %w", err)
	}

	hasPackageJSON := fileExists(filepath.Join(sourcePath, "package.json"))

	if len(globs) == 0 && !hasPackageJSON {
		return nil, nil
	}

	info := &WorkspaceInfo{
		AliasMap:      make(map[string]string),
		TSConfigPaths: make(map[string]string),
	}

	if len(globs) == 0 {
		info.WorkspaceType = "standalone"
		info.PackageManager = detectPackageManager(sourcePath)
		pkg, err := readPackageInfo(sourcePath, sourcePath)
		if err != nil {
			pkg = PackageInfo{Name: filepath.Base(sourcePath), Path: "."}
		}
		info.Packages = []PackageInfo{pkg}
	} else {
		info.WorkspaceType = "monorepo"
		info.PackageManager = detectPackageManager(sourcePath)
		packages, err := discoverPackages(sourcePath, globs)
		if err != nil {
			return nil, fmt.Errorf("discovering packages: %w", err)
		}
		info.Packages = packages
	}

	// Build alias map from discovered packages
	for i := range info.Packages {
		pkg := &info.Packages[i]
		if pkg.Name == "" {
			continue
		}
		entryPoint := findEntryPoint(filepath.Join(sourcePath, pkg.Path))
		pkg.EntryPoint = entryPoint
		if entryPoint != "" {
			info.AliasMap[pkg.Name] = filepath.Join(pkg.Path, entryPoint)
		}
	}

	// Read tsconfig paths from workspace root
	tsconfigPaths, err := readTSConfigPaths(sourcePath, sourcePath)
	if err == nil {
		maps.Copy(info.TSConfigPaths, tsconfigPaths)
	}

	// Also read tsconfig paths from each package
	for _, pkg := range info.Packages {
		pkgPath := filepath.Join(sourcePath, pkg.Path)
		paths, err := readTSConfigPaths(pkgPath, sourcePath)
		if err == nil {
			for k, v := range paths {
				if _, exists := info.TSConfigPaths[k]; !exists {
					info.TSConfigPaths[k] = v
				}
			}
		}
	}

	return info, nil
}

// detectWorkspaceGlobs returns the package glob patterns if a workspace config
// is found. Returns nil if the directory is standalone.
func detectWorkspaceGlobs(sourcePath string) ([]string, error) {
	// 1. Check pnpm-workspace.yaml
	pnpmPath := filepath.Join(sourcePath, "pnpm-workspace.yaml")
	if fileExists(pnpmPath) {
		globs, err := parsePnpmWorkspace(pnpmPath)
		if err != nil {
			return nil, fmt.Errorf("parsing pnpm-workspace.yaml: %w", err)
		}
		if len(globs) > 0 {
			return globs, nil
		}
	}

	// 2. Check package.json workspaces field
	pkgPath := filepath.Join(sourcePath, "package.json")
	if fileExists(pkgPath) {
		globs, err := parsePackageJSONWorkspaces(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("parsing package.json workspaces: %w", err)
		}
		if len(globs) > 0 {
			return globs, nil
		}
	}

	// 3. Check lerna.json
	lernaPath := filepath.Join(sourcePath, "lerna.json")
	if fileExists(lernaPath) {
		globs, err := parseLernaJSON(lernaPath)
		if err != nil {
			return nil, fmt.Errorf("parsing lerna.json: %w", err)
		}
		if len(globs) > 0 {
			return globs, nil
		}
	}

	return nil, nil
}

// parsePnpmWorkspace reads pnpm-workspace.yaml and extracts package globs.
// Uses simple line parsing to avoid a YAML dependency.
func parsePnpmWorkspace(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var globs []string
	inPackages := false
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if trimmed == "packages:" {
			inPackages = true
			continue
		}

		// A new top-level key ends the packages block
		if inPackages && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}

		if rest, ok := strings.CutPrefix(trimmed, "-"); inPackages && ok {
			glob := strings.TrimSpace(rest)
			glob = strings.Trim(glob, "'\"")
			if glob != "" {
				globs = append(globs, glob)
			}
		}
	}

	return globs, nil
}

// parsePackageJSONWorkspaces reads the workspaces field from package.json.
// Handles both array form and object form (yarn).
func parsePackageJSONWorkspaces(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	wsRaw, ok := raw["workspaces"]
	if !ok {
		return nil, nil
	}

	// Try array form: "workspaces": ["packages/*"]
	var globs []string
	if err := json.Unmarshal(wsRaw, &globs); err == nil {
		return globs, nil
	}

	// Try object form: "workspaces": { "packages": ["packages/*"] }
	var wsObj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(wsRaw, &wsObj); err == nil {
		return wsObj.Packages, nil
	}

	return nil, nil
}

// parseLernaJSON reads the packages field from lerna.json.
func parseLernaJSON(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var lerna struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(data, &lerna); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return lerna.Packages, nil
}

// detectPackageManager determines the package manager from lockfiles.
// Priority: pnpm > yarn > npm.
func detectPackageManager(sourcePath string) string {
	if fileExists(filepath.Join(sourcePath, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(sourcePath, "yarn.lock")) {
		return "yarn"
	}
	if fileExists(filepath.Join(sourcePath, "package-lock.json")) {
		return "npm"
	}
	return ""
}

// discoverPackages expands workspace globs and reads package info.
func discoverPackages(rootPath string, globs []string) ([]PackageInfo, error) {
	var packages []PackageInfo
	seen := make(map[string]bool)
	negations := extractNegations(globs)

	for _, pattern := range globs {
		if strings.HasPrefix(pattern, "!") {
			continue
		}

		matches, err := expandWorkspaceGlob(rootPath, pattern)
		if err != nil {
			return nil, fmt.Errorf("expanding glob %q: %w", pattern, err)
		}

		for _, match := range matches {
			relPath, err := filepath.Rel(rootPath, match)
			if err != nil {
				continue
			}

			if seen[relPath] {
				continue
			}

			if isNegated(relPath, negations) {
				continue
			}

			pkg, err := readPackageInfo(match, rootPath)
			if err != nil {
				// Skip dirs without package.json
				continue
			}

			seen[relPath] = true
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// expandWorkspaceGlob expands a workspace glob pattern to matching directories.
func expandWorkspaceGlob(rootPath, pattern string) ([]string, error) {
	absPattern := filepath.Join(rootPath, pattern)

	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	var dirs []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, m)
		}
	}

	return dirs, nil
}

// extractNegations pulls negation patterns (prefixed with !) from globs.
func extractNegations(globs []string) []string {
	var negations []string
	for _, g := range globs {
		if neg, ok := strings.CutPrefix(g, "!"); ok {
			negations = append(negations, neg)
		}
	}
	return negations
}

// isNegated checks if a relative path matches any negation pattern.
func isNegated(relPath string, negations []string) bool {
	for _, neg := range negations {
		matched, err := filepath.Match(neg, relPath)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// readPackageInfo reads package.json from a directory and extracts name/version.
func readPackageInfo(pkgDir, rootPath string) (PackageInfo, error) {
	pkgJSONPath := filepath.Join(pkgDir, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("reading package.json: %w", err)
	}

	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return PackageInfo{}, fmt.Errorf("parsing package.json: %w", err)
	}

	relPath, err := filepath.Rel(rootPath, pkgDir)
	if err != nil {
		relPath = pkgDir
	}

	return PackageInfo{
		Name:    pkg.Name,
		Path:    relPath,
		Version: pkg.Version,
	}, nil
}

// findEntryPoint looks for the source entry point of a JS/TS package.
// Heuristic: src/index.ts > src/index.tsx > main field from package.json.
func findEntryPoint(pkgDir string) string {
	candidates := []string{
		"src/index.ts",
		"src/index.tsx",
		"src/index.js",
		"src/index.jsx",
		"index.ts",
		"index.tsx",
		"index.js",
		"index.jsx",
	}

	for _, c := range candidates {
		if fileExists(filepath.Join(pkgDir, c)) {
			return c
		}
	}

	// Fall back to main field from package.json
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}

	var pkg struct {
		Main   string `json:"main"`
		Source string `json:"source"`
		Module string `json:"module"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	// Prefer source > module > main
	if pkg.Source != "" {
		return pkg.Source
	}
	if pkg.Module != "" {
		return pkg.Module
	}
	return pkg.Main
}

// readTSConfigPaths reads tsconfig.json and extracts compilerOptions.paths,
// following extends chains.
func readTSConfigPaths(dir, rootPath string) (map[string]string, error) {
	tsconfigPath := filepath.Join(dir, "tsconfig.json")
	if !fileExists(tsconfigPath) {
		return nil, fmt.Errorf("tsconfig.json not found in %s", dir)
	}

	return parseTSConfig(tsconfigPath, rootPath, 0)
}

// parseTSConfig reads a tsconfig.json, follows extends, and merges paths.
// maxDepth prevents infinite loops from circular extends.
func parseTSConfig(tsconfigPath, rootPath string, depth int) (map[string]string, error) {
	if depth > 10 {
		return nil, fmt.Errorf("tsconfig extends chain too deep")
	}

	data, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return nil, fmt.Errorf("reading tsconfig: %w", err)
	}

	// Strip single-line comments (tsconfig allows them)
	cleaned := stripJSONComments(data)

	var tsconfig struct {
		Extends         string `json:"extends"`
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	if err := json.Unmarshal(cleaned, &tsconfig); err != nil {
		return nil, fmt.Errorf("parsing tsconfig: %w", err)
	}

	paths := make(map[string]string)

	// Follow extends chain first (parent paths are overridden by child)
	if tsconfig.Extends != "" {
		parentPath := resolveExtendsPath(tsconfigPath, tsconfig.Extends)
		parentPaths, err := parseTSConfig(parentPath, rootPath, depth+1)
		if err == nil {
			maps.Copy(paths, parentPaths)
		}
	}

	// Apply this tsconfig's paths (override parent)
	tsconfigDir := filepath.Dir(tsconfigPath)
	baseURL := tsconfig.CompilerOptions.BaseURL
	if baseURL == "" {
		baseURL = "."
	}

	for alias, targets := range tsconfig.CompilerOptions.Paths {
		if len(targets) == 0 {
			continue
		}
		// Use the first target path
		target := targets[0]
		// Resolve relative to baseUrl and tsconfig directory
		absTarget := filepath.Join(tsconfigDir, baseURL, target)
		relTarget, err := filepath.Rel(rootPath, absTarget)
		if err != nil {
			relTarget = target
		}
		paths[alias] = relTarget
	}

	return paths, nil
}

// resolveExtendsPath resolves the extends field relative to the tsconfig location.
func resolveExtendsPath(tsconfigPath, extends string) string {
	dir := filepath.Dir(tsconfigPath)
	resolved := filepath.Join(dir, extends)

	if filepath.Ext(resolved) != ".json" {
		resolved += ".json"
	}
	return resolved
}

// stripJSONComments removes single-line (//) and multi-line (/* */) comments
// from JSON with comments (as used by tsconfig.json).
func stripJSONComments(data []byte) []byte {
	var result []byte
	i := 0
	inString := false

	for i < len(data) {
		// Handle string literals (don't strip inside strings)
		if data[i] == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
			result = append(result, data[i])
			i++
			continue
		}

		if inString {
			result = append(result, data[i])
			i++
			continue
		}

		// Single-line comment
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}

		// Multi-line comment
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			if i+1 < len(data) {
				i += 2
			}
			continue
		}

		result = append(result, data[i])
		i++
	}

	return result
}
