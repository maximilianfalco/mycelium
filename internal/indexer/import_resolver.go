package indexer

import (
	"path/filepath"
	"strings"

	"github.com/maximilianfalco/mycelium/internal/indexer/parsers"
)

// ResolvedEdge is an import or call edge with both specifier and resolved file path.
type ResolvedEdge struct {
	Source       string   `json:"source"`
	Target       string   `json:"target"`
	ResolvedPath string   `json:"resolvedPath"`
	Kind         string   `json:"kind"`
	Line         int      `json:"line"`
	Symbols      []string `json:"symbols,omitempty"`
}

// UnresolvedRef is an import or call that couldn't be resolved.
type UnresolvedRef struct {
	Source    string `json:"source"`
	RawImport string `json:"rawImport"`
	Kind      string `json:"kind"`
	Line      int    `json:"line"`
}

// ResolveResult holds the output of import resolution.
type ResolveResult struct {
	Resolved   []ResolvedEdge  `json:"resolved"`
	Unresolved []UnresolvedRef `json:"unresolved"`
	DependsOn  []ResolvedEdge  `json:"dependsOn"`
}

// nodeBuiltins is the set of Node.js built-in modules that should be skipped.
var nodeBuiltins = map[string]bool{
	"assert": true, "buffer": true, "child_process": true, "cluster": true,
	"crypto": true, "dgram": true, "dns": true, "domain": true,
	"events": true, "fs": true, "http": true, "https": true,
	"net": true, "os": true, "path": true, "perf_hooks": true,
	"process": true, "punycode": true, "querystring": true, "readline": true,
	"repl": true, "stream": true, "string_decoder": true, "sys": true,
	"timers": true, "tls": true, "tty": true, "url": true,
	"util": true, "v8": true, "vm": true, "worker_threads": true,
	"zlib": true, "console": true, "module": true,
}

// tsExtensions is the order in which TypeScript/JavaScript files are resolved.
var tsExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

// resolveStatus distinguishes "resolved", "skip" (builtin), and "unresolved".
type resolveStatus int

const (
	statusUnresolved resolveStatus = iota
	statusResolved
	statusSkipped
)

// ResolveImports takes raw edges from parsing, workspace alias maps, tsconfig
// paths, and a set of all parsed files, then resolves import specifiers to
// concrete file paths and traces call edges through imports.
func ResolveImports(
	rawEdges []parsers.EdgeInfo,
	aliasMap map[string]string,
	tsconfigPaths map[string]string,
	allNodes []parsers.NodeInfo,
	allFiles []string,
	rootPath string,
) *ResolveResult {
	result := &ResolveResult{}

	if aliasMap == nil {
		aliasMap = make(map[string]string)
	}
	if tsconfigPaths == nil {
		tsconfigPaths = make(map[string]string)
	}

	// Build lookup structures
	fileSet := buildFileSet(allFiles)
	nodesByFile := buildNodesByFile(rawEdges, allNodes)
	importedSymbols := buildImportedSymbolMap(rawEdges)
	nodesByName := buildNodesByName(allNodes)

	// Track package-level dependencies for depends_on edges
	packageDeps := make(map[string]map[string]bool)

	for _, edge := range rawEdges {
		switch edge.Kind {
		case "imports":
			resolved, status := resolveImportEdge(edge, aliasMap, tsconfigPaths, fileSet, rootPath)
			switch status {
			case statusResolved:
				result.Resolved = append(result.Resolved, *resolved)
				trackPackageDep(packageDeps, edge.Source, resolved.ResolvedPath, rootPath)
			case statusSkipped:
				// Builtin or stdlib — don't track
			case statusUnresolved:
				result.Unresolved = append(result.Unresolved, UnresolvedRef{
					Source:    edge.Source,
					RawImport: edge.Target,
					Kind:      "imports",
					Line:      edge.Line,
				})
			}

		case "calls":
			resolved := resolveCallEdge(edge, nodesByFile, importedSymbols, nodesByName)
			if resolved != nil {
				result.Resolved = append(result.Resolved, *resolved)
			}
		}
	}

	// Build depends_on edges from aggregated package-level imports
	for srcPkg, targets := range packageDeps {
		for tgtPkg := range targets {
			result.DependsOn = append(result.DependsOn, ResolvedEdge{
				Source: srcPkg,
				Target: tgtPkg,
				Kind:   "depends_on",
			})
		}
	}

	return result
}

