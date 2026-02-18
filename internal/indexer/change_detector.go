package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ChangeSet holds the result of change detection — which files were added, modified, or deleted
// since the last index.
type ChangeSet struct {
	IsGitRepo         bool     `json:"isGitRepo"`
	CurrentCommit     string   `json:"currentCommit"`
	CurrentBranch     string   `json:"currentBranch"`
	LastIndexedCommit string   `json:"lastIndexedCommit"`
	IsFullIndex       bool     `json:"isFullIndex"`
	AddedFiles        []string `json:"addedFiles"`
	ModifiedFiles     []string `json:"modifiedFiles"`
	DeletedFiles      []string `json:"deletedFiles"`
	ThresholdExceeded bool     `json:"thresholdExceeded"`
}

// DetectChanges compares the current state of sourcePath against its last indexed state.
// For git repos, uses git diff. For plain directories, uses file mtime.
// When force is true, always performs a full index regardless of threshold or previous state.
func DetectChanges(ctx context.Context, sourcePath string, lastIndexedCommit *string, lastIndexedAt *time.Time, maxAutoReindexFiles int, force bool) (*ChangeSet, error) {
	if force {
		return detectForceFullIndex(ctx, sourcePath)
	}

	isGit := isGitRepo(ctx, sourcePath)

	if isGit {
		return detectGitChanges(ctx, sourcePath, lastIndexedCommit, maxAutoReindexFiles)
	}
	return detectMtimeChanges(sourcePath, lastIndexedAt, maxAutoReindexFiles)
}

// detectForceFullIndex builds a change set that forces a complete re-index of all files.
func detectForceFullIndex(ctx context.Context, sourcePath string) (*ChangeSet, error) {
	cs := &ChangeSet{
		IsGitRepo:   isGitRepo(ctx, sourcePath),
		IsFullIndex: true,
	}

	if cs.IsGitRepo {
		commit, err := gitCurrentCommit(ctx, sourcePath)
		if err == nil {
			cs.CurrentCommit = commit
		}
		cs.CurrentBranch = gitCurrentBranch(ctx, sourcePath)
	}

	return populateFullIndex(cs, sourcePath)
}

