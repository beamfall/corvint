package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSealRefusesToDropAnUnarchivedBaseCEM reproduces V1-0137: a change that
// replaced BASE's tracked CEM must not seal while no .corvint/changes/ file
// keeps BASE's copy, and seals once one does.
func TestSealRefusesToDropAnUnarchivedBaseCEM(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "init", "-q")
	write(".corvint/change.cem.json", "earlier change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "earlier bind")
	earlier := testGit(t, root, "rev-parse", "HEAD")
	write("a.txt", "change\n")
	write(".corvint/change.cem.json", "this change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind")
	seal := func() (int, string) {
		var stdout, stderr bytes.Buffer
		run := newCheck(context.Background(), "dogfood-seal", CheckOptions{Root: root}, &stdout, &stderr)
		run.base = earlier
		code, err := boundary(context.Background(), run.seal, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String()
	}
	code, stderr := seal()
	want := "dogfood-seal: REFUSE unarchived-base-cem\n  BASE tracks .corvint/change.cem.json (bound at " + earlier + ")"
	if code != 2 || !strings.HasPrefix(stderr, want) {
		t.Fatalf("seal exit=%d stderr=%q; want exit 2 and %q", code, stderr, want)
	}
	if head := testGit(t, root, "log", "-1", "--format=%s"); head != "bind" {
		t.Fatalf("refused seal committed %q", head)
	}
	write(".corvint/changes/"+earlier+".cem.json", "earlier change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind with the earlier CEM archived")
	if code, stderr = seal(); code != 0 {
		t.Fatalf("archived seal exit=%d stderr=%q", code, stderr)
	}
}
