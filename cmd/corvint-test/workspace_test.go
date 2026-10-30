package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/changee2e"
)

// WSD-V0-004 WSD-V0-005: `e2e obligations` reads its aliased roots from --workspace FILE or the
// discovered --root/.corvint/workspace.json exactly as from --repo; a hosted-test prove, which
// cannot read a second root, refuses naming the alias the workspace declared.
func TestWSDV0004ObligationsWorkspace(t *testing.T) {
	root, _, head := fixture(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	e2e := t.TempDir()
	if err := os.CopyFS(filepath.Join(e2e, "e2e"), os.DirFS(filepath.Join(root, "e2e"))); err != nil {
		t.Fatal(err)
	}
	gitTest(t, e2e, "init", "-q")
	gitTest(t, e2e, "add", "-A")
	gitTest(t, e2e, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "suite")
	gitTest(t, root, "rm", "-rq", "e2e")
	if err := os.WriteFile(filepath.Join(root, "appmap.json"), []byte(strings.Replace(string(raw), `"tests": {`, `"tests": {"repo": "e2e",`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "split")
	split := gitTest(t, root, "rev-parse", "HEAD")
	obligations := []string{"e2e", "obligations", "--root", root, "--base", head, "--head", split}
	var want bytes.Buffer
	if err := run(ctx, append(append([]string{}, obligations...), "--repo", "e2e="+e2e), &want); err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(t.TempDir(), "workspace.json")
	if err := os.WriteFile(named, []byte(`{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"`+filepath.ToSlash(e2e)+`"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := run(ctx, append(append([]string{}, obligations...), "--workspace", named), &got); err != nil || !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("--workspace obligations: %v\n%s\nwant\n%s", err, got.Bytes(), want.Bytes())
	}
	rel, err := filepath.Rel(filepath.Join(root, ".corvint"), e2e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "workspace.json"), []byte(`{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"`+filepath.ToSlash(rel)+`"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got.Reset()
	if err := run(ctx, obligations, &got); err != nil || !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("discovered obligations: %v\n%s", err, got.Bytes())
	}
	var r *changee2e.Refusal
	hosted := []string{"e2e", "prove", "--root", root, "--base", head, "--head", split, "--out", t.TempDir(), "--hosted-test", "e2e/a.spec.ts"}
	if err := run(ctx, hosted, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-roots-unsupported" || !strings.Contains(r.Detail, "ignores alias e2e") {
		t.Fatalf("hosted prove with a discovered workspace: %v", err)
	}
	if err := run(ctx, append(hosted, "--no-workspace"), &bytes.Buffer{}); errors.As(err, &r) && r.Code == "e2e-roots-unsupported" {
		t.Fatalf("hosted prove with --no-workspace still refused: %v", err)
	}
}
