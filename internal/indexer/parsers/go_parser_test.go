package parsers

import (
	"testing"
)

func TestGoParseNodes(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 9 {
		t.Fatalf("expected 9 nodes, got %d: %v", len(result.Nodes), nodeNames(result.Nodes))
	}

	user := findNode(result.Nodes, "User")
	if user == nil || user.Kind != "struct" {
		t.Error("expected User struct")
	}

	admin := findNode(result.Nodes, "Admin")
	if admin == nil || admin.Kind != "struct" {
		t.Error("expected Admin struct")
	}

	serializer := findNode(result.Nodes, "Serializer")
	if serializer == nil || serializer.Kind != "interface" {
		t.Error("expected Serializer interface")
	}

	userID := findNode(result.Nodes, "UserID")
	if userID == nil || userID.Kind != "type_alias" {
		t.Error("expected UserID type_alias")
	}

	newUser := findNode(result.Nodes, "NewUser")
	if newUser == nil || newUser.Kind != "function" {
		t.Error("expected NewUser function")
	}

	_ = path
}

func TestGoMethods(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	greet := findNode(result.Nodes, "Greet")
	if greet == nil {
		t.Fatal("expected Greet method")
	}
	if greet.Kind != "method" {
		t.Errorf("Greet.Kind = %q, want 'method'", greet.Kind)
	}
	if greet.QualifiedName != "User.Greet" {
		t.Errorf("Greet.QualifiedName = %q, want 'User.Greet'", greet.QualifiedName)
	}

	str := findNode(result.Nodes, "String")
	if str == nil {
		t.Fatal("expected String method")
	}
	if str.QualifiedName != "User.String" {
		t.Errorf("String.QualifiedName = %q, want 'User.String'", str.QualifiedName)
	}

	promote := findNode(result.Nodes, "Promote")
	if promote == nil {
		t.Fatal("expected Promote method")
	}
	if promote.QualifiedName != "Admin.Promote" {
		t.Errorf("Promote.QualifiedName = %q, want 'Admin.Promote'", promote.QualifiedName)
	}

	_ = path
}

func TestGoDocstrings(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	user := findNode(result.Nodes, "User")
	if user == nil || user.Docstring != "User represents a person in the system." {
		t.Errorf("User docstring = %q", user.Docstring)
	}

	newUser := findNode(result.Nodes, "NewUser")
	if newUser == nil || newUser.Docstring != "NewUser creates a new User." {
		t.Errorf("NewUser docstring = %q", newUser.Docstring)
	}

	greet := findNode(result.Nodes, "Greet")
	if greet == nil || greet.Docstring != "Greet returns a greeting string." {
		t.Errorf("Greet docstring = %q", greet.Docstring)
	}

	_ = path
}

func TestGoImportEdges(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	imports := findEdges(result.Edges, "imports")
	if len(imports) != 3 {
		t.Fatalf("expected 3 import edges, got %d", len(imports))
	}

	if findEdge(result.Edges, "imports", path, "fmt") == nil {
		t.Error("expected import of fmt")
	}
	if findEdge(result.Edges, "imports", path, "strings") == nil {
		t.Error("expected import of strings")
	}

	pkgImport := findEdge(result.Edges, "imports", path, "github.com/go-chi/chi/v5")
	if pkgImport == nil {
		t.Fatal("expected import of github.com/go-chi/chi/v5")
	}
	if len(pkgImport.Symbols) != 1 || pkgImport.Symbols[0] != "myalias (alias)" {
		t.Errorf("expected alias symbol, got %v", pkgImport.Symbols)
	}
}

func TestGoEmbedEdges(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	embed := findEdge(result.Edges, "embeds", "Admin", "User")
	if embed == nil {
		t.Error("expected Admin embeds User")
	}

	embedEdges := findEdges(result.Edges, "embeds")
	if len(embedEdges) != 1 {
		t.Errorf("expected 1 embed edge, got %d", len(embedEdges))
	}

	_ = path
}

