package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0680_JournalAbsentMissNamesSnapshotRoute: a journal-absent miss
// names the queue snapshot export and the CORVINT_TASKS_SNAPSHOT retry, and
// the named route, run as written, serves the ticket on that host (V1-3262).
func TestCTSV0680_JournalAbsentMissNamesSnapshotRoute(t *testing.T) {
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
		`"sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"Only the primary holds it"}`
	created := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"ticket", "create", "--request-id", "snapshot-route", "--issued-at", "2026-11-01T12:00:00Z", "--payload", payload}})
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created)
	}
	id, _ := created.Items[0].Obj.Get("ticketId")

	got := projectionRead(t, Env{Cwd: clone.Root, Args: []string{"ticket", "show", id.Str}})
	text := strings.Join(got.Warnings, " ")
	if got.Outcome != wire.OutcomeRefused || len(got.Items) != 0 || got.Snapshot != nil || len(got.Codes) != 1 || got.Codes[0] != wire.CodeMalformed {
		t.Fatalf("clone read: %+v", got)
	}
	for _, want := range []string{"is not in this checkout's intent projection", "`corvint-tasks queue snapshot --out FILE`",
		"`" + SnapshotEnv + "=FILE corvint-tasks ticket show " + id.Str + "`", "(CTS-V0-680)"} {
		if !strings.Contains(text, want) {
			t.Errorf("clone refusal lacks %q: %s", want, text)
		}
	}

	// The named route works as written: export on the primary, read on the host.
	file := filepath.Join(t.TempDir(), "snapshot.json")
	if res := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"queue", "snapshot", "--out", file}}); res.Outcome != wire.OutcomeOK {
		t.Fatalf("queue snapshot: %+v", res)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	t.Setenv(SnapshotEnv, file)
	served := projectionRead(t, Env{Cwd: clone.Root, Args: []string{"ticket", "show", id.Str}})
	if served.Outcome != wire.OutcomeOK || len(served.Items) != 1 {
		t.Fatalf("snapshot read: %+v", served)
	}
}

// TestASDV02421_JournalAbsentMissNamesStagedSnapshot covers V1-4440 and
// V1-4226: on a lane host without the journal, the miss names the newest
// snapshot the dispatch kit staged under $HOME/lane-inputs with the exact
// retry, which then serves the ticket; with none staged it names the
// staging command. The refusal itself is unchanged (ASD-V0-2421).
func TestASDV02421_JournalAbsentMissNamesStagedSnapshot(t *testing.T) {
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
		`"sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"Only the primary holds it"}`
	created := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"ticket", "create", "--request-id", "staged-route", "--issued-at", "2026-11-03T12:00:00Z", "--payload", payload}})
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created)
	}
	id, _ := created.Items[0].Obj.Get("ticketId")
	miss := func() string {
		t.Helper()
		got := projectionRead(t, Env{Cwd: clone.Root, Args: []string{"ticket", "show", id.Str}})
		if got.Outcome != wire.OutcomeRefused || len(got.Items) != 0 || got.Snapshot != nil || len(got.Codes) != 1 || got.Codes[0] != wire.CodeMalformed {
			t.Fatalf("clone read: %+v", got)
		}
		return strings.Join(got.Warnings, " ")
	}

	// Nothing staged: the miss names the staging command.
	home := filepath.Join(t.TempDir(), "lane home")
	t.Setenv("HOME", home)
	text := miss()
	for _, want := range []string{"No queue snapshot is staged under $HOME/lane-inputs on this host",
		"`script/lane-stage-inputs.sh --host HOST LANE " + id.Str + "` (ASD-V0-2421)", "(CTS-V0-680)"} {
		if !strings.Contains(text, want) {
			t.Errorf("unstaged miss lacks %q: %s", want, text)
		}
	}

	// An older top-level snapshot, a newer per-lane one and a still newer
	// empty file: the newest non-empty file is named, quoted for its space.
	inputs := filepath.Join(home, "lane-inputs")
	older, newer, empty := filepath.Join(inputs, "queue-snapshot.json"), filepath.Join(inputs, "ticket-bodies-abc-w1", "queue-snapshot.json"),
		filepath.Join(inputs, "ticket-bodies-abc-w2", "queue-snapshot.json")
	for _, f := range []string{older, newer} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if res := projectionRead(t, Env{Cwd: primary.Root, Args: []string{"queue", "snapshot", "--out", f}}); res.Outcome != wire.OutcomeOK {
			t.Fatalf("queue snapshot: %+v", res)
		}
	}
	fixture.Write(t, empty, nil)
	now := time.Now()
	for f, age := range map[string]time.Duration{older: 5 * time.Hour, newer: 3 * time.Hour, empty: time.Minute} {
		if err := os.Chtimes(f, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	text = miss()
	quoted := "'" + newer + "'"
	for _, want := range []string{"A queue snapshot is staged on this host at " + quoted + " (modified 3 hour(s) ago)",
		"`" + SnapshotEnv + "=" + quoted + " corvint-tasks ticket show " + id.Str + "` (ASD-V0-2421)"} {
		if !strings.Contains(text, want) {
			t.Errorf("staged miss lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "No queue snapshot is staged") {
		t.Errorf("staged miss still says none is staged: %s", text)
	}

	// The named retry serves the ticket on this host.
	t.Setenv(SnapshotEnv, newer)
	served := projectionRead(t, Env{Cwd: clone.Root, Args: []string{"ticket", "show", id.Str}})
	if served.Outcome != wire.OutcomeOK || len(served.Items) != 1 {
		t.Fatalf("snapshot read: %+v", served)
	}
}
