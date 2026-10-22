package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0170_DeliveryScreenStoreVerdictsAndNotes covers V1-2397: a
// COMPLETED ticket is COMPLETED and a held one HELD (with holdId and reason)
// even when an open or merged PR cites it, the store outranking PR evidence;
// each row carries the latest operator note; the screen stays write-free
// (TCD-V0-170).
func TestTCDV0170_DeliveryScreenStoreVerdictsAndNotes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	compID, comp := tcdTicket(t, root, "comp", "")
	heldID, held := tcdTicket(t, root, "held", "")
	notedID, noted := tcdTicket(t, root, "noted", "")
	if x := atm(t, root, nil, "ticket", "complete-manual", "--request-id", "done", "--target", compID, "--expected-revision", "1",
		"--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "hold", "--target", heldID, "--expected-revision", "1", "--request-id", "hold",
		"--payload", `{"holdId":"owner-wait","reason":"waits for owner"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("hold: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "note", "set", notedID, "--request-id", "note", "--text", "rerun the gate once"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note set: %s", x.stdout)
	}
	prs := tdsExport(t, "prs.json", `[
{"number":10,"state":"OPEN","title":"lane","body":"`+tdsJSON("Refs "+comp)+`","labels":[]},
{"number":11,"state":"MERGED","title":"lane two","body":"`+tdsJSON("Refs "+held)+`","labels":[]}]`)
	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	x := tdsRun(t, root, "ticket", "delivery-screen", comp, held, noted, "--prs", prs)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("delivery-screen changed the intent or state tree")
	}
	got := tdsVerdicts(t, x)
	if got[comp] != "COMPLETED" || got[held] != "HELD" || got[noted] != "CLEAR" {
		t.Fatalf("verdicts %v: %s", got, x.stdout)
	}
	for _, r := range field(x.res.Items[0], "tickets").Arr {
		holds, note := field(r, "holds").Arr, field(r, "operatorNote")
		switch field(r, "local").Str {
		case held:
			if len(holds) != 1 || field(holds[0], "holdId").Str != "owner-wait" || field(holds[0], "reason").Str != "waits for owner" {
				t.Fatalf("held row: %s", wire.Encode(r))
			}
		case noted:
			if len(holds) != 0 || field(note, "state").Str != "CURRENT" || field(note, "text").Str != "rerun the gate once" {
				t.Fatalf("noted row: %s", wire.Encode(r))
			}
		case comp:
			if field(note, "state").Str != "NONE" {
				t.Fatalf("completed row: %s", wire.Encode(r))
			}
		}
	}
}

// TestTCDV0171_DeliveryScreenReadsPRsFromStdin covers V1-2397: `--prs -`
// reads the PR export from stdin with the same verdicts as a file, and an
// empty stdin is refused (TCD-V0-171).
func TestTCDV0171_DeliveryScreenReadsPRsFromStdin(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, inflight := tcdTicket(t, root, "inflight", "")
	body := []byte(`[{"number":10,"state":"OPEN","title":"lane","body":"` + tdsJSON("Refs "+inflight) + `","labels":[]}]`)
	stdin := func(in []byte) run {
		t.Helper()
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: root, Args: []string{"ticket", "delivery-screen", inflight, "--prs", "-"}, Stdin: bytes.NewReader(in), Stdout: &out})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("envelope: %v\n%s", err, out.Bytes())
		}
		return run{code: code, stdout: out.Bytes(), res: res}
	}
	file := tdsExport(t, "prs.json", string(body))
	if x := stdin(body); tdsVerdicts(t, x)[inflight] != "IN_FLIGHT" || field(x.res.Items[0], "github").Str != "OBSERVED" {
		t.Fatalf("stdin export: %s", x.stdout)
	}
	if x := tdsRun(t, root, "ticket", "delivery-screen", inflight, "--prs", file); tdsVerdicts(t, x)[inflight] != "IN_FLIGHT" {
		t.Fatalf("file export: %s", x.stdout)
	}
	if x := stdin(nil); x.code != 2 || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("empty stdin: exit %d %s", x.code, x.stdout)
	}
}
