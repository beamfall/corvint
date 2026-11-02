package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// TestDCWV0620InputsCheckRefusesTheV1_3633ReproFiles (V1-3633): the two files
// the friction report named as INTENTS entries, a build-log entry and the spec
// index docs/specs/README.md, are committed byte for byte into a non-record
// change. The inputs-check pre-flight refuses that manifest before any step,
// naming each file with the code lrfrepo.DeriveIntentAt (the derivation
// ocm-prepare runs after the bind commit) returns for it, which is
// invalid-requirements-section for both.
func TestDCWV0620InputsCheckRefusesTheV1_3633ReproFiles(t *testing.T) {
	repro := []string{
		"docs/build-log/2026-10-31-scripts-w5.md", //corvint:fixture-path
		"docs/specs/README.md",                    //corvint:fixture-path
	}
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	write := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", []byte("base\n"))
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	for _, name := range repro {
		// The file as checked out; test-confine admits only the declared
		// read scopes (.corvint/test-read-scopes.json), not .git, so a git
		// read of the committed blob exits 128 under confinement.
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		write(name, data)
	}
	// A code edit beside them makes this a non-record change, as in both
	// reports, so the manifest is derived rather than admitted as a record.
	write("a.txt", []byte("change\n"))
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	head := testGit(t, root, "rev-parse", "HEAD")
	scratch := t.TempDir()
	manifest := filepath.Join(scratch, "intents")
	plan := filepath.Join(scratch, "cites.tsv")
	verify := filepath.Join(scratch, "verify.txt")
	for path, data := range map[string]string{
		manifest: strings.Join(repro, "\n") + "\n",
		plan:     "1\ta.txt\t1:1\tspecification\n",
		verify:   "go test -count=1 -run TestA ./internal/a\n",
	} {
		if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 0
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: true, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: manifest, VerifyFile: verify, Outcome: "passed"}}, &stdout, &stderr)
	if err != nil || code != 2 || calls != 0 {
		t.Fatalf("code=%d err=%v step calls=%d; want a refusal before any step:\n%s", code, err, calls, &stderr)
	}
	if !strings.Contains(stderr.String(), ": REFUSE intent-invalid\n") {
		t.Fatalf("no intent-invalid refusal:\n%s", &stderr)
	}
	for _, name := range repro {
		refusal := intentRefusal(func() error { _, err := lrfrepo.DeriveIntentAt(context.Background(), root, head, name); return err }())
		if refusal == nil || refusal.Code != "invalid-requirements-section" {
			t.Fatalf("%s: ocm-prepare's derivation returned %+v, want invalid-requirements-section", name, refusal)
		}
		if want := "  intent=" + name + " code=invalid-requirements-section: "; !strings.Contains(stderr.String(), want) {
			t.Errorf("refusal lacks %q:\n%s", want, &stderr)
		}
	}
}
