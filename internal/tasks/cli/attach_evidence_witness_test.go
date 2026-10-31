package cli_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// attachStore is a native store with one ticket at revision 1.
func attachStore(t *testing.T) (*fixture.Repo, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %s", created.stdout)
	}
	return r, field(created.res.Items[0], "ticketId").Str
}

func attachArgs(id, rev, req, text string) []string {
	payload := `{"evidence":["` + string(wire.Sum([]byte(text))) + `"],"reason":"` + text + `"}`
	return []string{"ticket", "attach-evidence", "--target", id, "--expected-revision", rev,
		"--request-id", req, "--issued-at", "2026-10-05T12:00:00Z", "--payload", payload}
}

func ticketFile(t *testing.T, r *fixture.Repo, id string) string {
	t.Helper()
	return filepath.Join(r.IntentDir, "tickets", id[strings.LastIndex(id, ":")+1:]+".json")
}

// TestTEAV0001_ArchiveCarriesAttachedEvidence (V1-0813 item 1): `archive
// export` of a store holding an attachment-bearing record carries that
// record's exact bytes, `archive verify` accepts the stream, and `archive
// restore` into the emptied store rebuilds the record and every receipt
// byte-for-byte (TRL-V0-871).
func TestTEAV0001_ArchiveCarriesAttachedEvidence(t *testing.T) {
	r, id := attachStore(t)
	if x := atm(t, r.Root, nil, attachArgs(id, "1", "ev-1", "archived log")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %s", x.stdout)
	}
	want, err := os.ReadFile(ticketFile(t, r, id))
	if err != nil || !bytes.Contains(want, []byte("attachedEvidence")) {
		t.Fatalf("ticket projection: %v %s", err, want)
	}
	x := atm(t, r.Root, nil, "archive", "export")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("export: %s", x.stderr)
	}
	found := false
	tr := tar.NewReader(bytes.NewReader(x.stdout))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if body, _ := io.ReadAll(tr); bytes.Equal(body, want) {
			found = true
			t.Logf("archive member %s carries the attachment-bearing record", h.Name)
		}
	}
	if !found {
		t.Fatal("archive has no member equal to the attachment-bearing record")
	}
	if v := atm(t, r.Root, x.stdout, "archive", "verify"); v.res.Outcome != wire.OutcomeOK {
		t.Fatalf("verify: %s", v.stdout)
	}
	receipts := treeBytes(t, filepath.Join(r.StateDir, "receipts"))
	path := filepath.Join(fixture.TempDirOutside(t), "queue.tar")
	fixture.Write(t, path, x.stdout)
	if err := os.RemoveAll(r.StateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ticketFile(t, r, id)); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, r.Root, nil, "archive", "restore", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("restore: %s", x.stdout)
	}
	if got, err := os.ReadFile(ticketFile(t, r, id)); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("restored record differs: %v\n%s", err, got)
	}
	sameBytes(t, "receipts", receipts, treeBytes(t, filepath.Join(r.StateDir, "receipts")))
	shown := atm(t, r.Root, nil, "ticket", "show", id)
	if n := len(field(shown.res.Items[0], "attachedEvidence").Arr); n != 1 {
		t.Fatalf("restored ticket carries %d attachments: %s", n, shown.stdout)
	}
}

// TestTEAV0001_TwoProcessAttachEvidenceCAS (V1-0813 item 2): two processes
// racing ATTACH_EVIDENCE at the same expected revision yield exactly one
// success and one REVISION_CONFLICT refusal whose top-level codes carry
// STALE_TICKET (TCL-V0-870), and the record carries one entry.
func TestTEAV0001_TwoProcessAttachEvidenceCAS(t *testing.T) {
	bin := cli.CompiledTasksBinary(t)
	r, id := attachStore(t)
	var wg sync.WaitGroup
	results := make([]*wire.Result, 2)
	start := make(chan struct{})
	for i, text := range []string{"racer a", "racer b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, attachArgs(id, "1", "race-"+string(rune('a'+i)), text)...)
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
		case res.Outcome == wire.OutcomeOK:
			ok++
		case res == nil:
			t.Error("racer without a result")
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
	shown := atm(t, r.Root, nil, "ticket", "show", id)
	if n := len(field(shown.res.Items[0], "attachedEvidence").Arr); n != 1 || field(field(shown.res.Items[0], "record"), "revision").Str != "2" {
		t.Fatalf("after the race: %d entries: %s", n, shown.stdout)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit: %s", a.stdout)
	}
}

// TestTEAV0001_InterruptedAttachmentIsRedone (V1-0813 item 3): an
// attachment receipt whose head and ticket projection did not advance
// (crash point C2, after journal append and before commit) is redone by the
// next writer to the same record bytes.
func TestTEAV0001_InterruptedAttachmentIsRedone(t *testing.T) {
	r, id := attachStore(t)
	headPath, recordPath := filepath.Join(r.StateDir, "head.json"), ticketFile(t, r, id)
	head, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if x := atm(t, r.Root, nil, attachArgs(id, "1", "ev-1", "redo log")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %s", x.stdout)
	}
	want, err := os.ReadFile(recordPath)
	if err != nil || bytes.Equal(want, old) {
		t.Fatalf("attachment did not change the projection: %v", err)
	}
	fixture.Write(t, headPath, head)
	fixture.Write(t, recordPath, old)
	trigger := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-2", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if trigger.res.Outcome != wire.OutcomeOK {
		t.Fatalf("trigger write: %s", trigger.stdout)
	}
	if got, err := os.ReadFile(recordPath); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("redo did not restore the attachment record: %v\n%s\nwant\n%s", err, got, want)
	}
	if a := atm(t, r.Root, nil, "receipt", "audit"); field(a.res.Items[0], "structuralConsistency").Str != "CONSISTENT" ||
		field(a.res.Items[0], "projectionAgreement").Str != "AGREES" {
		t.Fatalf("audit after redo: %s", a.stdout)
	}
}
