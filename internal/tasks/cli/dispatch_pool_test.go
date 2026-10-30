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

// CAL-V0-097: Codex's dispatcher regression. With maxActiveAttempts 1, one
// free member, a higher-priority pool ticket and a lower-priority lane-free
// ticket, a dispatcher whose only role claims without --pool never assigns
// the pool ticket (no refused claim, cooldown or parking accrues on it) and
// its worker claims the lane-free ticket. A role that claims the pool then
// receives the pool ticket with {pool} bound and its claim is admitted.
func TestCALV0097_DispatchRoutesPoolTicketsAndKeepsLaneFreeProgress(t *testing.T) {
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
	created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-pool", "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create pool ticket: %+v", created.res)
	}
	pooled := field(created.res.Items[0], "ticketId").Str
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
	writeConfig := func(name string, roles ...any) []string {
		config := map[string]any{
			"profile": "taskman-dispatch/0", "stateDir": t.TempDir(), "workRoot": r.Root,
			"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
			"hosts": map[string]any{
				"plain":  map[string]any{"argv": claimArgv(), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
				"pooled": map[string]any{"argv": claimArgv("--pool", "{pool}"), "env": map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"}},
			},
			"roles":   roles,
			"backoff": map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
			"heal":    map[string]any{"handoff": true, "reap": true},
		}
		raw, _ := json.Marshal(config)
		path := filepath.Join(configDir, name+".json")
		fixture.Write(t, path, raw)
		return []string{"--program", "prog", "--config", path}
	}
	ok := func(args ...string) run {
		t.Helper()
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s %s", args, x.stdout, x.stderr)
		}
		return x
	}
	waitWorkers := func(base []string) {
		t.Helper()
		for _, w := range field(ok(append([]string{"dispatch", "status"}, base...)...).res.Items[0], "workers").Arr {
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
	entryState := func(id string) string {
		t.Helper()
		for _, en := range field(ok("plan", "preview").res.Items[0], "entries").Arr {
			if field(en, "ticketId").Str == id {
				return field(en, "state").Str
			}
		}
		t.Fatalf("no plan entry for %s", id)
		return ""
	}
	if entryState(pooled) != "SELECTED" || entryState(free) == "SELECTED" {
		t.Fatal("fixture: the pool ticket must hold the only window slot in the preview")
	}

	// Only a role that claims without --pool: the pool ticket is excluded
	// before it uses the dispatcher's window, so the lane-free ticket runs.
	generic := writeConfig("generic", role("impl", "plain", map[string]any{"planSelected": true}))
	var log strings.Builder
	first := ok(append([]string{"dispatch", "--once"}, generic...)...)
	log.Write(first.stderr)
	if field(first.res.Items[0], "workersRunning").Str != "1" || !strings.Contains(string(first.stderr), "launched impl slot 1 on "+local(free)) {
		t.Fatalf("first tick: %s %s", first.stdout, first.stderr)
	}
	waitWorkers(generic)
	if entryState(free) == "SELECTED" {
		t.Fatal("the lane-free worker did not claim its ticket")
	}
	for i := 0; i < 3; i++ {
		x := ok(append([]string{"dispatch", "--ticks", "1"}, generic...)...)
		log.Write(x.stderr)
		waitWorkers(generic)
	}
	if strings.Contains(log.String(), local(pooled)) {
		t.Fatalf("the generic role touched the pool ticket:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "handed off") {
		t.Fatalf("the lane-free attempt was not handed off:\n%s", log.String())
	}

	// A role that claims the pool receives the pool ticket with {pool}.
	laned := writeConfig("laned", role("impl", "plain", map[string]any{"planSelected": true}), role("laned", "pooled", map[string]any{"planSelected": true, "pool": "lanes"}))
	x := ok(append([]string{"dispatch", "--once"}, laned...)...)
	if !strings.Contains(string(x.stderr), "launched laned slot 1 on "+local(pooled)) || strings.Contains(string(x.stderr), "launched impl") {
		t.Fatalf("pool role tick: %s", x.stderr)
	}
	waitWorkers(laned)
	if entryState(pooled) == "SELECTED" {
		t.Fatal("the pool worker's --pool claim was not admitted")
	}
}
