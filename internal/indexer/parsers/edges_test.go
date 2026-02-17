package parsers

import (
	"strings"
	"testing"
)

func findEdge(edges []EdgeInfo, kind, source, target string) *EdgeInfo {
	for i := range edges {
		if edges[i].Kind == kind && edges[i].Source == source && edges[i].Target == target {
			return &edges[i]
		}
	}
	return nil
}

func findEdges(edges []EdgeInfo, kind string) []EdgeInfo {
	var found []EdgeInfo
	for _, e := range edges {
		if e.Kind == kind {
			found = append(found, e)
		}
	}
	return found
}

func TestImportEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	imports := findEdges(result.Edges, "imports")
	if len(imports) != 4 {
		t.Fatalf("expected 4 import edges, got %d", len(imports))
	}

	authImport := findEdge(result.Edges, "imports", path, "@company/auth")
	if authImport == nil {
		t.Fatal("expected import edge for @company/auth")
	}
	if len(authImport.Symbols) != 2 {
		t.Fatalf("expected 2 symbols from @company/auth, got %d", len(authImport.Symbols))
	}
	if authImport.Symbols[0] != "validateToken" || authImport.Symbols[1] != "hashPassword" {
		t.Errorf("unexpected symbols: %v", authImport.Symbols)
	}
	if authImport.Line != 2 {
		t.Errorf("expected import on line 2, got %d", authImport.Line)
	}

	utilsImport := findEdge(result.Edges, "imports", path, "./utils")
	if utilsImport == nil {
		t.Fatal("expected import edge for ./utils")
	}
	if len(utilsImport.Symbols) != 1 || utilsImport.Symbols[0] != "* as utils" {
		t.Errorf("expected namespace import '* as utils', got %v", utilsImport.Symbols)
	}

	reactImport := findEdge(result.Edges, "imports", path, "react")
	if reactImport == nil {
		t.Fatal("expected import edge for react")
	}
	if len(reactImport.Symbols) != 1 || reactImport.Symbols[0] != "React" {
		t.Errorf("expected default import 'React', got %v", reactImport.Symbols)
	}

	typesImport := findEdge(result.Edges, "imports", path, "./types")
	if typesImport == nil {
		t.Fatal("expected import edge for ./types")
	}
	if len(typesImport.Symbols) != 1 || typesImport.Symbols[0] != "UserType" {
		t.Errorf("expected type import 'UserType', got %v", typesImport.Symbols)
	}
}

func TestExtendsEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	extendsEdges := findEdges(result.Edges, "extends")
	if len(extendsEdges) != 1 {
		t.Fatalf("expected 1 extends edge, got %d", len(extendsEdges))
	}

	ext := findEdge(result.Edges, "extends", "Admin", "User")
	if ext == nil {
		t.Fatal("expected Admin extends User")
	}
	if ext.Line != 31 {
		t.Errorf("expected extends on line 31, got %d", ext.Line)
	}

	_ = path
}

func TestImplementsEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	implEdges := findEdges(result.Edges, "implements")
	if len(implEdges) != 2 {
		t.Fatalf("expected 2 implements edges, got %d", len(implEdges))
	}

	if findEdge(result.Edges, "implements", "Admin", "Serializable") == nil {
		t.Error("expected Admin implements Serializable")
	}
	if findEdge(result.Edges, "implements", "Admin", "Loggable") == nil {
		t.Error("expected Admin implements Loggable")
	}

	_ = path
}

func TestContainsEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	containsEdges := findEdges(result.Edges, "contains")

	if findEdge(result.Edges, "contains", path, "User") == nil {
		t.Error("expected file contains User")
	}
	if findEdge(result.Edges, "contains", path, "Admin") == nil {
		t.Error("expected file contains Admin")
	}
	if findEdge(result.Edges, "contains", path, "processUser") == nil {
		t.Error("expected file contains processUser")
	}
	if findEdge(result.Edges, "contains", path, "createAdmin") == nil {
		t.Error("expected file contains createAdmin")
	}
	if findEdge(result.Edges, "contains", path, "Serializable") == nil {
		t.Error("expected file contains Serializable")
	}
	if findEdge(result.Edges, "contains", path, "Loggable") == nil {
		t.Error("expected file contains Loggable")
	}
	if findEdge(result.Edges, "contains", path, "UserId") == nil {
		t.Error("expected file contains UserId")
	}

	if findEdge(result.Edges, "contains", "User", "User.constructor") == nil {
		t.Error("expected User contains User.constructor")
	}
	if findEdge(result.Edges, "contains", "User", "User.greet") == nil {
		t.Error("expected User contains User.greet")
	}
	if findEdge(result.Edges, "contains", "Admin", "Admin.constructor") == nil {
		t.Error("expected Admin contains Admin.constructor")
	}
	if findEdge(result.Edges, "contains", "Admin", "Admin.serialize") == nil {
		t.Error("expected Admin contains Admin.serialize")
	}
	if findEdge(result.Edges, "contains", "Admin", "Admin.promote") == nil {
		t.Error("expected Admin contains Admin.promote")
	}

	_ = containsEdges
}

func TestCallEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "calls", "processUser", "validateToken") == nil {
		t.Error("expected processUser calls validateToken")
	}
	if findEdge(result.Edges, "calls", "processUser", "hashPassword") == nil {
		t.Error("expected processUser calls hashPassword")
	}
	if findEdge(result.Edges, "calls", "processUser", "console.log") == nil {
		t.Error("expected processUser calls console.log")
	}
	if findEdge(result.Edges, "calls", "processUser", "Promise.resolve") == nil {
		t.Error("expected processUser calls Promise.resolve")
	}

	if findEdge(result.Edges, "calls", "Admin.constructor", "super") == nil {
		t.Error("expected Admin.constructor calls super")
	}
	if findEdge(result.Edges, "calls", "Admin.constructor", "this.init") == nil {
		t.Error("expected Admin.constructor calls this.init")
	}

	if findEdge(result.Edges, "calls", "Admin.promote", "validateToken") == nil {
		t.Error("expected Admin.promote calls validateToken")
	}
	if findEdge(result.Edges, "calls", "Admin.promote", "utils.notify") == nil {
		t.Error("expected Admin.promote calls utils.notify")
	}

	if findEdge(result.Edges, "calls", "Admin.serialize", "JSON.stringify") == nil {
		t.Error("expected Admin.serialize calls JSON.stringify")
	}

	if findEdge(result.Edges, "calls", "User.constructor", "hashPassword") == nil {
		t.Error("expected User.constructor calls hashPassword")
	}

	_ = path
}

