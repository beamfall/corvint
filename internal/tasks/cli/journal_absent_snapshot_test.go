package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
