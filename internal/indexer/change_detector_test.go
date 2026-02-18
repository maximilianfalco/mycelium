package indexer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
}

func gitAdd(t *testing.T, dir string, files ...string) {
	t.Helper()
	args := append([]string{"add"}, files...)
	run(t, dir, "git", args...)
}

func gitCommit(t *testing.T, dir, msg string) string {
	t.Helper()
	run(t, dir, "git", "commit", "--allow-empty", "-m", msg)
	out := runOutput(t, dir, "git", "rev-parse", "HEAD")
	return out
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2025-01-01T00:00:00", "GIT_COMMITTER_DATE=2025-01-01T00:00:00")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\noutput: %s", name, args, err, out)
	}
}

func runOutput(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("command %s %v failed: %v", name, args, err)
	}
	return string(out[:len(out)-1]) // trim trailing newline
}

func TestDetectChanges_FirstIndex(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "initial")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cs.IsGitRepo {
		t.Error("expected IsGitRepo=true")
	}
	if !cs.IsFullIndex {
		t.Error("expected IsFullIndex=true for first index")
	}
	if len(cs.AddedFiles) != 1 {
		t.Errorf("expected 1 added file, got %d", len(cs.AddedFiles))
	}
}

func TestDetectChanges_NoChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit := gitCommit(t, dir, "initial")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.IsFullIndex {
		t.Error("expected IsFullIndex=false when same commit")
	}
	if len(cs.AddedFiles) != 0 || len(cs.ModifiedFiles) != 0 || len(cs.DeletedFiles) != 0 {
		t.Error("expected no changes when same commit")
	}
}

func TestDetectChanges_AddedFile(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	writeFile(t, filepath.Join(dir, "new.ts"), 200)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "add new.ts")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.IsFullIndex {
		t.Error("expected incremental index")
	}
	if len(cs.AddedFiles) != 1 || cs.AddedFiles[0] != "new.ts" {
		t.Errorf("expected added=[new.ts], got %v", cs.AddedFiles)
	}
}

func TestDetectChanges_ModifiedFile(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("modified content"), 0o644)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "modify main.go")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cs.ModifiedFiles) != 1 || cs.ModifiedFiles[0] != "main.go" {
		t.Errorf("expected modified=[main.go], got %v", cs.ModifiedFiles)
	}
}

func TestDetectChanges_DeletedFile(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	writeFile(t, filepath.Join(dir, "old.ts"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	os.Remove(filepath.Join(dir, "old.ts"))
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "delete old.ts")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cs.DeletedFiles) != 1 || cs.DeletedFiles[0] != "old.ts" {
		t.Errorf("expected deleted=[old.ts], got %v", cs.DeletedFiles)
	}
}

func TestDetectChanges_RenamedFile(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	os.WriteFile(filepath.Join(dir, "old.ts"), []byte("some content for rename detection"), 0o644)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	run(t, dir, "git", "mv", "old.ts", "new.ts")
	gitCommit(t, dir, "rename old.ts -> new.ts")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cs.DeletedFiles) != 1 || cs.DeletedFiles[0] != "old.ts" {
		t.Errorf("expected deleted=[old.ts], got %v", cs.DeletedFiles)
	}
	if len(cs.AddedFiles) != 1 || cs.AddedFiles[0] != "new.ts" {
		t.Errorf("expected added=[new.ts], got %v", cs.AddedFiles)
	}
}

func TestDetectChanges_FiltersNonCodeFiles(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	writeFile(t, filepath.Join(dir, "readme.md"), 200)
	writeFile(t, filepath.Join(dir, "config.json"), 150)
	writeFile(t, filepath.Join(dir, "new.ts"), 100)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "add various files")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cs.AddedFiles) != 1 || cs.AddedFiles[0] != "new.ts" {
		t.Errorf("expected only code file new.ts in added, got %v", cs.AddedFiles)
	}
}

func TestDetectChanges_FiltersSkipDirs(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), 100)
	writeFile(t, filepath.Join(dir, "src", "app.ts"), 100)
	gitAdd(t, dir, "-f", ".") // -f to add node_modules
	gitCommit(t, dir, "add files in skip dirs")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range cs.AddedFiles {
		if filepath.Dir(f) == "node_modules/pkg" || filepath.Base(filepath.Dir(f)) == "node_modules" {
			t.Errorf("file in node_modules should be filtered: %s", f)
		}
	}
	if len(cs.AddedFiles) != 1 || cs.AddedFiles[0] != filepath.Join("src", "app.ts") {
		t.Errorf("expected only src/app.ts, got %v", cs.AddedFiles)
	}
}

func TestDetectChanges_ThresholdExceeded(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("file%d.ts", i)), 100)
	}
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "add many files")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cs.ThresholdExceeded {
		t.Error("expected ThresholdExceeded=true (5 files > threshold 3)")
	}
	if len(cs.AddedFiles) != 5 {
		t.Errorf("expected 5 added files even with threshold exceeded, got %d", len(cs.AddedFiles))
	}
}

