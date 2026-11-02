package cli_test

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0820_BoundedAuditIgnoresLaterInconsistentReceipt: a receipt after
// SEQ is corrupted, so the complete audit refuses the store, while
// `receipt audit --through SEQ` walks receipts 1..SEQ, reports CONSISTENT
// and names the bound (TCL-V0-820). Out-of-range or combined flags are
// refused MALFORMED.
func TestTCLV0820_BoundedAuditIgnoresLaterInconsistentReceipt(t *testing.T) {
	root, _ := leaseCLIStore(t, 2, time.Now().UTC().Truncate(time.Second))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	headRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	headSeq := head.LastSeq.Uint64()
	if headSeq < 3 {
		t.Fatalf("fixture has only %d receipts", headSeq)
	}
	full := atm(t, root, nil, "receipt", "audit")
	if full.code != 0 {
		t.Fatalf("intact store: %s", full.stdout)
	}
	if _, ok := full.res.Items[0].Obj.Get("throughSeq"); ok {
		t.Fatal("the complete audit reports a throughSeq")
	}

	names, err := filepath.Glob(filepath.Join(repo.StateDir, "receipts", "*"))
	if err != nil || uint64(len(names)) < headSeq {
		t.Fatalf("receipts: %v %v", names, err)
	}
	sort.Strings(names)
	through := headSeq - 2
	// Receipt through+1 is neither in the bound nor the head receipt the
	// snapshot reads.
	if err := os.WriteFile(names[through], []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, root, nil, "receipt", "audit"); x.code == 0 {
		t.Fatalf("complete audit accepted a corrupted receipt: %s", x.stdout)
	}

	seq := strconv.FormatUint(through, 10)
	x := atm(t, root, nil, "receipt", "audit", "--through", seq)
	if x.code != 0 || len(x.res.Items) != 1 {
		t.Fatalf("bounded audit: %s", x.stdout)
	}
	item := x.res.Items[0]
	for key, want := range map[string]string{"throughSeq": seq, "headSeq": seq, "structuralConsistency": "CONSISTENT", "projectionAgreement": "NOT_OBSERVED"} {
		if got := field(item, key).Str; got != want {
			t.Fatalf("%s = %q, want %q: %s", key, got, want, x.stdout)
		}
	}

	for _, args := range [][]string{
		{"--through", "0"},
		{"--through", "x"},
		{"--through", "01"},
		{"--through"},
		{"--through", strconv.FormatUint(headSeq+1, 10)},
		{"--through", seq, "--witness-receipts"},
	} {
		x := atm(t, root, nil, append([]string{"receipt", "audit"}, args...)...)
		if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
}

// TestTCLV0820_BoundedAuditIgnoresCorruptHeadReceipt: the head receipt is
// after SEQ, so a corrupt one refuses the complete audit but not
// `receipt audit --through SEQ` below the head (TCL-V0-820).
func TestTCLV0820_BoundedAuditIgnoresCorruptHeadReceipt(t *testing.T) {
	root, _ := leaseCLIStore(t, 2, time.Now().UTC().Truncate(time.Second))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	headRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	headSeq := head.LastSeq.Uint64()
	names, err := filepath.Glob(filepath.Join(repo.StateDir, "receipts", "*"))
	if err != nil || uint64(len(names)) < headSeq {
		t.Fatalf("receipts: %v %v", names, err)
	}
	sort.Strings(names)
	if err := os.WriteFile(names[headSeq-1], []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, root, nil, "receipt", "audit"); x.code == 0 {
		t.Fatalf("complete audit accepted a corrupted head receipt: %s", x.stdout)
	}
	seq := strconv.FormatUint(headSeq-1, 10)
	x := atm(t, root, nil, "receipt", "audit", "--through", seq)
	if x.code != 0 || len(x.res.Items) != 1 {
		t.Fatalf("bounded audit: %s", x.stdout)
	}
	for key, want := range map[string]string{"throughSeq": seq, "headSeq": seq, "structuralConsistency": "CONSISTENT", "projectionAgreement": "NOT_OBSERVED"} {
		if got := field(x.res.Items[0], key).Str; got != want {
			t.Fatalf("%s = %q, want %q: %s", key, got, want, x.stdout)
		}
	}
}
