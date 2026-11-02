//go:build darwin || linux

package cli_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0302_ReceiptCompactCLI: `receipt compact` is opt-in and refuses
// any argument but --dry-run. A dry run reports the superseded heartbeats
// and writes nothing; the real run elides them, after which receipt audit,
// doctor and a replay of an elided heartbeat still succeed, and a rerun
// finds nothing new.
func TestCALV0302_ReceiptCompactCLI(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// leaseCLIStore claims at base+10m with a five-minute lease.
	root, claims := leaseCLIStore(t, 1, now.Add(-11*time.Minute))
	c := claims[0]
	heartbeat := func(i int) run {
		return atm(t, root, nil, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "hb-"+strconv.Itoa(i))
	}
	for i := 0; i < 3; i++ {
		if h := heartbeat(i); h.code != 0 {
			t.Fatalf("heartbeat %d: %s", i, h.stdout)
		}
	}
	if bad := atm(t, root, nil, "receipt", "compact", "--force"); bad.code == 0 || bad.res.Outcome == wire.OutcomeOK {
		t.Fatalf("compact accepted --force: %s", bad.stdout)
	}
	compact := func(want string, args ...string) wire.Value {
		x := atm(t, root, nil, append([]string{"receipt", "compact"}, args...)...)
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || field(x.res.Items[0], "elided").Str != want {
			t.Fatalf("compact %v want elided %s: %s", args, want, x.stdout)
		}
		return x.res.Items[0]
	}
	dry := compact("2", "--dry-run")
	if !field(dry, "dryRun").Bool {
		t.Fatalf("dry run: %v", dry)
	}
	done := compact("2")
	before, _ := strconv.Atoi(field(done, "receiptBytesBefore").Str)
	after, _ := strconv.Atoi(field(done, "receiptBytesAfter").Str)
	if field(done, "dryRun").Bool || field(done, "alreadyElided").Str != "0" || after >= before || field(dry, "receiptBytesAfter").Str != field(done, "receiptBytesAfter").Str {
		t.Fatalf("compact: dry %v done %v", dry, done)
	}
	if a := atm(t, root, nil, "receipt", "audit"); a.code != 0 || a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("audit after compact: %s", a.stdout)
	}
	if d, _ := doctorOK(t, root); strings.Contains(string(d.stdout), "scan stopped") {
		t.Fatalf("doctor could not read the elided receipts: %s", d.stdout)
	}
	if r := heartbeat(0); r.code != 0 || r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("replay of an elided heartbeat: %s", r.stdout)
	}
	again := compact("0")
	if field(again, "alreadyElided").Str != "2" {
		t.Fatalf("rerun: %v", again)
	}
}

// TestTCLV0820_BoundedAuditRestoresElidedReceipts: after `receipt compact`
// elides superseded heartbeats, `receipt audit --through SEQ` restores them
// as the complete walk does, including an elided receipt at SEQ itself
// whose superseding heartbeat lies after the bound (TCL-V0-820).
func TestTCLV0820_BoundedAuditRestoresElidedReceipts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	root, claims := leaseCLIStore(t, 1, now.Add(-11*time.Minute))
	c := claims[0]
	for i := 0; i < 3; i++ {
		if h := atm(t, root, nil, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "hb-"+strconv.Itoa(i)); h.code != 0 {
			t.Fatalf("heartbeat %d: %s", i, h.stdout)
		}
	}
	if x := atm(t, root, nil, "receipt", "compact"); x.code != 0 || field(x.res.Items[0], "elided").Str != "2" {
		t.Fatalf("compact: %s", x.stdout)
	}
	full := atm(t, root, nil, "receipt", "audit")
	if full.code != 0 {
		t.Fatalf("complete audit after compact: %s", full.stdout)
	}
	headSeq, err := strconv.ParseUint(field(full.res.Items[0], "headSeq").Str, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, through := range []uint64{headSeq - 1, headSeq} {
		seq := strconv.FormatUint(through, 10)
		x := atm(t, root, nil, "receipt", "audit", "--through", seq)
		if x.code != 0 || len(x.res.Items) != 1 || field(x.res.Items[0], "structuralConsistency").Str != "CONSISTENT" || field(x.res.Items[0], "headSeq").Str != seq {
			t.Fatalf("--through %s over elided receipts: %s", seq, x.stdout)
		}
	}
}
