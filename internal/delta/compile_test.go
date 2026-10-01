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
	c := exec.Command("git", args...)
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
	if !bytes.Equal(a, b) {
		t.Fatalf("immutable output drift\n%s\n%s", a, b)
	}
	if bytes.Contains(a, []byte("PROSE_SENTINEL")) || bytes.Contains(a, []byte("TASK-123")) {
		t.Fatal("prose escaped")
	}
	if !one.RunFullSuite || !hasUnknown(one, "provider-coverage-missing") || one.Decision != "findings" || len(one.WorkKeys) != 1 {
		t.Fatalf("missing conservative evidence: %+v", one)
	}
	if len(one.Tests) == 0 || one.Denominators.AffectedUnits == 0 || one.Denominators.LexicalFlows == 0 {
		t.Fatalf("join missing: %+v", one)
	}
	if refs != git(t, root, "show-ref") || index != git(t, root, "ls-files", "--stage") || before != "" {
		t.Fatal("immutable read mutated git state")
	}
	for _, path := range []string{".corvint", ".git/corvint/index", ".git/corvint/trace"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("state created %s: %v", path, err)
		}
	}
}
func TestDeltaNoOpAndBadRevision(t *testing.T) {
	root, _, head := fixtureMerge(t)
	o := Options{Base: head, Head: head, Build: "163"}
	r, err := Compile(context.Background(), root, o)
	if err != nil || r.Decision != "no-op" || r.RunFullSuite || len(r.ChangedPaths) != 0 {
		t.Fatalf("noop %+v %v", r, err)
	}
	o.Base = "HEAD"
	if _, err := Compile(context.Background(), root, o); err == nil {
		t.Fatal("symbolic revision admitted")
	}
}
func TestDeltaPreviousGeneration(t *testing.T) {
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
	r.Unknowns = append(r.Unknowns, Unknown{Code: "PROSE_SENTINEL", Count: 1, Digest: strings.Repeat("a", 64)})
	if _, err := r.Canonical(); err == nil {
		t.Fatal("unknown enum escaped")
	}
}
