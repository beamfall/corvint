//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-265 (V1-2636 AC3): against a real store whose only pool member is
// QUARANTINED with a stalled recovery, the native observation marks the
// pool stalled, and `dispatch --once` starts no attempt for a ticket that
// needs the pool, reporting POOL_MEMBER_RECOVERY_STALLED with the pool
// status route, while a lane-free ticket still launches.
func TestPSRV0265_DispatchHoldsStalledPoolLaunchesCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a"})))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	ok := func(args ...string) run {
		t.Helper()
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s %s", args, x.stdout, x.stderr)
		}
		return x
	}
	ok("init")
	poolTicket := func(title, priority string) string {
		t.Helper()
		payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"`+priority+`"`, `"touchPaths":[]`, `"touchPaths":["`+title+`"]`, `"title":"Console ticket"`, `"title":"`+title+`"`, `"requirementRefs":[],`, `"requirementRefs":[],"requiresPool":"lanes",`).Replace(createPayloadJSON)
		return field(ok("ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload).res.Items[0], "ticketId").Str
	}
	// Member a ends QUARANTINED after its attempt, then fails the same
	// health check three times.
	first := poolTicket("first", "P1")
	claimed := ok("claim", first, "--holder", "builder", "--request-id", "claim-first", "--pool", "lanes", "--stage", "implement").res.Items[0]
	ok("release", "--attempt", field(claimed, "attemptId").Str, "--generation", field(claimed, "generation").Str, "--request-id", "release-first")
	detail := filepath.Join(resolvedTempDir(t), "health.log")
	if err := os.WriteFile(detail, []byte("probe timed out\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stalled := func() bool {
		t.Helper()
		obs, err := cli.ObserveDispatch(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		return obs.PoolStalled["lanes"]
	}
	for i := 1; i <= 3; i++ {
		if stalled() {
			t.Fatalf("pool observed stalled after %d failures", i-1)
		}
		ok("pool", "observe", "--member", "a", "--kind", "health", "--result", "FAILED", "--detail-file", detail, "--request-id", "obs-"+strconv.Itoa(i))
	}
	if !stalled() {
		t.Fatal("the native observation does not mark the stalled pool")
	}
	pooled := poolTicket("pooled", "P0")
	free := planTicket(t, r.Root, "free", "P2", `["free"]`)
	local := func(id string) string { return id[strings.LastIndexByte(id, ':')+1:] }

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	claimArgv := func(extra ...string) []string {
		argv := []string{self, "-test.run=^TestDispatchHelperCLI$", "--", "claim", "{ticket}", "--holder", "{holder}", "--stage", "implement", "--request-id", "claim-{worker}"}
		return append(argv, extra...)
	}
	role := func(name, host string, match map[string]any) map[string]any {
		return map[string]any{"name": name, "host": host, "cap": 1, "match": match, "prompt": "work {ticketLocal}", "idleSeconds": 30, "wallSeconds": 60}
	}
	configDir, err := filepath.EvalSymlinks(t.TempDir()) // the config reader refuses symlinked paths
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": t.TempDir(), "workRoot": r.Root,
		"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
		"hosts": map[string]any{
			"plain":  map[string]any{"argv": claimArgv(), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
			"pooled": map[string]any{"argv": claimArgv("--pool", "{pool}"), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
		},
		// The pool role matches without planSelected, so the native plan's
		// own pool deferral does not hide the tickets from the pool room.
		"roles":   []any{role("impl", "plain", map[string]any{"planSelected": true}), role("laned", "pooled", map[string]any{"pool": "lanes"})},
		"backoff": map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
	}
	raw, _ := json.Marshal(config)
	path := filepath.Join(configDir, "stall.json")
	fixture.Write(t, path, raw)
	base := []string{"--program", "prog", "--config", path}

	x := ok(append([]string{"dispatch", "--once"}, base...)...)
	stderr := string(x.stderr)
	t.Cleanup(func() {
		for _, w := range field(ok(append([]string{"dispatch", "status"}, base...)...).res.Items[0], "workers").Arr {
			pid, _ := strconv.Atoi(field(w, "pid").Str)
			for i := 0; i <= 600; i++ {
				if live, _ := supervisor.ProcessIdentity(pid); live == "" {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	})
	if strings.Contains(stderr, "launched laned") {
		t.Fatalf("a pool-needing attempt launched on a stalled pool:\n%s", stderr)
	}
	var wait string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, " pool-wait ") && strings.Contains(line, local(pooled)) {
			wait = line
		}
	}
	if !strings.Contains(wait, wire.CodePoolMemberRecoveryStalled) || !strings.Contains(wait, "pool status --pool lanes") {
		t.Fatalf("no stalled pool-wait for %s:\n%s", local(pooled), stderr)
	}
	if !strings.Contains(stderr, "launched impl slot 1 on "+local(free)) {
		t.Fatalf("the lane-free ticket did not launch:\n%s", stderr)
	}
}

// PSR-V0-267 (V1-2637): when every member of a pool is stalled, the default
// plan defers its ticket naming POOL_MEMBER_RECOVERY_STALLED rather than a
// plain no-free-member RESOURCE_COLLISION, ticket show reports that reason,
// and `dispatch --once` with a planSelected pool role starts no attempt for
// the pool and emits a pool-wait naming the stall.
func TestPSRV0267_PlanSelectedRoleReportsStalledPoolCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a"})))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	ok := func(args ...string) run {
		t.Helper()
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s %s", args, x.stdout, x.stderr)
		}
		return x
	}
	ok("init")
	poolTicket := func(title, priority string) string {
		t.Helper()
		payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"`+priority+`"`, `"touchPaths":[]`, `"touchPaths":["`+title+`"]`, `"title":"Console ticket"`, `"title":"`+title+`"`, `"requirementRefs":[],`, `"requirementRefs":[],"requiresPool":"lanes",`).Replace(createPayloadJSON)
		return field(ok("ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload).res.Items[0], "ticketId").Str
	}
	first := poolTicket("first", "P1")
	claimed := ok("claim", first, "--holder", "builder", "--request-id", "claim-first", "--pool", "lanes", "--stage", "implement").res.Items[0]
	ok("release", "--attempt", field(claimed, "attemptId").Str, "--generation", field(claimed, "generation").Str, "--request-id", "release-first")
	detail := filepath.Join(resolvedTempDir(t), "health.log")
	if err := os.WriteFile(detail, []byte("probe timed out\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pooled := poolTicket("pooled", "P0")
	free := planTicket(t, r.Root, "free", "P2", `["free"]`)
	local := func(id string) string { return id[strings.LastIndexByte(id, ':')+1:] }
	planReason := func() string {
		t.Helper()
		for _, e := range field(ok("plan", "preview").res.Items[0], "entries").Arr {
			if field(e, "ticketId").Str == pooled {
				return field(e, "state").Str + "/" + field(e, "reason").Str
			}
		}
		t.Fatalf("plan preview has no entry for %s", pooled)
		return ""
	}
	// Quarantined but not yet stalled: the generic deferral stands.
	if got := planReason(); got != "DEFERRED/"+wire.CodeResourceCollision {
		t.Fatalf("unstalled pool plan reason %s", got)
	}
	for i := 1; i <= 3; i++ {
		ok("pool", "observe", "--member", "a", "--kind", "health", "--result", "FAILED", "--detail-file", detail, "--request-id", "obs-"+strconv.Itoa(i))
	}
	if got := planReason(); got != "DEFERRED/"+wire.CodePoolMemberRecoveryStalled {
		t.Fatalf("stalled pool plan reason %s", got)
	}
	if got := field(ok("ticket", "show", pooled).res.Items[0], "claimabilityReason").Str; got != wire.CodePoolMemberRecoveryStalled {
		t.Fatalf("ticket show claimabilityReason %s", got)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	claimArgv := func(extra ...string) []string {
		argv := []string{self, "-test.run=^TestDispatchHelperCLI$", "--", "claim", "{ticket}", "--holder", "{holder}", "--stage", "implement", "--request-id", "claim-{worker}"}
		return append(argv, extra...)
	}
	role := func(name, host string, match map[string]any) map[string]any {
		return map[string]any{"name": name, "host": host, "cap": 1, "match": match, "prompt": "work {ticketLocal}", "idleSeconds": 30, "wallSeconds": 60}
	}
	configDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": t.TempDir(), "workRoot": r.Root,
		"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
		"hosts": map[string]any{
			"plain":  map[string]any{"argv": claimArgv(), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
			"pooled": map[string]any{"argv": claimArgv("--pool", "{pool}"), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
		},
		// Both roles match on planSelected, so the pool role sees the
		// stalled ticket only through the plan's deferral reason.
		"roles":   []any{role("impl", "plain", map[string]any{"planSelected": true}), role("laned", "pooled", map[string]any{"planSelected": true, "pool": "lanes"})},
		"backoff": map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
	}
	raw, _ := json.Marshal(config)
	path := filepath.Join(configDir, "stall.json")
	fixture.Write(t, path, raw)
	base := []string{"--program", "prog", "--config", path}

	x := ok(append([]string{"dispatch", "--once"}, base...)...)
	stderr := string(x.stderr)
	t.Cleanup(func() {
		for _, w := range field(ok(append([]string{"dispatch", "status"}, base...)...).res.Items[0], "workers").Arr {
			pid, _ := strconv.Atoi(field(w, "pid").Str)
			for i := 0; i <= 600; i++ {
				if live, _ := supervisor.ProcessIdentity(pid); live == "" {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	})
	if strings.Contains(stderr, "launched laned") {
		t.Fatalf("a pool-needing attempt launched on a stalled pool:\n%s", stderr)
	}
	var wait string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, " pool-wait ") && strings.Contains(line, local(pooled)) {
			wait = line
		}
	}
	if !strings.Contains(wait, wire.CodePoolMemberRecoveryStalled) || !strings.Contains(wait, "pool status --pool lanes") {
		t.Fatalf("no stalled pool-wait for %s:\n%s", local(pooled), stderr)
	}
	if !strings.Contains(stderr, "launched impl slot 1 on "+local(free)) {
		t.Fatalf("the lane-free ticket did not launch:\n%s", stderr)
	}
}
