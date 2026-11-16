// SPDX-License-Identifier: Apache-2.0
package langplugin_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LGP-V0-8600: importing providers cannot install process-wide hooks. The
// syntax guard complements the order/isolation tests of constructed values.
func TestExplicitWiringHasNoInitHooks(t *testing.T) {
	dirs := []string{"internal/langplugin", "internal/langplugin/dialect", "internal/rubydialect", "internal/shaderdialect", "internal/swiftimpact", "internal/contextindex", "internal/jstestprovider", "internal/jstestprovider/loopbackhttp", "internal/command/flows"}
	commands, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		if command.IsDir() {
			dirs = append(dirs, "cmd/"+command.Name())
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, dir, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range file.Imports {
				if imp.Name != nil && imp.Name.Name == "_" && (strings.Contains(imp.Path.Value, "/swiftimpact") || strings.Contains(imp.Path.Value, "/loopbackhttp") || strings.Contains(imp.Path.Value, "/rubydialect")) {
					t.Errorf("side-effect wiring in %s/%s", dir, name)
				}
			}
			if strings.HasPrefix(dir, "cmd/") {
				continue
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
					if fn.Name.Name == "init" && !(dir == "internal/contextindex" && name == "snapshot.go") {
						t.Errorf("init hook in %s/%s", dir, name)
					}
					if strings.HasPrefix(fn.Name.Name, "Register") {
						t.Errorf("global registration %s in %s/%s", fn.Name.Name, dir, name)
					}
				}
				if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.VAR {
					for _, spec := range group.Specs {
						for _, id := range spec.(*ast.ValueSpec).Names {
							switch id.Name {
							case "swiftPMReader", "loopbackGet", "impacts", "regions":
								t.Errorf("global hook %s in %s/%s", id.Name, dir, name)
							}
						}
					}
				}
			}
		}
	}
}
