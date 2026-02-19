package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	openai "github.com/sashabaranov/go-openai"

	"github.com/maximilianfalco/mycelium/internal/engine"
	"github.com/maximilianfalco/mycelium/internal/projects"
)

// NewServer creates an MCP server with mycelium's code intelligence tools.
func NewServer(pool *pgxpool.Pool, client *openai.Client) *server.MCPServer {
	s := server.NewMCPServer(
		"mycelium",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(searchTool(), searchHandler(pool, client))
	s.AddTool(queryGraphTool(), queryGraphHandler(pool))
	s.AddTool(listProjectsTool(), listProjectsHandler(pool))
	s.AddTool(detectProjectTool(), detectProjectHandler(pool))

	return s
}

// --- Tool definitions ---

func searchTool() mcp.Tool {
	return mcp.NewTool("search",
		mcp.WithDescription("Semantic search across indexed code in a project. Returns matching symbols with source code, file paths, signatures, docstrings, and similarity scores."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Natural language search query (e.g. 'authentication middleware', 'database connection pool')"),
		),
		mcp.WithString("project_id",
			mcp.Required(),
			mcp.Description("Project/colony ID. Use detect_project with your cwd to auto-detect, or list_projects to discover available IDs."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (1-100, default 10)"),
		),
		mcp.WithString("kinds",
			mcp.Description("Comma-separated node kinds to filter (e.g. 'function,class,interface')"),
		),
	)
}

func queryGraphTool() mcp.Tool {
	return mcp.NewTool("query_graph",
		mcp.WithDescription("Query the structural code graph to find callers, callees, importers, dependencies, or dependents of a symbol. First finds the symbol by qualified name, then traverses the graph."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("qualified_name",
			mcp.Required(),
			mcp.Description("Qualified name of the symbol to query (e.g. 'MyClass.myMethod', 'handleRequest'). Use 'search' first to discover qualified names."),
		),
		mcp.WithString("project_id",
			mcp.Required(),
			mcp.Description("Project/colony ID. Use detect_project with your cwd to auto-detect, or list_projects to discover available IDs."),
		),
		mcp.WithString("query_type",
			mcp.Required(),
			mcp.Description("Type of graph query to perform"),
			mcp.Enum("callers", "callees", "importers", "dependencies", "dependents", "file"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (1-100, default 10)"),
		),
	)
}

func listProjectsTool() mcp.Tool {
	return mcp.NewTool("list_projects",
		mcp.WithDescription("List all available projects/colonies. Returns project IDs, names, and descriptions. Use project IDs with the search and query_graph tools."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

func detectProjectTool() mcp.Tool {
	return mcp.NewTool("detect_project",
		mcp.WithDescription("Auto-detect which project/colony a directory belongs to by matching against indexed source paths. Pass the current working directory to get the project ID without needing to call list_projects. Returns the project ID, name, and matched source path."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute directory path to match (e.g. your current working directory)"),
		),
	)
}

// --- Tool handlers ---

func searchHandler(pool *pgxpool.Pool, client *openai.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: query"), nil
		}
		projectID, err := req.RequireString("project_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: project_id"), nil
		}

		limit := req.GetInt("limit", 10)

		var kinds []string
		if k := req.GetString("kinds", ""); k != "" {
			kinds = strings.Split(k, ",")
			for i := range kinds {
				kinds[i] = strings.TrimSpace(kinds[i])
			}
		}

		results, err := engine.HybridSearch(ctx, pool, client, query, projectID, limit, kinds)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText("No results found."), nil
		}

		return mcp.NewToolResultText(formatSearchResults(results)), nil
	}
}

func queryGraphHandler(pool *pgxpool.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		qualifiedName, err := req.RequireString("qualified_name")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: qualified_name"), nil
		}
		projectID, err := req.RequireString("project_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: project_id"), nil
		}
		queryType, err := req.RequireString("query_type")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: query_type"), nil
		}

		limit := req.GetInt("limit", 10)

		// For "file" queries, qualifiedName is actually a file path
		if queryType == "file" {
			results, err := engine.GetFileContext(ctx, pool, qualifiedName, projectID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("file query failed: %v", err)), nil
			}
			if len(results) == 0 {
				return mcp.NewToolResultText("No symbols found in this file."), nil
			}
			return mcp.NewToolResultText(formatNodeResults(results, "file")), nil
		}

		// Look up the node first
		node, err := engine.FindNodeByQualifiedName(ctx, pool, projectID, qualifiedName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("lookup failed: %v", err)), nil
		}
		if node == nil {
			return mcp.NewToolResultError(fmt.Sprintf("symbol %q not found in project %q", qualifiedName, projectID)), nil
		}

		var results []engine.NodeResult
		switch queryType {
		case "callers":
			results, err = engine.GetCallers(ctx, pool, node.NodeID, limit)
		case "callees":
			results, err = engine.GetCallees(ctx, pool, node.NodeID, limit)
		case "importers":
			results, err = engine.GetImporters(ctx, pool, node.NodeID, limit)
		case "dependencies":
			results, err = engine.GetDependencies(ctx, pool, node.NodeID, 5, limit)
		case "dependents":
			results, err = engine.GetDependents(ctx, pool, node.NodeID, 5, limit)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unknown query_type: %q", queryType)), nil
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("graph query failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No %s found for %q.", queryType, qualifiedName)), nil
		}

		return mcp.NewToolResultText(formatNodeResults(results, queryType)), nil
	}
}

