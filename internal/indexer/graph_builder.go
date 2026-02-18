package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maximilianfalco/mycelium/internal/indexer/detectors"
	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
	"github.com/pgvector/pgvector-go"
)

const batchSize = 1000

// BuildInput holds all the data from previous pipeline stages needed to build the graph.
type BuildInput struct {
	ProjectID  string
	SourceID   string
	SourcePath string
	Workspace  *detectors.WorkspaceInfo
	Nodes      []parsers.NodeInfo
	Edges      []parsers.EdgeInfo
	Resolved   []ResolvedEdge
	Unresolved []UnresolvedRef
	DependsOn  []ResolvedEdge
	Embeddings map[string][]float32 // qualifiedName -> vector
	FilePaths  []string             // relative paths of all current files
}

// BuildResult summarizes what was written to the database.
type BuildResult struct {
	WorkspaceID    string
	NodesUpserted  int
	EdgesUpserted  int
	UnresolvedRefs int
	NodesDeleted   int
	Duration       time.Duration
}

// BuildGraph writes all indexing pipeline output to Postgres in a single transaction.
func BuildGraph(ctx context.Context, pool *pgxpool.Pool, input *BuildInput) (*BuildResult, error) {
	start := time.Now()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	workspaceID := makeWorkspaceID(input.ProjectID, input.SourceID)
	language := detectLanguage(input.FilePaths)

	// 1. Upsert workspace
	if err := upsertWorkspace(ctx, tx, workspaceID, input); err != nil {
		return nil, err
	}

	// 2. Upsert packages
	packageIDs, err := upsertPackages(ctx, tx, workspaceID, input.Workspace)
	if err != nil {
		return nil, err
	}

	// 3. Upsert nodes
	nodesUpserted, err := upsertNodes(ctx, tx, workspaceID, packageIDs, input, language)
	if err != nil {
		return nil, err
	}

	// 4. Upsert edges (resolved imports, calls, structural, depends_on)
	edgesUpserted, err := upsertEdges(ctx, tx, workspaceID, packageIDs, input)
	if err != nil {
		return nil, err
	}

	// 5. Insert unresolved refs
	unresolvedCount, err := insertUnresolvedRefs(ctx, tx, workspaceID, packageIDs, input)
	if err != nil {
		return nil, err
	}

	// 6. Cleanup stale nodes from deleted files
	deleted, err := cleanupStale(ctx, tx, workspaceID, input.FilePaths)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	result := &BuildResult{
		WorkspaceID:    workspaceID,
		NodesUpserted:  nodesUpserted,
		EdgesUpserted:  edgesUpserted,
		UnresolvedRefs: unresolvedCount,
		NodesDeleted:   deleted,
		Duration:       time.Since(start),
	}

	slog.Info("graph built",
		"workspace", workspaceID,
		"nodes", nodesUpserted,
		"edges", edgesUpserted,
		"unresolved", unresolvedCount,
		"deleted", deleted,
		"duration", result.Duration,
	)

	return result, nil
}

// CleanupStale removes nodes from files that no longer exist in the workspace.
// Exported for use by the pipeline orchestrator.
func CleanupStale(ctx context.Context, pool *pgxpool.Pool, workspaceID string, currentFilePaths []string) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	deleted, err := cleanupStale(ctx, tx, workspaceID, currentFilePaths)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}
	return deleted, nil
}

