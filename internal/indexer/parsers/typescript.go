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
	p.extractEdges(source, root, filePath, result)
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
	// JSDoc comments are siblings of the export_statement, not the inner declaration.
	// Capture the docstring from the export node so we can attach it to the inner declaration.
	exportDocstring := extractDocstring(source, node)

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
					Docstring:     exportDocstring,
					BodyHash:      computeBodyHash(source, node),
				}
				result.Nodes = append(result.Nodes, info)
			} else {
				p.extractFunction(source, child, parentName, result)
				// Backfill export docstring if the inner node had none
				if exportDocstring != "" {
					for j := range result.Nodes {
						n := &result.Nodes[j]
						if n.Name == nodeContent(source, nameNode) && n.Docstring == "" {
							n.Docstring = exportDocstring
						}
					}
				}
			}

		case "class_declaration", "abstract_class_declaration":
			p.extractClass(source, child, result)
			if exportDocstring != "" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					name := nodeContent(source, nameNode)
					for j := range result.Nodes {
						if result.Nodes[j].QualifiedName == name && result.Nodes[j].Docstring == "" {
							result.Nodes[j].Docstring = exportDocstring
						}
					}
				}
			}

		case "interface_declaration":
			p.extractSimpleDecl(source, child, "interface", parentName, result)
			if exportDocstring != "" {
				p.backfillDocstring(source, child, result, exportDocstring)
			}

		case "type_alias_declaration":
			p.extractSimpleDecl(source, child, "type_alias", parentName, result)
			if exportDocstring != "" {
				p.backfillDocstring(source, child, result, exportDocstring)
			}

		case "enum_declaration":
			p.extractSimpleDecl(source, child, "enum", parentName, result)
			if exportDocstring != "" {
				p.backfillDocstring(source, child, result, exportDocstring)
			}

		case "lexical_declaration":
			p.extractLexicalDecl(source, child, parentName, result)
			if exportDocstring != "" {
				p.backfillExportLexicalDocstring(source, child, result, exportDocstring)
			}
		}
	}
}

// backfillDocstring sets the export-level docstring on the last-added node if it has none.
func (p *TypeScriptParser) backfillDocstring(source []byte, child *sitter.Node, result *ParseResult, docstring string) {
	nameNode := child.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)
	for j := range result.Nodes {
		if result.Nodes[j].Name == name && result.Nodes[j].Docstring == "" {
			result.Nodes[j].Docstring = docstring
		}
	}
}

