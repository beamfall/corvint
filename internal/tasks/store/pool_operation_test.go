//go:build unix

package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const operationTarget = "2222222222222222222222222222222222222222:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// operationPolicy installs pool db whose member a declares ops, each a /bin/sh
// script with the given timeout; member b declares no operation.
func operationPolicy(t *testing.T, s *leaseStore, timeout string, ops map[string]string) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	declared := wire.NewObject()
	for name, script := range ops {
		declared.Set(name, obj("argv", wire.Strings([]string{"/bin/sh", "-c", script}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str(timeout)))
	}
	cfg := obj("a", obj("operations", wire.ObjectValue(declared)))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a", "b"}), "memberConfig", cfg)))
	if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("operation-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
}

func operate(t *testing.T, s *leaseStore, id, member, op, source string) (*store.Report, error) {
	t.Helper()
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, Lease: transaction.LeaseRequest{Pool: "db", Member: member, Operation: op, Source: source}}
	return store.PoolOperate(psrLoaded(context.Background()), s.repo, operator(), choice)
}

func operationEntry(t *testing.T, s *leaseStore, member string) *snapshot.PoolEntry {
	t.Helper()
	for _, en := range psrPools(t, s).Entries {
		if en.MemberID == member {
			x := en
			return &x
		}
	}
	return nil
}

// PSR-V0-160: an undeclared operation is refused with a typed reason; the
// policy parser rejects an unknown operation key and an empty declaration.
func TestPSRV0160_UndeclaredOperationRefused(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	operationPolicy(t, s, "5", map[string]string{"provision": "printf x >> " + marker})
	for _, c := range []struct{ member, op string }{{"a", "teardown"}, {"b", "provision"}} {
		rep, e := operate(t, s, "undeclared-"+c.member, c.member, c.op, "")
		if e != nil || rep.Outcome.Outcome != mutation.OutcomeBlocked || !strings.HasPrefix(rep.Detail, transaction.ReasonOperationUndeclared+":") || !strings.Contains(rep.Detail, c.op) {
			t.Fatalf("%s/%s not refused typed: %+v %v", c.member, c.op, rep, e)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a refused operation ran a program: %v", err)
	}
	if _, e := operate(t, s, "unknown-op", "a", "rebuild", ""); wire.CodeOf(e) != wire.CodeMalformed {
		t.Fatalf("unknown operation name: %v", e)
	}
	for name, ops := range map[string]wire.Value{"unknown key": obj("rebuild", obj("argv", wire.Strings([]string{"/bin/true"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("5"))), "empty": wire.ObjectValue(wire.NewObject())} {
		v := fixture.PolicyValue()
		v.Obj.Set("policyVersion", str("4"))
		v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
		budgets, _ := v.Obj.Get("budgets")
		budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
		v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("operations", ops)))))
		if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("bad-ops-"+strings.ReplaceAll(name, " ", "-"), "3", wire.EncodeFile(v)), now(t)); e == nil && rep.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("%s operations admitted", name)
		}
	}
}

// PSR-V0-161: an operation started from an illegal member state is refused
// with a typed reason naming the operation, and nothing runs.
func TestPSRV0161_IllegalStateRefusedNothingRuns(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	operationPolicy(t, s, "5", map[string]string{"reset": "printf x >> " + marker, "provision": "exit 3", "teardown": "printf x >> " + marker})
	rep, e := operate(t, s, "reset-free", "a", "reset", "")
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeBlocked || !strings.HasPrefix(rep.Detail, transaction.ReasonOperationStateIllegal+": reset requires QUARANTINED") {
		t.Fatalf("reset of a FREE member: %+v %v", rep, e)
	}
	// A failed provision quarantines a; teardown then needs FREE.
	if rep, e = operate(t, s, "provision-fail", "a", "provision", ""); e != nil || rep.PoolOperation == nil || rep.PoolOperation.Passed {
		t.Fatalf("failing provision: %+v %v", rep, e)
	}
	rep, e = operate(t, s, "teardown-quarantined", "a", "teardown", "")
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeBlocked || !strings.Contains(rep.Detail, "member a is QUARANTINED") {
		t.Fatalf("teardown of a QUARANTINED member: %+v %v", rep, e)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a refused operation ran a program: %v", err)
	}
}

