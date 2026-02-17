package indexer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, make([]byte, size), 0o644)
}

func TestCrawlDirectory_BasicFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), 200)
	writeFile(t, filepath.Join(dir, "src", "app.tsx"), 300)
	writeFile(t, filepath.Join(dir, "src", "utils.js"), 150)
	writeFile(t, filepath.Join(dir, "README.md"), 500)
	writeFile(t, filepath.Join(dir, "config.json"), 80)

	t.Run("isCode=true", func(t *testing.T) {
		result, err := CrawlDirectory(dir, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Stats.Total != 4 {
			t.Errorf("expected 4 code files, got %d", result.Stats.Total)
		}

		exts := make(map[string]bool)
		for _, f := range result.Files {
			exts[f.Extension] = true
		}
		for _, ext := range []string{".go", ".ts", ".tsx", ".js"} {
			if !exts[ext] {
				t.Errorf("expected extension %s in results", ext)
			}
		}
		if exts[".md"] || exts[".json"] {
			t.Error("non-code extensions should be excluded with isCode=true")
		}
	})

	t.Run("isCode=false", func(t *testing.T) {
		result, err := CrawlDirectory(dir, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Stats.Total != 6 {
			t.Errorf("expected 6 files, got %d", result.Stats.Total)
		}
	})
}

func TestCrawlDirectory_SkipsDirs(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "src", "app.ts"), 100)
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), 100)
	writeFile(t, filepath.Join(dir, ".git", "config"), 100)
	writeFile(t, filepath.Join(dir, "dist", "bundle.js"), 100)
	writeFile(t, filepath.Join(dir, "vendor", "dep.go"), 100)
	writeFile(t, filepath.Join(dir, "build", "output.js"), 100)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 1 {
		t.Errorf("expected 1 file (src/app.ts), got %d", result.Stats.Total)
		for _, f := range result.Files {
			t.Logf("  found: %s", f.RelPath)
		}
	}

	if result.Stats.Skipped < 5 {
		t.Errorf("expected at least 5 skipped entries, got %d", result.Stats.Skipped)
	}
}

func TestCrawlDirectory_GitignoreRoot(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\ntemp/\n"), 0o644)
	writeFile(t, filepath.Join(dir, "app.ts"), 100)
	writeFile(t, filepath.Join(dir, "debug.log"), 100)
	writeFile(t, filepath.Join(dir, "temp", "cache.ts"), 100)

	result, err := CrawlDirectory(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 1 {
		t.Errorf("expected 1 code file (app.ts), got %d", result.Stats.Total)
		for _, f := range result.Files {
			t.Logf("  found: %s", f.RelPath)
		}
	}

	for _, f := range result.Files {
		if f.Extension == ".log" {
			t.Error("*.log files should be gitignored")
		}
		if filepath.Dir(f.RelPath) == "temp" {
			t.Error("temp/ directory should be gitignored")
		}
	}
}

func TestCrawlDirectory_GitignoreNested(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "src", "app.ts"), 100)
	writeFile(t, filepath.Join(dir, "src", "generated.ts"), 100)
	writeFile(t, filepath.Join(dir, "lib", "generated.ts"), 100)

	// Nested gitignore only in src/
	os.WriteFile(filepath.Join(dir, "src", ".gitignore"), []byte("generated.ts\n"), 0o644)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	relPaths := make(map[string]bool)
	for _, f := range result.Files {
		relPaths[f.RelPath] = true
	}

	if !relPaths[filepath.Join("src", "app.ts")] {
		t.Error("src/app.ts should be included")
	}
	if relPaths[filepath.Join("src", "generated.ts")] {
		t.Error("src/generated.ts should be excluded by nested .gitignore")
	}
	if !relPaths[filepath.Join("lib", "generated.ts")] {
		t.Error("lib/generated.ts should be included (not in scope of src/.gitignore)")
	}
}

