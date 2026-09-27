package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0004_CLICutoverPublishesImportedRecords(t *testing.T) {
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
	if x := atm(t, r.Root, nil, "import", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("import: %+v", x.res)
	}
	for _, args := range [][]string{{"cutover"}, {"cutover", "--decision"}, {"cutover", "--role", "OWNER"}, {"cutover", "--decision", strings.Repeat("d", wire.MaxRequestIDBytes+1)}} {
		if bad := atm(t, r.Root, nil, args...); bad.res.Outcome != wire.OutcomeError {
			t.Fatalf("%v: %+v", args, bad.res)
		}
	}
	x := atm(t, r.Root, nil, "cutover", "--decision", "decision-0500")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("cutover: %s", wire.Encode(x.res.Items[0]))
	}
	show := atm(t, r.Root, nil, "ticket", "blockers", fixture.TicketID("BF-1"))
	for _, b := range field(show.res.Items[0], "blockers").Arr {
		if field(b, "code").Str == wire.CodeCutoverMissing {
			t.Fatalf("still CUTOVER_MISSING: %s", wire.Encode(show.res.Items[0]))
		}
	}
	status := atm(t, r.Root, nil, "queue", "status")
	if field(status.res.Items[0], "canonicalWriter").Str != "NATIVE" || field(status.res.Items[0], "writeBarrier").Str != "CUTOVER" {
		t.Fatalf("status: %s", wire.Encode(status.res.Items[0]))
	}
}
