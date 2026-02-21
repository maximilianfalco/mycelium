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

	s.AddTool(exploreTool(), exploreHandler(pool, client))
	s.AddTool(listProjectsTool(), listProjectsHandler(pool))
	s.AddTool(detectProjectTool(), detectProjectHandler(pool))

	return s
}

// --- Tool definitions ---

func listProjectsTool() mcp.Tool {
	return mcp.NewTool("list_projects",
		mcp.WithDescription("List all available projects with IDs, names, and descriptions."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

func detectProjectTool() mcp.Tool {
	return mcp.NewTool("detect_project",
		mcp.WithDescription("Auto-detect which project a directory belongs to. Usually not needed — explore accepts a 'path' param directly."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute directory path to match (e.g. your cwd)"),
		),
	)
}

func exploreTool() mcp.Tool {
	return mcp.NewTool("explore",
		mcp.WithDescription("Hybrid search (keyword + semantic via RRF) across indexed code in a project. Returns matching symbols with file paths, signatures, and docstrings. Top results include full source code."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("query",
			mcp.Description("Natural language search query (e.g. 'authentication middleware', 'database connection pool')"),
		),
		mcp.WithArray("queries",
			mcp.Description("Multiple search queries to run in a single call. Use this to batch questions and minimize round-trips."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("project_id",
			mcp.Description("Project/colony ID. Use detect_project with your cwd to auto-detect, or list_projects to discover available IDs."),
		),
		mcp.WithString("path",
			mcp.Description("Absolute directory path for auto-detecting the project (e.g. your cwd)."),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget for the response (default 8000)."),
		),
	)
}

// --- Shared helpers ---

// resolveProjectID extracts the project ID from the request, trying project_id
// first and falling back to path-based auto-detection.
func resolveProjectID(ctx context.Context, pool *pgxpool.Pool, req mcp.CallToolRequest) (string, error) {
	if pid := req.GetString("project_id", ""); pid != "" {
		return pid, nil
	}
	if p := req.GetString("path", ""); p != "" {
		project, _, err := projects.DetectProjectByPath(ctx, pool, p)
		if err != nil {
			return "", fmt.Errorf("auto-detect failed: %w", err)
		}
		if project == nil {
			return "", fmt.Errorf("no project found for path %q — use list_projects to find the ID", p)
		}
		return project.ID, nil
	}
	return "", fmt.Errorf("provide either project_id or path for auto-detection")
}

// --- Tool handlers ---

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
		b.WriteString(fmt.Sprintf("\nUse `%s` as the `project_id` for the explore tool.", project.ID))

		return mcp.NewToolResultText(b.String()), nil
	}
}

func exploreHandler(pool *pgxpool.Pool, client *openai.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Accept either "query" (single) or "queries" (batch)
		var queries []string
		if q := req.GetString("query", ""); q != "" {
			queries = append(queries, q)
		}
		if qs := req.GetStringSlice("queries", nil); len(qs) > 0 {
			queries = append(queries, qs...)
		}
		if len(queries) == 0 {
			return mcp.NewToolResultError("provide either 'query' (string) or 'queries' (array of strings)"), nil
		}

		projectID, err := resolveProjectID(ctx, pool, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxTokens := req.GetInt("max_tokens", 8000)

		// Single query — simple path
		if len(queries) == 1 {
			assembled, err := engine.AssembleContext(ctx, pool, client, queries[0], projectID, maxTokens)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("explore failed: %v", err)), nil
			}
			return mcp.NewToolResultText(assembled.Text), nil
		}

		// Multiple queries — run each, concatenate results with headers
		perQueryBudget := maxTokens / len(queries)
		if perQueryBudget < 2000 {
			perQueryBudget = 2000
		}

		var b strings.Builder
		for i, q := range queries {
			assembled, err := engine.AssembleContext(ctx, pool, client, q, projectID, perQueryBudget)
			if err != nil {
				b.WriteString(fmt.Sprintf("## Query %d: %s\n\nError: %v\n\n", i+1, q, err))
				continue
			}
			b.WriteString(fmt.Sprintf("## Query %d: %s\n\n", i+1, q))
			b.WriteString(assembled.Text)
			b.WriteString("\n\n---\n\n")
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

