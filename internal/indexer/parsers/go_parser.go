package parsers

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

var _ Parser = (*GoParser)(nil)

type GoParser struct{}

func NewGoParser() *GoParser {
	return &GoParser{}
}

func (p *GoParser) Parse(filePath string, source []byte) (*ParseResult, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	result := &ParseResult{}
	root := tree.RootNode()
	p.extractNodes(source, root, result)
	p.extractEdges(source, root, filePath, result)
	return result, nil
}

// --- Node extraction ---

func (p *GoParser) extractNodes(source []byte, root *sitter.Node, result *ParseResult) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		switch child.Type() {
		case "function_declaration":
			p.extractFunction(source, child, result)
		case "method_declaration":
			p.extractMethod(source, child, result)
		case "type_declaration":
			p.extractTypeDecl(source, child, result)
		}
	}
}

func (p *GoParser) extractFunction(source []byte, node *sitter.Node, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)

	result.Nodes = append(result.Nodes, NodeInfo{
		Name:          name,
		QualifiedName: name,
		Kind:          "function",
		Signature:     goSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     goDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	})
}

func (p *GoParser) extractMethod(source []byte, node *sitter.Node, result *ParseResult) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)
	receiver := goReceiverType(source, node)
	qname := name
	if receiver != "" {
		qname = receiver + "." + name
	}

	result.Nodes = append(result.Nodes, NodeInfo{
		Name:          name,
		QualifiedName: qname,
		Kind:          "method",
		Signature:     goSignature(source, node),
		StartLine:     int(node.StartPoint().Row) + 1,
		EndLine:       int(node.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, node),
		Docstring:     goDocstring(source, node),
		BodyHash:      computeBodyHash(source, node),
	})
}

func (p *GoParser) extractTypeDecl(source []byte, node *sitter.Node, result *ParseResult) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		spec := node.NamedChild(i)
		if spec.Type() != "type_spec" {
			continue
		}
		p.extractTypeSpec(source, spec, node, result)
	}
}

func (p *GoParser) extractTypeSpec(source []byte, spec *sitter.Node, declNode *sitter.Node, result *ParseResult) {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nodeContent(source, nameNode)

	typeNode := spec.ChildByFieldName("type")
	kind := "type_alias"
	if typeNode != nil {
		switch typeNode.Type() {
		case "struct_type":
			kind = "struct"
		case "interface_type":
			kind = "interface"
		}
	}

	result.Nodes = append(result.Nodes, NodeInfo{
		Name:          name,
		QualifiedName: name,
		Kind:          kind,
		Signature:     goTypeSignature(source, spec, kind),
		StartLine:     int(declNode.StartPoint().Row) + 1,
		EndLine:       int(declNode.EndPoint().Row) + 1,
		SourceCode:    nodeContent(source, declNode),
		Docstring:     goDocstring(source, declNode),
		BodyHash:      computeBodyHash(source, declNode),
	})
}

// --- Edge extraction ---

func (p *GoParser) extractEdges(source []byte, root *sitter.Node, filePath string, result *ParseResult) {
	p.extractImportEdges(source, root, filePath, result)
	p.extractContainsEdges(filePath, result)
	p.extractHeritageEdges(source, root, result)
	p.extractCallEdgesGo(source, root, result)
	p.extractTypeEdgesGo(source, root, result)
}

func (p *GoParser) extractImportEdges(source []byte, root *sitter.Node, filePath string, result *ParseResult) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() != "import_declaration" {
			continue
		}

		// Single import: import "fmt"
		spec := findChildByType(child, "import_spec")
		if spec != nil {
			p.addImportEdge(source, spec, filePath, result)
			continue
		}

		// Grouped imports: import ( ... )
		specList := findChildByType(child, "import_spec_list")
		if specList == nil {
			continue
		}
		for j := 0; j < int(specList.NamedChildCount()); j++ {
			s := specList.NamedChild(j)
			if s.Type() == "import_spec" {
				p.addImportEdge(source, s, filePath, result)
			}
		}
	}
}

