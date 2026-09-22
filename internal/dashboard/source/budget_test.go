package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBudgetPreflightIsKeyedMonotonicAcrossAttempts(t *testing.T) {
	budget := NewBudget(12)
	assertBudgetCode(t, budget.Preflight("local-trace-v1", 7, 4), BudgetOK)
	assertBudgetCode(t, budget.Preflight("local-trace-v1", 7, 4), BudgetOK)
	assertBudgetCode(t, budget.Preflight("local-trace-v1", 7, 3), BudgetOK)
	if budget.Used() != 4 || budget.count != 1 {
		t.Fatalf("same-key retry double charged: used=%d count=%d", budget.Used(), budget.count)
	}
	assertBudgetCode(t, budget.Preflight("local-trace-v1", 7, 9), BudgetOK)
	if budget.Used() != 9 || budget.count != 1 {
		t.Fatalf("same-key growth was not delta charged: used=%d count=%d", budget.Used(), budget.count)
	}
	assertBudgetCode(t, budget.Preflight("local-trace-v1", 8, 4), BudgetLogicalExhausted)
	if budget.Used() != 9 || budget.count != 1 {
		t.Fatalf("rejected preflight mutated ledger: used=%d count=%d", budget.Used(), budget.count)
	}
}

func TestBudgetPreflightSeparatesAdapterAndOrdinal(t *testing.T) {
	budget := NewBudget(8)
	for _, input := range []struct {
		adapter string
		ordinal uint64
	}{
		{adapter: "local-trace-v1", ordinal: 0},
		{adapter: "local-trace-v1", ordinal: 1},
		{adapter: "stable-read-v0", ordinal: 0},
	} {
		assertBudgetCode(t, budget.Preflight(input.adapter, input.ordinal, 2), BudgetOK)
	}
	if budget.Used() != 6 || budget.count != 3 {
		t.Fatalf("logical keys collapsed: used=%d count=%d", budget.Used(), budget.count)
	}
	assertBudgetCode(t, budget.Preflight("", 0, 1), BudgetInvalid)
}

func TestBudgetPreflightEnforcesWholeInvocationKeyBound(t *testing.T) {
	budget := NewBudget(MaxAggregateBytes)
	for ordinal := uint64(0); ordinal < MaxArtifactCount; ordinal++ {
		assertBudgetCode(t, budget.Preflight("local-trace-v1", ordinal, 0), BudgetOK)
	}
	assertBudgetCode(t, budget.Preflight("local-trace-v1", MaxArtifactCount, 0), BudgetLogicalExhausted)
	if budget.count != MaxArtifactCount || budget.Used() != 0 {
		t.Fatalf("key bound ledger: count=%d used=%d", budget.count, budget.Used())
	}
}

func TestBudgetPhysicalLedgerIsMonotonicAndClosed(t *testing.T) {
	if MaxPhysicalBytes != 1_073_741_824 {
		t.Fatalf("physical bound = %d", MaxPhysicalBytes)
	}
	budget := NewBudget(1)
	budget.physicalLimit = 5
	assertBudgetCode(t, budget.ChargePhysical(2), BudgetOK)
	assertBudgetCode(t, budget.ChargePhysical(3), BudgetOK)
	assertBudgetCode(t, budget.ChargePhysical(1), BudgetPhysicalExhausted)
	if budget.PhysicalUsed() != 5 {
		t.Fatalf("rejected physical charge mutated ledger: %d", budget.PhysicalUsed())
	}
	assertBudgetCode(t, (*Budget)(nil).ChargePhysical(1), BudgetInvalid)
}

func TestReadOrdinalSharesLogicalKeyAndChargesEveryPhysicalPass(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("four"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)
	budget := NewBudget(MaxAggregateBytes)
	for attempt := 0; attempt < 2; attempt++ {
		got := ReadOrdinal(root, "source", 11, KindLocalTrace, VerifierLocalTraceV1, budget, nil)
		if got.Validity != ValidityStable {
			t.Fatalf("attempt %d = %#v", attempt, got)
		}
	}
	if budget.Used() != MaxSourceBytes || budget.count != 1 {
		t.Fatalf("outer retry double charged logical key: used=%d count=%d", budget.Used(), budget.count)
	}
	if budget.PhysicalUsed() != 16 {
		t.Fatalf("physical work = %d, want both reads across both attempts", budget.PhysicalUsed())
	}
}

