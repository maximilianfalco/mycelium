package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
	"github.com/maximilianfalco/mycelium/internal/projects"
)

const parseWorkers = 8

// IndexResult summarizes the outcome of a full project indexing run.
type IndexResult struct {
	SourcesProcessed int           `json:"sourcesProcessed"`
	SourcesSkipped   int           `json:"sourcesSkipped"`
	TotalNodes       int           `json:"totalNodes"`
	TotalEdges       int           `json:"totalEdges"`
	TotalEmbedded    int           `json:"totalEmbedded"`
	TotalDeleted     int           `json:"totalDeleted"`
	Duration         time.Duration `json:"duration"`
	Errors           []string      `json:"errors,omitempty"`
}

// IndexStatus tracks the progress of an ongoing or completed indexing job.
type IndexStatus struct {
	JobID     string       `json:"jobId"`
	ProjectID string       `json:"projectId"`
	Status    string       `json:"status"` // "running", "completed", "failed"
	Stage     string       `json:"stage"`
	Progress  string       `json:"progress"`
	Result    *IndexResult `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	StartedAt time.Time    `json:"startedAt"`
	DoneAt    *time.Time   `json:"doneAt,omitempty"`
}

// StatusStore is a thread-safe in-memory store for indexing job status.
type StatusStore struct {
	mu   sync.RWMutex
	jobs map[string]*IndexStatus
}

// NewStatusStore creates a new empty status store.
func NewStatusStore() *StatusStore {
	return &StatusStore{jobs: make(map[string]*IndexStatus)}
}

func (s *StatusStore) Set(jobID string, status *IndexStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[jobID] = status
}

func (s *StatusStore) Get(jobID string) *IndexStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[jobID]
}

// GetByProject returns the most recent job for a project.
func (s *StatusStore) GetByProject(projectID string) *IndexStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *IndexStatus
	for _, job := range s.jobs {
		if job == nil {
			continue
		}
		if job.ProjectID == projectID {
			if latest == nil || job.StartedAt.After(latest.StartedAt) {
				latest = job
			}
		}
	}
	return latest
}

// activeJobs tracks which projects are currently being indexed to prevent concurrent runs.
var activeJobs sync.Map

// IndexProject runs the full indexing pipeline for a project.
func IndexProject(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, oaiClient *openai.Client, projectID string, status *IndexStatus) *IndexResult {
	start := time.Now()
	result := &IndexResult{}

	updateStatus := func(stage, progress string) {
		if status != nil {
			status.Stage = stage
			status.Progress = progress
		}
		slog.Info("pipeline", "project", projectID, "stage", stage, "progress", progress)
	}

	// Prevent concurrent indexing of the same project
	if _, loaded := activeJobs.LoadOrStore(projectID, true); loaded {
		result.Errors = append(result.Errors, "indexing already in progress for this project")
		return result
	}
	defer activeJobs.Delete(projectID)

	// 1. Load project and sources
	updateStatus("loading", "fetching project and sources")
	project, err := projects.GetProject(ctx, pool, projectID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("loading project: %v", err))
		return result
	}
	if project == nil {
		result.Errors = append(result.Errors, "project not found")
		return result
	}

	sources, err := projects.ListSources(ctx, pool, projectID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("listing sources: %v", err))
		return result
	}

	// 2. Process each source
	for i, source := range sources {
		if !source.IsCode {
			result.SourcesSkipped++
			continue
		}

		updateStatus("indexing", fmt.Sprintf("source %d/%d: %s", i+1, len(sources), source.Alias))

		sourceResult, err := indexSource(ctx, pool, cfg, oaiClient, project.ID, &source, updateStatus)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("source %s: %v", source.Alias, err))
			continue
		}

		result.SourcesProcessed++
		result.TotalNodes += sourceResult.NodesUpserted
		result.TotalEdges += sourceResult.EdgesUpserted
		result.TotalEmbedded += sourceResult.NodesEmbedded
		result.TotalDeleted += sourceResult.NodesDeleted
	}

	result.Duration = time.Since(start)
	slog.Info("pipeline complete",
		"project", projectID,
		"sources", result.SourcesProcessed,
		"nodes", result.TotalNodes,
		"edges", result.TotalEdges,
		"embedded", result.TotalEmbedded,
		"duration", result.Duration,
	)
	return result
}

// sourceResult holds the outcome of indexing a single source.
type sourceResult struct {
	NodesUpserted int
	EdgesUpserted int
	NodesEmbedded int
	NodesDeleted  int
}

func indexSource(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg *config.Config,
	oaiClient *openai.Client,
	projectID string,
	source *projects.ProjectSource,
	updateStatus func(stage, progress string),
) (*sourceResult, error) {
	result := &sourceResult{}

	// Stage 0: Change detection
	updateStatus("changes", fmt.Sprintf("detecting changes for %s", source.Alias))
	changeSet, err := DetectChanges(ctx, source.Path, source.LastIndexedCommit, source.LastIndexedAt, cfg.MaxAutoReindexFiles)
	if err != nil {
		return nil, fmt.Errorf("change detection: %w", err)
	}

	if changeSet.ThresholdExceeded {
		return nil, fmt.Errorf("change threshold exceeded (%d files changed, max %d) — run manual reindex",
			len(changeSet.AddedFiles)+len(changeSet.ModifiedFiles)+len(changeSet.DeletedFiles),
			cfg.MaxAutoReindexFiles)
	}

	// No changes — skip
	totalChanged := len(changeSet.AddedFiles) + len(changeSet.ModifiedFiles) + len(changeSet.DeletedFiles)
	if !changeSet.IsFullIndex && totalChanged == 0 {
		slog.Info("no changes detected, skipping", "source", source.Alias)
		return result, nil
	}

	// Stage 1: Workspace detection
	updateStatus("workspace", fmt.Sprintf("detecting workspace for %s", source.Alias))
	wsInfo, err := detectors.DetectWorkspace(source.Path)
	if err != nil {
		return nil, fmt.Errorf("workspace detection: %w", err)
	}

	// Stage 2: File crawling
	updateStatus("crawling", fmt.Sprintf("crawling files for %s", source.Alias))
	crawlResult, err := CrawlDirectory(source.Path, source.IsCode)
	if err != nil {
		return nil, fmt.Errorf("crawling: %w", err)
	}

	// Build the set of files to parse based on change set
	filesToParse := buildFilesToParse(crawlResult, changeSet)
	allRelPaths := make([]string, 0, len(crawlResult.Files))
	for _, f := range crawlResult.Files {
		allRelPaths = append(allRelPaths, f.RelPath)
	}

	slog.Info("files to process",
		"source", source.Alias,
		"total", len(crawlResult.Files),
		"toParse", len(filesToParse),
		"fullIndex", changeSet.IsFullIndex,
	)

	// Stage 3: Parsing (parallel)
	updateStatus("parsing", fmt.Sprintf("parsing %d files for %s", len(filesToParse), source.Alias))
	allNodes, allEdges, parseErrors := parseFiles(ctx, filesToParse, source.Path)
	if len(parseErrors) > 0 {
		slog.Warn("parse errors", "count", len(parseErrors), "source", source.Alias)
	}

	// Stage 4: Import resolution
	updateStatus("resolving", fmt.Sprintf("resolving imports for %s", source.Alias))
	resolveResult := ResolveImports(
		allEdges,
		wsInfo.AliasMap,
		wsInfo.TSConfigPaths,
		allNodes,
		allRelPaths,
		source.Path,
	)

	// Stage 5: Body hash comparison + embedding
	updateStatus("embedding", fmt.Sprintf("embedding nodes for %s", source.Alias))
	embeddings, embeddedCount, err := embedChangedNodes(ctx, pool, oaiClient, cfg, projectID, source.ID, allNodes, wsInfo)
	if err != nil {
		return nil, fmt.Errorf("embedding: %w", err)
	}
	result.NodesEmbedded = embeddedCount

	// Stage 6: Build graph (storage)
	updateStatus("storing", fmt.Sprintf("writing graph for %s", source.Alias))
	buildInput := &BuildInput{
		ProjectID:  projectID,
		SourceID:   source.ID,
		SourcePath: source.Path,
		Workspace:  wsInfo,
		Nodes:      allNodes,
		Edges:      allEdges,
		Resolved:   resolveResult.Resolved,
		Unresolved: resolveResult.Unresolved,
		DependsOn:  resolveResult.DependsOn,
		Embeddings: embeddings,
		FilePaths:  allRelPaths,
	}

	buildResult, err := BuildGraph(ctx, pool, buildInput)
	if err != nil {
		return nil, fmt.Errorf("building graph: %w", err)
	}

	result.NodesUpserted = buildResult.NodesUpserted
	result.EdgesUpserted = buildResult.EdgesUpserted
	result.NodesDeleted = buildResult.NodesDeleted

	// Update source metadata (last indexed commit/branch/time)
	updateStatus("metadata", fmt.Sprintf("updating metadata for %s", source.Alias))
	if err := updateSourceMetadata(ctx, pool, source.ID, changeSet); err != nil {
		slog.Error("failed to update source metadata", "source", source.Alias, "error", err)
	}

	return result, nil
}

// buildFilesToParse determines which files need parsing based on the change set.
func buildFilesToParse(crawlResult *CrawlResult, changeSet *ChangeSet) []FileInfo {
	if changeSet.IsFullIndex {
		return crawlResult.Files
	}

	changedSet := make(map[string]bool)
	for _, f := range changeSet.AddedFiles {
		changedSet[f] = true
	}
	for _, f := range changeSet.ModifiedFiles {
		changedSet[f] = true
	}

	var files []FileInfo
	for _, f := range crawlResult.Files {
		if changedSet[f.RelPath] {
			files = append(files, f)
		}
	}
	return files
}

// parseFiles parses files in parallel using an errgroup with a worker limit.
func parseFiles(ctx context.Context, files []FileInfo, rootPath string) ([]parsers.NodeInfo, []parsers.EdgeInfo, []string) {
	type parseOutput struct {
		nodes  []parsers.NodeInfo
		edges  []parsers.EdgeInfo
		relErr string
	}

	results := make([]parseOutput, len(files))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parseWorkers)

	for i, f := range files {
		i, f := i, f
		g.Go(func() error {
			source, err := os.ReadFile(f.AbsPath)
			if err != nil {
				results[i] = parseOutput{relErr: fmt.Sprintf("%s: %v", f.RelPath, err)}
				return nil
			}

			pr, err := parsers.ParseFile(f.AbsPath, source)
			if err != nil {
				results[i] = parseOutput{relErr: fmt.Sprintf("%s: %v", f.RelPath, err)}
				return nil
			}

			// Rewrite absolute paths in edges to relative
			edges := make([]parsers.EdgeInfo, len(pr.Edges))
			copy(edges, pr.Edges)
			for j := range edges {
				if edges[j].Kind == "imports" || edges[j].Kind == "contains" {
					if strings.HasPrefix(edges[j].Source, "/") {
						if rel, relErr := filepath.Rel(rootPath, edges[j].Source); relErr == nil {
							edges[j].Source = rel
						}
					}
				}
			}

			results[i] = parseOutput{nodes: pr.Nodes, edges: edges}
			return nil
		})
	}

	_ = g.Wait()

	var allNodes []parsers.NodeInfo
	var allEdges []parsers.EdgeInfo
	var parseErrors []string

	for _, r := range results {
		if r.relErr != "" {
			parseErrors = append(parseErrors, r.relErr)
			continue
		}
		allNodes = append(allNodes, r.nodes...)
		allEdges = append(allEdges, r.edges...)
	}

	return allNodes, allEdges, parseErrors
}

// embedChangedNodes compares body hashes against existing DB data and only
// embeds nodes whose content has changed.
func embedChangedNodes(
	ctx context.Context,
	pool *pgxpool.Pool,
	oaiClient *openai.Client,
	cfg *config.Config,
	projectID, sourceID string,
	allNodes []parsers.NodeInfo,
	ws *detectors.WorkspaceInfo,
) (map[string][]float32, int, error) {
	embeddings := make(map[string][]float32)

	if oaiClient == nil {
		slog.Warn("no OpenAI client configured, skipping embeddings")
		return embeddings, 0, nil
	}

	// Load existing body hashes from DB
	workspaceID := makeWorkspaceID(projectID, sourceID)
	existingHashes, err := loadExistingHashes(ctx, pool, workspaceID)
	if err != nil {
		slog.Warn("could not load existing hashes, will embed all nodes", "error", err)
		existingHashes = make(map[string]string)
	}

	// Also load existing embeddings so we can reuse them for unchanged nodes
	existingEmbeddings, err := loadExistingEmbeddings(ctx, pool, workspaceID)
	if err != nil {
		slog.Warn("could not load existing embeddings", "error", err)
		existingEmbeddings = make(map[string][]float32)
	}

	// Determine which nodes need new embeddings
	var toEmbed []parsers.NodeInfo
	for _, node := range allNodes {
		oldHash, exists := existingHashes[node.QualifiedName]
		if exists && oldHash == node.BodyHash && len(existingEmbeddings[node.QualifiedName]) > 0 {
			// Unchanged — reuse existing embedding
			embeddings[node.QualifiedName] = existingEmbeddings[node.QualifiedName]
			continue
		}
		toEmbed = append(toEmbed, node)
	}

	if len(toEmbed) == 0 {
		slog.Info("all nodes unchanged, skipping embedding", "source", sourceID)
		return embeddings, 0, nil
	}

	slog.Info("embedding changed nodes", "changed", len(toEmbed), "total", len(allNodes), "reused", len(allNodes)-len(toEmbed))

	// Prepare embedding inputs
	texts := make([]string, len(toEmbed))
	for i, node := range toEmbed {
		chunk, err := PrepareEmbeddingInput(node.Signature, node.Docstring, node.SourceCode)
		if err != nil {
			slog.Warn("failed to prepare embedding input", "node", node.QualifiedName, "error", err)
			texts[i] = node.QualifiedName
			continue
		}
		texts[i] = chunk.Text
	}

	// Batch embed
	vectors, err := EmbedBatched(ctx, oaiClient, texts, cfg.MaxEmbeddingBatch)
	if err != nil {
		return nil, 0, fmt.Errorf("batch embedding: %w", err)
	}

	for i, node := range toEmbed {
		if i < len(vectors) && len(vectors[i]) > 0 {
			embeddings[node.QualifiedName] = vectors[i]
		}
	}

	return embeddings, len(toEmbed), nil
}

// loadExistingHashes returns a map of qualifiedName -> bodyHash for all nodes in a workspace.
func loadExistingHashes(ctx context.Context, pool *pgxpool.Pool, workspaceID string) (map[string]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT qualified_name, body_hash FROM nodes WHERE workspace_id = $1 AND body_hash IS NOT NULL`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying existing hashes: %w", err)
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var name, hash string
		if err := rows.Scan(&name, &hash); err != nil {
			return nil, fmt.Errorf("scanning hash row: %w", err)
		}
		hashes[name] = hash
	}
	return hashes, nil
}

