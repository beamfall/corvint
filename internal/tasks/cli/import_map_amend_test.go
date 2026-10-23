package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// amendMap is a canonical taskman-import-map/0 for the fixture queue mapping
// each {sourceQueueId, sourceItemId, local ticket} triple.
func amendMap(entries ...[3]string) []byte {
	body := ""
	for i, e := range entries {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"appliedSeq":"1","sourceItemId":%q,"sourceQueueId":%q,"sourceRevisionSha256":null,"ticketId":%q}`, e[1], e[0], fixture.TicketID(e[2]))
	}
	return []byte(`{"entries":[` + body + `],"profile":"taskman-import-map/0","queueId":"` + fixture.QueueID + `"}` + "\n")
}

// TestCTSV0021_ImportMapAmendThroughTheCLI proves `import-map amend` records
// an amended map in a mapped store and the readers still accept the store,
// refuses a stale digest with REVISION_CONFLICT and a dropped identity with
// DUPLICATE_ID writing nothing, and rejects a missing flag as usage.
func TestCTSV0021_ImportMapAmendThroughTheCLI(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	r := fixture.TempRepo(t)
	current := amendMap([3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	q.Obj.Set("canonicalWriter", wire.String("ROADMAP"))
	q.Obj.Set("importMapSha256", wire.String(string(wire.Sum(current))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, intent.ImportMapFile), current)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := store.Init(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, "init-mapped", wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("init: %+v %v", report, err)
	}
	write := func(name string, raw []byte) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	amend := func(id string, expected []byte, file string) run {
		t.Helper()
		return atm(t, r.Root, nil, "import-map", "amend", "--request-id", id, "--expected-import-map-sha256", string(wire.Sum(expected)), "--file", file)
	}

	if x := atm(t, r.Root, nil, "import-map"); x.res.Outcome != wire.OutcomeError || x.code == 0 {
		t.Fatalf("import-map without a verb: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, "import-map", "amend", "--request-id", "no-file"); x.res.Outcome != wire.OutcomeError || x.code == 0 {
		t.Fatalf("import-map amend without --file: %+v", x.res)
	}

	amended := amendMap([3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:other:main", "X-1", "BF-9"})
	x := amend("amend-1", current, write("amended.json", amended))
	item := x.res.Items[0]
	if x.res.Outcome != wire.OutcomeOK || x.code != 0 || field(item, "outcome").Str != mutation.OutcomeCompleted || field(item, "newImportMapSha256").Str != string(wire.Sum(amended)) || field(item, "oldImportMapSha256").Str != string(wire.Sum(current)) {
		t.Fatalf("amend: code %d %s", x.code, x.stdout)
	}
	if got, err := os.ReadFile(filepath.Join(r.IntentDir, intent.ImportMapFile)); err != nil || string(got) != string(amended) {
		t.Fatalf("map projection: %v", err)
	}
	for _, args := range [][]string{{"receipt", "audit"}, {"queue", "status"}} {
		if y := atm(t, r.Root, nil, args...); y.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v after the amendment: %s", args, y.stdout)
		}
	}
	help := atm(t, r.Root, nil, "import-map", "amend", "--help", "--verbose")
	if help.res.Outcome != wire.OutcomeOK || field(help.res.Items[0], "identityRule").Str == "" {
		t.Fatalf("import-map amend help: %s", help.stdout)
	}

	before := fixture.TreeSnapshot(t, r.StateDir)
	stale := amend("amend-stale", current, write("stale.json", amended))
	if stale.res.Outcome != wire.OutcomeRefused || stale.code == 0 || field(stale.res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict {
		t.Fatalf("stale digest: code %d %s", stale.code, stale.stdout)
	}
	dropped := amend("amend-drop", amended, write("dropped.json", amendMap([3]string{"queue:other:main", "X-1", "BF-9"})))
	if dropped.res.Outcome != wire.OutcomeRefused || dropped.code == 0 || len(dropped.res.Codes) != 1 || dropped.res.Codes[0] != wire.CodeDuplicateID {
		t.Fatalf("dropped identity: code %d %s", dropped.code, dropped.stdout)
	}
	if after := fixture.TreeSnapshot(t, r.StateDir); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatal("a refused amendment wrote to the state dir")
	}
}