func TestCallEdgesSkipNestedLambdas(t *testing.T) {
	src := []byte(`function outer(): void {
  const inner = (x: number) => doSomething(x);
  console.log("hello");
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "calls", "outer", "console.log") == nil {
		t.Error("expected outer calls console.log")
	}
	if findEdge(result.Edges, "calls", "outer", "doSomething") != nil {
		t.Error("should NOT capture doSomething inside nested arrow")
	}
}

func TestUsesTypeEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "uses_type", "processUser", "User") == nil {
		t.Error("expected processUser uses_type User")
	}
	if findEdge(result.Edges, "uses_type", "processUser", "Promise") == nil {
		t.Error("expected processUser uses_type Promise")
	}
	if findEdge(result.Edges, "uses_type", "processUser", "UserType") == nil {
		t.Error("expected processUser uses_type UserType")
	}

	if findEdge(result.Edges, "uses_type", "Admin.promote", "User") == nil {
		t.Error("expected Admin.promote uses_type User")
	}

	if findEdge(result.Edges, "uses_type", "createAdmin", "Admin") == nil {
		t.Error("expected createAdmin uses_type Admin")
	}

	_ = path
}

func TestUsesTypeSkipsBuiltins(t *testing.T) {
	src := []byte(`function foo(a: string, b: number): boolean {
  return true;
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	typeEdges := findEdges(result.Edges, "uses_type")
	if len(typeEdges) != 0 {
		t.Errorf("expected no uses_type edges for builtins, got %d: %v", len(typeEdges), typeEdges)
	}
}

func TestEdgesFromExistingFixtures(t *testing.T) {
	t.Run("classes fixture has extends", func(t *testing.T) {
		path, src := readFixture(t, "typescript", "classes.ts")
		result, err := ParseFile(path, src)
		if err != nil {
			t.Fatal(err)
		}

		ext := findEdge(result.Edges, "extends", "Circle", "Shape")
		if ext == nil {
			t.Error("expected Circle extends Shape")
		}
	})

	t.Run("tsx fixture has imports", func(t *testing.T) {
		path, src := readFixture(t, "tsx", "component.tsx")
		result, err := ParseFile(path, src)
		if err != nil {
			t.Fatal(err)
		}

		imp := findEdge(result.Edges, "imports", path, "react")
		if imp == nil {
			t.Error("expected import of react")
		}
		if imp != nil && (len(imp.Symbols) != 1 || imp.Symbols[0] != "React") {
			t.Errorf("expected default import 'React', got %v", imp.Symbols)
		}
	})
}

func TestEdgeStatsIncludesEdges(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	stats := result.Stats()
	edgeCount := stats["edgeCount"].(int)
	if edgeCount == 0 {
		t.Error("expected non-zero edge count in stats")
	}

	edgesByKind, ok := stats["edgesByKind"].(map[string]int)
	if !ok {
		t.Fatal("expected edgesByKind in stats")
	}
	if edgesByKind["imports"] != 4 {
		t.Errorf("expected 4 import edges in stats, got %d", edgesByKind["imports"])
	}
	if edgesByKind["extends"] != 1 {
		t.Errorf("expected 1 extends edge in stats, got %d", edgesByKind["extends"])
	}
	if edgesByKind["implements"] != 2 {
		t.Errorf("expected 2 implements edges in stats, got %d", edgesByKind["implements"])
	}

	_ = path
}

func TestNoEdgesForEmptyFile(t *testing.T) {
	result, err := ParseFile("empty.ts", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Edges) != 0 {
		t.Errorf("expected 0 edges for empty file, got %d", len(result.Edges))
	}
}

func TestEdgeLineNumbers(t *testing.T) {
	src := []byte(`import { foo } from "./foo";

function bar(): void {
  foo();
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	imp := findEdge(result.Edges, "imports", "test.ts", "./foo")
	if imp == nil {
		t.Fatal("expected import edge")
	}
	if imp.Line != 1 {
		t.Errorf("import line: got %d, want 1", imp.Line)
	}

	call := findEdge(result.Edges, "calls", "bar", "foo")
	if call == nil {
		t.Fatal("expected call edge from bar to foo")
	}
	if call.Line != 4 {
		t.Errorf("call line: got %d, want 4", call.Line)
	}
}

func TestMultipleCallsSameTarget(t *testing.T) {
	src := []byte(`function process(): void {
  validate();
  validate();
  validate();
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	var callEdges []EdgeInfo
	for _, e := range result.Edges {
		if e.Kind == "calls" && e.Target == "validate" {
			callEdges = append(callEdges, e)
		}
	}
	if len(callEdges) != 3 {
		t.Errorf("expected 3 call edges to validate, got %d", len(callEdges))
	}
}

func TestEdgesWithExportedDeclarations(t *testing.T) {
	src := []byte(`import { dep } from "./dep";

export class Service implements Handler {
  handle(): void {
    dep();
  }
}

export function helper(s: Service): void {
  dep();
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "implements", "Service", "Handler") == nil {
		t.Error("expected Service implements Handler")
	}
	if findEdge(result.Edges, "contains", "test.ts", "Service") == nil {
		t.Error("expected file contains Service")
	}
	if findEdge(result.Edges, "contains", "test.ts", "helper") == nil {
		t.Error("expected file contains helper")
	}
	if findEdge(result.Edges, "calls", "Service.handle", "dep") == nil {
		t.Error("expected Service.handle calls dep")
	}
	if findEdge(result.Edges, "calls", "helper", "dep") == nil {
		t.Error("expected helper calls dep")
	}
	if findEdge(result.Edges, "uses_type", "helper", "Service") == nil {
		t.Error("expected helper uses_type Service")
	}

	_ = strings.TrimSpace
}

func TestAllSixEdgeKinds(t *testing.T) {
	path, src := readFixture(t, "typescript", "edges.ts")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	kinds := make(map[string]bool)
	for _, e := range result.Edges {
		kinds[e.Kind] = true
	}

	for _, k := range []string{"imports", "calls", "extends", "implements", "uses_type", "contains"} {
		if !kinds[k] {
			t.Errorf("missing edge kind: %s", k)
		}
	}

	_ = path
}

func TestImportOnlyFile(t *testing.T) {
	src := []byte(`import { foo } from "./foo";
import { bar } from "./bar";`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Edges) != 2 {
		t.Errorf("expected 2 import edges, got %d", len(result.Edges))
	}
	for _, e := range result.Edges {
		if e.Kind != "imports" {
			t.Errorf("unexpected edge kind: %s", e.Kind)
		}
	}
}

func TestClassNoHeritage(t *testing.T) {
	src := []byte(`class Simple {
  run(): void {}
}`)
	result, err := ParseFile("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Edges {
		if e.Kind == "extends" || e.Kind == "implements" {
			t.Errorf("should not have %s edge for class without heritage", e.Kind)
		}
	}
}
