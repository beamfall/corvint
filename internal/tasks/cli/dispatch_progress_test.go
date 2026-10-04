//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0064_DispatchCLIFileProgressAndReplay(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "progress", "P1", `["src/"]`)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "work.json")
	host, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": filepath.Join(dir, "state"), "workRoot": root,
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts": map[string]any{"true": map[string]any{"argv": []string{host, "{prompt}"}}},
		"roles": []any{map[string]any{"name": "impl", "host": "true", "cap": 1,
			"match":  map[string]any{"states": []string{"work"}, "planSelected": true},
			"prompt": "work on {ticket}", "idleSeconds": 30, "wallSeconds": 60}},
		"workState": map[string]any{"kind": "command", "argv": []string{"/bin/cat", source}},
		"backoff":   map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "dispatch.json")
	fixture.Write(t, path, raw)
	write := func(token string) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{id: map[string]string{"state": "work", "progress": token}})
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, source, raw)
	}
	tick := func() run {
		t.Helper()
		r := atm(t, root, nil, "dispatch", "--once", "--program", "prog", "--config", path)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("tick: %s %s", r.stdout, r.stderr)
		}
		return r
	}
	wait := func(r run) {
		t.Helper()
		if field(r.res.Items[0], "workersRunning").Str != "1" {
			t.Fatalf("not launched: %s %s", r.stdout, r.stderr)
		}
		c := &dispatch.Config{StateDir: filepath.Join(dir, "state")}
		l, err := dispatch.LoadLedger(dispatch.ProgramDir(c, "prog"), "prog")
		if err != nil || len(l.Workers) != 1 {
			t.Fatalf("worker ledger: %v", err)
		}
		pid := l.Workers[0].PID
		until := time.Now().Add(5 * time.Second)
		for {
			live, err := supervisor.ProcessIdentity(pid)
			if err != nil {
				t.Fatal(err)
			}
			if live == "" {
				return
			}
			if time.Now().After(until) {
				t.Fatalf("owned true worker %d did not retire", pid)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	before := fixture.TreeSnapshot(t, repo.StateDir)
	write("file-A")
	wait(tick())
	parked := tick()
	if !strings.Contains(string(parked.stderr), "parked") {
		t.Fatalf("same token did not park: %s", parked.stderr)
	}
	write("file-B")
	changed := tick()
	if !strings.Contains(string(changed.stderr), "declared progress changed") {
		t.Fatalf("changed file did not unpark: %s", changed.stderr)
	}
	wait(changed)
	tick() // the second run did not change its launch token, so it parks again
	write("file-A")
	stale := tick()
	if n, _ := strconv.Atoi(field(stale.res.Items[0], "workersRunning").Str); n != 0 {
		t.Fatalf("observed stale token relaunched worker: %s", stale.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("private explicit progress changed the native queue")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}
