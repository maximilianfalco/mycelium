package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeResult represents a node returned from a structural query.
type NodeResult struct {
	NodeID        string `json:"nodeId"`
	QualifiedName string `json:"qualifiedName"`
	FilePath      string `json:"filePath"`
	Kind          string `json:"kind"`
	Signature     string `json:"signature"`
	SourceCode    string `json:"sourceCode,omitempty"`
	Docstring     string `json:"docstring,omitempty"`
	Depth         int    `json:"depth,omitempty"`
	SourceAlias   string `json:"sourceAlias,omitempty"`
}

// EdgeResult represents an edge returned from cross-package queries.
type EdgeResult struct {
	SourceNodeID string  `json:"sourceNodeId"`
	SourceQName  string  `json:"sourceQName"`
	TargetNodeID string  `json:"targetNodeId"`
	TargetQName  string  `json:"targetQName"`
	Kind         string  `json:"kind"`
	Weight       float64 `json:"weight"`
	LineNumber   *int    `json:"lineNumber,omitempty"`
}

// FindNodeByQualifiedName looks up a node by its qualified name within a project.
// Returns nil, nil if no matching node is found.
func FindNodeByQualifiedName(ctx context.Context, pool *pgxpool.Pool, projectID, qualifiedName string) (*NodeResult, error) {
	sql := `
		SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
		       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
		       COALESCE(n.docstring, ''), COALESCE(ps.alias, '')
		FROM nodes n
		JOIN workspaces ws ON n.workspace_id = ws.id
		LEFT JOIN project_sources ps ON ws.source_id = ps.id
		WHERE ws.project_id = $1 AND n.qualified_name = $2
		LIMIT 1`

	var r NodeResult
	err := pool.QueryRow(ctx, sql, projectID, qualifiedName).Scan(
		&r.NodeID, &r.QualifiedName, &r.FilePath, &r.Kind, &r.Signature, &r.SourceCode, &r.Docstring, &r.SourceAlias,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding node by qualified name: %w", err)
	}
	return &r, nil
}

// GetCallers returns nodes that call the given node (incoming "calls" edges).
func GetCallers(ctx context.Context, pool *pgxpool.Pool, nodeID string, limit int) ([]NodeResult, error) {
	return getRelated(ctx, pool, nodeID, "calls", "incoming", limit)
}

// GetCallees returns nodes that the given node calls (outgoing "calls" edges).
func GetCallees(ctx context.Context, pool *pgxpool.Pool, nodeID string, limit int) ([]NodeResult, error) {
	return getRelated(ctx, pool, nodeID, "calls", "outgoing", limit)
}

// GetImporters returns nodes that import the given node (incoming "imports" edges).
func GetImporters(ctx context.Context, pool *pgxpool.Pool, nodeID string, limit int) ([]NodeResult, error) {
	return getRelated(ctx, pool, nodeID, "imports", "incoming", limit)
}

// getRelated is the shared implementation for single-hop traversals.
func getRelated(ctx context.Context, pool *pgxpool.Pool, nodeID, edgeKind, direction string, limit int) ([]NodeResult, error) {
	limit = clampLimit(limit)

	var sql string
	if direction == "incoming" {
		// Find nodes that have an edge pointing TO nodeID
		sql = `
			SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
			       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
			       COALESCE(n.docstring, ''), COALESCE(ps.alias, '')
			FROM nodes n
			JOIN edges e ON e.source_id = n.id
			JOIN workspaces ws ON n.workspace_id = ws.id
			LEFT JOIN project_sources ps ON ws.source_id = ps.id
			WHERE e.target_id = $1 AND e.kind = $2
			ORDER BY e.weight DESC
			LIMIT $3`
	} else {
		// Find nodes that nodeID has an edge pointing TO
		sql = `
			SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
			       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
			       COALESCE(n.docstring, ''), COALESCE(ps.alias, '')
			FROM nodes n
			JOIN edges e ON e.target_id = n.id
			JOIN workspaces ws ON n.workspace_id = ws.id
			LEFT JOIN project_sources ps ON ws.source_id = ps.id
			WHERE e.source_id = $1 AND e.kind = $2
			ORDER BY e.weight DESC
			LIMIT $3`
	}

	return queryNodes(ctx, pool, sql, nodeID, edgeKind, limit)
}

// GetDependencies returns all nodes reachable via outgoing calls/imports/uses_type
// edges up to maxDepth hops. Uses a recursive CTE with UNION for cycle safety.
func GetDependencies(ctx context.Context, pool *pgxpool.Pool, nodeID string, maxDepth, limit int) ([]NodeResult, error) {
	return getTransitive(ctx, pool, nodeID, "outgoing", maxDepth, limit)
}

// GetDependents returns all nodes that transitively depend on the given node
// (incoming calls/imports/uses_type edges) up to maxDepth hops.
func GetDependents(ctx context.Context, pool *pgxpool.Pool, nodeID string, maxDepth, limit int) ([]NodeResult, error) {
	return getTransitive(ctx, pool, nodeID, "incoming", maxDepth, limit)
}

