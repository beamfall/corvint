//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestDispatchHelperCLI is the worker host of the dispatch integration test:
// the test binary re-executes itself and runs the CLI arguments after "--".
func TestDispatchHelperCLI(t *testing.T) {
	if os.Getenv("CORVINT_TEST_DISPATCH_HELPER") != "1" {
		t.Skip("dispatch worker helper")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	cwd, _ := os.Getwd()
	os.Exit(cli.Run(cli.Env{Cwd: cwd, Args: args, Stdin: bytes.NewReader(nil), Stdout: os.Stdout, Stderr: os.Stderr}))
}

// CAL-V0-048..054: one real dispatcher tick launches a worker that claims
// its pinned ticket through the CLI; the next tick sees the worker ended
// with a live attempt, hands it off with evidence, accounts a no-progress
// run, and cools the ticket down. Status reads never touch the store.
func TestCALV0048_DispatchCLIClaimHandoffAndStatus(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "dispatch", "P1", `["src/"]`)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": stateDir, "workRoot": root,
		"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
		"hosts": map[string]any{"helper": map[string]any{
			"argv": []string{self, "-test.run=^TestDispatchHelperCLI$", "--", "claim", "{ticket}", "--holder", "{holder}", "--stage", "implement", "--request-id", "claim-{worker}"},
			"env":  map[string]string{"CORVINT_TEST_DISPATCH_HELPER": "1"},
		}},
		"roles":   []any{map[string]any{"name": "impl", "host": "helper", "cap": 1, "match": map[string]any{"planSelected": true}, "prompt": "implement {ticketLocal}", "idleSeconds": 30, "wallSeconds": 60}},
		"backoff": map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":    map[string]any{"handoff": true, "reap": true},
	}
	raw, _ := json.Marshal(config)
	configDir, err := filepath.EvalSymlinks(t.TempDir()) // the config reader refuses symlinked paths
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "dispatch.json")
	fixture.Write(t, configPath, raw)
	base := []string{"--program", "prog", "--config", configPath}
	ok := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s %s", args, r.stdout, r.stderr)
		}
		return r
	}

	first := ok(append([]string{"dispatch", "--once"}, base...)...)
	if field(first.res.Items[0], "workersRunning").Str != "1" || !strings.Contains(string(first.stderr), "launched impl slot 1") {
		t.Fatalf("first tick: %s %s", first.stdout, first.stderr)
	}
	before := fixture.TreeSnapshot(t, repo.StateDir)
	status := ok(append([]string{"dispatch", "status"}, base...)...)
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("dispatch status touched the native store")
	}
	workers := field(status.res.Items[0], "workers").Arr
	if len(workers) != 1 || field(status.res.Items[0], "dispatcherRunning").Str != "NOT_RUNNING" {
		t.Fatalf("status: %s", status.stdout)
	}
	pid, _ := strconv.Atoi(field(workers[0], "pid").Str)
	for i := 0; ; i++ {
		if live, _ := supervisor.ProcessIdentity(pid); live == "" {
			break
		}
		if i > 600 {
			t.Fatal("worker did not exit")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if plan := ok("plan", "preview"); field(field(plan.res.Items[0], "entries").Arr[0], "state").Str == "SELECTED" {
		t.Fatalf("worker did not claim: %s", plan.stdout)
	}

	second := ok(append([]string{"dispatch", "--ticks", "1"}, base...)...)
	log := string(second.stderr)
	for _, want := range []string{"finished on", "handed off", "cools down"} {
		if !strings.Contains(log, want) {
			t.Fatalf("second tick lacks %q:\n%s", want, log)
		}
	}
	if field(second.res.Items[0], "workersRunning").Str != "0" {
		t.Fatalf("cooled ticket relaunched: %s", second.stdout)
	}
	if plan := ok("plan", "preview"); field(field(plan.res.Items[0], "entries").Arr[0], "state").Str != "SELECTED" {
		t.Fatalf("handoff did not free the ticket: %s", plan.stdout)
	}

	unpark := ok(append([]string{"dispatch", "unpark", "--key", id}, base...)...)
	if _, err := os.Stat(field(unpark.res.Items[0], "request").Str); err != nil {
		t.Fatal(err)
	}
	if bad := atm(t, root, nil, "dispatch", "--program", "Bad", "--config", configPath); bad.res.Outcome == wire.OutcomeOK {
		t.Fatal("invalid program name accepted")
	}
}
