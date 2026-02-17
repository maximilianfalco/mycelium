package parsers

import (
	"crypto/sha256"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func nodeContent(source []byte, node *sitter.Node) string {
	return string(source[node.StartByte():node.EndByte()])
}

func computeBodyHash(source []byte, node *sitter.Node) string {
	h := sha256.Sum256(source[node.StartByte():node.EndByte()])
	return fmt.Sprintf("%x", h)
}

// extractDocstring looks for JSDoc (/** ... */) or consecutive // comments
// immediately preceding the given node.
func extractDocstring(source []byte, node *sitter.Node) string {
	prev := node.PrevNamedSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		text := nodeContent(source, prev)
		// JSDoc block comment
		if strings.HasPrefix(text, "/**") {
			return cleanDocstring(text)
		}
		// Consecutive line comments — collect upward
		lines := []string{text}
		cur := prev
		for {
			p := cur.PrevNamedSibling()
			if p == nil || p.Type() != "comment" {
				break
			}
			t := nodeContent(source, p)
			if !strings.HasPrefix(t, "//") {
				break
			}
			// Must be on the immediately preceding line
			if cur.StartPoint().Row-p.EndPoint().Row > 1 {
				break
			}
			lines = append([]string{t}, lines...)
			cur = p
		}
		if strings.HasPrefix(lines[0], "//") {
			var cleaned []string
			for _, l := range lines {
				cleaned = append(cleaned, strings.TrimPrefix(strings.TrimPrefix(l, "//"), " "))
			}
			return strings.Join(cleaned, "\n")
		}
	}
	return ""
}

func cleanDocstring(s string) string {
	s = strings.TrimPrefix(s, "/**")
	s = strings.TrimSuffix(s, "*/")
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "*")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