func TestDetectChanges_InvalidLastCommit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "initial")

	badCommit := "0000000000000000000000000000000000000000"
	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &badCommit, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cs.IsFullIndex {
		t.Error("expected full index fallback when last commit doesn't exist")
	}
}

func TestDetectChanges_NonGitDirectory(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	writeFile(t, filepath.Join(dir, "app.ts"), 200)

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.IsGitRepo {
		t.Error("expected IsGitRepo=false")
	}
	if !cs.IsFullIndex {
		t.Error("expected IsFullIndex=true for first mtime index")
	}
	if len(cs.AddedFiles) != 2 {
		t.Errorf("expected 2 added files, got %d: %v", len(cs.AddedFiles), cs.AddedFiles)
	}
}

func TestDetectChanges_MtimeModified(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	writeFile(t, filepath.Join(dir, "old.ts"), 100)

	pastTime := time.Now().Add(-1 * time.Hour)

	// Set mtime of old.ts to the past so it appears unchanged
	os.Chtimes(filepath.Join(dir, "old.ts"), pastTime, pastTime)

	indexTime := time.Now().Add(-30 * time.Minute)

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, nil, &indexTime, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.IsFullIndex {
		t.Error("expected incremental mtime index")
	}

	// main.go was just created (mtime is now), old.ts was set to past
	if len(cs.ModifiedFiles) != 1 || cs.ModifiedFiles[0] != "main.go" {
		t.Errorf("expected only main.go modified, got %v", cs.ModifiedFiles)
	}
}

func TestDetectChanges_EmptyGitRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	// No commits at all

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cs.IsGitRepo {
		t.Error("expected IsGitRepo=true")
	}
	if !cs.IsFullIndex {
		t.Error("expected IsFullIndex=true for empty repo")
	}
}

func TestDetectChanges_BranchInfo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "main.go"), 100)
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "initial")

	// Create and switch to a new branch
	run(t, dir, "git", "checkout", "-b", "feature/test")

	writeFile(t, filepath.Join(dir, "new.ts"), 100)
	gitAdd(t, dir, ".")
	commit := gitCommit(t, dir, "add on branch")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.CurrentBranch != "feature/test" {
		t.Errorf("expected branch feature/test, got %q", cs.CurrentBranch)
	}
}

func TestDetectChanges_MixedChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, filepath.Join(dir, "keep.ts"), 100)
	os.WriteFile(filepath.Join(dir, "modify.go"), []byte("original"), 0o644)
	writeFile(t, filepath.Join(dir, "delete.tsx"), 100)
	gitAdd(t, dir, ".")
	commit1 := gitCommit(t, dir, "initial")

	writeFile(t, filepath.Join(dir, "added.js"), 100)
	os.WriteFile(filepath.Join(dir, "modify.go"), []byte("changed"), 0o644)
	os.Remove(filepath.Join(dir, "delete.tsx"))
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "mixed changes")

	ctx := context.Background()
	cs, err := DetectChanges(ctx, dir, &commit1, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cs.AddedFiles) != 1 || cs.AddedFiles[0] != "added.js" {
		t.Errorf("expected added=[added.js], got %v", cs.AddedFiles)
	}
	if len(cs.ModifiedFiles) != 1 || cs.ModifiedFiles[0] != "modify.go" {
		t.Errorf("expected modified=[modify.go], got %v", cs.ModifiedFiles)
	}
	if len(cs.DeletedFiles) != 1 || cs.DeletedFiles[0] != "delete.tsx" {
		t.Errorf("expected deleted=[delete.tsx], got %v", cs.DeletedFiles)
	}
}

// --- filterCodeFiles unit tests ---

func TestFilterCodeFiles_Basic(t *testing.T) {
	input := []string{"main.go", "app.ts", "readme.md", "config.json"}
	got := filterCodeFiles(input)
	if len(got) != 2 {
		t.Errorf("expected 2 code files, got %d: %v", len(got), got)
	}
}

func TestFilterCodeFiles_SkipDirs(t *testing.T) {
	input := []string{"node_modules/pkg/index.js", "src/app.ts", ".hidden/secret.go"}
	got := filterCodeFiles(input)
	if len(got) != 1 || got[0] != "src/app.ts" {
		t.Errorf("expected [src/app.ts], got %v", got)
	}
}

func TestFilterCodeFiles_Empty(t *testing.T) {
	got := filterCodeFiles(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestFilterCodeFiles_AllExtensions(t *testing.T) {
	input := []string{"a.ts", "b.tsx", "c.js", "d.jsx", "e.go"}
	got := filterCodeFiles(input)
	if len(got) != 5 {
		t.Errorf("expected 5, got %d: %v", len(got), got)
	}
}
