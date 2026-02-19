-- Migration: Add full-text search vector for hybrid search (RRF)
-- Run once on existing databases:
--   docker exec mycelium-db-1 psql -U mycelium -d mycelium -f /dev/stdin < internal/db/migrations/002_add_search_vector.sql

SET maintenance_work_mem = '256MB';

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(qualified_name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(signature, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(docstring, '')), 'C')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_nodes_search_vector ON nodes USING GIN (search_vector);