// resolveImportEdge attempts to resolve a single import edge to a file path.
func resolveImportEdge(
	edge parsers.EdgeInfo,
	aliasMap map[string]string,
	tsconfigPaths map[string]string,
	fileSet map[string]bool,
	rootPath string,
) (*ResolvedEdge, resolveStatus) {
	specifier := edge.Target
	sourceFile := edge.Source

	sourceExt := filepath.Ext(sourceFile)
	isGoSource := sourceExt == ".go"

	// 1. Node built-in check (only for JS/TS source files)
	if !isGoSource && isNodeBuiltin(specifier) {
		return nil, statusSkipped
	}

	// 2. Go standard library check (only for Go source files)
	if isGoSource && isGoStdlib(specifier) {
		return nil, statusSkipped
	}

	makeResolved := func(resolvedPath string) *ResolvedEdge {
		return &ResolvedEdge{
			Source:       edge.Source,
			Target:       edge.Target,
			ResolvedPath: resolvedPath,
			Kind:         "imports",
			Line:         edge.Line,
			Symbols:      edge.Symbols,
		}
	}

	// 3. Check alias map (monorepo package names like @company/auth)
	if resolved := resolveViaAliasMap(specifier, aliasMap, fileSet); resolved != "" {
		return makeResolved(resolved), statusResolved
	}

	// 4. Check tsconfig path aliases (e.g., @/* → src/*)
	if resolved := resolveViaTSConfigPaths(specifier, tsconfigPaths, fileSet); resolved != "" {
		return makeResolved(resolved), statusResolved
	}

	// 5. Relative imports (./foo, ../bar)
	if strings.HasPrefix(specifier, ".") {
		sourceDir := filepath.Dir(sourceFile)
		if resolved := resolveRelativeImport(specifier, sourceDir, fileSet); resolved != "" {
			return makeResolved(resolved), statusResolved
		}
	}

	// 6. Go module import path check (non-relative, non-builtin)
	if resolved := resolveGoModuleImport(specifier, aliasMap, fileSet); resolved != "" {
		return makeResolved(resolved), statusResolved
	}

	// Unresolved — external npm package or unknown module
	return nil, statusUnresolved
}

// resolveViaAliasMap checks if the specifier matches a monorepo package name.
func resolveViaAliasMap(specifier string, aliasMap map[string]string, fileSet map[string]bool) string {
	// Exact match: @company/auth → packages/auth/src/index.ts
	if entryPoint, ok := aliasMap[specifier]; ok {
		if fileSet[entryPoint] {
			return entryPoint
		}
		// Try resolving with extensions in case entry point has none
		if resolved := tryExtensions(entryPoint, fileSet); resolved != "" {
			return resolved
		}
	}

	// Subpath match: @company/auth/validators → look relative to package root
	for alias, entryPoint := range aliasMap {
		if !strings.HasPrefix(specifier, alias+"/") {
			continue
		}
		rest := strings.TrimPrefix(specifier, alias+"/")

		// Derive package root from entry point.
		// Entry point: "packages/core/src/index.ts" → package root: "packages/core"
		pkgRoot := entryPointToPackageRoot(entryPoint)

		// Try: pkgRoot/rest (e.g., packages/core/src/validator)
		candidate := filepath.Join(pkgRoot, rest)
		if resolved := tryExtensions(candidate, fileSet); resolved != "" {
			return resolved
		}

		// Try: pkgRoot/src/rest
		candidate = filepath.Join(pkgRoot, "src", rest)
		if resolved := tryExtensions(candidate, fileSet); resolved != "" {
			return resolved
		}
	}

	return ""
}

// entryPointToPackageRoot extracts the package root directory from an entry point path.
// "packages/core/src/index.ts" → "packages/core"
// "packages/core/index.ts" → "packages/core"
func entryPointToPackageRoot(entryPoint string) string {
	dir := filepath.Dir(entryPoint) // "packages/core/src"
	base := filepath.Base(dir)      // "src"
	if base == "src" || base == "lib" || base == "dist" {
		return filepath.Dir(dir) // "packages/core"
	}
	return dir
}