func getTransitive(ctx context.Context, pool *pgxpool.Pool, nodeID, direction string, maxDepth, limit int) ([]NodeResult, error) {
	limit = clampLimit(limit)
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	edgeKinds := []string{"calls", "imports", "uses_type"}

	var sql string
	if direction == "outgoing" {
		sql = `
			WITH RECURSIVE traversal AS (
				SELECT e.target_id AS node_id, 1 AS depth
				FROM edges e
				WHERE e.source_id = $1 AND e.kind = ANY($2)
				UNION
				SELECT e.target_id, t.depth + 1
				FROM edges e
				JOIN traversal t ON e.source_id = t.node_id
				WHERE e.kind = ANY($2) AND t.depth < $3
			)
			SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
			       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
			       COALESCE(n.docstring, ''),
			       MIN(t.depth) AS min_depth,
			       COALESCE(ps.alias, '')
			FROM nodes n
			JOIN traversal t ON n.id = t.node_id
			JOIN workspaces ws ON n.workspace_id = ws.id
			LEFT JOIN project_sources ps ON ws.source_id = ps.id
			GROUP BY n.id, n.qualified_name, n.name, n.file_path, n.kind, n.signature, n.source_code, n.docstring, ps.alias
			ORDER BY min_depth, n.qualified_name
			LIMIT $4`
	} else {
		sql = `
			WITH RECURSIVE traversal AS (
				SELECT e.source_id AS node_id, 1 AS depth
				FROM edges e
				WHERE e.target_id = $1 AND e.kind = ANY($2)
				UNION
				SELECT e.source_id, t.depth + 1
				FROM edges e
				JOIN traversal t ON e.target_id = t.node_id
				WHERE e.kind = ANY($2) AND t.depth < $3
			)
			SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
			       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
			       COALESCE(n.docstring, ''),
			       MIN(t.depth) AS min_depth,
			       COALESCE(ps.alias, '')
			FROM nodes n
			JOIN traversal t ON n.id = t.node_id
			JOIN workspaces ws ON n.workspace_id = ws.id
			LEFT JOIN project_sources ps ON ws.source_id = ps.id
			GROUP BY n.id, n.qualified_name, n.name, n.file_path, n.kind, n.signature, n.source_code, n.docstring, ps.alias
			ORDER BY min_depth, n.qualified_name
			LIMIT $4`
	}

	rows, err := pool.Query(ctx, sql, nodeID, edgeKinds, maxDepth, limit)
	if err != nil {
		return nil, fmt.Errorf("transitive query: %w", err)
	}
	defer rows.Close()

	var results []NodeResult
	for rows.Next() {
		var r NodeResult
		if err := rows.Scan(&r.NodeID, &r.QualifiedName, &r.FilePath, &r.Kind, &r.Signature, &r.SourceCode, &r.Docstring, &r.Depth, &r.SourceAlias); err != nil {
			return nil, fmt.Errorf("scanning transitive row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating transitive rows: %w", err)
	}
	if results == nil {
		results = []NodeResult{}
	}
	return results, nil
}

// GetCrossPackageDeps returns all edges between nodes in two packages.
func GetCrossPackageDeps(ctx context.Context, pool *pgxpool.Pool, packageA, packageB string, limit int) ([]EdgeResult, error) {
	limit = clampLimit(limit)

	sql := `
		SELECT e.source_id, COALESCE(n_src.qualified_name, n_src.name),
		       e.target_id, COALESCE(n_tgt.qualified_name, n_tgt.name),
		       e.kind, e.weight, e.line_number
		FROM edges e
		JOIN nodes n_src ON e.source_id = n_src.id
		JOIN nodes n_tgt ON e.target_id = n_tgt.id
		WHERE n_src.package_id = $1 AND n_tgt.package_id = $2
		  AND e.kind IN ('calls', 'imports', 'uses_type', 'extends', 'implements')
		ORDER BY e.weight DESC
		LIMIT $3`

	rows, err := pool.Query(ctx, sql, packageA, packageB, limit)
	if err != nil {
		return nil, fmt.Errorf("cross-package query: %w", err)
	}
	defer rows.Close()

	var results []EdgeResult
	for rows.Next() {
		var r EdgeResult
		if err := rows.Scan(&r.SourceNodeID, &r.SourceQName, &r.TargetNodeID, &r.TargetQName, &r.Kind, &r.Weight, &r.LineNumber); err != nil {
			return nil, fmt.Errorf("scanning cross-package row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cross-package rows: %w", err)
	}
	if results == nil {
		results = []EdgeResult{}
	}
	return results, nil
}

// GetFileContext returns all nodes defined in a specific file within a project.
func GetFileContext(ctx context.Context, pool *pgxpool.Pool, filePath, projectID string) ([]NodeResult, error) {
	sql := `
		SELECT n.id, COALESCE(n.qualified_name, n.name), n.file_path, n.kind,
		       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
		       COALESCE(n.docstring, ''), COALESCE(ps.alias, '')
		FROM nodes n
		JOIN workspaces ws ON n.workspace_id = ws.id
		LEFT JOIN project_sources ps ON ws.source_id = ps.id
		WHERE ws.project_id = $1 AND n.file_path = $2
		ORDER BY n.start_line`

	return queryNodes(ctx, pool, sql, projectID, filePath)
}

// queryNodes is a helper that runs a query and scans results into NodeResult slices.
func queryNodes(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]NodeResult, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	var results []NodeResult
	for rows.Next() {
		var r NodeResult
		if err := rows.Scan(&r.NodeID, &r.QualifiedName, &r.FilePath, &r.Kind, &r.Signature, &r.SourceCode, &r.Docstring, &r.SourceAlias); err != nil {
			return nil, fmt.Errorf("scanning node row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating node rows: %w", err)
	}
	if results == nil {
		results = []NodeResult{}
	}
	return results, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}
