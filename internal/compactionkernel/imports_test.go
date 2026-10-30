package compactionkernel

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// forbiddenImport reports the first process or network import in source, or
// "" if none is present. Mirrors internal/semescalate's spy so CKN-V0-007's
// "no process spawn, no network" claim is checked the same way elsewhere in
// this repository.
func forbiddenImport(t *testing.T, name string, source []byte) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "os/exec" || path == "net" || strings.HasPrefix(path, "net/") || path == "plugin" || path == "syscall" {
			return path
		}
	}
	return ""
}

// TestPackageHasNoProcessOrNetworkImports holds CKN-V0-007's read-only,
// content-free claim: the kernel package itself spawns no process (beyond the
// index build the host verb already performs, which lives outside this
// package) and touches no network. It is a static import spy, not a runtime
// one, because the package's only external calls are to internal/gokernel and
// internal/contextindex, both pure in-process lookups with no os/exec or net
// import of their own.
func TestPackageHasNoProcessOrNetworkImports(t *testing.T) {
	for _, control := range []string{`import "net/http"`, `import x "os/exec"`, `import "syscall"`} {
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
