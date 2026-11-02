// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// TestDCWV0406ChangeRetiresStrandedTracesOnce (V1-3604): three bind commits
// reset away leave three stranded worktree-local traces. The first dogfood
// change run retires all three; a second consecutive run retires none.
func TestDCWV0406ChangeRetiresStrandedTracesOnce(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".context-corvint/\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	traces := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(traces, 0o700); err != nil {
		t.Fatal(err)
	}
	var bound []string
	for k := range 3 {
		name := filepath.Join(root, "bound.txt")
		if err := os.WriteFile(name, []byte(strings.Repeat("x", k+1)+"\n"), 0o666); err != nil {
			t.Fatal(err)
		}
		testGit(t, root, "add", "bound.txt")
		testGit(t, root, "commit", "-q", "-m", "bind")
		revision := testGit(t, root, "rev-parse", "HEAD")
		record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: revision, Task: "bind", Outcome: "passed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(traces, revision+".jsonl"), row, 0o600); err != nil {
			t.Fatal(err)
		}
		bound = append(bound, revision)
		testGit(t, root, "reset", "-q", "--hard", "HEAD~1")
	}
	run := func() string {
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
			t.Fatal(err)
		}
		return stderr.String()
	}
	first := run()
	for _, revision := range bound {
		if !strings.Contains(first, "RETIRE stale trace "+revision+" quarantined byte-preserved") {
			t.Fatalf("first run did not retire %s: %q", revision, first)
		}
	}
	if second := run(); strings.Contains(second, "RETIRE") || strings.Contains(second, "REFUSE reused-worktree-state") {
		t.Fatalf("second consecutive run retired or refused again: %q", second)
	}
}
