package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"
)

const systemPrompt = `You are a code intelligence assistant. You have access to indexed code from the user's project, which may span multiple source repositories.

Use the following code context to answer the user's question. Cite specific file paths, function names, and source repositories. Be concise and direct.`

// Source represents a code reference cited in a chat response.
type Source struct {
	NodeID        string `json:"nodeId"`
	FilePath      string `json:"filePath"`
	QualifiedName string `json:"qualifiedName"`
}

// ChatResponse is the result of a chat query.
type ChatResponse struct {
	Message string   `json:"message"`
	Sources []Source `json:"sources"`
}

// Chat assembles relevant code context, sends the query to an LLM, and
// returns the response with source citations.
func Chat(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, model string, maxContextTokens int) (*ChatResponse, error) {
	assembled, err := AssembleContext(ctx, pool, client, query, projectID, maxContextTokens)
	if err != nil {
		return nil, fmt.Errorf("assemble context: %w", err)
	}

	projectCtx := buildProjectContext(ctx, pool, projectID)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt + "\n\n" + projectCtx + "\n\n" + assembled.Text,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: query,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("chat completion returned no choices")
	}

	return &ChatResponse{
		Message: resp.Choices[0].Message.Content,
		Sources: extractSources(assembled.Nodes),
	}, nil
}

// ChatStreamResult holds the sources (known upfront from context assembly)
// and the OpenAI stream to read deltas from.
type ChatStreamResult struct {
	Sources []Source
	Stream  *openai.ChatCompletionStream
}

// ChatMessage represents a previous message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// needsCodeContext asks the LLM whether a query requires searching the codebase.
// Returns false for greetings, small talk, and follow-ups that don't reference code.
func needsCodeContext(ctx context.Context, client *openai.Client, query string) bool {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       "gpt-4o-mini",
		MaxTokens:   1,
		Temperature: 0,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You classify whether a user query requires searching a codebase to answer. Reply with exactly one character: Y if the query asks about code, architecture, files, functions, bugs, or technical implementation. N if it's a greeting, small talk, meta-question about the conversation, or doesn't need code context.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: query,
			},
		},
	})
	if err != nil {
		return true
	}
	if len(resp.Choices) > 0 && len(resp.Choices[0].Message.Content) > 0 {
		return resp.Choices[0].Message.Content[0] == 'Y'
	}
	return true
}

// ChatStream assembles context and opens a streaming chat completion.
// The caller is responsible for reading from Stream and closing it.
// history contains previous messages in the conversation (oldest first).
func ChatStream(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, model string, maxContextTokens int, history []ChatMessage) (*ChatStreamResult, error) {
	var systemContent string
	var sources []Source

	projectCtx := buildProjectContext(ctx, pool, projectID)

	if needsCodeContext(ctx, client, query) {
		assembled, err := AssembleContext(ctx, pool, client, query, projectID, maxContextTokens)
		if err != nil {
			return nil, fmt.Errorf("assemble context: %w", err)
		}
		systemContent = systemPrompt + "\n\n" + projectCtx + "\n\n" + assembled.Text
		sources = extractSources(assembled.Nodes)
	} else {
		systemContent = systemPrompt + "\n\n" + projectCtx
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemContent,
		},
	}
	for _, m := range history {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: query,
	})

	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Stream:   true,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("chat completion stream: %w", err)
	}

	return &ChatStreamResult{
		Sources: sources,
		Stream:  stream,
	}, nil
}

// ReadStreamDeltas reads all deltas from a ChatCompletionStream and calls
// onDelta for each text chunk. Returns the full accumulated message.
func ReadStreamDeltas(stream *openai.ChatCompletionStream, onDelta func(delta string)) (string, error) {
	defer stream.Close()

	var full string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return full, nil
		}
		if err != nil {
			return full, fmt.Errorf("stream recv: %w", err)
		}
		if len(resp.Choices) > 0 {
			delta := resp.Choices[0].Delta.Content
			if delta != "" {
				full += delta
				onDelta(delta)
			}
		}
	}
}

// buildProjectContext queries project metadata and formats it as a preamble
// for the system prompt so the LLM knows what it's looking at.
func buildProjectContext(ctx context.Context, pool *pgxpool.Pool, projectID string) string {
	var b strings.Builder

	// Project name + description
	var name, description string
	err := pool.QueryRow(ctx,
		"SELECT name, COALESCE(description, '') FROM projects WHERE id = $1", projectID,
	).Scan(&name, &description)
	if err != nil {
		slog.Warn("buildProjectContext: could not load project", "error", err)
		return ""
	}

	b.WriteString(fmt.Sprintf("## Project: %s\n", name))
	if description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", description))
	}

	// Sources with their packages
	type sourceInfo struct {
		alias      string
		sourceType string
		packages   []string
	}

	rows, err := pool.Query(ctx, `
		SELECT ps.alias, ps.source_type, COALESCE(
			(SELECT array_agg(p.name ORDER BY p.name)
			 FROM packages p
			 JOIN workspaces w ON p.workspace_id = w.id
			 WHERE w.source_id = ps.id),
			ARRAY[]::text[]
		) as pkg_names
		FROM project_sources ps
		WHERE ps.project_id = $1
		ORDER BY ps.alias`, projectID)
	if err != nil {
		slog.Warn("buildProjectContext: could not load sources", "error", err)
		return b.String()
	}
	defer rows.Close()

	var sources []sourceInfo
	for rows.Next() {
		var s sourceInfo
		if err := rows.Scan(&s.alias, &s.sourceType, &s.packages); err != nil {
			continue
		}
		sources = append(sources, s)
	}

	if len(sources) > 0 {
		b.WriteString("\n### Sources\n")
		for _, s := range sources {
			pkgSummary := ""
			if len(s.packages) > 0 {
				if len(s.packages) <= 5 {
					pkgSummary = fmt.Sprintf(" — %d package%s: %s",
						len(s.packages), plural(len(s.packages)), strings.Join(s.packages, ", "))
				} else {
					pkgSummary = fmt.Sprintf(" — %d packages: %s, ...",
						len(s.packages), strings.Join(s.packages[:5], ", "))
				}
			}
			b.WriteString(fmt.Sprintf("- %s (%s)%s\n", s.alias, s.sourceType, pkgSummary))
		}
	}

	// Node + edge counts
	var nodeCount, edgeCount int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM nodes n
		JOIN workspaces w ON n.workspace_id = w.id
		WHERE w.project_id = $1`, projectID).Scan(&nodeCount)
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM edges e
		JOIN nodes n ON e.source_id = n.id
		JOIN workspaces w ON n.workspace_id = w.id
		WHERE w.project_id = $1`, projectID).Scan(&edgeCount)

	if nodeCount > 0 {
		b.WriteString(fmt.Sprintf("\n### Stats\n%d code symbols, %d relationships\n", nodeCount, edgeCount))
	}

	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func extractSources(nodes []ContextNode) []Source {
	sources := make([]Source, 0, len(nodes))
	seen := make(map[string]bool)
	for _, n := range nodes {
		if seen[n.NodeID] {
			continue
		}
		seen[n.NodeID] = true
		sources = append(sources, Source{
			NodeID:        n.NodeID,
			FilePath:      n.FilePath,
			QualifiedName: n.QualifiedName,
		})
	}
	return sources
}
