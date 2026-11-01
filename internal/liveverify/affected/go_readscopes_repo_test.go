package affected_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRepositoryBuiltReaderDeclarations_AFPV0873 checks the AFP-V0-873
// declarations that took internal/perfbudget and
// internal/opencodequalification off the unbounded floor (V1-3763, V1-3587,
// V1-3788). Each package's tests go build a repository binary, so they are
// real readers of that binary's whole import closure: every non-standard
// directory the build compiles, and go.mod and go.sum, must lie in a declared
// entry, so a change anywhere in the closure still selects the package. The
// declarations name no docs/specs path, the Makefile or a root script, which
// those tests never read, so a spec, REQUIREMENTS.tsv or Makefile edit no
// longer selects them.
func TestRepositoryBuiltReaderDeclarations_AFPV0873(t *testing.T) {
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".corvint", "test-read-scopes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	unbounded, err := os.ReadFile(filepath.Join(root, ".corvint", "unbounded-readers.json"))
	if err != nil {
		t.Fatal(err)
	}
	for pkg, built := range map[string][]string{
		"internal/perfbudget":            {"./cmd/corvint", "./cmd/corvint-corpus-mcp"},
		"internal/opencodequalification": {"./tools/qualify-opencode"},
	} {
		entries := declared.Packages[pkg]
		if len(entries) == 0 {
			t.Fatalf("%s is not declared", pkg)
		}
		if strings.Contains(string(unbounded), `"`+pkg+`"`) {
			t.Errorf("%s is declared but still recorded as an unbounded reader", pkg)
		}
		covered := func(path string) bool {
			return slices.ContainsFunc(entries, func(entry string) bool {
				return entry == path || strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)
			})
		}
		for _, file := range []string{"go.mod", "go.sum"} {
			if !covered(file) {
				t.Errorf("%s builds with go but does not declare %s", pkg, file)
			}
		}
		for _, path := range []string{"docs/specs/affected-plan-v0.md", "docs/specs/REQUIREMENTS.tsv", "Makefile", "script/heavy.sh"} {
			if covered(path) {
				t.Errorf("%s declares %s, which its tests never read", pkg, path)
			}
		}
		list := exec.Command("go", append([]string{"list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}"}, built...)...)
		list.Dir = root
		list.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
		output, err := list.Output()
		if err != nil {
			t.Fatalf("go list %v: %v", built, err)
		}
		directories := listedDirectories(string(output))
		if len(directories) == 0 {
			t.Fatalf("go list %v listed nothing", built)
		}
		for _, directory := range directories {
			relative, err := filepath.Rel(root, directory)
			if err != nil || strings.HasPrefix(relative, "..") {
				t.Errorf("%s builds %s, outside the repository", pkg, directory)
				continue
			}
			if relative = filepath.ToSlash(relative) + "/"; !covered(relative) {
				t.Errorf("%s builds %v, which compiles %s outside its declared entries", pkg, built, relative)
			}
		}
	}
}
