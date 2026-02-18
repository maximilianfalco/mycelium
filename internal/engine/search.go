package engine

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/indexer"
)

// SearchResult represents a single semantic search hit.
type SearchResult struct {
	NodeID        string  `json:"nodeId"`
	QualifiedName string  `json:"qualifiedName"`
	FilePath      string  `json:"filePath"`
	Kind          string  `json:"kind"`
	Similarity    float64 `json:"similarity"`
	Signature     string  `json:"signature"`
	SourceCode    string  `json:"sourceCode,omitempty"`
	Docstring     string  `json:"docstring,omitempty"`
}

// SemanticSearch embeds the query text via OpenAI, then runs a pgvector cosine
// similarity search against all indexed nodes in the given project.
func SemanticSearch(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, limit int, kinds []string) ([]SearchResult, error) {
	queryVec, err := indexer.EmbedText(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	return SemanticSearchWithVector(ctx, pool, queryVec, projectID, limit, kinds)
}

// SemanticSearchWithVector runs the pgvector similarity search using a
// pre-computed query vector. Useful for testing without an OpenAI client.
func SemanticSearchWithVector(ctx context.Context, pool *pgxpool.Pool, queryVec []float32, projectID string, limit int, kinds []string) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	vec := pgvector.NewVector(queryVec)

	// cosine distance operator <=> returns distance (0 = identical),
	// so similarity = 1 - distance
	sql := `
		SELECT
			n.id,
			COALESCE(n.qualified_name, n.name),
			n.file_path,
			n.kind,
			1 - (n.embedding <=> $1) AS similarity,
			COALESCE(n.signature, ''),
			COALESCE(n.source_code, ''),
			COALESCE(n.docstring, '')
		FROM nodes n
		JOIN workspaces ws ON n.workspace_id = ws.id
		WHERE ws.project_id = $2
		  AND n.embedding IS NOT NULL`

	args := []any{vec, projectID}
	argIdx := 3

	if len(kinds) > 0 {
		sql += fmt.Sprintf(` AND n.kind = ANY($%d)`, argIdx)
		args = append(args, kinds)
		argIdx++
	}

	sql += fmt.Sprintf(`
		ORDER BY n.embedding <=> $1
		LIMIT $%d`, argIdx)
	args = append(args, limit)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL ivfflat.probes = 10"); err != nil {
		return nil, fmt.Errorf("setting ivfflat.probes: %w", err)
	}

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("querying vectors: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.NodeID, &r.QualifiedName, &r.FilePath, &r.Kind, &r.Similarity, &r.Signature, &r.SourceCode, &r.Docstring); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	if results == nil {
		results = []SearchResult{}
	}

	return results, nil
}