func TestGoCallEdges(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "calls", "User.Greet", "fmt.Sprintf") == nil {
		t.Error("expected User.Greet calls fmt.Sprintf")
	}
	if findEdge(result.Edges, "calls", "User.String", "strings.Join") == nil {
		t.Error("expected User.String calls strings.Join")
	}
	if findEdge(result.Edges, "calls", "Admin.Promote", "fmt.Println") == nil {
		t.Error("expected Admin.Promote calls fmt.Println")
	}
	if findEdge(result.Edges, "calls", "Admin.Promote", "myalias.NewRouter") == nil {
		t.Error("expected Admin.Promote calls myalias.NewRouter")
	}
	if findEdge(result.Edges, "calls", "process", "fmt.Println") == nil {
		t.Error("expected process calls fmt.Println")
	}

	_ = path
}

func TestGoContainsEdges(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "contains", path, "User") == nil {
		t.Error("expected file contains User")
	}
	if findEdge(result.Edges, "contains", path, "Admin") == nil {
		t.Error("expected file contains Admin")
	}
	if findEdge(result.Edges, "contains", path, "Serializer") == nil {
		t.Error("expected file contains Serializer")
	}
	if findEdge(result.Edges, "contains", path, "UserID") == nil {
		t.Error("expected file contains UserID")
	}
	if findEdge(result.Edges, "contains", path, "NewUser") == nil {
		t.Error("expected file contains NewUser")
	}
	if findEdge(result.Edges, "contains", path, "process") == nil {
		t.Error("expected file contains process")
	}

	if findEdge(result.Edges, "contains", "User", "User.Greet") == nil {
		t.Error("expected User contains User.Greet")
	}
	if findEdge(result.Edges, "contains", "User", "User.String") == nil {
		t.Error("expected User contains User.String")
	}
	if findEdge(result.Edges, "contains", "Admin", "Admin.Promote") == nil {
		t.Error("expected Admin contains Admin.Promote")
	}
}

func TestGoUsesTypeEdges(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, err := ParseFile(path, src)
	if err != nil {
		t.Fatal(err)
	}

	if findEdge(result.Edges, "uses_type", "NewUser", "User") == nil {
		t.Error("expected NewUser uses_type User")
	}
	if findEdge(result.Edges, "uses_type", "Admin.Promote", "User") == nil {
		t.Error("expected Admin.Promote uses_type User")
	}
	if findEdge(result.Edges, "uses_type", "process", "User") == nil {
		t.Error("expected process uses_type User")
	}

	_ = path
}

func TestGoUsesTypeSkipsBuiltins(t *testing.T) {
	src := []byte(`package main

func foo(a string, b int) bool {
	return true
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	typeEdges := findEdges(result.Edges, "uses_type")
	if len(typeEdges) != 0 {
		t.Errorf("expected no uses_type edges for builtins, got %d", len(typeEdges))
	}
}

func TestGoEmptyFile(t *testing.T) {
	src := []byte(`package main`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(result.Nodes))
	}
}

func TestGoBodyHash(t *testing.T) {
	src := []byte(`package main

func foo() {}`)
	r1, _ := ParseFile("test.go", src)
	r2, _ := ParseFile("test.go", src)
	if len(r1.Nodes) == 0 {
		t.Fatal("expected nodes")
	}
	if r1.Nodes[0].BodyHash != r2.Nodes[0].BodyHash {
		t.Error("body hash should be deterministic")
	}
}

func TestGoPointerReceiver(t *testing.T) {
	src := []byte(`package main

type Svc struct{}

func (s *Svc) Run() {}
func (s Svc) Stop() {}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}

	run := findNode(result.Nodes, "Run")
	if run == nil || run.QualifiedName != "Svc.Run" {
		t.Errorf("expected Svc.Run, got %v", run)
	}
	stop := findNode(result.Nodes, "Stop")
	if stop == nil || stop.QualifiedName != "Svc.Stop" {
		t.Errorf("expected Svc.Stop, got %v", stop)
	}
}