// PSR-V0-162, PSR-V0-163: a run records operation, request id, exit class and
// output digest; a passed provision leaves the member FREE.
func TestPSRV0162_ReceiptRecordedMemberFree(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	operationPolicy(t, s, "5", map[string]string{"provision": "printf provisioned"})
	rep, e := operate(t, s, "provision-1", "a", "provision", "")
	if e != nil || rep.PoolOperation == nil {
		t.Fatalf("provision: %+v %v", rep, e)
	}
	r := rep.PoolOperation
	if r.Operation != "provision" || r.RequestID != "provision-1" || r.Class != "EXIT_ZERO" || !r.Passed || r.OutputSha256 == nil || r.Source != nil {
		t.Fatalf("receipt %+v", r)
	}
	if operationEntry(t, s, "a") != nil {
		t.Fatal("passed provision left an occupancy entry")
	}
	state := psrPools(t, s)
	if got := state.PoolOperationReceiptFor("a", "provision"); got == nil || got.RequestID != "provision-1" {
		t.Fatalf("pools.json receipt %+v", state.Operations)
	}
	if raw, e := state.Encode(); e != nil {
		t.Fatalf("encode: %v", e)
	} else if back, e := snapshot.DecodePools(raw); e != nil || len(back.Operations) != 1 {
		t.Fatalf("pools.json with receipts does not round-trip: %v", e)
	}
}

// PSR-V0-163: a passed operation that started from QUARANTINED leaves the
// member QUARANTINED for cleanup and safe-reuse confirmation; an actor
// without operator attestation is refused and nothing runs.
func TestPSRV0163_PassedResetStaysQuarantined(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	operationPolicy(t, s, "5", map[string]string{"refresh": "exit 1", "reset": "printf x >> " + marker})
	if rep, e := operate(t, s, "refresh-fail", "a", "refresh", ""); e != nil || rep.PoolOperation == nil {
		t.Fatalf("refresh: %+v %v", rep, e)
	}
	worker := mutation.Binding{ID: "agent", Role: "WORKER"}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "reset-worker", Root: s.root, Lease: transaction.LeaseRequest{Pool: "db", Member: "a", Operation: "reset"}}
	if rep, e := store.PoolOperate(psrLoaded(context.Background()), s.repo, worker, choice); e == nil && rep.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("worker reset admitted: %+v", rep)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a refused reset ran: %v", err)
	}
	rep, e := operate(t, s, "reset-pass", "a", "reset", "")
	if e != nil || rep.PoolOperation == nil || !rep.PoolOperation.Passed {
		t.Fatalf("reset: %+v %v", rep, e)
	}
	en := operationEntry(t, s, "a")
	if en == nil || en.State != "QUARANTINED" || en.CleanupPassed || !strings.HasPrefix(en.Reason, "reset EXIT_ZERO; operator safe-reuse confirmation required") {
		t.Fatalf("passed reset left %+v", en)
	}
}

// PSR-V0-164: a same request id replays the recorded outcome and never runs
// the command twice, including after a fault between commit and execution.
func TestPSRV0164_SameRequestReplaysNeverRunsTwice(t *testing.T) {
	// Not parallel: the prepared-fault hook is package-global.
	s := newLeaseStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	operationPolicy(t, s, "5", map[string]string{"provision": "printf x >> " + marker, "refresh": "printf x >> " + marker})
	first, e := operate(t, s, "once", "a", "provision", "")
	if e != nil || first.PoolOperation == nil {
		t.Fatalf("first: %+v %v", first, e)
	}
	replay, e := operate(t, s, "once", "a", "provision", "")
	if e != nil || replay.Kind != "Replay" || replay.PoolOperation == nil || replay.PoolOperation.RequestID != "once" {
		t.Fatalf("replay: %+v %v", replay, e)
	}
	if raw, _ := os.ReadFile(marker); string(raw) != "x" {
		t.Fatalf("command ran %d times", len(raw))
	}
	restore := store.SetPoolPreparedFaultForTest(func() error {
		return wire.Errorf(wire.CodeRedoPending, "head", "injected post-preparation failure")
	})
	rep, e := operate(t, s, "faulted", "a", "refresh", "")
	restore()
	if wire.CodeOf(e) != wire.CodeRedoPending || rep == nil || !rep.Unretryable {
		t.Fatalf("committed operation reported retryable: %+v %v", rep, e)
	}
	again, e := operate(t, s, "faulted", "a", "refresh", "")
	if e != nil || again.Kind != "Replay" || again.PoolOperation != nil || !strings.HasPrefix(again.Detail, "operation pending") {
		t.Fatalf("pending replay: %+v %v", again, e)
	}
	if raw, _ := os.ReadFile(marker); string(raw) != "x" {
		t.Fatalf("a faulted or replayed operation ran: %q", raw)
	}
}

