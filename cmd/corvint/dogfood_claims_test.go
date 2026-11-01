package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCKCV0002RecordClaimsCLI: `dogfood record-claims` records a reviewed
// plan and `dogfood propose-citations --refresh CLAIMS` compiles it back to
// the same hunk-ID rows; record-claims takes --plan, --claims and --draft
// only, and refuses without --plan or --claims.
func TestCKCV0002RecordClaimsCLI(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	commit := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
		gitOutput(t, root, "add", ".")
		gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", name)
	}
	gitOutput(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte("# Spec\n\n- `S-1`: one.\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	commit("a.go", "package a\n\nfunc A() int { return 1 }\n")
	base := gitOutput(t, root, "rev-parse", "HEAD")
	commit("a.go", "package a\n\nfunc A() int { return 2 }\n")

	code, draft, stderr := runCLI(t, "--root", root, "dogfood", "propose-citations", base)
	if code != 0 || !strings.HasPrefix(draft, "hunk:sha256:") {
		t.Fatalf("draft = %d\n%s\n%s", code, draft, stderr)
	}
	id, _, _ := strings.Cut(draft, "\t")
	plan := filepath.Join(t.TempDir(), "plan.tsv")
	if err := os.WriteFile(plan, []byte(id+"\tspec.md\t3:3\tspecification\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claims := filepath.Join(t.TempDir(), "claims.json")
	code, stdout, stderr := runCLI(t, "--root", root, "dogfood", "record-claims", "--plan", plan, "--claims", claims, base)
	if code != 0 || stdout != "" || !strings.Contains(stderr, "dogfood-record-claims: RECORDED claims=1 rows=1 tombstones=0 deferred=0") {
		t.Fatalf("record-claims = %d\n%s\n%s", code, stdout, stderr)
	}
	code, stdout, stderr = runCLI(t, "--root", root, "dogfood", "propose-citations", "--refresh", claims, base)
	if code != 0 || stdout != id+"\tspec.md\t3:3\tspecification\n" || !strings.Contains(stderr, "carried=1 reassess=0 gone=0 suppressed=0 deferred=0") {
		t.Fatalf("refresh = %d\n%s\n%s", code, stdout, stderr)
	}

	if code, _, stderr := runCLI(t, "--root", root, "dogfood", "record-claims", "--plan", plan, base); code != 2 || !strings.Contains(stderr, "REFUSE claims-arguments") {
		t.Errorf("record-claims without --claims = %d\n%s", code, stderr)
	}
	if code, _, stderr := runCLI(t, "--root", root, "dogfood", "record-claims", "--refresh", plan, base); code != 2 || !strings.Contains(stderr, "does not take --refresh") {
		t.Errorf("record-claims accepted --refresh = %d\n%s", code, stderr)
	}
}
