package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const importExportJSONL = `{"profile":"corvint-tasks-import/0","sourceQueueId":"queue:beamfall:main"}
{"block":"## BF-1\n","sourceItemId":"BF-1","ticket":{"acceptanceCriteria":["it exists"],"archivedFrom":null,"body":"## BF-1\n","capabilities":[],"completion":null,"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS","holds":[],"kind":"FEATURE","labels":["alias:bf-one"],"milestone":null,"order":"1","owner":null,"priority":"P2","requiredGates":["verify"],"requirementRefs":[],"status":"OPEN","supersededBy":null,"supersedes":null,"title":"Imported BF-1"}}
`

func TestCTSV0003_CLIImportWritesShadowRecordsBlockedOnCutover(t *testing.T) {
	r := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("canonicalWriter", wire.String("ROADMAP"))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "export.jsonl")
	fixture.Write(t, path, []byte(importExportJSONL))
	x := atm(t, r.Root, nil, "import", "--file", path)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "written").Str != "1" || len(field(x.res.Items[0], "receipts").Arr) != 1 {
		t.Fatalf("import: %+v", x.res)
	}
	show := atm(t, r.Root, nil, "ticket", "blockers", fixture.TicketID("BF-1"))
	if show.res.Outcome != wire.OutcomeOK || field(show.res.Items[0], "eligibility").Str != "BLOCKED" || field(field(show.res.Items[0], "blockers").Arr[0], "code").Str != wire.CodeCutoverMissing {
		t.Fatalf("blockers: %s", wire.Encode(show.res.Items[0]))
	}
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	again := atm(t, r.Root, nil, "import", "--file", path)
	if again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "unchanged").Str != "1" || len(field(again.res.Items[0], "receipts").Arr) != 0 {
		t.Fatalf("re-import: %+v", again.res)
	}
	sameStore(t, r, state, intents, "idempotent re-import")
	if bad := atm(t, r.Root, nil, "import"); bad.res.Outcome != wire.OutcomeError {
		t.Fatalf("missing --file: %+v", bad.res)
	}
}
