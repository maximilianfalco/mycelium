package indexer

import (
	"fmt"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

const maxEmbeddingTokens = 8191

type ChunkResult struct {
	Text       string `json:"text"`
	TokenCount int    `json:"tokenCount"`
	Truncated  bool   `json:"truncated"`
}

// PrepareEmbeddingInput concatenates a node's signature, docstring, and source
// code into a single string suitable for embedding. Truncates from the end of
// source code if the result exceeds the model's token limit.
func PrepareEmbeddingInput(signature, docstring, sourceCode string) (*ChunkResult, error) {
	tke, err := getEncoding()
	if err != nil {
		return nil, err
	}

	text := buildText(signature, docstring, sourceCode)
	tokens := tke.Encode(text, nil, nil)

	if len(tokens) <= maxEmbeddingTokens {
		return &ChunkResult{
			Text:       text,
			TokenCount: len(tokens),
			Truncated:  false,
		}, nil
	}

	// Truncate: preserve signature + docstring, trim source code
	prefix := buildText(signature, docstring, "")
	prefixTokens := tke.Encode(prefix, nil, nil)

	remaining := maxEmbeddingTokens - len(prefixTokens)
	if remaining <= 0 {
		// Even signature + docstring exceeds limit — truncate the whole thing
		truncated := tke.Decode(tokens[:maxEmbeddingTokens])
		return &ChunkResult{
			Text:       truncated,
			TokenCount: maxEmbeddingTokens,
			Truncated:  true,
		}, nil
	}

	sourceTokens := tke.Encode(sourceCode, nil, nil)
	if len(sourceTokens) > remaining {
		sourceTokens = sourceTokens[:remaining]
	}
	truncatedSource := tke.Decode(sourceTokens)
	finalText := buildText(signature, docstring, truncatedSource)

	return &ChunkResult{
		Text:       finalText,
		TokenCount: len(prefixTokens) + len(sourceTokens),
		Truncated:  true,
	}, nil
}

// CountTokens returns the token count for a given text using the embedding model's encoding.
func CountTokens(text string) (int, error) {
	tke, err := getEncoding()
	if err != nil {
		return 0, err
	}
	return len(tke.Encode(text, nil, nil)), nil
}

func buildText(signature, docstring, sourceCode string) string {
	var parts []string
	if signature != "" {
		parts = append(parts, signature)
	}
	if docstring != "" {
		parts = append(parts, docstring)
	}
	if sourceCode != "" {
		parts = append(parts, sourceCode)
	}
	return strings.Join(parts, "\n")
}

var cachedEncoding *tiktoken.Tiktoken

func getEncoding() (*tiktoken.Tiktoken, error) {
	if cachedEncoding != nil {
		return cachedEncoding, nil
	}
	tke, err := tiktoken.EncodingForModel("text-embedding-3-small")
	if err != nil {
		return nil, fmt.Errorf("tiktoken encoding: %w", err)
	}
	cachedEncoding = tke
	return tke, nil
}
