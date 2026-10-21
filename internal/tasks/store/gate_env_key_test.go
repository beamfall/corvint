package store_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0158_ReuseKeyCoversDeclaredEnvironmentValues: a reusable gate
// declaring TCDV0158_TOOLCHAIN passes under one value; a second attempt at
// the same tree under another value executes the gate instead of reusing
// that PASS, and a third under the original value reuses it (TCD-V0-158).
// It sets a process variable, so it does not run in parallel.
func TestTCDV0158_ReuseKeyCoversDeclaredEnvironmentValues(t *testing.T) {
	const name = "TCDV0158_TOOLCHAIN"
	gate := reuseGate("verify", "0", "30", true, true)
	gate.Obj.Set("env", wire.Strings([]string{name}))
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("gates", wire.Array(gate))
	v.Obj.Set("environment", obj("allowedEnvKeys", wire.Strings([]string{name})))
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	if r, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("policy-env", "2", wire.EncodeFile(v)), now(t)); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", r, err)
	}
	claims, _, _ := s.sharedTree(t, "src", "lib", "cmd")
	t.Setenv(name, "go1.27.2")
	s.passes(t, "gate-a", gateOf(claims[0], "verify"), 2)
	t.Setenv(name, "go1.26.0")
	report, err := s.gate(t, "gate-b", gateOf(claims[1], "verify"), 3)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom != nil || s.runs(t, "verify") != 2 {
		t.Fatalf("a PASS under another value was reused: %+v %v (%d runs)", report, err, s.runs(t, "verify"))
	}
	t.Setenv(name, "go1.27.2")
	report, err = s.gate(t, "gate-c", gateOf(claims[2], "verify"), 4)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom == nil || s.runs(t, "verify") != 2 {
		t.Fatalf("a PASS under the same value was not reused: %+v %v (%d runs)", report, err, s.runs(t, "verify"))
	}
}
