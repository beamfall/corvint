package semescalate

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const packagePath = "github.com/Beamfall/corvint/internal/semescalate"

// forbiddenImport reports the first process, network, loader, or raw-syscall import in source.
func forbiddenImport(t *testing.T, name string, source []byte) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "os" || path == "os/exec" || path == "net" || strings.HasPrefix(path, "net/") || path == "plugin" || path == "syscall" || path == "unsafe" {
			return path
		}
	}
	return ""
}

func TestPackageHasNoProcessNetworkOrFilesystemImports(t *testing.T) {
	for _, control := range []string{`import "net/http"`, `import x "os/exec"`, `import "os"`} {
		if forbiddenImport(t, "control.go", []byte("package p\n"+control+"\n")) == "" {
			t.Fatalf("SPY_INEFFECTIVE %s", control)
		}
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := forbiddenImport(t, name, source); got != "" {
			t.Errorf("%s imports %s", name, got)
		}
	}
}

// The gate stays out of every serving, ranking, and command path (SEG-016; AGENTS.md invariants 3 and 7).
func TestNoProductionPackageImportsTheGate(t *testing.T) {
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "testdata" || filepath.Base(path) == "semescalate") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				if spec.Path.Value == strconv.Quote(packagePath) {
					t.Errorf("%s imports the semantic escalation gate", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