// loadExistingEmbeddings returns a map of qualifiedName -> embedding vector for reuse.
func loadExistingEmbeddings(ctx context.Context, pool *pgxpool.Pool, workspaceID string) (map[string][]float32, error) {
	rows, err := pool.Query(ctx,
		`SELECT qualified_name, embedding FROM nodes WHERE workspace_id = $1 AND embedding IS NOT NULL`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying existing embeddings: %w", err)
	}
	defer rows.Close()

	embeds := make(map[string][]float32)
	for rows.Next() {
		var name string
		var vec []float32
		if err := rows.Scan(&name, &vec); err != nil {
			// pgvector scanning might fail — skip rather than fail
			continue
		}
		embeds[name] = vec
	}
	return embeds, nil
}

// updateSourceMetadata writes the indexed commit/branch/time back to project_sources.
func updateSourceMetadata(ctx context.Context, pool *pgxpool.Pool, sourceID string, cs *ChangeSet) error {
	now := time.Now()

	var commit, branch *string
	if cs.CurrentCommit != "" {
		commit = &cs.CurrentCommit
	}
	if cs.CurrentBranch != "" {
		branch = &cs.CurrentBranch
	}

	_, err := pool.Exec(ctx,
		`UPDATE project_sources SET last_indexed_commit = $1, last_indexed_branch = $2, last_indexed_at = $3 WHERE id = $4`,
		commit, branch, now, sourceID,
	)
	if err != nil {
		return fmt.Errorf("updating source metadata: %w", err)
	}
	return nil
}