// resolveViaTSConfigPaths checks tsconfig path aliases.
func resolveViaTSConfigPaths(specifier string, tsconfigPaths map[string]string, fileSet map[string]bool) string {
	for alias, target := range tsconfigPaths {
		// Wildcard alias: @/* → src/*
		if strings.HasSuffix(alias, "/*") {
			prefix := strings.TrimSuffix(alias, "/*")
			if strings.HasPrefix(specifier, prefix+"/") {
				rest := strings.TrimPrefix(specifier, prefix+"/")
				targetDir := strings.TrimSuffix(target, "/*")
				// Remove trailing separator if present
				targetDir = strings.TrimRight(targetDir, "/")
				candidate := filepath.Join(targetDir, rest)
				if resolved := tryExtensions(candidate, fileSet); resolved != "" {
					return resolved
				}
			}
		} else {
			// Exact alias match
			if specifier == alias {
				if resolved := tryExtensions(target, fileSet); resolved != "" {
					return resolved
				}
			}
		}
	}
	return ""
}

// resolveRelativeImport resolves a relative import like ./utils or ../shared.
func resolveRelativeImport(specifier, sourceDir string, fileSet map[string]bool) string {
	candidate := filepath.Join(sourceDir, specifier)
	// Clean the path (handles ../ properly)
	candidate = filepath.Clean(candidate)
	return tryExtensions(candidate, fileSet)
}

// resolveGoModuleImport checks if a Go import path matches a known module.
func resolveGoModuleImport(specifier string, aliasMap map[string]string, fileSet map[string]bool) string {
	for modulePath, relDir := range aliasMap {
		if specifier == modulePath {
			if dirHasGoFiles(relDir, fileSet) {
				return relDir
			}
			continue
		}
		if strings.HasPrefix(specifier, modulePath+"/") {
			rest := strings.TrimPrefix(specifier, modulePath+"/")
			var candidate string
			if relDir == "." {
				candidate = rest
			} else {
				candidate = filepath.Join(relDir, rest)
			}
			if dirHasGoFiles(candidate, fileSet) {
				return candidate
			}
		}
	}
	return ""
}

// resolveCallEdge attempts to resolve a call edge by tracing through imports.
func resolveCallEdge(
	edge parsers.EdgeInfo,
	nodesByFile map[string][]parsers.NodeInfo,
	importedSymbols map[string]map[string]string,
	nodesByName map[string][]parsers.NodeInfo,
) *ResolvedEdge {
	callerName := edge.Source
	calleeName := edge.Target

	if isGlobalCall(calleeName) {
		return nil
	}

	// 1. Check if the callee is defined in the same file
	callerFile := findFileForNode(callerName, nodesByFile)
	if callerFile != "" {
		for _, node := range nodesByFile[callerFile] {
			if node.QualifiedName == calleeName || node.Name == calleeName {
				return &ResolvedEdge{
					Source:       edge.Source,
					Target:       node.QualifiedName,
					ResolvedPath: callerFile,
					Kind:         "calls",
					Line:         edge.Line,
				}
			}
		}
	}

	// 2. Check if the callee was imported
	simpleName := calleeName
	if idx := strings.LastIndex(calleeName, "."); idx != -1 {
		simpleName = calleeName[idx+1:]
	}

	if callerFile != "" {
		if imports, ok := importedSymbols[callerFile]; ok {
			if sourceFile, found := imports[simpleName]; found {
				for _, node := range nodesByFile[sourceFile] {
					if node.Name == simpleName || node.QualifiedName == simpleName {
						return &ResolvedEdge{
							Source:       edge.Source,
							Target:       node.QualifiedName,
							ResolvedPath: sourceFile,
							Kind:         "calls",
							Line:         edge.Line,
						}
					}
				}
			}
		}
	}

	// 3. Global search by name — only if unambiguous (exactly one match)
	// Skip if the callee is a member expression with a common prototype method name,
	// e.g. "user.email.split" → "split" would falsely match a user-defined split().
	isMemberCall := strings.Contains(calleeName, ".")
	if isMemberCall && isBuiltinMethodName(simpleName) {
		return nil
	}
	if matches, ok := nodesByName[simpleName]; ok && len(matches) == 1 {
		return &ResolvedEdge{
			Source:       edge.Source,
			Target:       matches[0].QualifiedName,
			ResolvedPath: findFileForNode(matches[0].QualifiedName, nodesByFile),
			Kind:         "calls",
			Line:         edge.Line,
		}
	}

	return nil
}

