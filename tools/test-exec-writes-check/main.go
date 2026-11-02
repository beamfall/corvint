// Command test-exec-writes-check refuses a Go test file that writes an executable with
// os.WriteFile (docs/specs/corvint-self-development-v0.md, ASD-V0-929). A file a test writes
// and then executes can fail to start with ETXTBSY when a parallel test forks while the write
// descriptor is open; internal/execfile.WriteFile (or, on the Tasks side, the
// internal/tasks/fixture.WriteExecutable twin) holds syscall.ForkLock across the write and
// closes that race (WQO-V0-093, V1-2587). Run from the repository root, it parses every
// tracked *_test.go file outside testdata and internal/execfile and prints one FAIL line per
// os.WriteFile call whose permission argument is an integer literal with an execute bit. It
// exits 1 on a finding, 2 when it cannot list or read the files, and 0 otherwise. It reads
// files only.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// exemptDirs lists test directories the check skips, each with its reason. An entry is
// removed when its owner converts the files.
var exemptDirs = map[string]string{
	// internal/tasks/cli/ was owned by an in-flight lane (taw-w15) when V1-2587 landed; its
	// files convert to fixture.WriteExecutable once that lane merges.
	"internal/tasks/cli/": "V1-2587 follow-up after taw-w15",
}

// nestedModules are the separate Go modules in the tree. They cannot import
// internal/execfile, so each keeps a package-local ForkLock-guarded writer.
var nestedModules = []string{"interop/cem01-go/", "tools/local-authority/"}

func main() {
	os.Exit(run(".", os.Stdout, os.Stderr))
}

func run(root string, stdout, stderr io.Writer) int {
	list := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*_test.go")
	raw, err := list.Output()
	if err != nil {
		fmt.Fprintf(stderr, "test-exec-writes-check: cannot list test files: %v\n", err)
		return 2
	}
	failed := false
	for _, path := range strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00") {
		if skipped(path) {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(stderr, "test-exec-writes-check: %v\n", err)
			return 2
		}
		findings, err := scan(path, source)
		if err != nil {
			fmt.Fprintf(stderr, "test-exec-writes-check: %v\n", err)
			return 2
		}
		for _, finding := range findings {
			fmt.Fprintln(stdout, finding)
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}

func skipped(path string) bool {
	if path == "" || strings.HasPrefix(path, "internal/execfile/") {
		return true
	}
	for dir := range exemptDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return strings.HasPrefix(path, "testdata/") || strings.Contains(path, "/testdata/")
}

// scan returns one FAIL line per os.WriteFile call in source whose permission is an
// executable integer literal.
func scan(path string, source []byte) ([]string, error) {
	if !bytes.Contains(source, []byte("WriteFile")) {
		return nil, nil
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	osName := ""
	for _, spec := range file.Imports {
		if spec.Path.Value != `"os"` {
			continue
		}
		osName = "os"
		if spec.Name != nil {
			osName = spec.Name.Name
		}
	}
	if osName == "" || osName == "_" {
		return nil, nil
	}
	fix := "use execfile.WriteFile (github.com/Beamfall/corvint/internal/execfile)"
	if strings.HasPrefix(path, "internal/tasks/") || strings.HasPrefix(path, "cmd/corvint-tasks/") {
		fix = "use fixture.WriteExecutable (github.com/Beamfall/corvint/internal/tasks/fixture)"
	}
	for _, module := range nestedModules {
		if strings.HasPrefix(path, module) {
			fix = "use the package's writeExecutable test helper, which holds syscall.ForkLock.RLock across os.WriteFile (" + module + " is its own module and cannot import internal/execfile)"
		}
	}
	var findings []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 3 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "WriteFile" {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); !ok || ident.Name != osName {
			return true
		}
		if mode, ok := literalMode(call.Args[2]); ok && mode&0o111 != 0 {
			position := files.Position(call.Pos())
			findings = append(findings, fmt.Sprintf("test-exec-writes-check: FAIL %s:%d: os.WriteFile writes an executable (mode %#o) that a forking parallel test can hold open (ETXTBSY); fix: %s", path, position.Line, mode, fix))
		}
		return true
	})
	return findings, nil
}

// literalMode reads an integer literal permission, through a conversion such as
// fs.FileMode(0o755) or os.FileMode(0755).
func literalMode(expression ast.Expr) (uint64, bool) {
	for {
		switch value := expression.(type) {
		case *ast.ParenExpr:
			expression = value.X
		case *ast.CallExpr:
			if len(value.Args) != 1 {
				return 0, false
			}
			expression = value.Args[0]
		case *ast.BasicLit:
			if value.Kind != token.INT {
				return 0, false
			}
			mode, err := strconv.ParseUint(strings.ReplaceAll(value.Value, "_", ""), 0, 32)
			return mode, err == nil
		default:
			return 0, false
		}
	}
}
