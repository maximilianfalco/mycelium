package engine

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GraphVizNode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	Kind          string `json:"kind"`
	FilePath      string `json:"filePath"`
	SourceAlias   string `json:"sourceAlias"`
	Degree        int    `json:"degree"`
}

type GraphVizEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

type GraphVizStats struct {
	NodeCount int            `json:"nodeCount"`
	EdgeCount int            `json:"edgeCount"`
	Kinds     map[string]int `json:"kinds"`
}

type GraphVizData struct {
	Nodes []GraphVizNode `json:"nodes"`
	Edges []GraphVizEdge `json:"edges"`
	Stats GraphVizStats  `json:"stats"`
}

func GetProjectGraph(ctx context.Context, pool *pgxpool.Pool, projectID string) (*GraphVizData, error) {
	nodeSQL := `
		SELECT n.id, n.name, COALESCE(n.qualified_name, n.name), n.kind, n.file_path,
		       COALESCE(ps.alias, ''),
		       COALESCE(deg.degree, 0)
		FROM nodes n
		JOIN workspaces ws ON n.workspace_id = ws.id
		LEFT JOIN project_sources ps ON ws.source_id = ps.id
		LEFT JOIN (
			SELECT node_id, COUNT(*) AS degree FROM (
				SELECT source_id AS node_id FROM edges WHERE kind != 'contains'
				UNION ALL
				SELECT target_id AS node_id FROM edges WHERE kind != 'contains'
			) sub
			GROUP BY node_id
		) deg ON deg.node_id = n.id
		WHERE ws.project_id = $1`

	nodeRows, err := pool.Query(ctx, nodeSQL, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying graph nodes: %w", err)
	}
	defer nodeRows.Close()

	var nodes []GraphVizNode
	kinds := make(map[string]int)
	for nodeRows.Next() {
		var n GraphVizNode
		if err := nodeRows.Scan(&n.ID, &n.Name, &n.QualifiedName, &n.Kind, &n.FilePath, &n.SourceAlias, &n.Degree); err != nil {
			return nil, fmt.Errorf("scanning graph node: %w", err)
		}
		kinds[n.Kind]++
		nodes = append(nodes, n)
	}
	if err := nodeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating graph nodes: %w", err)
	}
	if nodes == nil {
		nodes = []GraphVizNode{}
	}

	edgeSQL := `
		SELECT e.source_id, e.target_id, e.kind
		FROM edges e
		JOIN nodes n ON e.source_id = n.id
		JOIN workspaces ws ON n.workspace_id = ws.id
		WHERE ws.project_id = $1 AND e.kind != 'contains'`

	edgeRows, err := pool.Query(ctx, edgeSQL, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying graph edges: %w", err)
	}
	defer edgeRows.Close()

	var edges []GraphVizEdge
	for edgeRows.Next() {
		var e GraphVizEdge
		if err := edgeRows.Scan(&e.Source, &e.Target, &e.Kind); err != nil {
			return nil, fmt.Errorf("scanning graph edge: %w", err)
		}
		edges = append(edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating graph edges: %w", err)
	}
	if edges == nil {
		edges = []GraphVizEdge{}
	}

	return &GraphVizData{
		Nodes: nodes,
		Edges: edges,
		Stats: GraphVizStats{
			NodeCount: len(nodes),
			EdgeCount: len(edges),
			Kinds:     kinds,
		},
	}, nil
}

type GraphNodeDetail struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	Kind          string `json:"kind"`
	FilePath      string `json:"filePath"`
	Signature     string `json:"signature"`
	SourceCode    string `json:"sourceCode"`
	Docstring     string `json:"docstring"`
	SourceAlias   string `json:"sourceAlias"`
	Callers       int    `json:"callers"`
	Callees       int    `json:"callees"`
	Importers     int    `json:"importers"`
}

func GetGraphNodeDetail(ctx context.Context, pool *pgxpool.Pool, projectID, nodeID string) (*GraphNodeDetail, error) {
	sql := `
		SELECT n.id, n.name, COALESCE(n.qualified_name, n.name), n.kind, n.file_path,
		       COALESCE(n.signature, ''), COALESCE(n.source_code, ''),
		       COALESCE(n.docstring, ''), COALESCE(ps.alias, ''),
		       (SELECT COUNT(*) FROM edges WHERE target_id = n.id AND kind = 'calls'),
		       (SELECT COUNT(*) FROM edges WHERE source_id = n.id AND kind = 'calls'),
		       (SELECT COUNT(*) FROM edges WHERE target_id = n.id AND kind = 'imports')
		FROM nodes n
		JOIN workspaces ws ON n.workspace_id = ws.id
		LEFT JOIN project_sources ps ON ws.source_id = ps.id
		WHERE ws.project_id = $1 AND n.id = $2
		LIMIT 1`

	var d GraphNodeDetail
	err := pool.QueryRow(ctx, sql, projectID, nodeID).Scan(
		&d.ID, &d.Name, &d.QualifiedName, &d.Kind, &d.FilePath,
		&d.Signature, &d.SourceCode, &d.Docstring, &d.SourceAlias,
		&d.Callers, &d.Callees, &d.Importers,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("querying graph node detail: %w", err)
	}
	return &d, nil
}
