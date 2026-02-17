package parsers

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixtureDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "..", "tests", "fixtures", "parser")
}

func readFixture(t *testing.T, parts ...string) (string, []byte) {
	t.Helper()
	path := filepath.Join(append([]string{fixtureDir()}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	return path, data
}

func findNode(nodes []NodeInfo, name string) *NodeInfo {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}
	return nil
}

func TestParseFunctions(t *testing.T) {
	path, src := readFixture(t, "typescript", "functions.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d: %v", len(result.Nodes), nodeNames(result.Nodes))
	}

	add := findNode(result.Nodes, "add")
	if add == nil {
		t.Fatal("expected 'add' node")
	}
	if add.Kind != "function" {
		t.Errorf("add.Kind = %q, want 'function'", add.Kind)
	}
	if add.Docstring == "" {
		t.Error("add should have a docstring")
	}

	greet := findNode(result.Nodes, "greet")
	if greet == nil {
		t.Fatal("expected 'greet' node")
	}

	multiply := findNode(result.Nodes, "multiply")
	if multiply == nil {
		t.Fatal("expected 'multiply' node (arrow function)")
	}
	if multiply.Kind != "function" {
		t.Errorf("multiply.Kind = %q, want 'function'", multiply.Kind)
	}

	double := findNode(result.Nodes, "double")
	if double == nil {
		t.Fatal("expected 'double' node (concise arrow)")
	}

	fetchData := findNode(result.Nodes, "fetchData")
	if fetchData == nil {
		t.Fatal("expected 'fetchData' node")
	}
}

func TestParseClasses(t *testing.T) {
	path, src := readFixture(t, "typescript", "classes.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	shape := findNode(result.Nodes, "Shape")
	if shape == nil {
		t.Fatal("expected 'Shape' node")
	}
	if shape.Kind != "class" {
		t.Errorf("Shape.Kind = %q, want 'class'", shape.Kind)
	}
	if shape.Docstring == "" {
		t.Error("Shape should have a docstring")
	}

	constructor := findNode(result.Nodes, "constructor")
	if constructor == nil {
		t.Fatal("expected 'constructor' method")
	}
	if constructor.Kind != "method" {
		t.Errorf("constructor.Kind = %q, want 'method'", constructor.Kind)
	}

	areaNodes := findNodes(result.Nodes, "area")
	if len(areaNodes) < 2 {
		t.Fatalf("expected at least 2 'area' methods, got %d", len(areaNodes))
	}

	circle := findNode(result.Nodes, "Circle")
	if circle == nil {
		t.Fatal("expected 'Circle' node")
	}

	unit := findNode(result.Nodes, "unit")
	if unit == nil {
		t.Fatal("expected 'unit' static method")
	}
	if unit.QualifiedName != "Circle.unit" {
		t.Errorf("unit.QualifiedName = %q, want 'Circle.unit'", unit.QualifiedName)
	}
}

func TestParseInterfaces(t *testing.T) {
	path, src := readFixture(t, "typescript", "interfaces.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 3 {
		t.Fatalf("expected 3 interfaces, got %d: %v", len(result.Nodes), nodeNames(result.Nodes))
	}

	for _, n := range result.Nodes {
		if n.Kind != "interface" {
			t.Errorf("%s.Kind = %q, want 'interface'", n.Name, n.Kind)
		}
	}

	repo := findNode(result.Nodes, "Repository")
	if repo == nil {
		t.Fatal("expected 'Repository' interface")
	}
}

func TestParseTypesAndEnums(t *testing.T) {
	path, src := readFixture(t, "typescript", "types.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %v", len(result.Nodes), nodeNames(result.Nodes))
	}

	status := findNode(result.Nodes, "Status")
	if status == nil || status.Kind != "type_alias" {
		t.Error("expected 'Status' type_alias")
	}

	pair := findNode(result.Nodes, "Pair")
	if pair == nil || pair.Kind != "type_alias" {
		t.Error("expected 'Pair' type_alias")
	}

	direction := findNode(result.Nodes, "Direction")
	if direction == nil || direction.Kind != "enum" {
		t.Error("expected 'Direction' enum")
	}

	color := findNode(result.Nodes, "Color")
	if color == nil || color.Kind != "enum" {
		t.Error("expected 'Color' enum")
	}
}