// backfillExportLexicalDocstring handles lexical declarations (const x = () => {}) inside exports.
func (p *TypeScriptParser) backfillExportLexicalDocstring(source []byte, lexNode *sitter.Node, result *ParseResult, docstring string) {
	for i := 0; i < int(lexNode.NamedChildCount()); i++ {
		decl := lexNode.NamedChild(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		nameNode := decl.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		name := nodeContent(source, nameNode)
		for j := range result.Nodes {
			if result.Nodes[j].Name == name && result.Nodes[j].Docstring == "" {
				result.Nodes[j].Docstring = docstring
			}
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

// --- Edge extraction ---

func (p *TypeScriptParser) extractEdges(source []byte, root *sitter.Node, filePath string, result *ParseResult) {
	p.extractImportEdges(source, root, filePath, result)
	p.extractContainsEdges(filePath, result)
	p.extractClassEdges(source, root, result)
	p.extractCallEdges(source, root, result)
	p.extractTypeEdges(source, root, result)
}

func (p *TypeScriptParser) extractImportEdges(source []byte, root *sitter.Node, filePath string, result *ParseResult) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() != "import_statement" {
			continue
		}

		moduleNode := findChildByType(child, "string")
		if moduleNode == nil {
			continue
		}
		module := stripQuotes(nodeContent(source, moduleNode))

		var symbols []string
		clause := findChildByType(child, "import_clause")
		if clause != nil {
			symbols = extractImportSymbols(source, clause)
		}

		result.Edges = append(result.Edges, EdgeInfo{
			Source:  filePath,
			Target:  module,
			Kind:    "imports",
			Line:    int(child.StartPoint().Row) + 1,
			Symbols: symbols,
		})
	}
}

func extractImportSymbols(source []byte, clause *sitter.Node) []string {
	var symbols []string
	for i := 0; i < int(clause.ChildCount()); i++ {
		child := clause.Child(i)
		switch child.Type() {
		case "identifier":
			symbols = append(symbols, nodeContent(source, child))
		case "named_imports":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				spec := child.NamedChild(j)
				if spec.Type() == "import_specifier" {
					name := spec.ChildByFieldName("name")
					if name != nil {
						symbols = append(symbols, nodeContent(source, name))
					}
				}
			}
		case "namespace_import":
			for j := 0; j < int(child.ChildCount()); j++ {
				c := child.Child(j)
				if c.Type() == "identifier" {
					symbols = append(symbols, "* as "+nodeContent(source, c))
					break
				}
			}
		}
	}
	return symbols
}

func (p *TypeScriptParser) extractContainsEdges(filePath string, result *ParseResult) {
	for _, node := range result.Nodes {
		switch node.Kind {
		case "class", "function", "interface", "type_alias", "enum":
			result.Edges = append(result.Edges, EdgeInfo{
				Source: filePath,
				Target: node.QualifiedName,
				Kind:   "contains",
				Line:   node.StartLine,
			})
		case "method":
			parts := strings.SplitN(node.QualifiedName, ".", 2)
			if len(parts) == 2 {
				result.Edges = append(result.Edges, EdgeInfo{
					Source: parts[0],
					Target: node.QualifiedName,
					Kind:   "contains",
					Line:   node.StartLine,
				})
			}
		}
	}
}

func (p *TypeScriptParser) extractClassEdges(source []byte, root *sitter.Node, result *ParseResult) {
	p.walkForClassEdges(source, root, result)
}

func (p *TypeScriptParser) walkForClassEdges(source []byte, node *sitter.Node, result *ParseResult) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "class_declaration", "abstract_class_declaration":
			p.extractHeritageEdges(source, child, result)
		case "export_statement":
			p.walkForClassEdges(source, child, result)
		}
	}
}

func (p *TypeScriptParser) extractHeritageEdges(source []byte, classNode *sitter.Node, result *ParseResult) {
	nameNode := classNode.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	className := nodeContent(source, nameNode)

	heritage := findChildByType(classNode, "class_heritage")
	if heritage == nil {
		return
	}

	for i := 0; i < int(heritage.ChildCount()); i++ {
		child := heritage.Child(i)
		switch child.Type() {
		case "extends_clause":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				target := child.NamedChild(j)
				result.Edges = append(result.Edges, EdgeInfo{
					Source: className,
					Target: nodeContent(source, target),
					Kind:   "extends",
					Line:   int(child.StartPoint().Row) + 1,
				})
			}
		case "implements_clause":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				target := child.NamedChild(j)
				if target.Type() == "type_identifier" {
					result.Edges = append(result.Edges, EdgeInfo{
						Source: className,
						Target: nodeContent(source, target),
						Kind:   "implements",
						Line:   int(child.StartPoint().Row) + 1,
					})
				}
			}
		}
	}
}

func (p *TypeScriptParser) extractCallEdges(source []byte, root *sitter.Node, result *ParseResult) {
	for _, node := range result.Nodes {
		if node.Kind != "function" && node.Kind != "method" {
			continue
		}
		// Find the AST node for this declaration by matching line numbers
		astNode := findDeclAtLine(root, node.StartLine-1)
		if astNode == nil {
			continue
		}
		body := findBody(astNode)
		if body == nil {
			continue
		}
		p.collectCalls(source, body, node.QualifiedName, result)
	}
}

func (p *TypeScriptParser) collectCalls(source []byte, node *sitter.Node, callerName string, result *ParseResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)

		// Skip nested arrow/function expressions to avoid capturing calls inside lambdas
		if child.Type() == "arrow_function" || child.Type() == "function_expression" || child.Type() == "function_declaration" {
			continue
		}

		if child.Type() == "call_expression" {
			callee := child.ChildByFieldName("function")
			if callee != nil {
				calleeName := extractCalleeName(source, callee)
				if calleeName != "" {
					result.Edges = append(result.Edges, EdgeInfo{
						Source: callerName,
						Target: calleeName,
						Kind:   "calls",
						Line:   int(child.StartPoint().Row) + 1,
					})
				}
			}
		}

		p.collectCalls(source, child, callerName, result)
	}
}

