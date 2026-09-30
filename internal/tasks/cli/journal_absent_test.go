package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func projectionRead(t *testing.T, env Env) *wire.Result {
	t.Helper()
	var out bytes.Buffer
	env.Stdout = &out
	env.Stderr = &out
	Run(env)
	r, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCTS002JournalAbsentReads(t *testing.T) {
	t.Run("CTS-V0-002 four inventory verbs are unaudited and read only", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		before := fixture.TreeSnapshot(t, r.Root)
		for _, args := range [][]string{{"queue", "status"}, {"roadmap"}, {"ticket", "show", "A"}, {"ticket", "search", "--status", "OPEN"}} {
			got := projectionRead(t, Env{Cwd: r.Root, Args: args})
			if got.Outcome != wire.OutcomeOK || got.Snapshot != nil || len(got.Items) == 0 || !strings.Contains(strings.Join(got.Warnings, " "), "journal-absent; unaudited") {
				t.Fatalf("%v: %+v", args, got)
			}
			if args[0] == "ticket" && args[1] == "show" {
				current, _ := got.Items[0].Obj.Get("currentAttempt")
				unknowns, _ := got.Items[0].Obj.Get("unknowns")
				if current.Str != "NOT_OBSERVED" || !strings.Contains(string(wire.Encode(unknowns)), "attempt liveness NOT_OBSERVED (no journal available to this reader)") {
					t.Fatalf("invented reservation evidence: %s", wire.Encode(got.Items[0]))
				}
			}
			if args[0] == "queue" {
				for _, name := range []string{"headSeq", "generation", "attempts"} {
					v, _ := got.Items[0].Obj.Get(name)
					if v.Str != "NOT_OBSERVED" {
						t.Fatalf("invented %s: %+v", name, v)
					}
				}
				for _, name := range []string{"barrier", "liveAttempts"} {
					v, _ := got.Items[0].Obj.Get(name)
					if v.Kind != wire.KindNull {
						t.Fatalf("invented %s", name)
					}
				}
			}
		}
		for _, args := range [][]string{{"receipt", "audit"}, {"ticket", "blockers", "A"}, {"ticket", "list"}, {"ticket", "export"}, {"init", "--request-id", "must-refuse"}, {"ticket", "set-gates", "--request-id", "must-refuse", "--target", "A", "--expected-revision", "1", "--payload", `{"requiredGates":["verify"]}`}} {
			got := projectionRead(t, Env{Cwd: r.Root, Args: args})
			if got.Outcome == wire.OutcomeOK {
				t.Fatalf("authority admitted: %v", args)
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) {
			t.Fatal("reads or refused writers changed repository")
		}
	})
}

func TestCTS002NoFallbackForExistingJournal(t *testing.T) {
	for _, name := range []string{"empty", "head-missing", "corrupt", "redo", "file", "symlink"} {
		t.Run("CTS-V0-002 "+name, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			switch name {
			case "empty":
				if err := os.MkdirAll(r.StateDir, 0700); err != nil {
					t.Fatal(err)
				}
			case "file":
				fixture.Write(t, r.StateDir, []byte("not directory"))
			case "symlink":
				if err := os.Symlink(t.TempDir(), r.StateDir); err != nil {
					t.Fatal(err)
				}
			default:
				fixture.WriteState(t, r)
				if name == "head-missing" {
					os.Remove(filepath.Join(r.StateDir, "head.json"))
				}
				if name == "corrupt" {
					fixture.Write(t, filepath.Join(r.StateDir, "head.json"), []byte("{}\n"))
				}
				if name == "redo" {
					fixture.PlantReceipt(t, r, 2)
				}
			}
			for _, args := range [][]string{{"queue", "status"}, {"roadmap"}, {"ticket", "show", "A"}, {"ticket", "search"}} {
				got := projectionRead(t, Env{Cwd: r.Root, Args: args})
				if got.Outcome == wire.OutcomeOK || strings.Contains(strings.Join(got.Warnings, " "), "unaudited") {
					t.Fatalf("fallback for %s: %+v", name, got)
				}
			}
		})
	}
}

func TestCTS002ProjectionRaces(t *testing.T) {
	t.Run("CTS-V0-002 bounded changing projection", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		calls := 0
		got := ticketShow(Env{Cwd: r.Root, afterRead: func() {
			calls++
			v := fixture.Ticket("A")
			v.Title = strings.Repeat("x", calls+1)
			fixture.WriteIntent(t, r, v)
		}}, []string{"A"}, true)
		if calls != 4 || len(got.Codes) != 1 || got.Codes[0] != wire.CodeSnapshotMoved || got.Snapshot != nil || len(got.Items) != 0 {
			t.Fatalf("calls=%d result=%+v", calls, got)
		}
	})
	t.Run("CTS-V0-002 journal appears during read", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		got := ticketShow(Env{Cwd: r.Root, afterRead: func() {
			if err := os.MkdirAll(r.StateDir, 0700); err != nil {
				t.Fatal(err)
			}
		}}, []string{"A"}, true)
		if got.Outcome == wire.OutcomeOK || len(got.Items) != 0 || got.Snapshot != nil {
			t.Fatalf("mixed read: %+v", got)
		}
	})
	t.Run("CTS-V0-002 malformed intent never answers", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "A.json"), []byte("{}\n"))
		got := ticketShow(Env{Cwd: r.Root}, []string{"A"}, true)
		if len(got.Codes) != 1 || got.Codes[0] != wire.CodeMalformed {
			t.Fatalf("%+v", got)
		}
	})
}
