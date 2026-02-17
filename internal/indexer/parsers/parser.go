package parsers

import (
	"fmt"
	"path/filepath"
)

type NodeInfo struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	Kind          string `json:"kind"`
	Signature     string `json:"signature"`
	StartLine     int    `json:"startLine"`
	EndLine       int    `json:"endLine"`
	SourceCode    string `json:"sourceCode"`
	Docstring     string `json:"docstring"`
	BodyHash      string `json:"bodyHash"`
}

type EdgeInfo struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

type ParseResult struct {
	Nodes []NodeInfo `json:"nodes"`
	Edges []EdgeInfo `json:"edges"`
}

func (r *ParseResult) Stats() map[string]any {
	byKind := make(map[string]int)
	for _, n := range r.Nodes {
		byKind[n.Kind]++
	}
	return map[string]any{
		"nodeCount": len(r.Nodes),
		"edgeCount": len(r.Edges),
		"byKind":    byKind,
	}
}

type Parser interface {
	Parse(filePath string, source []byte) (*ParseResult, error)
}

var registry map[string]Parser

func init() {
	ts := NewTypeScriptParser()
	registry = map[string]Parser{
		".ts":  ts,
		".tsx": ts,
		".js":  ts,
		".jsx": ts,
	}
}

func ParseFile(filePath string, source []byte) (*ParseResult, error) {
	ext := filepath.Ext(filePath)
	p, ok := registry[ext]
	if !ok {
		return nil, fmt.Errorf("no parser registered for extension %q", ext)
	}
	return p.Parse(filePath, source)
}
