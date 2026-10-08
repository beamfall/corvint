package delta

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	raw, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}
func put(t *testing.T, root, p, s string) {
	t.Helper()
	name := filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}
func fixtureMerge(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.name", "fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	put(t, root, "go.mod", "module example.invalid/demo\n\ngo 1.27\n")
	put(t, root, "pkg/a.go", "package demo\nfunc A() int { return 1 }\n")
	put(t, root, "pkg/a_test.go", "package demo\nimport \"testing\"\nfunc TestA(t *testing.T) { if A()!=1 {t.Fatal(\"PROSE_SENTINEL\")} }\n")
	put(t, root, "web/a.js", "export function PROSE_SENTINEL() { return 1; }\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	git(t, root, "checkout", "-qb", "feature")
	put(t, root, "pkg/a.go", "package demo\n// PROSE_SENTINEL\nfunc A() int { return 2 }\n")
	git(t, root, "add", ".")
	put(t, root, "web/a.js", "export function PROSE_SENTINEL() { return 2; }\n")
	git(t, root, "add", "web/a.js")
	git(t, root, "commit", "-qm", "PROSE_SENTINEL TASK-123")
	git(t, root, "checkout", "-q", "main")
	put(t, root, "README.md", "PROSE_SENTINEL\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "docs")
	git(t, root, "merge", "--no-ff", "feature", "-m", "PROSE_SENTINEL TASK-123")
	return root, base, git(t, root, "rev-parse", "HEAD")
}
func hasUnknown(r Record, code string) bool {
	for _, u := range r.Unknowns {
		if u.Code == code {
			return true
		}
	}
	return false
}
func TestDeltaFixtureMergeDeterministicSourceFree(t *testing.T) {
	root, base, head := fixtureMerge(t)
	ctx := context.Background()
	o := Options{Base: base, Head: head, Build: "163", WorkKeyPattern: `TASK-[0-9]+`}
	before := git(t, root, "status", "--porcelain=v1")
	refs := git(t, root, "show-ref")
	index := git(t, root, "ls-files", "--stage")
	one, err := Compile(ctx, root, o)
	if err != nil {
		t.Fatal(err)
	}
	a, err := one.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "pkg/a.go", "ambient invalid syntax PROSE_SENTINEL")
	two, err := Compile(ctx, root, o)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := two.Canonical()
	t.Run("DLT-V0-010 two-run byte identity without writes", func(t *testing.T) {
		if !bytes.Equal(a, b) {
			t.Fatalf("immutable output drift\n%s\n%s", a, b)
		}
		if refs != git(t, root, "show-ref") || index != git(t, root, "ls-files", "--stage") || before != "" {
			t.Fatal("immutable read mutated git state")
		}
		for _, path := range []string{".corvint", ".git/corvint/index", ".git/corvint/trace"} {
			if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
				t.Fatalf("state created %s: %v", path, err)
			}
		}
	})
	t.Run("DLT-V0-001 explicit revision binding", func(t *testing.T) {
		tree := git(t, root, "rev-parse", head+"^{tree}")
		if one.Schema != Schema || one.Base != base || one.Head != head || one.Tree != tree || one.Build != "163" || len(one.ChangedPathDigest) != 64 {
			t.Fatalf("binding %+v", one)
		}
		changed := strings.Join(one.ChangedPaths, ",")
		if changed != "README.md,pkg/a.go,web/a.js" {
			t.Fatalf("changed paths %q", changed)
		}
	})
	t.Run("DLT-V0-008 source and prose free output", func(t *testing.T) {
		if bytes.Contains(a, []byte("PROSE_SENTINEL")) || bytes.Contains(a, []byte("TASK-123")) {
			t.Fatal("prose escaped")
		}
	})
	t.Run("DLT-V0-007 opaque hashed work keys", func(t *testing.T) {
		if len(one.WorkKeys) != 1 || strings.Contains(one.WorkKeys[0], "TASK") {
			t.Fatalf("work keys %v", one.WorkKeys)
		}
	})
	t.Run("DLT-V0-005 missing provider requires full suite", func(t *testing.T) {
		if !one.RunFullSuite || !hasUnknown(one, "provider-coverage-missing") {
			t.Fatalf("missing provider narrowed: %+v", one)
		}
	})
	t.Run("DLT-V0-006 separate flow and unit denominators", func(t *testing.T) {
		if len(one.Tests) == 0 || one.Denominators.AffectedUnits == 0 || one.Denominators.LexicalFlows == 0 || one.Denominators.Runtime != "unknown" {
			t.Fatalf("join missing: %+v", one)
		}
	})
	t.Run("DLT-V0-009 unknowns force findings", func(t *testing.T) {
		if one.Decision != "findings" || one.Denominators.Complete {
			t.Fatalf("decision %s complete %v", one.Decision, one.Denominators.Complete)
		}
	})
}
func TestDeltaIncompleteProviderRequiresFullSuite(t *testing.T) {
	root, base, head := fixtureMerge(t)
	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, provider, reason string }{
		{"DLT-V0-005 malformed provider run full suite", malformed, "external-coverage-incomplete"},
		{"DLT-V0-005 absent provider file run full suite", filepath.Join(dir, "absent.json"), "provider-capture-unavailable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := Compile(context.Background(), root, Options{Base: base, Head: head, Build: "163", Providers: []string{c.provider}})
			if err != nil {
				t.Fatal(err)
			}
			if !r.RunFullSuite || !hasUnknown(r, c.reason) || !hasUnknown(r, "external-coverage-incomplete") || r.Decision != "findings" {
				t.Fatalf("incomplete provider narrowed: %+v", r)
			}
		})
	}
}
func TestDeltaNoOpAndBadRevision(t *testing.T) {
	root, _, head := fixtureMerge(t)
	o := Options{Base: head, Head: head, Build: "163"}
	r, err := Compile(context.Background(), root, o)
	t.Run("DLT-V0-009 complete empty change is no-op", func(t *testing.T) {
		if err != nil || r.Decision != "no-op" || r.RunFullSuite || len(r.ChangedPaths) != 0 {
			t.Fatalf("noop %+v %v", r, err)
		}
	})
	t.Run("DLT-V0-001 symbolic revision refused", func(t *testing.T) {
		o.Base = "HEAD"
		if _, err := Compile(context.Background(), root, o); err == nil {
			t.Fatal("symbolic revision admitted")
		}
	})
}
func TestDeltaPreviousGeneration(t *testing.T) {
	t.Run("DLT-V0-004 previous generation baseline binding", testDeltaPreviousGeneration)
}
func testDeltaPreviousGeneration(t *testing.T) {
	root, base, head := fixtureMerge(t)
	ctx := context.Background()
	previous, err := flowdocs.Generate(ctx, root, flowdocs.Options{Revision: base, Scope: "pkg"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := flowdocs.Encode(previous.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "previous.json")
	if err := os.WriteFile(name, raw, 0644); err != nil {
		t.Fatal(err)
	}
	o := Options{Base: base, Head: head, Build: "163", PreviousGeneration: name}
	r, err := Compile(ctx, root, o)
	if err != nil {
		t.Fatal(err)
	}
	if hasUnknown(r, "documentation-baseline-invalid") || hasUnknown(r, "documentation-baseline-unavailable") || len(r.InputDigests) == 0 {
		t.Fatalf("prior binding %+v", r)
	}
	o.PreviousGeneration = name + "-missing"
	r, err = Compile(ctx, root, o)
	if err != nil || !hasUnknown(r, "documentation-baseline-invalid") {
		t.Fatalf("missing baseline %+v %v", r, err)
	}
}
func TestDeltaRecordRejectsProseAndInvalidEnums(t *testing.T) {
	root, _, head := fixtureMerge(t)
	r, err := Compile(context.Background(), root, Options{Base: head, Head: head, Build: "163"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("DLT-V0-008 closed uncertainty enum", func(t *testing.T) {
		r.Unknowns = append(r.Unknowns, Unknown{Code: "PROSE_SENTINEL", Count: 1, Digest: strings.Repeat("a", 64)})
		if _, err := r.Canonical(); err == nil {
			t.Fatal("unknown enum escaped")
		}
	})
}