func (p *GoParser) addImportEdge(source []byte, spec *sitter.Node, filePath string, result *ParseResult) {
	pathNode := findChildByType(spec, "interpreted_string_literal")
	if pathNode == nil {
		return
	}
	importPath := stripQuotes(nodeContent(source, pathNode))

	var symbols []string
	if findChildByType(spec, "dot") != nil {
		symbols = append(symbols, ". (dot import)")
	} else if findChildByType(spec, "blank_identifier") != nil {
		symbols = append(symbols, "_ (side effect)")
	} else {
		aliasNode := findChildByType(spec, "package_identifier")
		if aliasNode != nil {
			symbols = append(symbols, nodeContent(source, aliasNode)+" (alias)")
		}
	}

	result.Edges = append(result.Edges, EdgeInfo{
		Source:  filePath,
		Target:  importPath,
		Kind:    "imports",
		Line:    int(spec.StartPoint().Row) + 1,
		Symbols: symbols,
	})
}

func (p *GoParser) extractContainsEdges(filePath string, result *ParseResult) {
	for _, node := range result.Nodes {
		switch node.Kind {
		case "function", "struct", "interface", "type_alias":
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

func (p *GoParser) extractHeritageEdges(source []byte, root *sitter.Node, result *ParseResult) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() != "type_declaration" {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			spec := child.NamedChild(j)
			if spec.Type() != "type_spec" {
				continue
			}
			p.extractEmbedEdges(source, spec, result)
		}
	}
}

func (p *GoParser) extractEmbedEdges(source []byte, spec *sitter.Node, result *ParseResult) {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	structName := nodeContent(source, nameNode)

	typeNode := spec.ChildByFieldName("type")
	if typeNode == nil || typeNode.Type() != "struct_type" {
		return
	}

	fieldList := findChildByType(typeNode, "field_declaration_list")
	if fieldList == nil {
		return
	}

	for i := 0; i < int(fieldList.NamedChildCount()); i++ {
		field := fieldList.NamedChild(i)
		if field.Type() != "field_declaration" {
			continue
		}
		// Embedded field: has a type but no field name
		if isEmbeddedField(field) {
			embeddedType := extractEmbeddedTypeName(source, field)
			if embeddedType != "" {
				result.Edges = append(result.Edges, EdgeInfo{
					Source: structName,
					Target: embeddedType,
					Kind:   "embeds",
					Line:   int(field.StartPoint().Row) + 1,
				})
			}
		}
	}
}

func isEmbeddedField(field *sitter.Node) bool {
	// An embedded field has no field_identifier child — only a type
	for i := 0; i < int(field.NamedChildCount()); i++ {
		child := field.NamedChild(i)
		if child.Type() == "field_identifier" {
			return false
		}
	}
	return true
}

func extractEmbeddedTypeName(source []byte, field *sitter.Node) string {
	for i := 0; i < int(field.NamedChildCount()); i++ {
		child := field.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			return nodeContent(source, child)
		case "pointer_type":
			// *EmbeddedType
			for j := 0; j < int(child.NamedChildCount()); j++ {
				inner := child.NamedChild(j)
				if inner.Type() == "type_identifier" {
					return nodeContent(source, inner)
				}
			}
		case "qualified_type":
			return nodeContent(source, child)
		}
	}
	return ""
}

func (p *GoParser) extractCallEdgesGo(source []byte, root *sitter.Node, result *ParseResult) {
	for _, node := range result.Nodes {
		if node.Kind != "function" && node.Kind != "method" {
			continue
		}
		astNode := goFindDeclAtLine(root, node.StartLine-1)
		if astNode == nil {
			continue
		}
		body := astNode.ChildByFieldName("body")
		if body == nil {
			continue
		}
		p.collectCallsGo(source, body, node.QualifiedName, result)
	}
}

func (p *GoParser) collectCallsGo(source []byte, node *sitter.Node, callerName string, result *ParseResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)

		// Traverse into func literals (closures, goroutines) — attribute calls to the enclosing named function
		if child.Type() == "func_literal" {
			body := child.ChildByFieldName("body")
			if body != nil {
				p.collectCallsGo(source, body, callerName, result)
			}
			continue
		}

		if child.Type() == "call_expression" {
			fn := child.ChildByFieldName("function")
			if fn != nil {
				callee := goCalleeName(source, fn)
				if callee != "" {
					result.Edges = append(result.Edges, EdgeInfo{
						Source: callerName,
						Target: callee,
						Kind:   "calls",
						Line:   int(child.StartPoint().Row) + 1,
					})
				}
			}
		}

		p.collectCallsGo(source, child, callerName, result)
	}
}

