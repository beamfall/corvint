//go:build darwin || linux

package archive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stubStageSleep replaces the stage wait's sleep for one test, recording
// each pause and running then (if set) in place of sleeping.
func stubStageSleep(t *testing.T, then func(n int)) *[]time.Duration {
	t.Helper()
	pauses := &[]time.Duration{}
	saved := stageSleep
	stageSleep = func(d time.Duration) {
		*pauses = append(*pauses, d)
		if then != nil {
			then(len(*pauses))
		}
	}
	t.Cleanup(func() { stageSleep = saved })
	return pauses
}

func totalPause(pauses []time.Duration) (total time.Duration) {
	for _, d := range pauses {
		total += d
	}
	return total
}

// CTS-V0-008: an export that starts while a native writer holds a
// descriptor-less staging slot waits for the writer's deferred RemoveStage
// and exports the settled store, instead of returning the MALFORMED
// "unassigned stage slot" of V1-0825. The sleep hook drives the interleaving.
func TestCTSV0008_ArchiveWaitsForWriterHeldStageSlot(t *testing.T) {
	r, repo := repoWithStore(t)
	slot := filepath.Join(r.StateDir, "staging", "a00")
	fixture.Write(t, slot, []byte("x"))
	pauses := stubStageSleep(t, func(int) {
		if e := os.Remove(slot); e != nil {
			t.Fatal(e)
		}
	})
	out, result, err := export(t, repo, fixture.TempDirOutside(t))
	if err != nil {
		t.Fatalf("export beside a writer-held slot: %v", err)
	}
	if len(*pauses) != 1 || (*pauses)[0] != snapshot.StageSlotPause {
		t.Fatalf("pauses %v; want one of %v", *pauses, snapshot.StageSlotPause)
	}
	if _, err := Verify(bytes.NewReader(out)); err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Manifest.Files {
		if strings.HasPrefix(f.Path, "staging/") {
			t.Fatalf("staged slot exported: %s", f.Path)
		}
	}
}

// CTS-V0-008: the export's wait is bounded like the journal audit's. A killed
// writer's orphan slot still earns the same MALFORMED, unchanged and with no
// output, once the two-second budget is spent.
func TestCTSV0008_ArchiveOrphanSlotMalformedAfterBoundedWait(t *testing.T) {
	r, repo := repoWithStore(t)
	for _, s := range []string{"a03", "a00"} {
		fixture.Write(t, filepath.Join(r.StateDir, "staging", s), []byte("x"))
	}
	pauses := stubStageSleep(t, nil)
	out, result, err := export(t, repo, fixture.TempDirOutside(t))
	e, ok := err.(*wire.Error)
	if !ok || e.Error() != wire.Errorf(wire.CodeMalformed, "a00", "unassigned stage slot").Error() {
		t.Fatalf("refusal changed: %#v", err)
	}
	if len(out) != 0 || result != nil {
		t.Fatal("pre-delivery output")
	}
	if total := totalPause(*pauses); total != snapshot.StageSlotPatience || len(*pauses) > 16 {
		t.Fatalf("waited %v in %v; want exactly %v", total, *pauses, snapshot.StageSlotPatience)
	}
}

// CTS-V0-008: only the descriptor-less shape is waited for. A slot beside a
// stage descriptor or a descriptor temp is refused at once.
func TestCTSV0008_ArchiveSlotBesideDescriptorRefusedAtOnce(t *testing.T) {
	for _, name := range []string{"active.json", "active.json.tmp"} {
		t.Run(name, func(t *testing.T) {
			r, repo := repoWithStore(t)
			dir := filepath.Join(r.StateDir, "staging")
			fixture.Write(t, filepath.Join(dir, name), stageDescriptor(t, r, false))
			fixture.Write(t, filepath.Join(dir, "a07"), []byte("x"))
			pauses := stubStageSleep(t, nil)
			_, _, err := export(t, repo, fixture.TempDirOutside(t))
			if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeMalformed || e.Where != "a07" {
				t.Fatalf("got %v; want MALFORMED at a07", err)
			}
			if len(*pauses) != 0 {
				t.Fatalf("waited %v", *pauses)
			}
		})
	}
}

// CTS-V0-008: the stage wait and the four moved attempts compose in the
// export as in the journal audit. A move never resets the wait, a pause never
// spends an attempt, and four moves still end in SNAPSHOT_MOVED.
func TestCTSV0008_ArchiveMovedAttemptsAndSpentWaitCompose(t *testing.T) {
	for _, tc := range []struct {
		name  string
		move  func(capture int, spent bool) bool
		code  string
		moves int
	}{
		{"moves while waiting, then budget spent", func(c int, _ bool) bool { return c == 2 || c == 4 }, wire.CodeMalformed, 2},
		{"four moves after the budget is spent", func(_ int, spent bool) bool { return spent }, wire.CodeSnapshotMoved, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, repo := repoWithStore(t)
			fixture.Write(t, filepath.Join(r.StateDir, "staging", "a00"), []byte("x"))
			pauses := stubStageSleep(t, nil)
			captures, moves := 0, 0
			var out bytes.Buffer
			_, err := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterCapture: func() {
				captures++
				if !tc.move(captures, totalPause(*pauses) == snapshot.StageSlotPatience) {
					return
				}
				moves++
				raw := []byte(fmt.Sprint(captures))
				fixture.Write(t, filepath.Join(r.StateDir, "evidence", string(wire.Sum(raw))), raw)
			}})
			if code(err) != tc.code || out.Len() != 0 {
				t.Fatalf("err %v bytes %d; want %s", err, out.Len(), tc.code)
			}
			if total := totalPause(*pauses); total != snapshot.StageSlotPatience {
				t.Fatalf("waited %v in %v; want exactly %v", total, *pauses, snapshot.StageSlotPatience)
			}
			want := len(*pauses) + moves
			if tc.code == wire.CodeMalformed {
				want++ // the final stable attempt that returns the refusal
			}
			if moves != tc.moves || captures != want {
				t.Fatalf("attempts %d with %d moves and %d pauses; want %d attempts, %d moves", captures, moves, len(*pauses), want, tc.moves)
			}
		})
	}
}
