package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// cliImportMap is a taskman-import-map/0 for the fixture queue mapping each
// [sourceQueueId, sourceItemId, local ticket] triple.
func cliImportMap(entries ...[3]string) []byte {
	values := []wire.Value{}
	for _, e := range entries {
		o := wire.NewObject().Set("appliedSeq", wire.String("1")).Set("sourceItemId", wire.String(e[1])).Set("sourceQueueId", wire.String(e[0])).Set("sourceRevisionSha256", wire.Null()).Set("ticketId", wire.String(fixture.TicketID(e[2])))
		values = append(values, wire.ObjectValue(o))
	}
	return wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("entries", wire.Array(values...)).Set("profile", wire.String(intent.ProfileImportMap)).Set("queueId", wire.String(fixture.QueueID))))
}

// TestCTSV0020_ImportMapUpdateThroughTheCLI: `import-map update` commits an
// amended map for a store initialized from one, names the new digest, keeps
// receipt audit CONSISTENT, refuses a conflicting amendment DUPLICATE_ID and a
// stale expected digest, and refuses missing flags as usage (CTS-V0-020..022).
func TestCTSV0020_ImportMapUpdateThroughTheCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	current := cliImportMap([3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	q.Obj.Set("importMapSha256", wire.String(string(wire.Sum(current))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, intent.ImportMapFile), current)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	next := cliImportMap([3]string{"queue:beamfall:renamed", "BF-1", "BF-1"})
	nextPath := write("next.json", next)
	if x := atm(t, r.Root, nil, "import-map", "update", "--request-id", "amend-1", "--file", nextPath); x.res.Outcome == wire.OutcomeOK || x.code == 0 {
		t.Fatalf("missing --expected-import-map-sha256 accepted: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, "import-map", "update", "--request-id", "amend-1", "--expected-import-map-sha256", string(wire.Sum(current)), "--file", nextPath)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "newImportMapSha256").Str != string(wire.Sum(next)) {
		t.Fatalf("import-map update: %s", x.stdout)
	}
	if audit := atm(t, r.Root, nil, "receipt", "audit"); audit.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit after the amendment: %s", audit.stdout)
	}
	if got, err := os.ReadFile(filepath.Join(r.IntentDir, intent.ImportMapFile)); err != nil || string(got) != string(next) {
		t.Fatalf("import map projection: %v", err)
	}
	moved := write("moved.json", cliImportMap([3]string{"queue:beamfall:renamed", "BF-1", "OTHER-1"}))
	if y := atm(t, r.Root, nil, "import-map", "update", "--request-id", "amend-2", "--expected-import-map-sha256", string(wire.Sum(next)), "--file", moved); y.res.Outcome == wire.OutcomeOK || !hasCode(y.res, wire.CodeDuplicateID) {
		t.Fatalf("conflicting amendment: %s", y.stdout)
	}
	if y := atm(t, r.Root, nil, "import-map", "update", "--request-id", "amend-3", "--expected-import-map-sha256", string(wire.Sum(current)), "--file", nextPath); y.res.Outcome == wire.OutcomeOK {
		t.Fatalf("stale expected digest accepted: %s", y.stdout)
	}
}
