package detectors

import (
	"fmt"
	"os"
	"path/filepath"
)

type WorkspaceInfo struct {
	WorkspaceType  string            `json:"workspaceType"`
	PackageManager string            `json:"packageManager"`
	Packages       []PackageInfo     `json:"packages"`
	AliasMap       map[string]string `json:"aliasMap"`
	TSConfigPaths  map[string]string `json:"tsconfigPaths"`
}

type PackageInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Version    string `json:"version"`
	EntryPoint string `json:"entryPoint"`
}

// LanguageDetector detects workspace structure for a specific language ecosystem.
// Returns nil, nil if the directory is not a project of this type.
type LanguageDetector interface {
	Detect(sourcePath string) (*WorkspaceInfo, error)
}

// detectors is the ordered list of language detectors. First match wins.
// To add a new language: create a detector struct, implement Detect, add it here.
var detectors = []LanguageDetector{
	&NodeDetector{},
	&GoDetector{},
}

// DetectWorkspace analyzes a source directory to determine its workspace
// structure: monorepo vs standalone, package manager, packages, and alias maps.
func DetectWorkspace(sourcePath string) (*WorkspaceInfo, error) {
	if !dirExists(sourcePath) {
		return nil, fmt.Errorf("source path does not exist: %s", sourcePath)
	}

	for _, d := range detectors {
		info, err := d.Detect(sourcePath)
		if err != nil {
			return nil, err
		}
		if info != nil {
			return info, nil
		}
	}

	// No detector matched — anonymous standalone
	return &WorkspaceInfo{
		WorkspaceType: "standalone",
		Packages:      []PackageInfo{{Name: filepath.Base(sourcePath), Path: "."}},
		AliasMap:      make(map[string]string),
		TSConfigPaths: make(map[string]string),
	}, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
