package cli

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCALV0225_FileArgumentsResolveAgainstEnvCwd: a relative file argument
// names a file under the command's working directory, not the process
// directory: commandPath feeds the shared readers, and archive verify opens
// its argument through it (V1-3643).
func TestCALV0225_FileArgumentsResolveAgainstEnvCwd(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "in.json"), []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if raw, err := readBoundedFile("--review", commandPath(cwd, "in.json"), 64); err != nil || string(raw) != `{"a":1}` {
		t.Fatalf("readBoundedFile = %q, %v", raw, err)
	}
	if raw, err := readPlanFile(commandPath(cwd, "in.json"), 64); err != nil || string(raw) != `{"a":1}` {
		t.Fatalf("readPlanFile = %q, %v", raw, err)
	}
	res := archiveVerify(Env{Cwd: cwd, Stdin: bytes.NewReader(nil)}, []string{"absent.tar"})
	out, err := res.Encode()
	if err != nil || !strings.Contains(string(out), filepath.Join(cwd, "absent.tar")) {
		t.Fatalf("archive verify did not open the file under env.Cwd: %s %v", out, err)
	}
}

// handRolledResolve is the inline relative-path join that commandPath
// replaces; every file argument goes through commandPath instead.
var handRolledResolve = regexp.MustCompile(`if !filepath\.IsAbs\((\w+)\) \{\s*\w+ = filepath\.Join\(\w*\.?Cwd, `)

// TestCALV0225_NoDirectOpenOfArguments is the V1-3643 lint: no non-test
// source opens or reads an argv element directly, and none re-implements the
// relative join outside commandPath.
func TestCALV0225_NoDirectOpenOfArguments(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	readers := map[string]bool{"os.Open": true, "os.OpenFile": true, "os.ReadFile": true, "intent.ReadFile": true, "openRegular": true, "readBounded": true}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := handRolledResolve.Find(src); m != nil {
			t.Errorf("%s: hand-rolled relative join %q; use commandPath(env.Cwd, path)", f, m)
		}
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok {
					name = x.Name + "." + fn.Sel.Name
				}
			}
			if !readers[name] {
				return true
			}
			if ix, ok := call.Args[0].(*ast.IndexExpr); ok {
				if id, ok := ix.X.(*ast.Ident); ok && (id.Name == "args" || id.Name == "rest") {
					t.Errorf("%s: %s(%s[...]) opens an argument directly; resolve it with commandPath(env.Cwd, ...)", fset.Position(call.Pos()), name, id.Name)
				}
			}
			return true
		})
	}
}