func listProjectsHandler(pool *pgxpool.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ps, err := projects.ListProjects(ctx, pool)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
		}

		if len(ps) == 0 {
			return mcp.NewToolResultText("No projects found. Create a colony in the mycelium UI first."), nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("## %d project(s)\n\n", len(ps)))
		for _, p := range ps {
			b.WriteString(fmt.Sprintf("- **%s** (id: `%s`)", p.Name, p.ID))
			if p.Description != "" {
				b.WriteString(fmt.Sprintf(" — %s", p.Description))
			}
			b.WriteByte('\n')
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

func detectProjectHandler(pool *pgxpool.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dirPath, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: path"), nil
		}

		project, source, err := projects.DetectProjectByPath(ctx, pool, dirPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("detection failed: %v", err)), nil
		}

		if project == nil {
			return mcp.NewToolResultText("No matching project found for this directory. Use list_projects to see available projects, or index this directory first via the mycelium UI."), nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("## Detected project\n\n"))
		b.WriteString(fmt.Sprintf("- **Project:** %s\n", project.Name))
		b.WriteString(fmt.Sprintf("- **Project ID:** `%s`\n", project.ID))
		b.WriteString(fmt.Sprintf("- **Matched source:** %s\n", source.Path))
		if source.Alias != "" {
			b.WriteString(fmt.Sprintf("- **Alias:** %s\n", source.Alias))
		}
		b.WriteString(fmt.Sprintf("\nUse `%s` as the `project_id` for search and query_graph tools.", project.ID))

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- Formatters ---

func formatSearchResults(results []engine.SearchResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %d result(s)\n\n", len(results)))

	for _, r := range results {
		b.WriteString(fmt.Sprintf("### `%s` (%s) — %.2f similarity\n", r.QualifiedName, r.Kind, r.Similarity))
		b.WriteString(fmt.Sprintf("**File:** %s\n", r.FilePath))
		if r.Signature != "" {
			b.WriteString(fmt.Sprintf("**Signature:** `%s`\n", r.Signature))
		}
		if r.Docstring != "" {
			b.WriteString(fmt.Sprintf("**Docstring:** %s\n", r.Docstring))
		}
		if r.SourceCode != "" {
			lang := langFromPath(r.FilePath)
			b.WriteString(fmt.Sprintf("\n```%s\n%s\n```\n", lang, r.SourceCode))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func formatNodeResults(results []engine.NodeResult, queryType string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %d %s result(s)\n\n", len(results), queryType))

	for _, r := range results {
		b.WriteString(fmt.Sprintf("### `%s` (%s)\n", r.QualifiedName, r.Kind))
		b.WriteString(fmt.Sprintf("**File:** %s\n", r.FilePath))
		if r.Signature != "" {
			b.WriteString(fmt.Sprintf("**Signature:** `%s`\n", r.Signature))
		}
		if r.Docstring != "" {
			b.WriteString(fmt.Sprintf("**Docstring:** %s\n", r.Docstring))
		}
		if r.Depth > 0 {
			b.WriteString(fmt.Sprintf("**Depth:** %d\n", r.Depth))
		}
		if r.SourceCode != "" {
			lang := langFromPath(r.FilePath)
			b.WriteString(fmt.Sprintf("\n```%s\n%s\n```\n", lang, r.SourceCode))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func langFromPath(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, ".ts"), strings.HasSuffix(filePath, ".tsx"):
		return "typescript"
	case strings.HasSuffix(filePath, ".js"), strings.HasSuffix(filePath, ".jsx"):
		return "javascript"
	case strings.HasSuffix(filePath, ".go"):
		return "go"
	default:
		return ""
	}
}
