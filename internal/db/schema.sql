CREATE EXTENSION IF NOT EXISTS vector;

-- Top-level user-facing grouping
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Each local repo/directory linked to a project
CREATE TABLE project_sources (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    source_type TEXT NOT NULL,
    is_code BOOLEAN DEFAULT TRUE,
    alias TEXT,
    last_indexed_commit TEXT,
    last_indexed_branch TEXT,
    last_indexed_at TIMESTAMP,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- A monorepo root or standalone repo
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_id TEXT REFERENCES project_sources(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    workspace_type TEXT,
    package_manager TEXT,
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- A package within a workspace
CREATE TABLE packages (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    version TEXT,
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Every code symbol
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    package_id TEXT REFERENCES packages(id) ON DELETE SET NULL,
    file_path TEXT NOT NULL,
    name TEXT NOT NULL,
    qualified_name TEXT,
    kind TEXT NOT NULL,
    language TEXT,
    signature TEXT,
    start_line INTEGER,
    end_line INTEGER,
    source_code TEXT,
    docstring TEXT,
    body_hash TEXT,
    embedding vector(1536),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Relationships between nodes
CREATE TABLE edges (
    source_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    weight FLOAT DEFAULT 1.0,
    line_number INTEGER,
    metadata JSONB,
    PRIMARY KEY (source_id, target_id, kind)
);

-- Imports/calls that couldn't be resolved
CREATE TABLE unresolved_refs (
    id SERIAL PRIMARY KEY,
    source_node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    raw_import TEXT NOT NULL,
    kind TEXT NOT NULL,
    line_number INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_project_sources_project ON project_sources(project_id);
CREATE INDEX idx_workspaces_project ON workspaces(project_id);
CREATE INDEX idx_nodes_workspace ON nodes(workspace_id);
CREATE INDEX idx_nodes_package ON nodes(package_id);
CREATE INDEX idx_nodes_file ON nodes(file_path);
CREATE INDEX idx_nodes_kind ON nodes(kind);
CREATE INDEX idx_nodes_name ON nodes(name);
CREATE INDEX idx_nodes_qualified ON nodes(qualified_name);

CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_edges_kind ON edges(kind);

-- Vector similarity search (IVFFlat)
-- Note: IVFFlat requires rows to exist before building the index.
-- This index will be created empty and rebuilt after first data load.
CREATE INDEX idx_nodes_embedding ON nodes
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Full-text search support (hybrid search with RRF)
-- Generated column auto-maintained by Postgres on insert/update.
-- Weights: A = name/qualified_name, B = signature, C = docstring.
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(qualified_name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(signature, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(docstring, '')), 'C')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_nodes_search_vector ON nodes USING GIN (search_vector);
