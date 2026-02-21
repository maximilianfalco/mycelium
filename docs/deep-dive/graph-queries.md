# 🗺️ Structural Graph Queries

Mycelium stores code relationships as a directed graph — nodes are code symbols, edges are relationships between them. Structural queries traverse this graph to answer questions like "who calls this function?" or "what does this module depend on?"

## Query Types

| Query | Direction | Edge kinds | Depth | Description |
|---|---|---|---|---|
| `callers` | Incoming | `calls` | 1 hop | Functions that call the target |
| `callees` | Outgoing | `calls` | 1 hop | Functions called by the target |
| `importers` | Incoming | `imports` | 1 hop | Files that import the target |
| `dependencies` | Outgoing | `calls`, `imports`, `uses_type` | Up to 5 hops | Transitive dependencies via recursive CTE |
| `dependents` | Incoming | `calls`, `imports`, `uses_type` | Up to 5 hops | Transitive dependents via recursive CTE |
| `file` | — | `contains` | — | All symbols in the same file |

## How It Works

### Direct Queries (callers, callees, importers)

Simple edge traversal — one SQL join:

```sql
-- Example: callers
SELECT n.id, n.qualified_name, n.file_path, n.kind, n.signature, n.source_code, n.docstring
FROM edges e
JOIN nodes n ON n.id = e.source_id
WHERE e.target_id = $node_id AND e.kind = 'calls'
ORDER BY e.weight DESC
LIMIT $limit
```

### Transitive Queries (dependencies, dependents)

Uses Postgres recursive CTEs to walk the graph up to N hops:

```sql
WITH RECURSIVE deps AS (
    -- Base case: direct edges from the target node
    SELECT e.target_id AS id, 1 AS depth
    FROM edges e WHERE e.source_id = $node_id

    UNION

    -- Recursive case: follow edges from discovered nodes
    SELECT e.target_id, d.depth + 1
    FROM edges e
    JOIN deps d ON e.source_id = d.id
    WHERE d.depth < $max_depth
)
SELECT DISTINCT n.* FROM deps d JOIN nodes n ON n.id = d.id
```

Depth defaults to 5 hops. This is enough to trace most dependency chains without exploding on circular references (the `UNION` deduplicates).

### File Context

Returns all nodes with the same `file_path`:

```sql
SELECT n.* FROM nodes n
JOIN workspaces ws ON n.workspace_id = ws.id
WHERE n.file_path = $file_path AND ws.project_id = $project_id
ORDER BY n.start_line
```

## Node Lookup

All structural queries require a node ID. The entry point is `FindNodeByQualifiedName`:

```go
func FindNodeByQualifiedName(ctx, pool, projectID, qualifiedName string) (*NodeResult, error)
```

This looks up a node by its qualified name within a project. The MCP `explore` tool and the API `/search/structural` endpoint use this internally to resolve symbols before running graph queries.

## Edge Kinds

| Kind | Source → Target | Created by |
|---|---|---|
| `imports` | File → Module/File | Import resolution (stage 4) |
| `calls` | Function → Function | Parser (stage 3) |
| `extends` | Class → Class | Parser |
| `implements` | Class → Interface | Parser |
| `contains` | File → Symbol | Parser |
| `uses_type` | Function → Type | Parser |
| `depends_on` | Package → Package | Import resolution |

## Edge Weights

| Kind | Weight | Rationale |
|---|---|---|
| `contains`, `extends`, `implements`, `embeds` | 1.0 | Structural, always relevant |
| `imports`, `calls`, `depends_on`, `uses_type` | 0.5 | Less direct relationship |

Higher-weight edges are returned first in query results.

## Result Format

All queries return `[]NodeResult`:

```go
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
```