func TestGoPointerEmbed(t *testing.T) {
	src := []byte(`package main

type Base struct{}
type Derived struct {
	*Base
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	embed := findEdge(result.Edges, "embeds", "Derived", "Base")
	if embed == nil {
		t.Error("expected Derived embeds Base (pointer embed)")
	}
}

func TestGoSingleImport(t *testing.T) {
	src := []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello")
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	imp := findEdge(result.Edges, "imports", "test.go", "fmt")
	if imp == nil {
		t.Error("expected import of fmt (single import syntax)")
	}
}

func TestGoCallTraversesClosure(t *testing.T) {
	src := []byte(`package main

func outer() {
	fn := func() {
		hidden()
	}
	fn()
	visible()
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if findEdge(result.Edges, "calls", "outer", "hidden") == nil {
		t.Error("expected outer calls hidden (inside closure)")
	}
	if findEdge(result.Edges, "calls", "outer", "visible") == nil {
		t.Error("expected outer calls visible")
	}
	if findEdge(result.Edges, "calls", "outer", "fn") == nil {
		t.Error("expected outer calls fn")
	}
}

func TestGoCallInsideGoroutine(t *testing.T) {
	src := []byte(`package main

func launcher() {
	go func() {
		doWork()
		fmt.Println("done")
	}()
	cleanup()
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if findEdge(result.Edges, "calls", "launcher", "doWork") == nil {
		t.Error("expected launcher calls doWork (inside goroutine)")
	}
	if findEdge(result.Edges, "calls", "launcher", "fmt.Println") == nil {
		t.Error("expected launcher calls fmt.Println (inside goroutine)")
	}
	if findEdge(result.Edges, "calls", "launcher", "cleanup") == nil {
		t.Error("expected launcher calls cleanup")
	}
}

func TestGoCallNestedClosures(t *testing.T) {
	src := []byte(`package main

func deep() {
	fn := func() {
		go func() {
			innerCall()
		}()
	}
	fn()
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if findEdge(result.Edges, "calls", "deep", "innerCall") == nil {
		t.Error("expected deep calls innerCall (nested closure + goroutine)")
	}
	if findEdge(result.Edges, "calls", "deep", "fn") == nil {
		t.Error("expected deep calls fn")
	}
}

func TestGoSignatures(t *testing.T) {
	src := []byte(`package main

func add(a, b int) int {
	return a + b
}

func (u *User) Greet(name string) string {
	return "hi"
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}

	add := findNode(result.Nodes, "add")
	if add == nil {
		t.Fatal("expected add")
	}
	if add.Signature != "func add(a, b int) int" {
		t.Errorf("add signature = %q", add.Signature)
	}

	greet := findNode(result.Nodes, "Greet")
	if greet == nil {
		t.Fatal("expected Greet")
	}
	if greet.Signature != "func (u *User) Greet(name string) string" {
		t.Errorf("Greet signature = %q", greet.Signature)
	}
}

func TestGoStats(t *testing.T) {
	path, src := readFixture(t, "go", "sample.go")
	result, _ := ParseFile(path, src)
	stats := result.Stats()

	if stats["nodeCount"].(int) != 9 {
		t.Errorf("expected 9 nodes, got %d", stats["nodeCount"])
	}
	if stats["edgeCount"].(int) == 0 {
		t.Error("expected non-zero edge count")
	}

	byKind := stats["byKind"].(map[string]int)
	if byKind["struct"] != 2 {
		t.Errorf("expected 2 structs, got %d", byKind["struct"])
	}
	if byKind["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", byKind["interface"])
	}
	if byKind["method"] != 3 {
		t.Errorf("expected 3 methods, got %d", byKind["method"])
	}
}

func TestGoDotImport(t *testing.T) {
	src := []byte(`package main

import . "fmt"

func main() {
	Println("hello")
}`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	imp := findEdge(result.Edges, "imports", "test.go", "fmt")
	if imp == nil {
		t.Fatal("expected import of fmt")
	}
	if len(imp.Symbols) != 1 || imp.Symbols[0] != ". (dot import)" {
		t.Errorf("expected dot import symbol, got %v", imp.Symbols)
	}
}

func TestGoBlankImport(t *testing.T) {
	src := []byte(`package main

import _ "net/http/pprof"`)
	result, err := ParseFile("test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	imp := findEdge(result.Edges, "imports", "test.go", "net/http/pprof")
	if imp == nil {
		t.Fatal("expected import of net/http/pprof")
	}
	if len(imp.Symbols) != 1 || imp.Symbols[0] != "_ (side effect)" {
		t.Errorf("expected side-effect symbol, got %v", imp.Symbols)
	}
}