func TestPhysicalBoundaryChargesRejectedSecondRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("ab"), 0o600); err != nil {
		t.Fatal(err)
	}
	budget := NewBudget(MaxAggregateBytes)
	budget.physicalLimit = 3
	got := Read(mustOpenRoot(t, dir), "source", KindLocalTrace, VerifierLocalTraceV1, budget, nil)
	assertFailure(t, got, IssueAggregateBudgetExceeded)
	key := overflowProbeKey{source: budgetKey{adapterID: "local-trace-v1", configuredOrdinal: 0}}
	if budget.PhysicalUsed() != 3 || budget.overflowProbes[key] != 1 {
		t.Fatalf("physical boundary ledger: used=%d probes=%d", budget.PhysicalUsed(), budget.overflowProbes[key])
	}
}

func TestOverflowProbeAllowanceIsKeyedAndCappedPerWholeScanAttempt(t *testing.T) {
	invocation := NewBudget(MaxAggregateBytes)
	invocation.physicalLimit = 8
	budget := invocation.ForWholeScanAttempt()
	store := budgetKey{adapterID: "local-trace-v1", configuredOrdinal: 9}
	key := overflowProbeKey{source: store, member: "a.jsonl", storeAttempt: 0}
	for attempt := 0; attempt < 2; attempt++ {
		decision, overflow := budget.chargeBoundedRead(key, 2, 1)
		if !decision.Allowed() || !overflow {
			t.Fatalf("attempt %d = code %d overflow=%t", attempt, decision.Code(), overflow)
		}
	}
	decision, overflow := budget.chargeBoundedRead(key, 2, 1)
	if decision.Code() != BudgetPhysicalExhausted || overflow {
		t.Fatalf("third probe = code %d overflow=%t", decision.Code(), overflow)
	}
	for _, other := range []overflowProbeKey{
		{source: store, member: "b.jsonl", storeAttempt: 0},
		{source: store, member: "a.jsonl", storeAttempt: 1},
		{source: budgetKey{adapterID: "local-trace-v1", configuredOrdinal: 10}},
	} {
		decision, overflow = budget.chargeBoundedRead(other, 2, 1)
		if !decision.Allowed() || !overflow {
			t.Fatalf("other key %#v = code %d overflow=%t", other, decision.Code(), overflow)
		}
	}
	retryBudget := invocation.ForWholeScanAttempt()
	for attempt := 0; attempt < 2; attempt++ {
		decision, overflow = retryBudget.chargeBoundedRead(key, 2, 1)
		if !decision.Allowed() || !overflow {
			t.Fatalf("whole-scan retry attempt %d = code %d overflow=%t", attempt, decision.Code(), overflow)
		}
	}
	decision, overflow = retryBudget.chargeBoundedRead(key, 2, 1)
	if decision.Code() != BudgetPhysicalExhausted || overflow {
		t.Fatalf("whole-scan retry third probe = code %d overflow=%t", decision.Code(), overflow)
	}
	if budget.PhysicalUsed() != 7 || retryBudget.PhysicalUsed() != 7 {
		t.Fatalf("cumulative physical bytes=%d/%d, want 7", budget.PhysicalUsed(), retryBudget.PhysicalUsed())
	}
}

func TestUnstableAttemptPhysicalBytesAreNotRefunded(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "source")
	if err := os.WriteFile(filePath, []byte("aaaa"), 0o600); err != nil {
		t.Fatal(err)
	}
	budget := NewBudget(MaxAggregateBytes)
	got := readOrdinal(
		mustOpenRoot(t, dir), "source", 3, KindLocalTrace, VerifierLocalTraceV1,
		budget, ProcessClock(), nil, acquisitionHooks{afterStat3: func(attempt int) {
			if attempt == 0 {
				if err := os.WriteFile(filePath, []byte("bbbb"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}},
	)
	if got.Validity != ValidityStable || got.Bytes != 4 {
		t.Fatalf("retry result = %#v", got)
	}
	if budget.PhysicalUsed() != 16 {
		t.Fatalf("unstable attempt was refunded: physical=%d, want 16", budget.PhysicalUsed())
	}
}

func TestWindowsPlatformIdentityCombinesExactFields(t *testing.T) {
	identity, ok := windowsPlatformIdentity(0x12345678, 3, 0x9abcdef0, 0x13572468)
	if !ok {
		t.Fatal("qualified Windows identity rejected")
	}
	if identity.Kind != "WINDOWS" || identity.VolumeSerialNumber != 0x12345678 || identity.LinkCount != 3 || identity.FileIndex != 0x9abcdef013572468 {
		t.Fatalf("Windows identity = %#v", identity)
	}
	if _, ok := windowsPlatformIdentity(1, 0, 2, 3); ok {
		t.Fatal("zero-link Windows identity qualified")
	}
}

func assertBudgetCode(t *testing.T, result BudgetResult, want BudgetCode) {
	t.Helper()
	if result.Code() != want || result.Allowed() != (want == BudgetOK) {
		t.Fatalf("budget result = %d allowed=%t, want %d", result.Code(), result.Allowed(), want)
	}
}
