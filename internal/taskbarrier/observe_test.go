// SPDX-License-Identifier: Apache-2.0
package taskbarrier

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestPURV02210_CompanionWireFailuresStayUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Tasks companion does not support Windows")
	}
	root, bin := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".taskman"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskman", "queue.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := Observe(context.Background(), root); !strings.Contains(got, "companion unavailable") {
		t.Fatalf("missing: %s", got)
	}
	run := func(raw string) string {
		t.Helper()
		script := "#!/bin/sh\n[ \"$*\" = 'queue status --fields barrier' ] || exit 9\nprintf '%s' '" + raw + "'\n"
		if err := execfile.WriteFile(filepath.Join(bin, "corvint-tasks"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		return Observe(context.Background(), root)
	}
	if got := run("{}"); !strings.Contains(got, "NOT_OBSERVED") {
		t.Fatalf("malformed: %s", got)
	}
	for _, tc := range []struct {
		barrier wire.Value
		want    string
	}{
		{wire.Null(), ""},
		{wire.ObjectValue(wire.NewObject().Set("scope", wire.String("ADMISSION"))), "companion predates age warnings"},
		{wire.ObjectValue(wire.NewObject().Set("warningPolicySeconds", wire.String("3600"))), ""},
		{wire.ObjectValue(wire.NewObject().Set("warningPolicySeconds", wire.Bool(true))), "invalid age policy observation"},
	} {
		seq := wire.SizeOf(7)
		r := &wire.Result{Command: []string{"queue", "status"}, Outcome: wire.OutcomeOK, Snapshot: &wire.Snapshot{HeadSeq: &seq}, Items: []wire.Value{wire.ObjectValue(wire.NewObject().Set("barrier", tc.barrier))}}
		raw, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got := run(string(raw))
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Fatalf("wire: %s want %q", got, tc.want)
		}
	}
}

func TestPURV02210_OutputCapCannotBeBypassedByCopy(t *testing.T) {
	var b boundedOutput
	// LimitReader avoids strings.Reader.WriteTo so io.Copy also probes ReaderFrom.
	_, err := io.Copy(&b, io.LimitReader(strings.NewReader(strings.Repeat("x", 70000)), 70000))
	if err == nil || !b.overflow || b.buf.Len() > 65536 {
		t.Fatalf("cap bypassed: %d %v %v", b.buf.Len(), b.overflow, err)
	}
}

// PUR-V0-2210: a journal-absent or CORVINT_TASKS_SNAPSHOT read is OK with a
// null barrier and no envelope snapshot; that is not observed absence, and the
// companion's own NOT_OBSERVED warning survives.
func TestPURV02210_NullBarrierWithoutLiveSnapshotStaysUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Tasks companion does not support Windows")
	}
	root, bin := companionRoot(t)
	for _, warning := range []string{
		"journal-absent; unaudited current worktree intent projection; journal history and liveness NOT_OBSERVED",
		"served read-only from the CORVINT_TASKS_SNAPSHOT snapshot /tmp/s.json (host h); intent written after it, journal history and liveness are NOT_OBSERVED",
	} {
		r := &wire.Result{Command: []string{"queue", "status"}, Outcome: wire.OutcomeOK, Warnings: []string{warning}, Items: []wire.Value{wire.ObjectValue(wire.NewObject().Set("barrier", wire.Null()))}}
		raw, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		writeCompanion(t, bin, "printf '%s' '"+string(raw)+"'")
		got := Observe(context.Background(), root)
		if !strings.Contains(got, "NOT_OBSERVED: no live journal snapshot") || !strings.Contains(got, warning) {
			t.Fatalf("journal-absent null barrier treated as absence: %q", got)
		}
	}
}

// PUR-V0-2210: the companion's deadline derives from the hook's context and the
// child is killed and reaped before that deadline; an exhausted context spawns
// nothing.
func TestPURV02210_CompanionHonoursHookDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Tasks companion does not support Windows")
	}
	root, bin := companionRoot(t)
	marker := filepath.Join(t.TempDir(), "spawned")
	// The grandchild sleep holds stdout, so return also depends on WaitDelay.
	writeCompanion(t, bin, ": > '"+marker+"'\nsleep 5")
	deadline := time.Now().Add(300 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	got := Observe(ctx, root)
	if returned := time.Now(); !returned.Before(deadline) {
		t.Fatalf("Observe returned %v after the hook deadline: %q", returned.Sub(deadline), got)
	}
	if !strings.Contains(got, "NOT_OBSERVED") {
		t.Fatalf("slow companion: %q", got)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatalf("companion was not started: %v", err)
	}
	spent, stop := context.WithCancel(context.Background())
	stop()
	if got := Observe(spent, root); !strings.Contains(got, "hook deadline leaves no companion budget") {
		t.Fatalf("exhausted context: %q", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("companion spawned after the hook deadline: %v", err)
	}
}

func companionRoot(t *testing.T) (root, bin string) {
	t.Helper()
	root, bin = t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".taskman"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskman", "queue.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root, bin
}

func writeCompanion(t *testing.T, bin, body string) {
	t.Helper()
	script := "#!/bin/sh\n[ \"$*\" = 'queue status --fields barrier' ] || exit 9\n" + body + "\n"
	if err := execfile.WriteFile(filepath.Join(bin, "corvint-tasks"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
}
