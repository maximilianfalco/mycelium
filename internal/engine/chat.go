package engine

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	openai "github.com/sashabaranov/go-openai"
)

const systemPrompt = `You are a code intelligence assistant. You have access to indexed code from the user's repository.

Use the following code context to answer the user's question. When referencing code:
- Always cite specific file paths and function/class names
- If the context contains relevant source code, explain how it works
- If the context doesn't contain enough information to answer, say so honestly

Be concise and direct. Focus on the code, not general programming advice.`

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

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt + "\n\n" + assembled.Text,
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

	sources := make([]Source, 0, len(assembled.Nodes))
	seen := make(map[string]bool)
	for _, n := range assembled.Nodes {
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

	return &ChatResponse{
		Message: resp.Choices[0].Message.Content,
		Sources: sources,
	}, nil
}