// PSR-V0-165: a failed reset or retarget leaves the member QUARANTINED with
// the operation named in the reason; retarget receives its target pair.
func TestPSRV0165_FailedResetRetargetQuarantine(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	seen := filepath.Join(t.TempDir(), "target")
	operationPolicy(t, s, "5", map[string]string{"refresh": "exit 1", "reset": "exit 4", "retarget": "printf %s \"$" + store.PoolTargetSourceEnv + "\" > " + seen + "; exit 5"})
	if rep, e := operate(t, s, "refresh-fail", "a", "refresh", ""); e != nil || rep.PoolOperation == nil || rep.PoolOperation.Class != "EXIT_NONZERO" {
		t.Fatalf("refresh: %+v %v", rep, e)
	}
	for _, c := range []struct{ op, source string }{{"reset", ""}, {"retarget", operationTarget}} {
		rep, e := operate(t, s, c.op+"-fail", "a", c.op, c.source)
		if e != nil || rep.PoolOperation == nil || rep.PoolOperation.Passed || rep.PoolOperation.Class != "EXIT_NONZERO" {
			t.Fatalf("%s: %+v %v", c.op, rep, e)
		}
		en := operationEntry(t, s, "a")
		if en == nil || en.State != "QUARANTINED" || !strings.HasPrefix(en.Reason, c.op+" EXIT_NONZERO") || en.Operation != nil {
			t.Fatalf("%s left %+v", c.op, en)
		}
	}
	if raw, _ := os.ReadFile(seen); string(raw) != operationTarget {
		t.Fatalf("retarget saw %q", raw)
	}
	if r := psrPools(t, s).PoolOperationReceiptFor("a", "retarget"); r == nil || r.Source == nil || r.Source.String() != operationTarget {
		t.Fatalf("retarget receipt %+v", r)
	}
	if _, e := operate(t, s, "retarget-nosource", "a", "retarget", ""); wire.CodeOf(e) != wire.CodeMalformed {
		t.Fatalf("retarget without a source: %v", e)
	}
}

// PSR-V0-166: a timed-out operation's process group is killed, and output
// past the bound is OUTPUT_LIMIT.
func TestPSRV0166_TimeoutKillsGroupOutputBounded(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	pidfile := filepath.Join(t.TempDir(), "pid")
	operationPolicy(t, s, "1", map[string]string{"provision": "/bin/sleep 60 & echo $! > " + pidfile + "; wait", "refresh": "/usr/bin/head -c 200000 /dev/zero"})
	rep, e := operate(t, s, "slow", "a", "provision", "")
	if e != nil || rep.PoolOperation == nil || rep.PoolOperation.Class != "TIMEOUT" {
		t.Fatalf("timeout: %+v %v", rep, e)
	}
	raw, err := os.ReadFile(pidfile)
	pid, perr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || perr != nil {
		t.Fatalf("pidfile %q %v %v", raw, err, perr)
	}
	if syscall.Kill(pid, 0) == nil {
		t.Fatalf("background child %d survived the timeout", pid)
	}
	if en := operationEntry(t, s, "a"); en == nil || en.State != "QUARANTINED" || !strings.HasPrefix(en.Reason, "provision TIMEOUT") {
		t.Fatalf("timed-out provision left %+v", en)
	}
	rep, e = operate(t, s, "loud", "a", "refresh", "")
	if e != nil || rep.PoolOperation == nil || rep.PoolOperation.Class != "OUTPUT_LIMIT" {
		t.Fatalf("output bound: %+v %v", rep, e)
	}
}
