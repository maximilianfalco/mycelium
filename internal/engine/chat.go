package engine

import (
	"context"
	"fmt"
	"io"

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

// ChatStream assembles context and opens a streaming chat completion.
// The caller is responsible for reading from Stream and closing it.
// history contains previous messages in the conversation (oldest first).
func ChatStream(ctx context.Context, pool *pgxpool.Pool, client *openai.Client, query string, projectID string, model string, maxContextTokens int, history []ChatMessage) (*ChatStreamResult, error) {
	assembled, err := AssembleContext(ctx, pool, client, query, projectID, maxContextTokens)
	if err != nil {
		return nil, fmt.Errorf("assemble context: %w", err)
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt + "\n\n" + assembled.Text,
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

	sources := extractSources(assembled.Nodes)

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