// --- Helper functions ---

func buildFileSet(allFiles []string) map[string]bool {
	set := make(map[string]bool, len(allFiles))
	for _, f := range allFiles {
		set[f] = true
	}
	return set
}

// buildNodesByFile maps file path → nodes in that file using contains edges.
func buildNodesByFile(edges []parsers.EdgeInfo, nodes []parsers.NodeInfo) map[string][]parsers.NodeInfo {
	nodeMap := make(map[string]parsers.NodeInfo)
	for _, n := range nodes {
		nodeMap[n.QualifiedName] = n
	}

	byFile := make(map[string][]parsers.NodeInfo)
	for _, e := range edges {
		if e.Kind == "contains" {
			if n, ok := nodeMap[e.Target]; ok {
				byFile[e.Source] = append(byFile[e.Source], n)
			}
		}
	}
	return byFile
}

// buildImportedSymbolMap maps: file → (symbol name → import specifier).
func buildImportedSymbolMap(edges []parsers.EdgeInfo) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, e := range edges {
		if e.Kind != "imports" {
			continue
		}
		if result[e.Source] == nil {
			result[e.Source] = make(map[string]string)
		}
		for _, sym := range e.Symbols {
			sym = strings.TrimPrefix(sym, "* as ")
			result[e.Source][sym] = e.Target
		}
	}
	return result
}

func buildNodesByName(nodes []parsers.NodeInfo) map[string][]parsers.NodeInfo {
	byName := make(map[string][]parsers.NodeInfo)
	for _, n := range nodes {
		byName[n.Name] = append(byName[n.Name], n)
	}
	return byName
}

func findFileForNode(qualifiedName string, nodesByFile map[string][]parsers.NodeInfo) string {
	for file, nodes := range nodesByFile {
		for _, n := range nodes {
			if n.QualifiedName == qualifiedName {
				return file
			}
		}
	}
	return ""
}

// tryExtensions resolves a path by appending TS/JS extensions and /index variants.
func tryExtensions(candidate string, fileSet map[string]bool) string {
	if fileSet[candidate] {
		return candidate
	}

	for _, ext := range tsExtensions {
		if fileSet[candidate+ext] {
			return candidate + ext
		}
	}

	for _, ext := range tsExtensions {
		indexPath := filepath.Join(candidate, "index"+ext)
		if fileSet[indexPath] {
			return indexPath
		}
	}

	// Handle .js → .ts mapping (ESM imports)
	if strings.HasSuffix(candidate, ".js") {
		base := strings.TrimSuffix(candidate, ".js")
		if fileSet[base+".ts"] {
			return base + ".ts"
		}
		if fileSet[base+".tsx"] {
			return base + ".tsx"
		}
	}

	return ""
}

// dirHasGoFiles checks if any .go file in the file set lives in the given directory.
func dirHasGoFiles(dir string, fileSet map[string]bool) bool {
	for f := range fileSet {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		if filepath.Dir(f) == dir {
			return true
		}
	}
	return false
}

func isNodeBuiltin(specifier string) bool {
	mod := strings.TrimPrefix(specifier, "node:")
	if idx := strings.Index(mod, "/"); idx != -1 {
		mod = mod[:idx]
	}
	return nodeBuiltins[mod]
}

func isGoStdlib(specifier string) bool {
	// Go stdlib packages have no dots in the first path segment.
	// Third-party: github.com/..., golang.org/x/...
	firstSlash := strings.Index(specifier, "/")
	firstSegment := specifier
	if firstSlash != -1 {
		firstSegment = specifier[:firstSlash]
	}
	return !strings.Contains(firstSegment, ".")
}

