package snapshot

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func recoveryEntry(member, state string) PoolEntry {
	allocation := PoolAllocation{PoolID: "db", MemberID: member, AllocationID: wire.Sum([]byte(member)), DefinitionSha256: wire.Sum(nil), AllocatedSeq: "1"}
	return PoolEntry{PoolAllocation: allocation, State: state, Holder: "builder", Generation: "1", ChangedSeq: "2", PolicySha256: wire.Sum(nil), RequestSha256: wire.Sum(nil)}
}

func recoveryState(t *testing.T, entries ...PoolEntry) *PoolState {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", "queue:acme:main")
	if err != nil {
		t.Fatal(err)
	}
	return &PoolState{QueueID: q, Entries: entries}
}

func failedRecovery(n string) *PoolRecovery {
	return &PoolRecovery{Kind: "cleanup", Result: "FAILED", DetailSha256: wire.Sum([]byte("detail")), Consecutive: wire.Count(n), FirstObservedAt: "2026-10-25T08:00:00Z", ObservedAt: "2026-10-25T09:00:00Z", ObservedSeq: "7"}
}

// PSR-V0-261, PSR-V0-266: quarantinedAt and recovery are additive keys. A
// legacy entry encodes without them, byte-identical; recorded values round
// trip; they bind only a QUARANTINED or CLEANING entry; and a malformed
// recovery record refuses.
func TestPSRV0261_QuarantineRecoveryCodec(t *testing.T) {
	legacy, err := recoveryState(t, recoveryEntry("a", "QUARANTINED")).Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"quarantinedAt"`)) || bytes.Contains(legacy, []byte(`"recovery"`)) {
		t.Fatalf("legacy pools %v %s", err, legacy)
	}
	if st, err := DecodePools(legacy); err != nil || st.Entries[0].QuarantinedAt != "" || st.Entries[0].Recovery != nil {
		t.Fatalf("legacy decode %v", err)
	}
	en := recoveryEntry("a", "QUARANTINED")
	en.QuarantinedAt, en.Recovery = "2026-10-25T07:00:00Z", failedRecovery("2")
	raw, err := recoveryState(t, en).Encode()
	if err != nil {
		t.Fatal(err)
	}
	st, err := DecodePools(raw)
	if err != nil || st.Entries[0].QuarantinedAt != en.QuarantinedAt || *st.Entries[0].Recovery != *en.Recovery {
		t.Fatalf("roundtrip %v", err)
	}
	if again, _ := st.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("roundtrip is not byte-identical")
	}
	bad := strings.Replace(string(raw), `"state":"QUARANTINED"`, `"state":"ALLOCATED"`, 1)
	bad = strings.Replace(bad, `"attemptId":""`, `"attemptId":"attempt:acme:main:0123456789abcdef0123456789abcdef"`, 1)
	if _, err := DecodePools([]byte(bad)); wire.CodeOf(err) != wire.CodeMalformed || !strings.Contains(err.Error(), "quarantinedAt") {
		t.Fatalf("ALLOCATED with quarantinedAt: %v", err)
	}
	for name, x := range map[string]*PoolRecovery{
		"zero count":     failedRecovery("0"),
		"observed early": {Kind: "health", Result: "FAILED", DetailSha256: wire.Sum(nil), Consecutive: "1", FirstObservedAt: "2026-10-25T09:00:00Z", ObservedAt: "2026-10-25T08:00:00Z", ObservedSeq: "3"},
		"zero seq":       {Kind: "health", Result: "FAILED", DetailSha256: wire.Sum(nil), Consecutive: "1", FirstObservedAt: "2026-10-25T09:00:00Z", ObservedAt: "2026-10-25T09:00:00Z", ObservedSeq: "0"},
	} {
		en := recoveryEntry("a", "QUARANTINED")
		en.Recovery = x
		// Encode re-reads its own bytes, so it refuses what decode would.
		if _, err := recoveryState(t, en).Encode(); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// PSR-V0-262, PSR-V0-263: a QUARANTINED member stalls after three
// consecutive identical failures or 24 hours after a recorded quarantinedAt;
// a legacy entry without one never ages; other states never stall; and a
// pool is stalled only when every named member is.
func TestPSRV0262_RecoveryStallRule(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-25T10:00:00Z")
	counted := recoveryEntry("a", "QUARANTINED")
	counted.Recovery = failedRecovery("3")
	if trigger, ok := PoolRecoveryStall(counted, now); !ok || trigger != PoolStallConsecutiveFailures {
		t.Fatalf("three failures: %q %t", trigger, ok)
	}
	two := recoveryEntry("a", "QUARANTINED")
	two.Recovery = failedRecovery("2")
	if _, ok := PoolRecoveryStall(two, now); ok {
		t.Fatal("two failures stalled")
	}
	aged := recoveryEntry("b", "QUARANTINED")
	aged.QuarantinedAt = "2026-10-24T10:00:00Z"
	if trigger, ok := PoolRecoveryStall(aged, now); !ok || trigger != PoolStallQuarantineAge {
		t.Fatalf("24h quarantine: %q %t", trigger, ok)
	}
	young := aged
	young.QuarantinedAt = "2026-10-24T10:00:01Z"
	if _, ok := PoolRecoveryStall(young, now); ok {
		t.Fatal("young quarantine stalled")
	}
	legacy := recoveryEntry("c", "QUARANTINED")
	if _, ok := PoolRecoveryStall(legacy, now.Add(365*24*time.Hour)); ok {
		t.Fatal("a legacy quarantine without quarantinedAt aged")
	}
	cleaning := counted
	cleaning.State = "CLEANING"
	if _, ok := PoolRecoveryStall(cleaning, now); ok {
		t.Fatal("CLEANING stalled")
	}
	st := recoveryState(t, counted, aged, legacy)
	if !st.MembersStalled("db", []string{"a", "b"}, now) {
		t.Fatal("all stalled members not reported")
	}
	if st.MembersStalled("db", []string{"a", "b", "c"}, now) || st.MembersStalled("db", []string{"a", "free"}, now) || st.MembersStalled("db", nil, now) || st.MembersStalled("other", []string{"a"}, now) {
		t.Fatal("partial stall reported as pool stall")
	}
	var none *PoolState
	if none.MembersStalled("db", []string{"a"}, now) {
		t.Fatal("nil state stalled")
	}
}
