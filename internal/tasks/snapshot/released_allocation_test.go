package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-201: releasedPoolAllocation is additive. An attempt without it
// encodes exactly as before; one with it round-trips; a record that still
// holds an allocation, returned it before it was allocated, or is not an
// external-agent attempt refuses MALFORMED, as does the key under a binary
// that predates it (an unknown key).
func TestCALV0201_ReleasedAllocationCodec(t *testing.T) {
	a := accountingAttempt()
	legacy, err := a.Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"releasedPoolAllocation"`)) {
		t.Fatalf("legacy attempt %v %s", err, legacy)
	}
	allocation := PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: wire.Sum([]byte("a")), DefinitionSha256: wire.Sum(nil), AllocatedSeq: "3"}
	a.ReleasedPoolAllocation = &ReleasedPoolAllocation{Allocation: allocation, ReleasedSeq: "4"}
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeAttempt(raw)
	if err != nil || b.ReleasedPoolAllocation == nil || *b.ReleasedPoolAllocation != *a.ReleasedPoolAllocation {
		t.Fatalf("decode %v %+v", err, b)
	}
	if again, _ := b.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("attempt roundtrip")
	}
	for name, bad := range map[string]string{
		"early":   strings.Replace(string(raw), `"releasedSeq":"4"`, `"releasedSeq":"3"`, 1),
		"extra":   strings.Replace(string(raw), `"releasedSeq":"4"`, `"releasedSeq":"4","x":"1"`, 1),
		"unknown": strings.Replace(string(raw), `"releasedPoolAllocation":`, `"releasedPoolAllocationFuture":`, 1),
	} {
		if bad == string(raw) {
			t.Fatalf("%s: substitution missed", name)
		}
		if _, err := DecodeAttempt([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	held, other := *a, *a
	held.PoolAllocation = &allocation
	other.RuntimeID = "supervised"
	for name, x := range map[string]*Attempt{"holding": &held, "non-external": &other} {
		raw, err := x.Encode()
		if err == nil {
			_, err = DecodeAttempt(raw)
		}
		if err == nil {
			t.Fatalf("%s attempt accepted", name)
		}
	}
}
