package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0873_NotedTicketsArchiveRoundTrip (ON-V0-873, V1-0814 item 4,
// V1-3555): a store holding notes on two tickets, one with a two-event
// chain, exports, restores into its emptied state dir and ticket files to
// the same bytes, and renders the same note state and history.
func TestONV0873_NotedTicketsArchiveRoundTrip(t *testing.T) {
	r, first := attachStore(t)
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-2", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("second ticket: %s", created.stdout)
	}
	second := field(created.res.Items[0], "ticketId").Str
	noteWrite(t, r, first, 0, "SET:first note")
	noteWrite(t, r, second, 0, "SET:second ticket note")
	noteWrite(t, r, first, 1, "SET:first note, revised")
	render := func() []byte {
		var out bytes.Buffer
		for _, id := range []string{first, second} {
			out.Write(atm(t, r.Root, nil, "ticket", "note", "show", id).stdout)
			out.Write(atm(t, r.Root, nil, "ticket", "note", "history", id).stdout)
		}
		return out.Bytes()
	}
	notes := render()
	x := atm(t, r.Root, nil, "archive", "export")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("export: %s", x.stderr)
	}
	path := filepath.Join(fixture.TempDirOutside(t), "queue.tar")
	fixture.Write(t, path, x.stdout)
	state, intentTree := treeBytes(t, r.StateDir), treeBytes(t, r.IntentDir)
	if err := os.RemoveAll(r.StateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(r.IntentDir, "tickets")); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, r.Root, nil, "archive", "restore", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("restore: %s", x.stdout)
	}
	sameBytes(t, "state", state, treeBytes(t, r.StateDir))
	sameBytes(t, "intent", intentTree, treeBytes(t, r.IntentDir))
	if got := render(); !bytes.Equal(got, notes) {
		t.Fatalf("restored notes render differently:\n%s\nwant\n%s", got, notes)
	}
	shown := atm(t, r.Root, nil, "ticket", "note", "show", first)
	if field(shown.res.Items[0], "state").Str != "CURRENT" || field(shown.res.Items[0], "revision").Str != "2" {
		t.Fatalf("restored note: %s", shown.stdout)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit after restore: %s", a.stdout)
	}
}

// TestONV0873_TwoProcessNoteCAS (ON-V0-873, V1-0814 item 5): two processes
// racing NOTE_SET with --supersedes 0 yield exactly one success and one
// REVISION_CONFLICT whose top-level codes carry STALE_TICKET, and the ticket
// keeps one note at revision 1.
func TestONV0873_TwoProcessNoteCAS(t *testing.T) {
	bin := cli.CompiledTasksBinary(t)
	r, id := attachStore(t)
	var wg sync.WaitGroup
	results := make([]*wire.Result, 2)
	start := make(chan struct{})
	for i, text := range []string{"racer a", "racer b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, "ticket", "note", "set", id, "--text", text, "--supersedes", "0",
				"--request-id", "race-"+string(rune('a'+i)), "--issued-at", "2026-10-05T12:00:00Z")
			cmd.Dir = r.Root
			<-start
			out, _ := cmd.Output()
			res, err := wire.DecodeResult(out)
			if err != nil {
				t.Errorf("racer %d: %v %s", i, err, out)
				return
			}
			results[i] = res
		}()
	}
	close(start)
	wg.Wait()
	ok, stale := 0, 0
	for _, res := range results {
		switch {
		case res == nil:
			t.Error("racer without a result")
		case res.Outcome == wire.OutcomeOK:
			ok++
		case len(res.Items) == 1 && field(res.Items[0], "outcome").Str == mutation.OutcomeRevisionConflict:
			if !hasCode(res, wire.CodeStaleTicket) {
				t.Errorf("REVISION_CONFLICT without STALE_TICKET in the top-level codes (TCL-V0-870): %v", res.Codes)
			}
			stale++
		default:
			t.Errorf("racer result %s %v %v", res.Outcome, res.Codes, res.Warnings)
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("racers: %d OK, %d REVISION_CONFLICT; want one of each", ok, stale)
	}
	shown := atm(t, r.Root, nil, "ticket", "note", "show", id)
	if field(shown.res.Items[0], "state").Str != "CURRENT" || field(shown.res.Items[0], "revision").Str != "1" {
		t.Fatalf("after the race: %s", shown.stdout)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit: %s", a.stdout)
	}
}
