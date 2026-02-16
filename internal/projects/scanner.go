package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ScanDirectory(path string) ([]ScanResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var results []ScanResult
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}

		childPath := filepath.Join(path, entry.Name())
		result := ScanResult{
			Path: childPath,
			Name: entry.Name(),
		}

		hasGit := dirExists(filepath.Join(childPath, ".git"))
		hasPkgJSON := fileExists(filepath.Join(childPath, "package.json"))
		result.HasPackageJSON = hasPkgJSON

		if hasGit && hasPkgJSON && isMonorepo(childPath) {
			result.SourceType = "monorepo"
		} else if hasGit {
			result.SourceType = "git_repo"
		} else {
			result.SourceType = "directory"
		}

		results = append(results, result)
	}

	return results, nil
}

func isMonorepo(path string) bool {
	// Check pnpm-workspace.yaml
	if fileExists(filepath.Join(path, "pnpm-workspace.yaml")) {
		return true
	}

	// Check package.json for workspaces field
	pkgPath := filepath.Join(path, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}

	var pkg struct {
		Workspaces any `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	return pkg.Workspaces != nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
