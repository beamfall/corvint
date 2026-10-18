//go:build unix

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-160, PSR-V0-162, PSR-V0-164, PSR-V0-167: `pool operate` runs a
// declared operation once and reports its receipt, a same request id replays
// it, an undeclared operation is refused with its typed reason, and `pool
// status` shows the last receipt per declared operation only for members that
// declare operations.
func TestPSRV0162_PoolOperateCLIAndStatus(t *testing.T) {
	r := exclusionCLIRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	command := func(script string) wire.Value {
		return wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/bin/sh", "-c", script})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("5")))
	}
	ops := wire.NewObject().Set("provision", command("printf x >> "+marker)).Set("refresh", command("exit 0"))
	cfg := wire.NewObject().Set("a", wire.ObjectValue(wire.NewObject().Set("operations", wire.ObjectValue(ops))))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b"})).Set("memberConfig", wire.ObjectValue(cfg)))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init %s", x.stdout)
	}
	a := identityStatusMember(t, r, "a")
	if ops := field(a, "operations"); field(ops, "provision").Str != "NOT_OBSERVED" || field(ops, "refresh").Str != "NOT_OBSERVED" {
		t.Fatalf("unobserved operations %s", wire.Encode(a))
	}
	if s := atm(t, r.Root, nil, "pool", "status", "--member", "b"); bytes.Contains(s.stdout, []byte(`"operations"`)) {
		t.Fatalf("member without operations gained a key: %s", s.stdout)
	}
	argv := []string{"pool", "operate", "--pool", "db", "--member", "a", "--operation", "provision", "--request-id", "op-1"}
	first := atm(t, r.Root, nil, argv...)
	if first.res.Outcome != wire.OutcomeOK || field(field(first.res.Items[0], "operation"), "class").Str != "EXIT_ZERO" {
		t.Fatalf("operate %s", first.stdout)
	}
	again := atm(t, r.Root, nil, argv...)
	if again.res.Outcome != wire.OutcomeOK || field(field(again.res.Items[0], "operation"), "requestId").Str != "op-1" {
		t.Fatalf("replay %s", again.stdout)
	}
	if raw, _ := os.ReadFile(marker); string(raw) != "x" {
		t.Fatalf("operation ran %d times", len(raw))
	}
	a = identityStatusMember(t, r, "a")
	if ops := field(a, "operations"); field(field(ops, "provision"), "requestId").Str != "op-1" || field(ops, "refresh").Str != "NOT_OBSERVED" {
		t.Fatalf("status after operate %s", wire.Encode(a))
	}
	undeclared := atm(t, r.Root, nil, "pool", "operate", "--pool", "db", "--member", "b", "--operation", "provision", "--request-id", "op-b")
	if undeclared.res.Outcome == wire.OutcomeOK || !bytes.Contains(undeclared.stdout, []byte(transaction.ReasonOperationUndeclared)) {
		t.Fatalf("undeclared operation %s", undeclared.stdout)
	}
}
