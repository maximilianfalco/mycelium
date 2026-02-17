package indexer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

const defaultMaxFileSizeKB = 100

var codeExtensions = map[string]bool{
	".ts": true, ".tsx": true,
	".js": true, ".jsx": true,
	".go": true,
}

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	".next": true, "__pycache__": true, "vendor": true, "testdata": true,
	"bower_components": true,
}

var skipFiles = map[string]bool{
	"package-lock.json": true, "pnpm-lock.yaml": true, "yarn.lock": true,
	"go.sum": true,
}

type FileInfo struct {
	AbsPath   string `json:"absPath"`
	RelPath   string `json:"relPath"`
	Extension string `json:"extension"`
	SizeBytes int64  `json:"sizeBytes"`
}

type CrawlStats struct {
	Total       int            `json:"total"`
	Skipped     int            `json:"skipped"`
	ByExtension map[string]int `json:"byExtension"`
}

type CrawlResult struct {
	Files []FileInfo `json:"files"`
	Stats CrawlStats `json:"stats"`
}

// CrawlDirectory walks rootPath and returns a list of files to process.
// If isCode is true, only files with code extensions are included.
// Respects .gitignore at all directory levels, skips common junk directories,
// lockfiles, and files exceeding the size limit.
func CrawlDirectory(rootPath string, isCode bool, maxFileSizeKB ...int) (*CrawlResult, error) {
	maxBytes := int64(defaultMaxFileSizeKB) * 1024
	if len(maxFileSizeKB) > 0 && maxFileSizeKB[0] > 0 {
		maxBytes = int64(maxFileSizeKB[0]) * 1024
	}
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("stat root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", rootPath)
	}

	result := &CrawlResult{
		Stats: CrawlStats{
			ByExtension: make(map[string]int),
		},
	}

	var ignoreStack []ignoreEntry

	// Load root .gitignore if present
	rootGitignore := filepath.Join(rootPath, ".gitignore")
	if gi, err := ignore.CompileIgnoreFile(rootGitignore); err == nil {
		ignoreStack = append(ignoreStack, ignoreEntry{depth: 0, matcher: gi})
	}

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		// Calculate depth for gitignore stack management
		depth := 0
		if relPath != "." {
			depth = strings.Count(relPath, string(filepath.Separator)) + 1
		}

		// Pop gitignore matchers that are no longer in scope
		for len(ignoreStack) > 0 && ignoreStack[len(ignoreStack)-1].depth >= depth && depth > 0 {
			ignoreStack = ignoreStack[:len(ignoreStack)-1]
		}

		if d.IsDir() {
			if relPath == "." {
				return nil
			}

			name := d.Name()

			// Skip hardcoded directories
			if skipDirs[name] {
				result.Stats.Skipped++
				return filepath.SkipDir
			}

			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				result.Stats.Skipped++
				return filepath.SkipDir
			}

			// Check gitignore patterns
			if isGitignored(relPath, ignoreStack) {
				result.Stats.Skipped++
				return filepath.SkipDir
			}

			// Load .gitignore from this directory if present
			gi, loadErr := ignore.CompileIgnoreFile(filepath.Join(path, ".gitignore"))
			if loadErr == nil {
				ignoreStack = append(ignoreStack, ignoreEntry{depth: depth, matcher: gi})
			}

			return nil
		}

		// Skip symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			result.Stats.Skipped++
			return nil
		}

		name := d.Name()
		ext := filepath.Ext(name)

		// Skip lockfiles and log files
		if skipFiles[name] || ext == ".lock" || ext == ".log" {
			result.Stats.Skipped++
			return nil
		}

		// Check gitignore patterns
		if isGitignored(relPath, ignoreStack) {
			result.Stats.Skipped++
			return nil
		}

		// Check file size
		fileInfo, statErr := d.Info()
		if statErr != nil {
			result.Stats.Skipped++
			return nil
		}
		if fileInfo.Size() > maxBytes {
			result.Stats.Skipped++
			return nil
		}

		// Code extension filter
		if isCode && !codeExtensions[ext] {
			result.Stats.Skipped++
			return nil
		}

		result.Files = append(result.Files, FileInfo{
			AbsPath:   path,
			RelPath:   relPath,
			Extension: ext,
			SizeBytes: fileInfo.Size(),
		})
		result.Stats.Total++
		result.Stats.ByExtension[ext]++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return result, nil
}

type ignoreEntry struct {
	depth   int
	matcher *ignore.GitIgnore
}

func isGitignored(relPath string, stack []ignoreEntry) bool {
	for _, entry := range stack {
		if entry.matcher.MatchesPath(relPath) {
			return true
		}
	}
	return false
}
