package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-162, PSR-V0-167: the operation fields are additive. A state without
// them encodes exactly as before; a pending operation and receipts round-trip;
// malformed receipts and a pending operation on the wrong entry refuse.
func TestPSRV0162_OperationCodec(t *testing.T) {
	legacy, err := sharedPoolState(t).Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"operation`)) {
		t.Fatalf("legacy pools %v %s", err, legacy)
	}
	src, err := intent.ParsePoolSource("source", strings.Repeat("1", 40)+":"+strings.Repeat("2", 40))
	if err != nil {
		t.Fatal(err)
	}
	st := sharedPoolState(t)
	pending := st.Entries[0]
	pending.MemberID = "b"
	pending.AllocationID = wire.Sum([]byte("b"))
	pending.State = "CLEANING"
	pending.AttemptID = ""
	pending.Generation = "0"
	pending.Stage = ""
	pending.RunnerPID = "42"
	pending.RunnerStarted = "started"
	pending.CommandKind = "retarget"
	pending.CommandRevision = strings.Repeat("a", 40)
	pending.Operation = &PoolOperationPending{Name: "retarget", RequestID: "rt-1", Source: &src}
	st.Entries = append(st.Entries, pending)
	out := wire.Sum([]byte("out"))
	st.Operations = []PoolOperationReceipt{
		{PoolID: "db", MemberID: "b", Operation: "reset", RequestID: "r-1", AllocationID: pending.AllocationID, Class: "EXIT_NONZERO", OutputSha256: &out, ObservedSeq: "3"},
		{PoolID: "db", MemberID: "a", Operation: "provision", RequestID: "p-1", AllocationID: pending.AllocationID, Class: "EXIT_ZERO", Passed: true, OutputSha256: &out, ObservedSeq: "2"},
		{PoolID: "db", MemberID: "b", Operation: "refresh", RequestID: "f-1", AllocationID: pending.AllocationID, Class: PoolOperationOrphaned, ObservedSeq: "4"},
	}
	raw, err := st.Encode()
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePools(raw)
	if err != nil || len(p.Operations) != 3 || p.Operations[0].MemberID != "a" || p.Operations[1].Operation != "refresh" || p.Entries[1].Operation == nil || p.Entries[1].Operation.Source.String() != src.String() {
		t.Fatalf("decode %v %+v", err, p)
	}
	if again, _ := p.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("pools roundtrip")
	}
	if r := p.PoolOperationReceiptFor("b", "reset"); r == nil || r.RequestID != "r-1" {
		t.Fatalf("receipt lookup %+v", r)
	}
	for name, bad := range map[string]string{
		"orphan-digest":  strings.Replace(string(raw), `"class":"ORPHANED"`, `"class":"EXIT_NONZERO"`, 1),
		"passed-nonzero": strings.Replace(string(raw), `"class":"EXIT_ZERO"`, `"class":"TIMEOUT"`, 1),
		"class":          strings.Replace(string(raw), `"class":"EXIT_NONZERO"`, `"class":"EXIT_LATER"`, 1),
		"operation":      strings.Replace(string(raw), `"operation":"reset"`, `"operation":"rebuild"`, 1),
		"unsorted":       strings.Replace(string(raw), `"memberId":"a","observedSeq":"2","operation":"provision"`, `"memberId":"c","observedSeq":"2","operation":"provision"`, 1),
		"duplicate":      strings.Replace(string(raw), `"operation":"refresh"`, `"operation":"reset"`, 1),
		"profile":        strings.Replace(string(raw), `"profile":"taskman-pool-operation/0"`, `"profile":"taskman-pool-operation/1"`, 1),
		"empty":          string(raw[:bytes.Index(raw, []byte(`"operations":`))]) + `"operations":[]` + string(raw[bytes.LastIndex(raw, []byte(`],`))+1:]),
		"pending-kind":   strings.Replace(string(raw), `"commandKind":"retarget"`, `"commandKind":"reset"`, 1),
		"pending-source": strings.Replace(string(raw), `"name":"retarget"`, `"name":"reset"`, 1),
		"pending-extra":  strings.Replace(string(raw), `"name":"retarget"`, `"name":"retarget","x":"1"`, 1),
	} {
		if bad == string(raw) {
			t.Fatalf("%s: substitution missed", name)
		}
		if _, err := DecodePools([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	allocated := strings.Replace(string(raw), `"state":"CLEANING"`, `"state":"ALLOCATED"`, 1)
	if _, err := DecodePools([]byte(allocated)); err == nil {
		t.Fatal("pending operation on an ALLOCATED entry accepted")
	}
}
