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

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-128: a disabled (cap 0) role is excluded from planning. With
// maxActiveAttempts 1, a higher-priority ticket that only the disabled role's
// pool could take does not hold the window, so the enabled role's
// lower-priority ticket launches.
func TestCALV0128_DisabledRoleDoesNotStarveEnabledRole(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("1")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a"})))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"P0"`, `"touchPaths":[]`, `"touchPaths":["pool"]`, `"title":"Console ticket"`, `"title":"pool"`, `"requirementRefs":[],`, `"requirementRefs":[],"requiresPool":"lanes",`).Replace(createPayloadJSON)
	if created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-pool", "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload); created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create pool ticket: %+v", created.res)
	}
	free := planTicket(t, r.Root, "free", "P2", `["free"]`)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := func(extra ...string) []string {
		return append([]string{self, "-test.run=^TestDispatchHelperCLI$", "--", "claim", "{ticket}", "--holder", "{holder}", "--stage", "implement", "--request-id", "claim-{worker}"}, extra...)
	}
	role := func(name, host string, limit int, match map[string]any) map[string]any {
		return map[string]any{"name": name, "host": host, "cap": limit, "match": match, "prompt": "work {ticketLocal}", "idleSeconds": 30, "wallSeconds": 60}
	}
	configDir, err := filepath.EvalSymlinks(t.TempDir()) // the config reader refuses symlinked paths
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": t.TempDir(), "workRoot": r.Root,
		"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
		"hosts": map[string]any{
			"plain":  map[string]any{"argv": argv(), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
			"pooled": map[string]any{"argv": argv("--pool", "{pool}"), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
		},
		"roles":   []any{role("impl", "plain", 1, map[string]any{"planSelected": true}), role("laned", "pooled", 0, map[string]any{"planSelected": true, "pool": "lanes"})},
		"backoff": map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
		"heal":    map[string]any{"handoff": true, "reap": true},
	})
	path := filepath.Join(configDir, "dispatch.json")
	fixture.Write(t, path, raw)
	base := []string{"--program", "prog", "--config", path}

	x := atm(t, r.Root, nil, append([]string{"dispatch", "--once"}, base...)...)
	if x.res.Outcome != wire.OutcomeOK || !strings.Contains(string(x.stderr), "launched impl slot 1 on "+free[strings.LastIndexByte(free, ':')+1:]) {
		t.Fatalf("a disabled role's pool ticket starved the enabled role: %s %s", x.stdout, x.stderr)
	}
	st := atm(t, r.Root, nil, append([]string{"dispatch", "status"}, base...)...)
	if st.res.Outcome != wire.OutcomeOK {
		t.Fatalf("status: %s %s", st.stdout, st.stderr)
	}
	for _, w := range field(st.res.Items[0], "workers").Arr {
		pid, _ := strconv.Atoi(field(w, "pid").Str)
		for i := 0; ; i++ {
			if live, _ := supervisor.ProcessIdentity(pid); live == "" {
				break
			}
			if i > 600 {
				t.Fatal("worker did not exit")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
