package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChangeSOLV0001ReportsObserveRefusal pins that a refused dogfood-observe
// write is named once on stderr and stops further rows, instead of every row
// being discarded silently.
func TestChangeSOLV0001ReportsObserveRefusal(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	observeCalls := 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, stderr io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(guardingTestsMap), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 0 && args[0] == "dogfood-observe":
			observeCalls++
			_, _ = io.WriteString(stderr, `{"code":"internal-error","error":"self-observation ledger is not safely gitignored"}`+"\n")
			return 2
		}
		return 1
	}}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
		t.Fatal(err)
	}
	if observeCalls != 1 {
		t.Fatalf("dogfood-observe calls = %d, want 1 (stop after the first refusal)", observeCalls)
	}
	want := "NOTE self-observation rows not recorded: dogfood-observe exited 2: " +
		`{"code":"internal-error","error":"self-observation ledger is not safely gitignored"}`
	if got := stderr.String(); strings.Count(got, want) != 1 {
		t.Fatalf("stderr does not name the refusal once:\n%s", got)
	}
}