func extractCalleeName(source []byte, node *sitter.Node) string {
	switch node.Type() {
	case "identifier":
		return nodeContent(source, node)
	case "member_expression":
		return nodeContent(source, node)
	case "super":
		return "super"
	default:
		return ""
	}
}

func (p *TypeScriptParser) extractTypeEdges(source []byte, root *sitter.Node, result *ParseResult) {
	for _, node := range result.Nodes {
		if node.Kind != "function" && node.Kind != "method" {
			continue
		}
		astNode := findDeclAtLine(root, node.StartLine-1)
		if astNode == nil {
			continue
		}
		types := collectTypeAnnotations(source, astNode)
		seen := make(map[string]bool)
		for _, t := range types {
			if seen[t.name] {
				continue
			}
			seen[t.name] = true
			result.Edges = append(result.Edges, EdgeInfo{
				Source: node.QualifiedName,
				Target: t.name,
				Kind:   "uses_type",
				Line:   t.line,
			})
		}
	}
}

type typeRef struct {
	name string
	line int
}

func collectTypeAnnotations(source []byte, node *sitter.Node) []typeRef {
	var refs []typeRef

	// Check parameters
	params := node.ChildByFieldName("parameters")
	if params != nil {
		refs = append(refs, findTypeIdentifiers(source, params)...)
	}

	// Check return type annotation
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_annotation" {
			refs = append(refs, findTypeIdentifiers(source, child)...)
		}
	}

	// For arrow functions inside variable declarators, check the declarator
	if node.Type() == "variable_declarator" {
		value := node.ChildByFieldName("value")
		if value != nil {
			params := value.ChildByFieldName("parameters")
			if params != nil {
				refs = append(refs, findTypeIdentifiers(source, params)...)
			}
			for i := 0; i < int(value.ChildCount()); i++ {
				child := value.Child(i)
				if child.Type() == "type_annotation" {
					refs = append(refs, findTypeIdentifiers(source, child)...)
				}
			}
		}
	}

	return refs
}

func findTypeIdentifiers(source []byte, node *sitter.Node) []typeRef {
	var refs []typeRef
	if node.Type() == "type_identifier" {
		name := nodeContent(source, node)
		// Skip built-in types
		if !isBuiltinType(name) {
			refs = append(refs, typeRef{name: name, line: int(node.StartPoint().Row) + 1})
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		refs = append(refs, findTypeIdentifiers(source, node.Child(i))...)
	}
	return refs
}

func isBuiltinType(name string) bool {
	switch name {
	case "string", "number", "boolean", "void", "null", "undefined",
		"any", "never", "unknown", "object", "symbol", "bigint":
		return true
	}
	return false
}

// --- Helpers for edge extraction ---

func findChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

func stripQuotes(s string) string {
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return s
}

// findDeclAtLine finds a declaration node at the given 0-indexed row.
func findDeclAtLine(root *sitter.Node, row int) *sitter.Node {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "export_statement" {
			result := findDeclAtLine(child, row)
			if result != nil {
				return result
			}
		}
		if int(child.StartPoint().Row) == row {
			// For lexical_declaration, return the variable_declarator
			if child.Type() == "lexical_declaration" {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					decl := child.NamedChild(j)
					if decl.Type() == "variable_declarator" {
						return decl
					}
				}
			}
			return child
		}
		// Check class methods
		if child.Type() == "class_declaration" || child.Type() == "abstract_class_declaration" {
			body := child.ChildByFieldName("body")
			if body != nil {
				for j := 0; j < int(body.NamedChildCount()); j++ {
					method := body.NamedChild(j)
					if method.Type() == "method_definition" && int(method.StartPoint().Row) == row {
						return method
					}
				}
			}
		}
	}
	return nil
}

func findBody(node *sitter.Node) *sitter.Node {
	// For variable_declarator (arrow functions), the body is inside the value
	if node.Type() == "variable_declarator" {
		value := node.ChildByFieldName("value")
		if value != nil {
			body := value.ChildByFieldName("body")
			if body != nil {
				return body
			}
			// Concise arrow: the body is the expression itself
			return value
		}
		return nil
	}
	return node.ChildByFieldName("body")
}
