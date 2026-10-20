package wire

import (
	"bytes"
	"testing"
)

// TestCEMV1311StableReceiptLabels proves the stable reader accepts exactly two
// runner receipt labels: the legacy corvint-test-runner-receipt/0 that every
// committed map carries and the opaque/0 label of an unadmitted receipt, and
// refuses any other label at the same runner-limitation check (CEM-V1-311,
// CEM-V1-313).
func TestCEMV1311StableReceiptLabels(t *testing.T) {
	base := neutralStableMap(t)
	legacy := []byte(`"profile":"` + StableRunnerReceiptProfile + `"`)
	if bytes.Count(base, legacy) != 1 {
		t.Fatalf("public case carries %d legacy runner labels, want 1", bytes.Count(base, legacy))
	}
	for _, c := range []struct {
		label string
		ok    bool
	}{{StableRunnerReceiptProfile, true}, {StableOpaqueReceiptProfile, true}, {"opaque/1", false}, {"opaque", false}, {"", false}, {"corvint-test-runner-receipt/1", false}} {
		raw := bytes.Replace(base, legacy, []byte(`"profile":"`+c.label+`"`), 1)
		d, e := ParseStable(raw)
		if c.ok {
			if e != nil || len(d.Receipts) != 1 || d.Receipts[0].Profile != c.label || d.Receipts[0].ExecutionAuthority != "CALLER_OBSERVED" {
				t.Fatalf("label %q: %v", c.label, e)
			}
			continue
		}
		if e == nil {
			t.Fatalf("label %q was accepted", c.label)
		}
	}
}
