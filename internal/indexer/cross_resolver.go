package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrossResolveResult summarizes the outcome of cross-source import resolution.
type CrossResolveResult struct {
	ResolvedCount        int `json:"resolvedCount"`
	StillUnresolvedCount int `json:"stillUnresolvedCount"`
	EdgesCreated         int `json:"edgesCreated"`
}

type unresolvedEntry struct {
	ID           int
	SourceNodeID string
	RawImport    string
	Kind         string
	Line         int
	WorkspaceID  string
}

type packageEntry struct {
	ID          string
	WorkspaceID string
	Name        string
	Path        string
}

type crossEdge struct {
	sourceNodeID string
	targetNodeID string
	kind         string
	line         int
}

// ResolveCrossSources resolves imports that couldn't be resolved within a single
// source's workspace by matching against packages from sibling workspaces in the
// same project. It creates cross-workspace "imports" edges and removes resolved
// entries from unresolved_refs.
func ResolveCrossSources(ctx context.Context, pool *pgxpool.Pool, projectID string) (*CrossResolveResult, error) {
	start := time.Now()
	result := &CrossResolveResult{}

	refs, err := loadProjectUnresolvedRefs(ctx, pool, projectID)
	if err != nil {
		return nil, fmt.Errorf("loading unresolved refs: %w", err)
	}

	if len(refs) == 0 {
		return result, nil
	}

	packages, err := loadProjectPackages(ctx, pool, projectID)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	pkgByName := make(map[string][]packageEntry)
	for _, pkg := range packages {
		pkgByName[pkg.Name] = append(pkgByName[pkg.Name], pkg)
	}

	var resolvedIDs []int
	var edges []crossEdge

	for _, ref := range refs {
		if ref.Kind != "imports" {
			continue
		}

		pkgName, subpath := splitSpecifier(ref.RawImport)

		candidates, ok := pkgByName[pkgName]
		if !ok {
			continue
		}

		for _, pkg := range candidates {
			if pkg.WorkspaceID == ref.WorkspaceID {
				continue
			}

			targetNodeID, err := findTargetNode(ctx, pool, pkg, subpath)
			if err != nil {
				slog.Warn("cross-resolve: target lookup failed",
					"specifier", ref.RawImport, "package", pkg.Name, "error", err)
				continue
			}
			if targetNodeID == "" {
				continue
			}

			edges = append(edges, crossEdge{
				sourceNodeID: ref.SourceNodeID,
				targetNodeID: targetNodeID,
				kind:         "imports",
				line:         ref.Line,
			})

			resolvedIDs = append(resolvedIDs, ref.ID)
			break
		}
	}

	if len(edges) == 0 {
		result.StillUnresolvedCount = len(refs)
		return result, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	edgesCreated, err := insertCrossEdges(ctx, tx, edges)
	if err != nil {
		return nil, fmt.Errorf("inserting cross edges: %w", err)
	}

	if err := deleteResolvedRefs(ctx, tx, resolvedIDs); err != nil {
		return nil, fmt.Errorf("deleting resolved refs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	result.ResolvedCount = len(resolvedIDs)
	result.StillUnresolvedCount = len(refs) - len(resolvedIDs)
	result.EdgesCreated = edgesCreated

	slog.Info("cross-source resolution complete",
		"project", projectID,
		"resolved", result.ResolvedCount,
		"stillUnresolved", result.StillUnresolvedCount,
		"edgesCreated", result.EdgesCreated,
		"duration", time.Since(start),
	)

	return result, nil
}

// loadProjectUnresolvedRefs returns all unresolved_refs for nodes in workspaces
// belonging to the given project.
func loadProjectUnresolvedRefs(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]unresolvedEntry, error) {
	rows, err := pool.Query(ctx, `
		SELECT ur.id, ur.source_node_id, ur.raw_import, ur.kind, ur.line_number, n.workspace_id
		FROM unresolved_refs ur
		JOIN nodes n ON ur.source_node_id = n.id
		JOIN workspaces w ON n.workspace_id = w.id
		WHERE w.project_id = $1`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying unresolved refs: %w", err)
	}
	defer rows.Close()

	var refs []unresolvedEntry
	for rows.Next() {
		var r unresolvedEntry
		if err := rows.Scan(&r.ID, &r.SourceNodeID, &r.RawImport, &r.Kind, &r.Line, &r.WorkspaceID); err != nil {
			return nil, fmt.Errorf("scanning unresolved ref: %w", err)
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// loadProjectPackages returns all packages across all workspaces in the project.
func loadProjectPackages(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]packageEntry, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, p.workspace_id, p.name, p.path
		FROM packages p
		JOIN workspaces w ON p.workspace_id = w.id
		WHERE w.project_id = $1`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying packages: %w", err)
	}
	defer rows.Close()

	var packages []packageEntry
	for rows.Next() {
		var p packageEntry
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Path); err != nil {
			return nil, fmt.Errorf("scanning package: %w", err)
		}
		packages = append(packages, p)
	}
	return packages, rows.Err()
}

// splitSpecifier separates a package name from an optional subpath.
// "@company/auth/validators" -> ("@company/auth", "validators")
// "@company/auth"            -> ("@company/auth", "")
// "lodash/fp"                -> ("lodash", "fp")
// "lodash"                   -> ("lodash", "")
func splitSpecifier(specifier string) (string, string) {
	if strings.HasPrefix(specifier, "@") {
		// Scoped package: @scope/name[/subpath]
		slash := strings.Index(specifier, "/")
		if slash == -1 {
			return specifier, ""
		}
		secondSlash := strings.Index(specifier[slash+1:], "/")
		if secondSlash == -1 {
			return specifier, ""
		}
		boundary := slash + 1 + secondSlash
		return specifier[:boundary], specifier[boundary+1:]
	}

	// Unscoped package: name[/subpath]
	slash := strings.Index(specifier, "/")
	if slash == -1 {
		return specifier, ""
	}
	return specifier[:slash], specifier[slash+1:]
}

// entryPointFiles are the common entry point file patterns, ordered by priority.
var entryPointFiles = []string{
	"src/index.ts",
	"src/index.tsx",
	"src/index.js",
	"src/index.jsx",
	"index.ts",
	"index.tsx",
	"index.js",
	"index.jsx",
	"main.go",
}

// findTargetNode looks up a node in the target package that best represents the
// import target. If subpath is empty, it returns the first node in a common entry
// point file. If subpath is given, it looks for a file matching the subpath.
func findTargetNode(ctx context.Context, pool *pgxpool.Pool, pkg packageEntry, subpath string) (string, error) {
	if subpath != "" {
		return findNodeBySubpath(ctx, pool, pkg.ID, pkg.Path, subpath)
	}
	return findEntryPointNode(ctx, pool, pkg.ID, pkg.Path)
}

// findEntryPointNode finds a node in a common entry point file within the package.
func findEntryPointNode(ctx context.Context, pool *pgxpool.Pool, packageID, packagePath string) (string, error) {
	// Try each entry point pattern
	for _, ep := range entryPointFiles {
		var filePath string
		if packagePath == "." || packagePath == "" {
			filePath = ep
		} else {
			filePath = packagePath + "/" + ep
		}

		var nodeID string
		err := pool.QueryRow(ctx, `
			SELECT id FROM nodes
			WHERE package_id = $1 AND file_path = $2
			ORDER BY start_line ASC
			LIMIT 1`,
			packageID, filePath,
		).Scan(&nodeID)

		if err == nil {
			return nodeID, nil
		}
	}

	// Try just the package path as file_path prefix
	var nodeID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM nodes
		WHERE package_id = $1
		ORDER BY file_path ASC, start_line ASC
		LIMIT 1`,
		packageID,
	).Scan(&nodeID)
	if err != nil {
		return "", nil
	}
	return nodeID, nil
}

// findNodeBySubpath looks for a node matching a subpath within a package.
// e.g., for subpath "validators", looks for files like "src/validators.ts"
func findNodeBySubpath(ctx context.Context, pool *pgxpool.Pool, packageID, packagePath, subpath string) (string, error) {
	// Build candidate file paths from subpath
	candidates := []string{}

	bases := []string{subpath}
	if packagePath != "." && packagePath != "" {
		bases = append(bases, packagePath+"/"+subpath)
		bases = append(bases, packagePath+"/src/"+subpath)
	}

	for _, base := range bases {
		for _, ext := range tsExtensions {
			candidates = append(candidates, base+ext)
		}
		for _, ext := range tsExtensions {
			candidates = append(candidates, base+"/index"+ext)
		}
		// Also try .go
		candidates = append(candidates, base+".go")
	}

	for _, candidate := range candidates {
		var nodeID string
		err := pool.QueryRow(ctx, `
			SELECT id FROM nodes
			WHERE package_id = $1 AND file_path = $2
			ORDER BY start_line ASC
			LIMIT 1`,
			packageID, candidate,
		).Scan(&nodeID)
		if err == nil {
			return nodeID, nil
		}
	}

	// Fallback: search by file_path LIKE
	likePattern := "%" + subpath + "%"
	var nodeID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM nodes
		WHERE package_id = $1 AND file_path LIKE $2
		ORDER BY file_path ASC, start_line ASC
		LIMIT 1`,
		packageID, likePattern,
	).Scan(&nodeID)
	if err != nil {
		return "", nil
	}
	return nodeID, nil
}

// insertCrossEdges upserts cross-workspace edges into the edges table.
func insertCrossEdges(ctx context.Context, tx pgx.Tx, edges []crossEdge) (int, error) {
	// Deduplicate
	type edgeKey struct{ src, tgt, kind string }
	seen := make(map[edgeKey]bool)
	var unique []crossEdge
	for _, e := range edges {
		key := edgeKey{e.sourceNodeID, e.targetNodeID, e.kind}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, e)
		}
	}

	count := 0
	for i := 0; i < len(unique); i += batchSize {
		end := i + batchSize
		if end > len(unique) {
			end = len(unique)
		}
		chunk := unique[i:end]

		batch := &pgx.Batch{}
		for _, e := range chunk {
			batch.Queue(`
				INSERT INTO edges (source_id, target_id, kind, weight, line_number)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (source_id, target_id, kind) DO UPDATE SET
					weight = EXCLUDED.weight,
					line_number = EXCLUDED.line_number`,
				e.sourceNodeID, e.targetNodeID, e.kind, 0.5, e.line,
			)
		}

		br := tx.SendBatch(ctx, batch)
		for range chunk {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("upserting cross edge: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("closing cross edge batch: %w", err)
		}
		count += len(chunk)
	}

	return count, nil
}

// deleteResolvedRefs removes unresolved_refs entries by ID.
func deleteResolvedRefs(ctx context.Context, tx pgx.Tx, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	// Batch delete in chunks
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		batch := &pgx.Batch{}
		for _, id := range chunk {
			batch.Queue("DELETE FROM unresolved_refs WHERE id = $1", id)
		}

		br := tx.SendBatch(ctx, batch)
		for range chunk {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("deleting unresolved ref: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return fmt.Errorf("closing delete batch: %w", err)
		}
	}

	return nil
}
