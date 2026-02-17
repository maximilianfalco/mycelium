package parsers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

var _ Parser = (*TypeScriptParser)(nil)

type TypeScriptParser struct{}

func NewTypeScriptParser() *TypeScriptParser {
	return &TypeScriptParser{}
}

func (p *TypeScriptParser) Parse(filePath string, source []byte) (*ParseResult, error) {
	lang, err := p.languageForExt(filepath.Ext(filePath))
	if err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	result := &ParseResult{}
	root := tree.RootNode()
	p.walkTopLevel(source, root, "", result)
	return result, nil
}

func (p *TypeScriptParser) languageForExt(ext string) (*sitter.Language, error) {
	switch ext {
	case ".ts":
		return typescript.GetLanguage(), nil
	case ".tsx", ".jsx":
		return tsx.GetLanguage(), nil
	case ".js":
		return javascript.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}
}

func (p *TypeScriptParser) walkTopLevel(source []byte, node *sitter.Node, parentName string, result *ParseResult) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		p.extractNode(source, child, parentName, result)
	}
}

func (p *TypeScriptParser) extractNode(source []byte, node *sitter.Node, parentName string, result *ParseResult) {
	switch node.Type() {
	case "function_declaration":
		p.extractFunction(source, node, parentName, result)

	case "class_declaration", "abstract_class_declaration":
		p.extractClass(source, node, result)

	case "interface_declaration":
		p.extractSimpleDecl(source, node, "interface", parentName, result)

	case "type_alias_declaration":
		p.extractSimpleDecl(source, node, "type_alias", parentName, result)

	case "enum_declaration":
		p.extractSimpleDecl(source, node, "enum", parentName, result)

	case "lexical_declaration":
		p.extractLexicalDecl(source, node, parentName, result)

	case "export_statement":
		p.extractExport(source, node, parentName, result)
	}
}

func (p *TypeScriptParser) extractFunction(source []byte, node *sitter.Node, parentName string, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)
	qname := qualifiedName(parentName, name)

	// Skip overload signatures (no body)
	body := node.ChildByFieldName("body")
	if body == nil {
		return
	}

	info := NodeInfo{
		Name:          name,
		QualifiedName: qname,
		Kind:          "function",
		Signature:     extractSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     extractDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	}
	result.Nodes = append(result.Nodes, info)
}

func (p *TypeScriptParser) extractClass(source []byte, node *sitter.Node, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)

	info := NodeInfo{
		Name:          name,
		QualifiedName: name,
		Kind:          "class",
		Signature:     extractSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     extractDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	}
	result.Nodes = append(result.Nodes, info)

	// Recurse into class body for methods
	body := node.ChildByFieldName("body")
	if body == nil {
		return
	}
	for i := 0; i < int(body.NamedChildCount()); i++ {
		child := body.NamedChild(i)
		if child.Type() == "method_definition" {
			p.extractMethod(source, child, name, result)
		}
	}
}

func (p *TypeScriptParser) extractMethod(source []byte, node *sitter.Node, className string, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)

	info := NodeInfo{
		Name:          name,
		QualifiedName: className + "." + name,
		Kind:          "method",
		Signature:     extractSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     extractDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	}
	result.Nodes = append(result.Nodes, info)
}

func (p *TypeScriptParser) extractSimpleDecl(source []byte, node *sitter.Node, kind, parentName string, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)
	qname := qualifiedName(parentName, name)

	info := NodeInfo{
		Name:          name,
		QualifiedName: qname,
		Kind:          kind,
		Signature:     extractSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     extractDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	}
	result.Nodes = append(result.Nodes, info)
}

func (p *TypeScriptParser) extractLexicalDecl(source []byte, node *sitter.Node, parentName string, result *ParseResult) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		decl := node.NamedChild(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		value := decl.ChildByFieldName("value")
		if value == nil {
			continue
		}
		if value.Type() != "arrow_function" && value.Type() != "function_expression" && value.Type() != "function" {
			continue
		}

		nameNode := decl.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		name := nodeContent(source, nameNode)
		qname := qualifiedName(parentName, name)

		// Use the entire lexical_declaration as the node span for docstrings
		info := NodeInfo{
			Name:          name,
			QualifiedName: qname,
			Kind:          "function",
			Signature:     extractArrowSignature(source, decl),
			StartLine:     int(node.StartPoint().Row) + 1,
			EndLine:       int(node.EndPoint().Row) + 1,
			SourceCode:    nodeContent(source, node),
			Docstring:     extractDocstring(source, node),
			BodyHash:      computeBodyHash(source, node),
		}
		result.Nodes = append(result.Nodes, info)
	}
}

func (p *TypeScriptParser) extractExport(source []byte, node *sitter.Node, parentName string, result *ParseResult) {
	// export default function() → unwrap the declaration
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "function_declaration":
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				// export default function() — anonymous
				body := child.ChildByFieldName("body")
				if body == nil {
					continue
				}
				info := NodeInfo{
					Name:          "default",
					QualifiedName: qualifiedName(parentName, "default"),
					Kind:          "function",
					Signature:     extractSignature(source, child),
					StartLine:     int(node.StartPoint().Row) + 1,
					EndLine:       int(node.EndPoint().Row) + 1,
					SourceCode:    nodeContent(source, node),
					Docstring:     extractDocstring(source, node),
					BodyHash:      computeBodyHash(source, node),
				}
				result.Nodes = append(result.Nodes, info)
			} else {
				p.extractFunction(source, child, parentName, result)
			}

		case "class_declaration", "abstract_class_declaration":
			p.extractClass(source, child, result)

		case "interface_declaration":
			p.extractSimpleDecl(source, child, "interface", parentName, result)

		case "type_alias_declaration":
			p.extractSimpleDecl(source, child, "type_alias", parentName, result)

		case "enum_declaration":
			p.extractSimpleDecl(source, child, "enum", parentName, result)

		case "lexical_declaration":
			p.extractLexicalDecl(source, child, parentName, result)
		}
	}
}

func qualifiedName(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// extractSignature returns the first line of the node (declaration line).
func extractSignature(source []byte, node *sitter.Node) string {
	text := nodeContent(source, node)
	// Take everything up to the opening brace or end of first line
	if idx := strings.Index(text, "{"); idx != -1 {
		sig := strings.TrimSpace(text[:idx])
		return sig
	}
	lines := strings.SplitN(text, "\n", 2)
	return strings.TrimSpace(lines[0])
}

func extractArrowSignature(source []byte, declarator *sitter.Node) string {
	text := nodeContent(source, declarator)
	if idx := strings.Index(text, "=>"); idx != -1 {
		return strings.TrimSpace(text[:idx+2])
	}
	lines := strings.SplitN(text, "\n", 2)
	return strings.TrimSpace(lines[0])
}
