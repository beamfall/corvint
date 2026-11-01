package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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
			// ticket search carries a filter so it refuses for the journal, not as an
			// unfiltered usage error (V1-3278); every read refuses with the same reason.
			var want []string
			for i, args := range [][]string{{"queue", "status"}, {"roadmap"}, {"ticket", "show", "A"}, {"ticket", "search", "--status", "OPEN"}} {
				got := projectionRead(t, Env{Cwd: r.Root, Args: args})
				if got.Outcome == wire.OutcomeOK || strings.Contains(strings.Join(got.Warnings, " "), "unaudited") || len(got.Codes) == 0 || strings.Contains(strings.Join(got.Warnings, " "), "argv") {
					t.Fatalf("fallback for %s %v: %+v", name, args, got)
				}
				if i == 0 {
					want = got.Warnings
				} else if !slices.Equal(got.Warnings, want) {
					t.Fatalf("%s %v: warnings %v, want %v", name, args, got.Warnings, want)
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

// TestCTSV0010_JournalAbsentShowNamesTheStaleSnapshot: a plain clone has the
// committed .taskman snapshot and no journal. A ticket the primary's journal
// holds but the clone's snapshot does not is refused as unobserved in this
// checkout, naming the snapshot and how to reach the journal, never as a
// ticket that does not exist (V1-1379). A journal-backed miss keeps its wording.
func TestCTSV0010_JournalAbsentShowNamesTheStaleSnapshot(t *testing.T) {
	primary := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(primary.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(primary.IntentDir, "policy.json"), fixture.PolicyBytes())
	clone := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(clone.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(clone.IntentDir, "policy.json"), fixture.PolicyBytes())
	if got := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"init"}}); got.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", got)
	}
	payload := `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],"dependencies":[],"dueDate":null,` +
		`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]},"estimateMinutes":null,` +
		`"executionClass":"AUTONOMOUS","kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
		`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"queue:acme:main",` +
		`"sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"Created after the snapshot"}`
	created := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"ticket", "create", "--request-id", "after-snapshot", "--issued-at", "2026-10-15T12:00:00Z", "--payload", payload}})
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created)
	}
	id, _ := created.Items[0].Obj.Get("ticketId")
	if got := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"ticket", "show", id.Str}}); got.Outcome != wire.OutcomeOK {
		t.Fatalf("primary cannot read its own ticket: %+v", got)
	}

	got := projectionRead(t, Env{Cwd: clone.Root, Args: []string{"ticket", "show", id.Str}})
	text := strings.Join(got.Warnings, " ")
	if got.Outcome != wire.OutcomeRefused || len(got.Items) != 0 || got.Snapshot != nil {
		t.Fatalf("clone read: %+v", got)
	}
	for _, want := range []string{"journal-absent", "is not in this checkout's intent projection", ".taskman tree sha256 ", clone.StateDir,
		"created after the last committed .taskman snapshot is NOT_OBSERVED here, not proven absent", "corvint-tasks ticket show " + id.Str} {
		if !strings.Contains(text, want) {
			t.Errorf("clone refusal lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "does not exist in this queue") {
		t.Errorf("clone refusal still claims absence: %s", text)
	}

	miss := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"ticket", "show", "ZZ-999"}})
	if miss.Outcome != wire.OutcomeRefused || !strings.Contains(strings.Join(miss.Warnings, " "), "does not exist in this queue") {
		t.Fatalf("journal-backed miss: %+v", miss)
	}
}
