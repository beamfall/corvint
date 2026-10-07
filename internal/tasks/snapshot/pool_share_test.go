package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const shareSource = "attempt:acme:main:0123456789abcdef0123456789abcdef"

func sharedPoolState(t *testing.T, shared ...PoolShare) *PoolState {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", "queue:acme:main")
	if err != nil {
		t.Fatal(err)
	}
	allocation := PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: wire.Sum([]byte("a")), DefinitionSha256: wire.Sum(nil), AllocatedSeq: "1"}
	return &PoolState{QueueID: q, Entries: []PoolEntry{{PoolAllocation: allocation, State: "ALLOCATED", Holder: "builder", Stage: "implement", AttemptID: shareSource, Generation: "1", ChangedSeq: "2", PolicySha256: wire.Sum(nil), RequestSha256: wire.Sum(nil), Shared: shared}}}
}

// PSR-V0-021: the shared fields are additive. Unshared records encode exactly
// as before; shared ones round-trip; malformed shares refuse; and an unknown
// entry or attempt key refuses MALFORMED, which is how a binary predating
// these keys reads a store that holds them.
func TestPSRV0021_SharedCodec(t *testing.T) {
	legacy, err := sharedPoolState(t).Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"shared"`)) {
		t.Fatalf("legacy pools %v %s", err, legacy)
	}
	raw, err := sharedPoolState(t, PoolShare{AttemptID: testAttempt, Generation: "1"}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePools(raw)
	if err != nil || len(p.Entries[0].Shared) != 1 || p.Entries[0].Shared[0].AttemptID != testAttempt {
		t.Fatalf("decode %v %+v", err, p)
	}
	if again, _ := p.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("pools roundtrip")
	}
	other := "attempt:acme:main:fedcba9876543210fedcba9876543210"
	for name, bad := range map[string]string{
		"empty":     strings.Replace(string(raw), `"shared":[{"attemptId":"`+testAttempt+`","generation":"1"}]`, `"shared":[]`, 1),
		"primary":   strings.Replace(string(raw), `"attemptId":"`+testAttempt+`"`, `"attemptId":"`+shareSource+`"`, 1),
		"duplicate": strings.Replace(string(raw), `{"attemptId":"`+testAttempt+`","generation":"1"}`, `{"attemptId":"`+testAttempt+`","generation":"1"},{"attemptId":"`+testAttempt+`","generation":"1"}`, 1),
		"foreign":   strings.Replace(string(raw), `"attemptId":"`+testAttempt+`"`, `"attemptId":"attempt:other:main:57ddbdeca215924fd0ea543f91045dad"`, 1),
		"extra":     strings.Replace(string(raw), `"generation":"1"}]`, `"generation":"1","x":"1"}]`, 1),
		"state":     strings.Replace(string(raw), `"state":"ALLOCATED"`, `"state":"QUARANTINED"`, 1),
		"unknown":   strings.Replace(string(raw), `"shared":`, `"sharedFuture":`, 1),
	} {
		if bad == string(raw) {
			t.Fatalf("%s: substitution missed", name)
		}
		if _, err := DecodePools([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	over := []PoolShare{}
	for _, id := range []string{testAttempt, other, "attempt:acme:main:1123456789abcdef0123456789abcdef", "attempt:acme:main:2123456789abcdef0123456789abcdef"} {
		over = append(over, PoolShare{AttemptID: id, Generation: "1"})
	}
	if _, err := sharedPoolState(t, over...).Encode(); err == nil {
		t.Fatalf("%d shared attempts accepted", len(over))
	}
	if _, err := sharedPoolState(t, over[:MaxSharedAttempts-1]...).Encode(); err != nil {
		t.Fatalf("%d shared attempts refused: %v", MaxSharedAttempts-1, err)
	}

	a := accountingAttempt()
	a.PoolAllocation = &p.Entries[0].PoolAllocation
	a.SharedAllocation = &SharedAllocation{SourceAttemptID: shareSource, SourceGeneration: "1", BoundSeq: "2"}
	raw, err = a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeAttempt(raw)
	if err != nil || *b.SharedAllocation != *a.SharedAllocation {
		t.Fatalf("attempt decode %v", err)
	}
	if again, _ := b.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("attempt roundtrip")
	}
	if _, err := DecodeAttempt(bytes.Replace(raw, []byte(`"sharedAllocation":`), []byte(`"sharedAllocationFuture":`), 1)); err == nil {
		t.Fatal("unknown attempt key accepted")
	}
	if _, err := DecodeAttempt(bytes.Replace(raw, []byte(`"boundSeq":"2"`), []byte(`"boundSeq":"2","x":"1"`), 1)); err == nil {
		t.Fatal("open sharedAllocation accepted")
	}
	for name, change := range map[string]func(*Attempt){
		"allocation": func(a *Attempt) { a.PoolAllocation = nil },
		"direct":     func(a *Attempt) { a.DirectPoolAdmission = &DirectPoolAdmission{} },
		"self":       func(a *Attempt) { a.SharedAllocation.SourceAttemptID = a.AttemptID },
		"seq":        func(a *Attempt) { a.SharedAllocation.BoundSeq = "0" },
		"foreign": func(a *Attempt) {
			a.SharedAllocation.SourceAttemptID = "attempt:other:main:57ddbdeca215924fd0ea543f91045dad"
		},
	} {
		x := accountingAttempt()
		x.PoolAllocation = &p.Entries[0].PoolAllocation
		x.SharedAllocation = &SharedAllocation{SourceAttemptID: shareSource, SourceGeneration: "1", BoundSeq: "2"}
		change(x)
		if _, err := x.Encode(); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