func upsertWorkspace(ctx context.Context, tx pgx.Tx, workspaceID string, input *BuildInput) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		INSERT INTO workspaces (id, project_id, source_id, name, path, workspace_type, package_manager, indexed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			workspace_type = EXCLUDED.workspace_type,
			package_manager = EXCLUDED.package_manager,
			indexed_at = EXCLUDED.indexed_at`,
		workspaceID,
		input.ProjectID,
		input.SourceID,
		filepath.Base(input.SourcePath),
		input.SourcePath,
		input.Workspace.WorkspaceType,
		input.Workspace.PackageManager,
		now,
	)
	if err != nil {
		return fmt.Errorf("upserting workspace: %w", err)
	}
	return nil
}

// upsertPackages returns a map of package name -> package ID
func upsertPackages(ctx context.Context, tx pgx.Tx, workspaceID string, ws *detectors.WorkspaceInfo) (map[string]string, error) {
	now := time.Now()
	packageIDs := make(map[string]string)

	for _, pkg := range ws.Packages {
		pkgID := makePackageID(workspaceID, pkg.Name)
		packageIDs[pkg.Name] = pkgID

		_, err := tx.Exec(ctx, `
			INSERT INTO packages (id, workspace_id, name, path, version, indexed_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				path = EXCLUDED.path,
				version = EXCLUDED.version,
				indexed_at = EXCLUDED.indexed_at`,
			pkgID, workspaceID, pkg.Name, pkg.Path, pkg.Version, now,
		)
		if err != nil {
			return nil, fmt.Errorf("upserting package %s: %w", pkg.Name, err)
		}
	}

	return packageIDs, nil
}

func upsertNodes(ctx context.Context, tx pgx.Tx, workspaceID string, packageIDs map[string]string, input *BuildInput, language string) (int, error) {
	now := time.Now()
	count := 0

	for i := 0; i < len(input.Nodes); i += batchSize {
		end := i + batchSize
		if end > len(input.Nodes) {
			end = len(input.Nodes)
		}
		chunk := input.Nodes[i:end]

		batch := &pgx.Batch{}
		for _, node := range chunk {
			filePath := nodeFilePath(node, input.Edges)
			pkgID := findPackageID(filePath, input.Workspace, packageIDs)
			nodeID := makeNodeID(workspaceID, pkgID, filePath, node.QualifiedName)

			var emb *pgvector.Vector
			if vec, ok := input.Embeddings[node.QualifiedName]; ok && len(vec) > 0 {
				v := pgvector.NewVector(vec)
				emb = &v
			}

			batch.Queue(`
				INSERT INTO nodes (id, workspace_id, package_id, file_path, name, qualified_name, kind, language, signature, start_line, end_line, source_code, docstring, body_hash, embedding, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
				ON CONFLICT (id) DO UPDATE SET
					file_path = EXCLUDED.file_path,
					name = EXCLUDED.name,
					qualified_name = EXCLUDED.qualified_name,
					kind = EXCLUDED.kind,
					language = EXCLUDED.language,
					signature = EXCLUDED.signature,
					start_line = EXCLUDED.start_line,
					end_line = EXCLUDED.end_line,
					source_code = EXCLUDED.source_code,
					docstring = EXCLUDED.docstring,
					body_hash = EXCLUDED.body_hash,
					embedding = EXCLUDED.embedding,
					updated_at = EXCLUDED.updated_at`,
				nodeID, workspaceID, nilIfEmpty(pkgID), filePath, node.Name, node.QualifiedName,
				node.Kind, language, node.Signature, node.StartLine, node.EndLine,
				node.SourceCode, node.Docstring, node.BodyHash, emb, now,
			)
		}

		br := tx.SendBatch(ctx, batch)
		for range chunk {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("upserting node batch: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("closing node batch: %w", err)
		}
		count += len(chunk)
	}

	return count, nil
}

func upsertEdges(ctx context.Context, tx pgx.Tx, workspaceID string, packageIDs map[string]string, input *BuildInput) (int, error) {
	// Collect all edges: resolved imports/calls + structural contains edges + depends_on
	type edgeRow struct {
		sourceID string
		targetID string
		kind     string
		weight   float64
		line     int
	}

	var rows []edgeRow

	// Build a node lookup: qualifiedName -> nodeID
	nodeIDLookup := make(map[string]string)
	// Also build filePath -> first nodeID for edges that use file paths as sources
	fileNodeLookup := make(map[string]string)
	for _, node := range input.Nodes {
		filePath := nodeFilePath(node, input.Edges)
		pkgID := findPackageID(filePath, input.Workspace, packageIDs)
		nodeID := makeNodeID(workspaceID, pkgID, filePath, node.QualifiedName)
		nodeIDLookup[node.QualifiedName] = nodeID
		if _, exists := fileNodeLookup[filePath]; !exists {
			fileNodeLookup[filePath] = nodeID
		}
	}

	// lookupID resolves both qualified names and file paths to node IDs
	lookupID := func(key string) (string, bool) {
		if id, ok := nodeIDLookup[key]; ok {
			return id, true
		}
		if id, ok := fileNodeLookup[key]; ok {
			return id, true
		}
		return "", false
	}

	// Resolved edges (imports, calls, extends, implements, uses_type, embeds)
	for _, e := range input.Resolved {
		srcID, srcOK := lookupID(e.Source)
		tgtID, tgtOK := lookupID(e.Target)
		if !srcOK || !tgtOK {
			continue
		}
		rows = append(rows, edgeRow{
			sourceID: srcID,
			targetID: tgtID,
			kind:     e.Kind,
			weight:   edgeWeight(e.Kind),
			line:     e.Line,
		})
	}

	// Structural "contains" edges from raw edges
	for _, e := range input.Edges {
		if e.Kind != "contains" {
			continue
		}
		srcID, srcOK := lookupID(e.Source)
		tgtID, tgtOK := lookupID(e.Target)
		if !srcOK || !tgtOK {
			continue
		}
		rows = append(rows, edgeRow{
			sourceID: srcID,
			targetID: tgtID,
			kind:     "contains",
			weight:   1.0,
			line:     e.Line,
		})
	}

	// depends_on edges (package level — map to workspace-level nodes or skip if no match)
	for _, e := range input.DependsOn {
		srcID, srcOK := lookupID(e.Source)
		tgtID, tgtOK := lookupID(e.Target)
		if !srcOK || !tgtOK {
			continue
		}
		rows = append(rows, edgeRow{
			sourceID: srcID,
			targetID: tgtID,
			kind:     "depends_on",
			weight:   1.0,
			line:     e.Line,
		})
	}

	// Deduplicate: same (source, target, kind) should pick highest weight
	type edgeKey struct{ src, tgt, kind string }
	deduped := make(map[edgeKey]edgeRow)
	for _, r := range rows {
		key := edgeKey{r.sourceID, r.targetID, r.kind}
		if existing, ok := deduped[key]; !ok || r.weight > existing.weight {
			deduped[key] = r
		}
	}

	// Batch upsert
	count := 0
	edgeSlice := make([]edgeRow, 0, len(deduped))
	for _, r := range deduped {
		edgeSlice = append(edgeSlice, r)
	}

	for i := 0; i < len(edgeSlice); i += batchSize {
		end := i + batchSize
		if end > len(edgeSlice) {
			end = len(edgeSlice)
		}
		chunk := edgeSlice[i:end]

		batch := &pgx.Batch{}
		for _, r := range chunk {
			batch.Queue(`
				INSERT INTO edges (source_id, target_id, kind, weight, line_number)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (source_id, target_id, kind) DO UPDATE SET
					weight = EXCLUDED.weight,
					line_number = EXCLUDED.line_number`,
				r.sourceID, r.targetID, r.kind, r.weight, r.line,
			)
		}

		br := tx.SendBatch(ctx, batch)
		for range chunk {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("upserting edge batch: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("closing edge batch: %w", err)
		}
		count += len(chunk)
	}

	return count, nil
}

func insertUnresolvedRefs(ctx context.Context, tx pgx.Tx, workspaceID string, packageIDs map[string]string, input *BuildInput) (int, error) {
	if len(input.Unresolved) == 0 {
		return 0, nil
	}

	// Build node ID lookup (qualifiedName -> nodeID) and file path lookup (filePath -> first nodeID)
	nodeIDLookup := make(map[string]string)
	fileNodeLookup := make(map[string]string)
	for _, node := range input.Nodes {
		filePath := nodeFilePath(node, input.Edges)
		pkgID := findPackageID(filePath, input.Workspace, packageIDs)
		nodeID := makeNodeID(workspaceID, pkgID, filePath, node.QualifiedName)
		nodeIDLookup[node.QualifiedName] = nodeID
		if _, exists := fileNodeLookup[filePath]; !exists {
			fileNodeLookup[filePath] = nodeID
		}
	}

	// Clear old unresolved refs for this workspace before inserting new ones
	_, err := tx.Exec(ctx, `
		DELETE FROM unresolved_refs
		WHERE source_node_id IN (SELECT id FROM nodes WHERE workspace_id = $1)`,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("clearing old unresolved refs: %w", err)
	}

	count := 0
	now := time.Now()

	for i := 0; i < len(input.Unresolved); i += batchSize {
		end := i + batchSize
		if end > len(input.Unresolved) {
			end = len(input.Unresolved)
		}
		chunk := input.Unresolved[i:end]

		batch := &pgx.Batch{}
		batchCount := 0
		for _, ref := range chunk {
			srcID, ok := nodeIDLookup[ref.Source]
			if !ok {
				// ref.Source is often a file path for import edges
				srcID, ok = fileNodeLookup[ref.Source]
			}
			if !ok {
				continue
			}
			batch.Queue(`
				INSERT INTO unresolved_refs (source_node_id, raw_import, kind, line_number, created_at)
				VALUES ($1, $2, $3, $4, $5)`,
				srcID, ref.RawImport, ref.Kind, ref.Line, now,
			)
			batchCount++
		}

		if batchCount == 0 {
			continue
		}

		br := tx.SendBatch(ctx, batch)
		for range batchCount {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("inserting unresolved ref batch: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("closing unresolved ref batch: %w", err)
		}
		count += batchCount
	}

	return count, nil
}

func cleanupStale(ctx context.Context, tx pgx.Tx, workspaceID string, currentFilePaths []string) (int, error) {
	if len(currentFilePaths) == 0 {
		// No files means full cleanup — delete all nodes in workspace
		tag, err := tx.Exec(ctx, "DELETE FROM nodes WHERE workspace_id = $1", workspaceID)
		if err != nil {
			return 0, fmt.Errorf("cleaning up all nodes: %w", err)
		}
		return int(tag.RowsAffected()), nil
	}

	tag, err := tx.Exec(ctx, `
		DELETE FROM nodes
		WHERE workspace_id = $1 AND NOT (file_path = ANY($2))`,
		workspaceID, currentFilePaths,
	)
	if err != nil {
		return 0, fmt.Errorf("cleaning up stale nodes: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// --- ID generation ---

func makeWorkspaceID(projectID, sourceID string) string {
	return fmt.Sprintf("%s/%s", projectID, sourceID)
}

func makePackageID(workspaceID, packageName string) string {
	return fmt.Sprintf("%s/%s", workspaceID, packageName)
}

func makeNodeID(workspaceID, packageID, filePath, qualifiedName string) string {
	prefix := workspaceID
	if packageID != "" {
		prefix = packageID
	}
	return fmt.Sprintf("%s/%s::%s", prefix, filePath, qualifiedName)
}

// --- Helpers ---

// nodeFilePath finds the file path for a node by looking at contains edges.
// Falls back to the qualified name prefix if no contains edge exists.
func nodeFilePath(node parsers.NodeInfo, edges []parsers.EdgeInfo) string {
	for _, e := range edges {
		if e.Kind == "contains" && e.Target == node.QualifiedName {
			return e.Source
		}
	}
	// For top-level nodes, the source of a "contains" edge from a file
	// If no edge, use the qualified name itself (shouldn't happen in practice)
	return node.QualifiedName
}

// findPackageID maps a file path to its containing package.
func findPackageID(filePath string, ws *detectors.WorkspaceInfo, packageIDs map[string]string) string {
	for _, pkg := range ws.Packages {
		if strings.HasPrefix(filePath, pkg.Path) {
			if id, ok := packageIDs[pkg.Name]; ok {
				return id
			}
		}
	}
	return ""
}

func edgeWeight(kind string) float64 {
	switch kind {
	case "contains", "extends", "implements", "embeds":
		return 1.0
	default:
		return 0.5
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func detectLanguage(filePaths []string) string {
	counts := make(map[string]int)
	for _, p := range filePaths {
		ext := filepath.Ext(p)
		switch ext {
		case ".ts", ".tsx":
			counts["typescript"]++
		case ".js", ".jsx":
			counts["javascript"]++
		case ".go":
			counts["go"]++
		}
	}
	best := ""
	bestCount := 0
	for lang, c := range counts {
		if c > bestCount {
			best = lang
			bestCount = c
		}
	}
	return best
}
