package detectors

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// GoDetector detects Go workspaces (go.work) and modules (go.mod).
type GoDetector struct{}

// Detect checks for go.work or go.mod.
// Returns nil, nil if no Go project indicators are found.
func (d *GoDetector) Detect(sourcePath string) (*WorkspaceInfo, error) {
	return detectGoWorkspace(sourcePath)
}

// detectGoWorkspace checks for Go workspace (go.work) or module (go.mod) and
// returns WorkspaceInfo if found, or nil if the directory is not a Go project.
func detectGoWorkspace(sourcePath string) (*WorkspaceInfo, error) {
	// Check for go.work (multi-module workspace)
	goWorkPath := filepath.Join(sourcePath, "go.work")
	if fileExists(goWorkPath) {
		moduleDirs, goVersion, err := parseGoWork(goWorkPath)
		if err != nil {
			return nil, fmt.Errorf("parsing go.work: %w", err)
		}

		info := &WorkspaceInfo{
			WorkspaceType:  "monorepo",
			PackageManager: "go",
			AliasMap:       make(map[string]string),
			TSConfigPaths:  make(map[string]string),
		}

		for _, moduleDir := range moduleDirs {
			goModPath := filepath.Join(sourcePath, moduleDir, "go.mod")
			modulePath, _, err := parseGoMod(goModPath)
			if err != nil {
				continue
			}

			packages, aliases := discoverGoPackages(sourcePath, modulePath, moduleDir)
			for i := range packages {
				packages[i].Version = goVersion
			}
			info.Packages = append(info.Packages, packages...)
			maps.Copy(info.AliasMap, aliases)
		}

		return info, nil
	}

	// Check for go.mod (standalone module)
	goModPath := filepath.Join(sourcePath, "go.mod")
	if fileExists(goModPath) {
		modulePath, goVersion, err := parseGoMod(goModPath)
		if err != nil {
			return nil, fmt.Errorf("parsing go.mod: %w", err)
		}

		packages, aliases := discoverGoPackages(sourcePath, modulePath, ".")
		for i := range packages {
			packages[i].Version = goVersion
		}

		return &WorkspaceInfo{
			WorkspaceType:  "standalone",
			PackageManager: "go",
			Packages:       packages,
			AliasMap:       aliases,
			TSConfigPaths:  make(map[string]string),
		}, nil
	}

	return nil, nil
}

// parseGoWork reads a go.work file and extracts use directives and Go version.
func parseGoWork(path string) ([]string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading file: %w", err)
	}

	var dirs []string
	var goVersion string
	inUseBlock := false

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if ver, ok := strings.CutPrefix(trimmed, "go "); ok && goVersion == "" {
			goVersion = strings.TrimSpace(ver)
			continue
		}

		// Block form: use ( ... )
		if trimmed == "use (" {
			inUseBlock = true
			continue
		}
		if inUseBlock && trimmed == ")" {
			inUseBlock = false
			continue
		}
		if inUseBlock {
			dir := strings.TrimPrefix(trimmed, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
			continue
		}

		// Single-line form: use ./path
		if rest, ok := strings.CutPrefix(trimmed, "use "); ok {
			dir := strings.TrimSpace(rest)
			dir = strings.TrimPrefix(dir, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}

	return dirs, goVersion, nil
}

// parseGoMod reads a go.mod file and extracts the module path and Go version.
func parseGoMod(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("reading file: %w", err)
	}

	var modulePath, goVersion string
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if mod, ok := strings.CutPrefix(trimmed, "module "); ok && modulePath == "" {
			modulePath = strings.TrimSpace(mod)
			continue
		}
		if ver, ok := strings.CutPrefix(trimmed, "go "); ok && goVersion == "" {
			goVersion = strings.TrimSpace(ver)
			continue
		}
	}

	if modulePath == "" {
		return "", "", fmt.Errorf("no module directive found in %s", path)
	}

	return modulePath, goVersion, nil
}

// discoverGoPackages walks a Go module directory and finds all packages
// (directories containing .go files). Returns packages and an alias map.
func discoverGoPackages(rootPath, modulePath, moduleDir string) ([]PackageInfo, map[string]string) {
	absModuleDir := filepath.Join(rootPath, moduleDir)
	var packages []PackageInfo
	aliasMap := make(map[string]string)

	filepath.WalkDir(absModuleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		if !dirHasGoFiles(path) {
			return nil
		}

		relToModule, _ := filepath.Rel(absModuleDir, path)
		relToRoot, _ := filepath.Rel(rootPath, path)

		var importPath string
		if relToModule == "." {
			importPath = modulePath
		} else {
			importPath = modulePath + "/" + filepath.ToSlash(relToModule)
		}

		entryPoint := findGoEntryPoint(path)

		pkg := PackageInfo{
			Name:       importPath,
			Path:       relToRoot,
			EntryPoint: entryPoint,
		}
		packages = append(packages, pkg)
		aliasMap[importPath] = relToRoot

		return nil
	})

	return packages, aliasMap
}

// findGoEntryPoint checks for a main.go entry point in a Go package directory.
func findGoEntryPoint(pkgDir string) string {
	if fileExists(filepath.Join(pkgDir, "main.go")) {
		return "main.go"
	}
	return ""
}

// dirHasGoFiles checks if a directory contains any .go files.
func dirHasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}