func goCalleeName(source []byte, node *sitter.Node) string {
	switch node.Type() {
	case "identifier":
		return nodeContent(source, node)
	case "selector_expression":
		return nodeContent(source, node)
	default:
		return ""
	}
}

func (p *GoParser) extractTypeEdgesGo(source []byte, root *sitter.Node, result *ParseResult) {
	for _, node := range result.Nodes {
		if node.Kind != "function" && node.Kind != "method" {
			continue
		}
		astNode := goFindDeclAtLine(root, node.StartLine-1)
		if astNode == nil {
			continue
		}
		types := goCollectParamTypes(source, astNode)
		seen := make(map[string]bool)
		for _, t := range types {
			if seen[t.name] || isGoBuiltinType(t.name) {
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

func goCollectParamTypes(source []byte, node *sitter.Node) []typeRef {
	var refs []typeRef

	// Collect from all parameter_list and result nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "parameter_list":
			refs = append(refs, goFindTypeRefs(source, child)...)
		case "type_identifier":
			name := nodeContent(source, child)
			refs = append(refs, typeRef{name: name, line: int(child.StartPoint().Row) + 1})
		case "pointer_type", "slice_type", "array_type", "map_type", "channel_type":
			refs = append(refs, goFindTypeRefs(source, child)...)
		}
	}

	// Check for result type (named result)
	result := node.ChildByFieldName("result")
	if result != nil {
		refs = append(refs, goFindTypeRefs(source, result)...)
	}

	return refs
}

func goFindTypeRefs(source []byte, node *sitter.Node) []typeRef {
	var refs []typeRef
	if node.Type() == "type_identifier" {
		name := nodeContent(source, node)
		refs = append(refs, typeRef{name: name, line: int(node.StartPoint().Row) + 1})
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		refs = append(refs, goFindTypeRefs(source, node.Child(i))...)
	}
	return refs
}

func isGoBuiltinType(name string) bool {
	switch name {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"bool", "byte", "rune", "error", "any":
		return true
	}
	return false
}

// --- Go-specific helpers ---

func goReceiverType(source []byte, method *sitter.Node) string {
	// method_declaration has the receiver as the first parameter_list
	recv := method.ChildByFieldName("receiver")
	if recv == nil {
		return ""
	}
	// Walk into the parameter_list to find the type
	for i := 0; i < int(recv.NamedChildCount()); i++ {
		param := recv.NamedChild(i)
		if param.Type() == "parameter_declaration" {
			// Find the type identifier, stripping pointer
			typeNode := param.ChildByFieldName("type")
			if typeNode != nil {
				return goExtractBaseType(source, typeNode)
			}
		}
	}
	return ""
}

func goExtractBaseType(source []byte, node *sitter.Node) string {
	switch node.Type() {
	case "type_identifier":
		return nodeContent(source, node)
	case "pointer_type":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "type_identifier" {
				return nodeContent(source, child)
			}
		}
	}
	return ""
}

func goSignature(source []byte, node *sitter.Node) string {
	text := nodeContent(source, node)
	if idx := strings.Index(text, "{"); idx != -1 {
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
}

func goTypeSignature(source []byte, spec *sitter.Node, kind string) string {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}
	name := nodeContent(source, nameNode)
	switch kind {
	case "struct":
		return "type " + name + " struct"
	case "interface":
		return "type " + name + " interface"
	default:
		return "type " + nodeContent(source, spec)
	}
}

func goDocstring(source []byte, node *sitter.Node) string {
	// Go uses // comments above declarations
	prev := node.PrevNamedSibling()
	if prev == nil || prev.Type() != "comment" {
		return ""
	}

	lines := []string{nodeContent(source, prev)}
	cur := prev
	for {
		p := cur.PrevNamedSibling()
		if p == nil || p.Type() != "comment" {
			break
		}
		text := nodeContent(source, p)
		if !strings.HasPrefix(text, "//") {
			break
		}
		if cur.StartPoint().Row-p.EndPoint().Row > 1 {
			break
		}
		lines = append([]string{text}, lines...)
		cur = p
	}

	var cleaned []string
	for _, l := range lines {
		l = strings.TrimPrefix(l, "//")
		l = strings.TrimPrefix(l, " ")
		cleaned = append(cleaned, l)
	}
	return strings.Join(cleaned, "\n")
}

func goFindDeclAtLine(root *sitter.Node, row int) *sitter.Node {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if int(child.StartPoint().Row) == row {
			return child
		}
	}
	return nil
}
