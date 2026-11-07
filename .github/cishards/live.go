package cishards

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// AFP-V0-2701: the live test tier is discovered from the source, never listed by hand. A live test
// is a top-level TestXxx function in a tracked root-module _test.go file whose body calls
// testsupport.RequireLiveTier (AFP-V0-2700). PR shards build without the live tag, so those tests
// skip there (AFP-V0-2702); `make live-test` runs exactly this set with -tags live.

// LiveTierCall is the selector name that marks a test as live.
const LiveTierCall = "RequireLiveTier"

// LivePackage is one package directory and its live tests, sorted by name.
type LivePackage struct {
	Dir   string // slash-separated, relative to the repository root
	Tests []string
}

// LiveTests lists the live tier of the repository at root from its tracked files. It skips
// testdata, vendor, hidden directories and nested modules, matching what `go test ./...` reaches.
func LiveTests(root string) ([]LivePackage, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*_test.go", "*go.mod").Output()
	if err != nil {
		return nil, fmt.Errorf("live tier: git ls-files in %s: %w", root, err)
	}
	var files, modules []string
	for _, name := range strings.Split(string(bytes.TrimRight(out, "\x00")), "\x00") {
		switch {
		case name == "" || name == "go.mod":
		case path.Base(name) == "go.mod":
			modules = append(modules, path.Dir(name)+"/")
		case strings.HasSuffix(name, "_test.go"):
			files = append(files, name)
		}
	}
	byDir := map[string][]string{}
	for _, name := range files {
		if !rootModuleSource(name, modules) {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return nil, fmt.Errorf("live tier: %w", err)
		}
		tests, err := liveTestsInFile(name, src)
		if err != nil {
			return nil, err
		}
		dir := path.Dir(name)
		byDir[dir] = append(byDir[dir], tests...)
	}
	packages := make([]LivePackage, 0, len(byDir))
	for dir, tests := range byDir {
		if len(tests) == 0 {
			continue
		}
		sort.Strings(tests)
		packages = append(packages, LivePackage{Dir: dir, Tests: tests})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Dir < packages[j].Dir })
	return packages, nil
}

// rootModuleSource reports whether a tracked file belongs to the root module's ./... pattern.
func rootModuleSource(name string, modules []string) bool {
	for _, module := range modules {
		if strings.HasPrefix(name, module) {
			return false
		}
	}
	dir := path.Dir(name)
	if dir == "." {
		return true // a module-root file: path.Dir yields ".", which is not a hidden directory
	}
	for _, part := range strings.Split(dir, "/") {
		if part == "testdata" || part == "vendor" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
			return false
		}
	}
	return true
}

// liveTestsInFile returns the top-level TestXxx functions in src whose body calls RequireLiveTier.
func liveTestsInFile(name string, src []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("live tier: %w", err)
	}
	var tests []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		live := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == LiveTierCall {
					live = true
				}
			}
			return !live
		})
		if live {
			tests = append(tests, fn.Name.Name)
		}
	}
	return tests, nil
}