func isGlobalCall(name string) bool {
	switch name {
	case "console.log", "console.error", "console.warn", "console.info",
		"JSON.stringify", "JSON.parse",
		"Promise.resolve", "Promise.reject", "Promise.all",
		"Math.round", "Math.floor", "Math.ceil", "Math.random",
		"parseInt", "parseFloat", "setTimeout", "setInterval",
		"clearTimeout", "clearInterval",
		"require", "super",
		"make", "len", "cap", "append", "copy", "delete", "close",
		"panic", "recover", "new", "print", "println",
		"fmt.Sprintf", "fmt.Printf", "fmt.Println", "fmt.Fprintf",
		"fmt.Errorf":
		return true
	}
	for _, prefix := range []string{"console.", "Math.", "Object.", "Array.", "String.", "Number.", "Promise."} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// isBuiltinMethodName returns true for method names that are common on JS/Go
// built-in types (String, Array, Map, etc.). When a call like "obj.split()"
// is extracted, we don't want tier-3 global resolution to match it to a
// user-defined function named "split".
var builtinMethodNames = map[string]bool{
	// JS String methods
	"split": true, "replace": true, "replaceAll": true, "match": true,
	"trim": true, "trimStart": true, "trimEnd": true, "toLowerCase": true,
	"toUpperCase": true, "startsWith": true, "endsWith": true, "includes": true,
	"indexOf": true, "lastIndexOf": true, "slice": true, "substring": true,
	"charAt": true, "charCodeAt": true, "padStart": true, "padEnd": true,
	"repeat": true, "normalize": true, "search": true, "at": true,
	// JS Array methods
	"push": true, "pop": true, "shift": true, "unshift": true,
	"map": true, "filter": true, "reduce": true, "reduceRight": true,
	"find": true, "findIndex": true, "some": true, "every": true,
	"forEach": true, "flat": true, "flatMap": true, "sort": true,
	"reverse": true, "concat": true, "join": true, "fill": true,
	"splice": true, "keys": true, "values": true, "entries": true,
	// JS Object methods
	"hasOwnProperty": true, "toString": true, "valueOf": true,
	"toJSON": true, "toLocaleString": true,
	// JS Promise methods
	"then": true, "catch": true, "finally": true,
	// JS Map/Set methods
	"get": true, "set": true, "has": true, "clear": true, "add": true,
	// JS Date methods
	"getTime": true, "toISOString": true, "toDateString": true,
	// JS common DOM/Node
	"addEventListener": true, "removeEventListener": true,
	"querySelector": true, "querySelectorAll": true,
	"getAttribute": true, "setAttribute": true,
	"createElement": true, "appendChild": true, "removeChild": true,
	// Go common methods that shadow user functions
	"Error": true, "String": true, "Close": true, "Read": true, "Write": true,
	"Scan": true, "Next": true, "Err": true, "Rows": true,
}

func isBuiltinMethodName(name string) bool {
	return builtinMethodNames[name]
}

// trackPackageDep records a package-level dependency based on file-level imports.
func trackPackageDep(deps map[string]map[string]bool, sourceFile, targetFile, rootPath string) {
	srcPkg := packageForFile(sourceFile)
	tgtPkg := packageForFile(targetFile)
	if srcPkg == tgtPkg || srcPkg == "" || tgtPkg == "" {
		return
	}
	if deps[srcPkg] == nil {
		deps[srcPkg] = make(map[string]bool)
	}
	deps[srcPkg][tgtPkg] = true
}

// packageForFile extracts the package directory from a file path.
// "packages/auth/src/validators.ts" → "packages/auth"
// "apps/web/src/index.tsx" → "apps/web"
func packageForFile(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	if len(parts) < 2 {
		return ""
	}
	for i, part := range parts {
		if (part == "packages" || part == "apps" || part == "libs" || part == "services") && i+1 < len(parts) {
			return parts[i] + "/" + parts[i+1]
		}
	}
	for i, part := range parts {
		if (part == "internal" || part == "cmd" || part == "pkg") && i+1 < len(parts) {
			return parts[i] + "/" + parts[i+1]
		}
	}
	return ""
}