func isGitRepo(ctx context.Context, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func gitCurrentCommit(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCurrentBranch(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Detached HEAD — no branch name
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectGitChanges(ctx context.Context, sourcePath string, lastIndexedCommit *string, maxAutoReindexFiles int) (*ChangeSet, error) {
	cs := &ChangeSet{
		IsGitRepo: true,
	}

	// Get current commit
	currentCommit, err := gitCurrentCommit(ctx, sourcePath)
	if err != nil {
		// No commits yet (empty repo) — return empty change set
		slog.Warn("could not read HEAD (empty repo?)", "path", sourcePath, "error", err)
		cs.IsFullIndex = true
		return cs, nil
	}
	cs.CurrentCommit = currentCommit
	cs.CurrentBranch = gitCurrentBranch(ctx, sourcePath)

	// First index — no previous commit
	if lastIndexedCommit == nil || *lastIndexedCommit == "" {
		cs.IsFullIndex = true
		cs.LastIndexedCommit = ""
		return populateFullIndex(cs, sourcePath)
	}

	cs.LastIndexedCommit = *lastIndexedCommit

	// Same commit — no changes
	if *lastIndexedCommit == currentCommit {
		return cs, nil
	}

	// Run git diff
	added, modified, deleted, err := gitDiff(ctx, sourcePath, *lastIndexedCommit, currentCommit)
	if err != nil {
		// Diff failed — likely force push or shallow clone. Fall back to full index.
		slog.Warn("git diff failed, falling back to full index",
			"path", sourcePath,
			"lastCommit", *lastIndexedCommit,
			"currentCommit", currentCommit,
			"error", err,
		)
		cs.IsFullIndex = true
		return populateFullIndex(cs, sourcePath)
	}

	cs.AddedFiles = filterCodeFiles(added)
	cs.ModifiedFiles = filterCodeFiles(modified)
	cs.DeletedFiles = filterCodeFiles(deleted)

	totalChanged := len(cs.AddedFiles) + len(cs.ModifiedFiles) + len(cs.DeletedFiles)
	if maxAutoReindexFiles > 0 && totalChanged > maxAutoReindexFiles {
		cs.ThresholdExceeded = true
		slog.Warn("change threshold exceeded",
			"path", sourcePath,
			"changedFiles", totalChanged,
			"threshold", maxAutoReindexFiles,
		)
	}

	return cs, nil
}

// gitDiff runs git diff and categorizes files by change type.
func gitDiff(ctx context.Context, dir, fromCommit, toCommit string) (added, modified, deleted []string, err error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-status", "--diff-filter=ACDMR", fromCommit+".."+toCommit)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git diff: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		status := parts[0]
		file := parts[1]

		switch {
		case status == "A" || status == "C":
			added = append(added, file)
		case status == "M":
			modified = append(modified, file)
		case status == "D":
			deleted = append(deleted, file)
		case strings.HasPrefix(status, "R"):
			// Rename: git shows "RXXX\told\tnew" with --name-status
			// But with SplitN on \t with limit 2, we get the rest as one string
			// Re-split to handle rename properly
			renameParts := strings.SplitN(line, "\t", 3)
			if len(renameParts) == 3 {
				deleted = append(deleted, renameParts[1])
				added = append(added, renameParts[2])
			}
		}
	}

	return added, modified, deleted, nil
}

// populateFullIndex crawls the source path and marks all files as added.
func populateFullIndex(cs *ChangeSet, sourcePath string) (*ChangeSet, error) {
	result, err := CrawlDirectory(sourcePath, true)
	if err != nil {
		return nil, fmt.Errorf("crawling for full index: %w", err)
	}

	for _, f := range result.Files {
		cs.AddedFiles = append(cs.AddedFiles, f.RelPath)
	}

	// Don't apply threshold on first index — it's always intentional
	return cs, nil
}

// filterCodeFiles keeps only files with code extensions, excluding lockfiles and other junk.
func filterCodeFiles(files []string) []string {
	var filtered []string
	for _, f := range files {
		ext := filepath.Ext(f)
		name := filepath.Base(f)

		if !codeExtensions[ext] {
			continue
		}
		if skipFiles[name] {
			continue
		}

		// Skip files in known skip directories
		skip := false
		parts := strings.Split(f, "/")
		for _, p := range parts[:len(parts)-1] {
			if skipDirs[p] || strings.HasPrefix(p, ".") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		filtered = append(filtered, f)
	}
	return filtered
}

// detectMtimeChanges uses file modification times for non-git directories.
func detectMtimeChanges(sourcePath string, lastIndexedAt *time.Time, maxAutoReindexFiles int) (*ChangeSet, error) {
	cs := &ChangeSet{
		IsGitRepo: false,
	}

	// First index — no threshold, always allowed
	if lastIndexedAt == nil {
		cs.IsFullIndex = true
		result, err := CrawlDirectory(sourcePath, true)
		if err != nil {
			return nil, fmt.Errorf("crawling for mtime detection: %w", err)
		}
		for _, f := range result.Files {
			cs.AddedFiles = append(cs.AddedFiles, f.RelPath)
		}
		return cs, nil
	}

	// Walk and compare mtimes
	result, err := CrawlDirectory(sourcePath, true)
	if err != nil {
		return nil, fmt.Errorf("crawling for mtime detection: %w", err)
	}

	for _, f := range result.Files {
		info, err := os.Stat(f.AbsPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(*lastIndexedAt) {
			cs.ModifiedFiles = append(cs.ModifiedFiles, f.RelPath)
		}
	}

	totalChanged := len(cs.ModifiedFiles)
	if maxAutoReindexFiles > 0 && totalChanged > maxAutoReindexFiles {
		cs.ThresholdExceeded = true
	}

	return cs, nil
}