func TestCrawlDirectory_LargeFileSkip(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "small.ts"), 1000)
	writeFile(t, filepath.Join(dir, "large.ts"), 200*1024) // 200KB > 100KB limit

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 1 {
		t.Errorf("expected 1 file (small.ts only), got %d", result.Stats.Total)
	}

	for _, f := range result.Files {
		if f.RelPath == "large.ts" {
			t.Error("large file (>100KB) should be skipped")
		}
	}
}

func TestCrawlDirectory_Symlinks(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "real.ts"), 100)
	target := filepath.Join(dir, "real.ts")
	link := filepath.Join(dir, "link.ts")
	os.Symlink(target, link)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range result.Files {
		if f.RelPath == "link.ts" {
			t.Error("symlink should not be included")
		}
	}
}

func TestCrawlDirectory_Stats(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "a.ts"), 100)
	writeFile(t, filepath.Join(dir, "b.ts"), 200)
	writeFile(t, filepath.Join(dir, "c.tsx"), 300)
	writeFile(t, filepath.Join(dir, "d.go"), 400)
	writeFile(t, filepath.Join(dir, "e.json"), 50)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Stats.Total)
	}

	if result.Stats.ByExtension[".ts"] != 2 {
		t.Errorf("expected 2 .ts files, got %d", result.Stats.ByExtension[".ts"])
	}
	if result.Stats.ByExtension[".tsx"] != 1 {
		t.Errorf("expected 1 .tsx file, got %d", result.Stats.ByExtension[".tsx"])
	}
	if result.Stats.ByExtension[".go"] != 1 {
		t.Errorf("expected 1 .go file, got %d", result.Stats.ByExtension[".go"])
	}
	if result.Stats.ByExtension[".json"] != 1 {
		t.Errorf("expected 1 .json file, got %d", result.Stats.ByExtension[".json"])
	}
}

func TestCrawlDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := CrawlDirectory(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 0 {
		t.Errorf("expected 0 files, got %d", result.Stats.Total)
	}

	if len(result.Files) != 0 {
		t.Errorf("expected empty file list, got %d files", len(result.Files))
	}
}

func TestCrawlDirectory_HiddenDirs(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "visible.ts"), 100)
	writeFile(t, filepath.Join(dir, ".hidden", "secret.ts"), 100)
	writeFile(t, filepath.Join(dir, ".cache", "temp.ts"), 100)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 1 {
		t.Errorf("expected 1 file (visible.ts), got %d", result.Stats.Total)
		for _, f := range result.Files {
			t.Logf("  found: %s", f.RelPath)
		}
	}
}

func TestCrawlDirectory_SkipsLockfiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "app.ts"), 100)
	writeFile(t, filepath.Join(dir, "package-lock.json"), 5000)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), 3000)
	writeFile(t, filepath.Join(dir, "yarn.lock"), 4000)
	writeFile(t, filepath.Join(dir, "go.sum"), 2000)
	writeFile(t, filepath.Join(dir, "something.lock"), 1000)

	result, err := CrawlDirectory(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats.Total != 1 {
		t.Errorf("expected 1 file (app.ts), got %d", result.Stats.Total)
		for _, f := range result.Files {
			t.Logf("  found: %s", f.RelPath)
		}
	}
}

func TestCrawlDirectory_RelPaths(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "src", "components", "Button.tsx"), 100)
	writeFile(t, filepath.Join(dir, "lib", "utils.ts"), 100)

	result, err := CrawlDirectory(dir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	relPaths := make([]string, len(result.Files))
	for i, f := range result.Files {
		relPaths[i] = f.RelPath
	}
	sort.Strings(relPaths)

	expected := []string{
		filepath.Join("lib", "utils.ts"),
		filepath.Join("src", "components", "Button.tsx"),
	}

	if len(relPaths) != len(expected) {
		t.Fatalf("expected %d files, got %d", len(expected), len(relPaths))
	}

	for i, exp := range expected {
		if relPaths[i] != exp {
			t.Errorf("expected relPath %q at index %d, got %q", exp, i, relPaths[i])
		}
	}

	for _, f := range result.Files {
		if !filepath.IsAbs(f.AbsPath) {
			t.Errorf("expected absolute path, got %q", f.AbsPath)
		}
	}
}
