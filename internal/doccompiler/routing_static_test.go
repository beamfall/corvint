package doccompiler_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type productionFunction struct {
	decl  *ast.FuncDecl
	edges map[string]struct{}
}

func TestProductionEntrypointsCannotReachNativeExecution(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate static routing test")
	}
	packageRoot := filepath.Dir(testFile)
	entries, err := os.ReadDir(packageRoot)
	if err != nil {
		t.Fatal(err)
	}

	files := make([]*ast.File, 0, len(entries))
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, filepath.Join(packageRoot, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		files = append(files, file)
	}

	functions := make(map[string]*productionFunction)
	freeByName := make(map[string]string)
	methodsByName := make(map[string][]string)
	roots := make(map[string]struct{})
	for _, file := range files {
		for _, declaration := range file.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if !isFunction || function.Body == nil {
				continue
			}
			key := functionKey(function)
			functions[key] = &productionFunction{decl: function, edges: make(map[string]struct{})}
			if function.Recv == nil {
				freeByName[function.Name.Name] = key
			} else {
				methodsByName[function.Name.Name] = append(methodsByName[function.Name.Name], key)
			}
			if ast.IsExported(function.Name.Name) || function.Name.Name == "init" {
				roots[key] = struct{}{}
			}
		}
	}

	for _, function := range functions {
		ast.Inspect(function.decl.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			switch value := call.Fun.(type) {
			case *ast.Ident:
				if target, exists := freeByName[value.Name]; exists {
					function.edges[target] = struct{}{}
				}
			case *ast.SelectorExpr:
				for _, target := range methodsByName[value.Sel.Name] {
					function.edges[target] = struct{}{}
				}
			}
			return true
		})
	}

	sinks := map[string]struct{}{
		"func:build":      {},
		"func:discover":   {},
		"func:runCommand": {},
	}
	for sink := range sinks {
		if _, exists := functions[sink]; !exists {
			t.Fatalf("native execution sink %s is missing; update the closed routing test deliberately", sink)
		}
	}

	// TPN-V0-001 permits the fixed probe and read-only Git edges only from the
	// explicit trusted entrypoints. Default entrypoints retain the full guard.
	trustedRoots := map[string]bool{
		"func:InspectTrustedNavigation": true,
		"func:TrustedNavigation":        true,
	}
	for root := range trustedRoots {
		if _, exists := roots[root]; !exists {
			t.Fatalf("trusted entrypoint %s is missing", root)
		}
	}
	trustedFunctions := maps.Clone(functions)
	for _, caller := range []string{"func:navProbe", "func:navGitText"} {
		function := functions[caller]
		if function == nil {
			t.Fatalf("trusted execution caller %s is missing", caller)
		}
		if _, exists := function.edges["func:runCommand"]; !exists {
			t.Fatalf("trusted execution edge %s -> runCommand is missing", caller)
		}
		copy := *function
		copy.edges = maps.Clone(function.edges)
		delete(copy.edges, "func:runCommand")
		trustedFunctions[caller] = &copy
	}

	rootKeys := make([]string, 0, len(roots))
	for root := range roots {
		rootKeys = append(rootKeys, root)
	}
	sort.Strings(rootKeys)
	for _, root := range rootKeys {
		graph := functions
		if trustedRoots[root] {
			graph = trustedFunctions
		}
		if path := routeToSink(root, graph, sinks, nil); len(path) > 0 {
			t.Fatalf("production entrypoint reaches native host execution: %s", strings.Join(path, " -> "))
		}
	}
}

func functionKey(function *ast.FuncDecl) string {
	if function.Recv == nil {
		return "func:" + function.Name.Name
	}
	receiver := "unknown"
	if len(function.Recv.List) == 1 {
		switch value := function.Recv.List[0].Type.(type) {
		case *ast.Ident:
			receiver = value.Name
		case *ast.StarExpr:
			if name, ok := value.X.(*ast.Ident); ok {
				receiver = name.Name
			}
		}
	}
	return "method:" + receiver + "." + function.Name.Name
}

func routeToSink(current string, functions map[string]*productionFunction, sinks map[string]struct{}, path []string) []string {
	for _, visited := range path {
		if visited == current {
			return nil
		}
	}
	path = append(append([]string(nil), path...), current)
	if _, isSink := sinks[current]; isSink {
		return path
	}
	function, exists := functions[current]
	if !exists {
		return nil
	}
	targets := make([]string, 0, len(function.edges))
	for target := range function.edges {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	for _, target := range targets {
		if result := routeToSink(target, functions, sinks, path); len(result) > 0 {
			return result
		}
	}
	return nil
}
