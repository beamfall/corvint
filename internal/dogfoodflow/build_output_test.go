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

// TestChangeDCWV0386NotesUntrackedBuildOutputs pins V1-3132: a binary that
// `go build ./cmd/NAME` left untracked at the worktree top level, NAME or
// NAME.exe beside a cmd/NAME directory, is named before prepare with the
// scratch build route; another untracked file, an untracked file that names no
// cmd directory, and a clean worktree get no note.
func TestChangeDCWV0386NotesUntrackedBuildOutputs(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"cmd/tool/main.go": "package main\n\nfunc main() {}\n", "a.txt": "base\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		if len(args) > 0 && (args[0] == "query" || args[0] == "impact" || args[0] == "dogfood-observe") {
			return 0
		}
		return 1
	}}
	run := func() string {
		t.Helper()
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
			t.Fatal(err)
		}
		return stderr.String()
	}
	if out := run(); strings.Contains(out, "untracked-build-output") {
		t.Fatalf("a clean worktree got a build-output note:\n%s", out)
	}
	for _, name := range []string{"tool", "tool.exe", "notes.txt", "other"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out := run()
	want := "dogfood-change: NOTE untracked-build-output tool, tool.exe\n"
	if !strings.Contains(out, want) || !strings.Contains(out, `go build -o "$TMPDIR/NAME" ./cmd/NAME (DCW-V0-386)`) {
		t.Fatalf("stderr lacks %q and the scratch route:\n%s", want, out)
	}
}
