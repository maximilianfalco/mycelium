package indexer

import (
	"testing"

	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
)

func TestFindPackageID(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		packages   []detectors.PackageInfo
		packageIDs map[string]string
		want       string
	}{
		{
			name:     "root package with dot path matches any file",
			filePath: "src/index.ts",
			packages: []detectors.PackageInfo{{Name: "@readme/markdown", Path: "."}},
			packageIDs: map[string]string{
				"@readme/markdown": "pkg-markdown",
			},
			want: "pkg-markdown",
		},
		{
			name:     "root package with empty path matches any file",
			filePath: "lib/utils.go",
			packages: []detectors.PackageInfo{{Name: "mylib", Path: ""}},
			packageIDs: map[string]string{
				"mylib": "pkg-mylib",
			},
			want: "pkg-mylib",
		},
		{
			name:     "subdirectory package matches files under its path",
			filePath: "packages/oas/src/index.ts",
			packages: []detectors.PackageInfo{{Name: "oas", Path: "packages/oas"}},
			packageIDs: map[string]string{
				"oas": "pkg-oas",
			},
			want: "pkg-oas",
		},
		{
			name:     "subdirectory package does not match files outside its path",
			filePath: "packages/other/src/index.ts",
			packages: []detectors.PackageInfo{{Name: "oas", Path: "packages/oas"}},
			packageIDs: map[string]string{
				"oas": "pkg-oas",
			},
			want: "",
		},
		{
			name:     "first matching package wins",
			filePath: "src/index.ts",
			packages: []detectors.PackageInfo{
				{Name: "root", Path: "."},
				{Name: "also-root", Path: "."},
			},
			packageIDs: map[string]string{
				"root":      "pkg-root",
				"also-root": "pkg-also-root",
			},
			want: "pkg-root",
		},
		{
			name:       "no packages returns empty",
			filePath:   "src/index.ts",
			packages:   nil,
			packageIDs: map[string]string{},
			want:       "",
		},
		{
			name:     "package exists but not in packageIDs map",
			filePath: "src/index.ts",
			packages: []detectors.PackageInfo{{Name: "missing", Path: "."}},
			packageIDs: map[string]string{
				"other": "pkg-other",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &detectors.WorkspaceInfo{Packages: tt.packages}
			got := findPackageID(tt.filePath, ws, tt.packageIDs)
			if got != tt.want {
				t.Errorf("findPackageID(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}
