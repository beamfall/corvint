package store

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type noteConcurrencyWorker struct {
	Repo, Ticket, ID, Out string
	Start                 int64
}

type noteConcurrencySample struct {
	Code, Declined string
	Completed      bool
	WallMS         float64
}

// TestONV0012_ConcurrentNoteWritesAfterNote is an opt-in profile for
// beamfall/corvint#1072: at CORVINT_TASKS_HISTORY_SIZES receipts (default
// 2000), after a note on one ticket, CORVINT_TASKS_NOTE_WRITERS separate
// processes (default 9) each set a note on a different ticket at the same
// instant. Every write must commit with no LOCK_TIMEOUT, which held when each
// write re-audited the journal under the store lock. The report at
// CORVINT_TASKS_NOTE_CONCURRENCY keeps every sample.
func TestONV0012_ConcurrentNoteWritesAfterNote(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_NOTE_CONCURRENCY")
	if output == "" {
		t.Skip("set CORVINT_TASKS_NOTE_CONCURRENCY to a report path")
	}
	sizes := []int{2000}
	if raw := os.Getenv("CORVINT_TASKS_HISTORY_SIZES"); raw != "" {
		sizes = nil
		for _, f := range strings.Split(raw, ",") {
			n, err := strconv.Atoi(f)
			if err != nil || n <= historyTickets+1 {
				t.Fatalf("size %q", f)
			}
			sizes = append(sizes, n)
		}
	}
	writers := 9
	if raw := os.Getenv("CORVINT_TASKS_NOTE_WRITERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n >= historyTickets {
			t.Fatalf("CORVINT_TASKS_NOTE_WRITERS=%q", raw)
		}
		writers = n
	}
	type row struct {
		Receipts, Writers, Completed, LockTimeouts int
		Codes                                      map[string]int
		Declined                                   []string
		WallMS                                     []float64
		ElapsedMS                                  float64
	}
	var rows []row
	for _, n := range sizes {
		repo := historyStore(t, n)
		historyMutate(t, repo, "note-concurrency-seed")
		entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
		if err != nil || len(entries) <= writers {
			t.Fatalf("tickets %d %v", len(entries), err)
		}
		ticketOf := func(i int) string { return fixture.TicketID(strings.TrimSuffix(entries[i].Name(), ".json")) }
		if run := stagedWrite(writerEnvelope(repo, writerNoteEnvelope(fmt.Sprintf("note-concurrency-%d-first", n), ticketOf(0), "0", "First note."), WallClock()), nil); !run.completed() {
			t.Fatalf("%d first note: %+v %v %v", n, run.rep, run.err, run.stages)
		}
		dir := t.TempDir()
		start := time.Now().Add(2 * time.Second)
		procs := make([]*exec.Cmd, writers)
		outs := make([]string, writers)
		for i := range procs {
			outs[i] = filepath.Join(dir, fmt.Sprintf("writer-%d.json", i))
			spec, _ := json.Marshal(noteConcurrencyWorker{Repo: repo.PrimaryWorktree, Ticket: ticketOf(i + 1), ID: fmt.Sprintf("note-concurrency-%d-%d", n, i), Out: outs[i], Start: start.UnixMilli()})
			cmd := exec.Command(os.Args[0], "-test.run=^TestONV0012_ConcurrentNoteWriter$", "-test.count=1", "-test.timeout=0")
			cmd.Env = append(os.Environ(), "CORVINT_TASKS_NOTE_CONCURRENCY=", "CORVINT_TASKS_NOTE_WRITER="+string(spec))
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			procs[i] = cmd
		}
		for i, cmd := range procs {
			if err := cmd.Wait(); err != nil {
				t.Errorf("%d writer %d: %v", n, i, err)
			}
		}
		elapsed := time.Since(start)
		r := row{Receipts: n, Writers: writers, Codes: map[string]int{}, Declined: []string{}, ElapsedMS: ms(elapsed)}
		for _, path := range outs {
			var s noteConcurrencySample
			if err := json.Unmarshal(mustReadFile(t, path), &s); err != nil {
				t.Fatal(err)
			}
			if s.Completed {
				r.Completed++
			}
			if s.Code != "" {
				r.Codes[s.Code]++
			}
			if s.Code == string(wire.CodeLockTimeout) {
				r.LockTimeouts++
			}
			if s.Declined != "" {
				r.Declined = append(r.Declined, s.Declined)
			}
			r.WallMS = append(r.WallMS, s.WallMS)
		}
		sort.Float64s(r.WallMS)
		rows = append(rows, r)
		t.Logf("receipts=%d writers=%d completed=%d LOCK_TIMEOUT=%d codes=%v declined=%v wall=%v elapsed=%s", n, writers, r.Completed, r.LockTimeouts, r.Codes, r.Declined, r.WallMS, elapsed)
		if r.LockTimeouts != 0 || r.Completed != writers {
			t.Errorf("%d: %d of %d note writes committed, %d LOCK_TIMEOUT", n, r.Completed, writers, r.LockTimeouts)
		}
	}
	report := map[string]any{"ticket": "V1-1650", "issue": "beamfall/corvint#1072", "requirement": "ON-V0-012", "fixture": fmt.Sprintf("historyStore: %d created tickets; after a seed CREATE and a NOTE_SET on the first ticket, each writer process sets a note on its own ticket (last posted in the prefix), all released at the same instant", historyTickets), "statistic": "per-write wall milliseconds, sorted; elapsed from the common start to the last exit", "rows": rows, "hostLoad": hostLoad()}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestONV0012_ConcurrentNoteWriter is one writer process of
// TestONV0012_ConcurrentNoteWritesAfterNote: a fresh process, as a CLI call
// is, that waits for the common start and sets one note.
func TestONV0012_ConcurrentNoteWriter(t *testing.T) {
	raw := os.Getenv("CORVINT_TASKS_NOTE_WRITER")
	if raw == "" {
		t.Skip("started only by TestONV0012_ConcurrentNoteWritesAfterNote")
	}
	var w noteConcurrencyWorker
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(w.Repo)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(time.UnixMilli(w.Start)))
	start := time.Now()
	run := stagedWrite(writerEnvelope(repo, writerNoteEnvelope(w.ID, w.Ticket, "0", "Concurrent note."), WallClock()), nil)
	s := noteConcurrencySample{Completed: run.completed(), Declined: run.declined(), WallMS: ms(time.Since(start))}
	if run.err != nil {
		s.Code = string(wire.CodeOf(run.err))
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(w.Out, out, 0o600); err != nil {
		t.Fatal(err)
	}
}
