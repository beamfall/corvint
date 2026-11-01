package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0991_ReplayNamesTheOriginalReceipt: replaying a create by its
// request id reports replayed true, an empty receipt and the original
// receipt's name in originalReceipt, writes no receipt, and verbose help
// documents the envelope (TCL-V0-991, V1-3766).
func TestTCLV0991_ReplayNamesTheOriginalReceipt(t *testing.T) {
	r := receiptFixture(t)
	sk := templateRun(t, r.Root, "--skeleton").Items[0]
	sk.Obj.Set("title", wire.String("Replay receipt ticket"))
	args := []string{"ticket", "create", "--request-id", "replay-1", "--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin"}
	first := atm(t, r.Root, wire.Encode(sk), args...)
	if first.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", first.res)
	}
	original := field(first.res.Items[0], "receipt").Str
	if _, has := first.res.Items[0].Obj.Get("originalReceipt"); original == "" || has || field(first.res.Items[0], "replayed").Bool {
		t.Fatalf("first create envelope: %s", first.stdout)
	}
	receipts := fixture.TreeSnapshot(t, filepath.Join(r.StateDir, "receipts"))
	again := atm(t, r.Root, wire.Encode(sk), args...)
	if !fixture.SameTree(receipts, fixture.TreeSnapshot(t, filepath.Join(r.StateDir, "receipts"))) {
		t.Fatal("a replay wrote a receipt")
	}
	item := again.res.Items[0]
	if again.res.Outcome != wire.OutcomeOK || !field(item, "replayed").Bool || field(item, "receipt").Str != "" || field(item, "originalReceipt").Str != original {
		t.Fatalf("replay envelope (want receipt %s): %s", original, again.stdout)
	}
	help := atm(t, r.Root, nil, "ticket", "create", "--help", "--verbose")
	if !strings.Contains(string(help.stdout), "originalReceipt names the receipt that committed the original") {
		t.Fatalf("help does not document the replay envelope: %s", help.stdout)
	}
}