func TestParseEdgeCases(t *testing.T) {
	path, src := readFixture(t, "typescript", "edge-cases.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	defaultFn := findNode(result.Nodes, "defaultHandler")
	if defaultFn == nil {
		t.Fatal("expected 'defaultHandler' (export default function)")
	}

	format := findNode(result.Nodes, "format")
	if format == nil {
		t.Fatal("expected 'format' (overloaded, only implementation)")
	}

	asyncArrow := findNode(result.Nodes, "asyncArrow")
	if asyncArrow == nil {
		t.Fatal("expected 'asyncArrow'")
	}

	baseService := findNode(result.Nodes, "BaseService")
	if baseService == nil {
		t.Fatal("expected 'BaseService' abstract class")
	}
	if baseService.Kind != "class" {
		t.Errorf("BaseService.Kind = %q, want 'class'", baseService.Kind)
	}

	outer := findNode(result.Nodes, "outer")
	if outer == nil {
		t.Fatal("expected 'outer' function")
	}

	inner := findNode(result.Nodes, "inner")
	if inner != nil {
		t.Error("nested arrow 'inner' should NOT be extracted")
	}
}

func TestParseJavaScript(t *testing.T) {
	path, src := readFixture(t, "javascript", "functions.js")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(result.Nodes), nodeNames(result.Nodes))
	}

	add := findNode(result.Nodes, "add")
	if add == nil {
		t.Fatal("expected 'add'")
	}
	if add.Docstring == "" {
		t.Error("JS add should still have docstring")
	}
}

func TestParseTSX(t *testing.T) {
	path, src := readFixture(t, "tsx", "component.tsx")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	button := findNode(result.Nodes, "Button")
	if button == nil {
		t.Fatal("expected 'Button' arrow component")
	}

	counter := findNode(result.Nodes, "Counter")
	if counter == nil {
		t.Fatal("expected 'Counter' class component")
	}
	if counter.Kind != "class" {
		t.Errorf("Counter.Kind = %q, want 'class'", counter.Kind)
	}

	iface := findNode(result.Nodes, "ButtonProps")
	if iface == nil {
		t.Fatal("expected 'ButtonProps' interface")
	}
}

func TestUnknownExtension(t *testing.T) {
	_, err := ParseFile("test.rb", []byte("puts 'hello'"))
	if err == nil {
		t.Fatal("expected error for unknown extension")
	}
}

func TestEmptyFile(t *testing.T) {
	result, err := ParseFile("empty.ts", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes for empty file, got %d", len(result.Nodes))
	}
}

func TestSyntaxErrors(t *testing.T) {
	src := []byte("function broken( { return }")
	result, err := ParseFile("broken.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	_ = result
}

func TestBodyHashDeterminism(t *testing.T) {
	path, src := readFixture(t, "typescript", "functions.ts")
	r1, _ := ParseFile(path, src)
	r2, _ := ParseFile(path, src)

	if len(r1.Nodes) != len(r2.Nodes) {
		t.Fatal("node counts differ between runs")
	}
	for i := range r1.Nodes {
		if r1.Nodes[i].BodyHash != r2.Nodes[i].BodyHash {
			t.Errorf("body hash differs for %s", r1.Nodes[i].Name)
		}
	}
}

func TestBodyHashChangesOnModification(t *testing.T) {
	src1 := []byte("function foo(): void { console.log(1); }")
	src2 := []byte("function foo(): void { console.log(2); }")

	r1, _ := ParseFile("test.ts", src1)
	r2, _ := ParseFile("test.ts", src2)

	if len(r1.Nodes) == 0 || len(r2.Nodes) == 0 {
		t.Fatal("expected nodes from both sources")
	}
	if r1.Nodes[0].BodyHash == r2.Nodes[0].BodyHash {
		t.Error("body hash should change when source changes")
	}
}

func TestStats(t *testing.T) {
	path, src := readFixture(t, "typescript", "functions.ts")
	result, _ := ParseFile(path, src)
	stats := result.Stats()

	if stats["nodeCount"].(int) != len(result.Nodes) {
		t.Error("stats nodeCount mismatch")
	}
	byKind := stats["byKind"].(map[string]int)
	if byKind["function"] != len(result.Nodes) {
		t.Errorf("expected all nodes to be functions, got byKind: %v", byKind)
	}
}

func nodeNames(nodes []NodeInfo) []string {
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	return names
}

func findNodes(nodes []NodeInfo, name string) []NodeInfo {
	var found []NodeInfo
	for _, n := range nodes {
		if n.Name == name {
			found = append(found, n)
		}
	}
	return found
}
