package indexer

import (
	"strings"
	"testing"
)

func TestPrepareEmbeddingInput_BasicConcatenation(t *testing.T) {
	result, err := PrepareEmbeddingInput("func Add(a, b int) int", "Adds two numbers", "return a + b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Truncated {
		t.Error("expected truncated=false for short input")
	}
	if result.TokenCount == 0 {
		t.Error("expected non-zero token count")
	}
	if !strings.Contains(result.Text, "func Add(a, b int) int") {
		t.Error("expected signature in output text")
	}
	if !strings.Contains(result.Text, "Adds two numbers") {
		t.Error("expected docstring in output text")
	}
	if !strings.Contains(result.Text, "return a + b") {
		t.Error("expected source code in output text")
	}
}

func TestPrepareEmbeddingInput_EmptyParts(t *testing.T) {
	result, err := PrepareEmbeddingInput("", "", "x := 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "x := 1" {
		t.Errorf("expected only source code, got %q", result.Text)
	}
	if result.Truncated {
		t.Error("expected truncated=false")
	}
}

func TestPrepareEmbeddingInput_AllEmpty(t *testing.T) {
	result, err := PrepareEmbeddingInput("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "" {
		t.Errorf("expected empty text, got %q", result.Text)
	}
	if result.TokenCount != 0 {
		t.Errorf("expected 0 tokens, got %d", result.TokenCount)
	}
}

func TestPrepareEmbeddingInput_Truncation(t *testing.T) {
	signature := "func BigFunction() string"
	docstring := "Returns a very large string"
	// ~3 tokens per word, 8191 tokens ≈ 2700 words. Generate way more than that.
	sourceCode := strings.Repeat("variable assignment statement here ", 5000)

	result, err := PrepareEmbeddingInput(signature, docstring, sourceCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Truncated {
		t.Error("expected truncated=true for oversized input")
	}
	if result.TokenCount > maxEmbeddingTokens {
		t.Errorf("token count %d exceeds max %d", result.TokenCount, maxEmbeddingTokens)
	}
	if !strings.Contains(result.Text, signature) {
		t.Error("signature should be preserved during truncation")
	}
	if !strings.Contains(result.Text, docstring) {
		t.Error("docstring should be preserved during truncation")
	}
}

func TestCountTokens(t *testing.T) {
	count, err := CountTokens("hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tokens for 'hello world', got %d", count)
	}
}

func TestCountTokens_Empty(t *testing.T) {
	count, err := CountTokens("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", count)
	}
}

func TestBuildText(t *testing.T) {
	tests := []struct {
		sig, doc, src string
		want          string
	}{
		{"sig", "doc", "src", "sig\ndoc\nsrc"},
		{"sig", "", "src", "sig\nsrc"},
		{"", "doc", "", "doc"},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		got := buildText(tt.sig, tt.doc, tt.src)
		if got != tt.want {
			t.Errorf("buildText(%q, %q, %q) = %q, want %q", tt.sig, tt.doc, tt.src, got, tt.want)
		}
	}
}
