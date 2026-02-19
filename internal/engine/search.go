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

// HybridSearch combines vector similarity with Postgres full-text search using
// Reciprocal Rank Fusion (RRF). Keyword matches boost exact symbol name hits
// while semantic search preserves conceptual relevance.
func HybridSearch(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, limit int, kinds []string) ([]SearchResult, error) {
	queryVec, err := indexer.EmbedText(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	return HybridSearchWithVector(ctx, pool, queryVec, query, projectID, limit, kinds)
}

// HybridSearchWithVector runs both vector cosine similarity and full-text keyword
// search, then merges results via RRF scoring. The query string is used for
// keyword matching while the vector is used for semantic similarity.
func HybridSearchWithVector(ctx context.Context, pool *pgxpool.Pool, queryVec []float32, query string, projectID string, limit int, kinds []string) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	vec := pgvector.NewVector(queryVec)

	// Fetch more candidates from each source than the final limit so RRF
	// fusion has enough to work with.
	candidateLimit := limit * 3
	if candidateLimit > 200 {
		candidateLimit = 200
	}

	// Build the hybrid query with two CTEs (vector + keyword) merged via RRF.
	// RRF formula: score = 1/(k + rank_vector) + 1/(k + rank_keyword), k=60.
	sql := `
		WITH vector_results AS (
			SELECT n.id, ROW_NUMBER() OVER (ORDER BY n.embedding <=> $1) AS rank_v
			FROM nodes n
			JOIN workspaces ws ON n.workspace_id = ws.id
			WHERE ws.project_id = $2
			  AND n.embedding IS NOT NULL`

	args := []any{vec, projectID, query}
	argIdx := 4

	if len(kinds) > 0 {
		sql += fmt.Sprintf(` AND n.kind = ANY($%d)`, argIdx)
		args = append(args, kinds)
		argIdx++
	}

	sql += fmt.Sprintf(`
			ORDER BY n.embedding <=> $1
			LIMIT $%d
		),
		keyword_results AS (
			SELECT n.id, ROW_NUMBER() OVER (ORDER BY ts_rank(n.search_vector, query) DESC) AS rank_k
			FROM nodes n
			JOIN workspaces ws ON n.workspace_id = ws.id,
				 plainto_tsquery('english', $3) query
			WHERE ws.project_id = $2
			  AND n.search_vector @@ plainto_tsquery('english', $3)`, argIdx)
	args = append(args, candidateLimit)
	argIdx++

	if len(kinds) > 0 {
		// kinds was already appended; reuse the same parameter index
		kindsArgIdx := 4 // always $4 when kinds are present
		sql += fmt.Sprintf(` AND n.kind = ANY($%d)`, kindsArgIdx)
	}

	sql += fmt.Sprintf(`
			ORDER BY ts_rank(n.search_vector, query) DESC
			LIMIT $%d
		),
		fused AS (
			SELECT
				COALESCE(v.id, k.id) AS id,
				COALESCE(1.0 / (60 + v.rank_v), 0) + COALESCE(1.0 / (60 + k.rank_k), 0) AS rrf_score
			FROM vector_results v
			FULL OUTER JOIN keyword_results k ON v.id = k.id
			ORDER BY rrf_score DESC
			LIMIT $%d
		)
		SELECT
			n.id,
			COALESCE(n.qualified_name, n.name),
			n.file_path,
			n.kind,
			f.rrf_score,
			COALESCE(n.signature, ''),
			COALESCE(n.source_code, ''),
			COALESCE(n.docstring, '')
		FROM fused f
		JOIN nodes n ON f.id = n.id
		ORDER BY f.rrf_score DESC`, argIdx, argIdx+1)
	args = append(args, candidateLimit, limit)

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
		return nil, fmt.Errorf("querying hybrid search: %w", err)
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
