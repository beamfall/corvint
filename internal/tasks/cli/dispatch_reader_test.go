//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func readerCLIConfig(t *testing.T, root string, script string) (string, string) {
	t.Helper()
	stateDir := t.TempDir()
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": stateDir, "workRoot": root,
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts":     map[string]any{"sh": map[string]any{"argv": []string{"/bin/sh", "-c", "exit 0"}}},
		"roles":     []any{map[string]any{"name": "impl", "host": "sh", "cap": 1, "match": map[string]any{"labels": []string{"never-launch"}}, "prompt": "work", "idleSeconds": 30, "wallSeconds": 60}},
		"backoff":   map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":      map[string]any{"handoff": true, "reap": true},
		"workState": map[string]any{"kind": "command", "argv": []string{"/bin/sh", "-c", script}},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "dispatch.json")
	fixture.Write(t, path, raw)
	return path, filepath.Join(stateDir, "prog")
}

// This prefix is the frozen dispatch-reader-cli selector. Every top-level
// witness must emit an actual non-skipped PASS in the qualification ledger.
func TestCALV0053_DispatchReaderCLIQuarantineAndStatus(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "reader", "P1", `["src/"]`)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	config, dir := readerCLIConfig(t, root, `printf '{}'`)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "reader-lifecycle.json")
	raw := []byte(`{"profile":"taskman-dispatch-reader-lifecycle/0","program":"prog","run":"00000000000000000000000000000000","lifecycle":"UNKNOWN"}`)
	fixture.Write(t, marker, raw)
	beforeNative := fixture.TreeSnapshot(t, repo.StateDir)
	base := []string{"--program", "prog", "--config", config}
	for _, bound := range [][]string{{"--once"}, {"--ticks", "1"}} {
		args := append([]string{"dispatch"}, bound...)
		args = append(args, base...)
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeError || !containsReaderCode(r.res.Codes, wire.CodeQuiescenceUnproved) {
			t.Fatalf("quarantine became success: %s %s", r.stdout, r.stderr)
		}
		if strings.Contains(string(r.stderr), "left running for the next dispatcher") {
			t.Fatal("unsafe handoff wording")
		}
	}
	beforePrivate := fixture.TreeSnapshot(t, dir)
	r := atm(t, root, nil, append([]string{"dispatch", "status"}, base...)...)
	if r.res.Outcome != wire.OutcomeOK || field(r.res.Items[0], "readerContainment").Str != "UNKNOWN" || !field(r.res.Items[0], "readerQuarantined").Bool {
		t.Fatalf("status: %s", r.stdout)
	}
	if !fixture.SameTree(beforePrivate, fixture.TreeSnapshot(t, dir)) || !fixture.SameTree(beforeNative, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("status/quarantine mutated private or native state")
	}
	for _, name := range []string{"state.json", "events.jsonl", "requests"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatal("quarantined Open created", name, err)
		}
	}
}

func containsReaderCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

// A released child changes the marker before result handling. Physical
// containment is complete, but the CLI must still refuse an unsafe restart.
func TestCALV0053_DispatchReaderCLIClearFailure(t *testing.T) {
	for _, bound := range [][]string{{"--once"}, {"--ticks", "1"}} {
		t.Run(strings.Join(bound, "-"), func(t *testing.T) {
			root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
			planTicket(t, root, "reader", "P1", `["src/"]`)
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			config, dir := readerCLIConfig(t, root, `printf '{}'`)
			var value map[string]any
			raw, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			value["workState"] = map[string]any{"kind": "command", "argv": []string{"/bin/sh", "-c", `printf changed > "$1"; printf '{}'`, "reader", filepath.Join(dir, "reader-lifecycle.json")}}
			raw, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, config, raw)
			before := fixture.TreeSnapshot(t, repo.StateDir)
			args := append([]string{"dispatch"}, bound...)
			args = append(args, "--program", "prog", "--config", config)
			r := atm(t, root, nil, args...)
			if r.res.Outcome != wire.OutcomeError || !containsReaderCode(r.res.Codes, wire.CodeQuiescenceUnproved) || field(r.res.Items[0], "readerContainment").Str != "UNKNOWN" || !field(r.res.Items[0], "readerQuarantined").Bool {
				t.Fatalf("lost containment error: %s %s", r.stdout, r.stderr)
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("held reader mutated native queue")
			}
			if strings.Contains(string(r.stderr), "left running for the next dispatcher") {
				t.Fatal("held reader reported safe handoff")
			}
			if raw, err := os.ReadFile(filepath.Join(dir, "reader-lifecycle.json")); err != nil || string(raw) != "changed" {
				t.Fatal("changed marker removed", err)
			}
		})
	}
}

func TestCALV0053_DispatchReaderCLIReleased(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "reader", "P1", `["src/"]`)
	config, _ := readerCLIConfig(t, root, `printf '{}'`)
	r := atm(t, root, nil, "dispatch", "--once", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeOK || field(r.res.Items[0], "readerContainment").Str != "NOT_OBSERVED" || field(r.res.Items[0], "readerQuarantined").Bool {
		t.Fatalf("released reader: %s %s", r.stdout, r.stderr)
	}
}

func TestCALV0053_DispatchReaderCLICrashRetainsQuarantine(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "reader", "P1", `["src/"]`)
	ready := filepath.Join(root, "reader-ready")
	config, dir := readerCLIConfig(t, root, `echo $$ > '`+ready+`'; exec /bin/sleep 120`)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestDispatchHelperCLI$", "--", "dispatch", "--once", "--program", "prog", "--config", config)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CORVINT_TEST_DISPATCH_HELPER=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	joined := make(chan error, 1)
	done := make(chan struct{})
	go func() { defer close(done); joined <- cmd.Wait() }()
	readerPID := 0
	readerIdentity := ""
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("crashed CLI fixture join UNKNOWN")
		}
		if readerPID > 0 && readerIdentity != "" {
			if id, _ := supervisor.ProcessIdentity(readerPID); id == readerIdentity {
				_ = syscall.Kill(readerPID, syscall.SIGKILL)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				id, e := supervisor.ProcessIdentity(readerPID)
				if e == nil && id != readerIdentity {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Error("crash fixture reader identity remains; physical cleanup UNKNOWN")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, e := os.ReadFile(ready)
		if e == nil {
			readerPID, e = strconv.Atoi(strings.TrimSpace(string(raw)))
			if e == nil && readerPID > 0 {
				readerIdentity, e = supervisor.ProcessIdentity(readerPID)
				if e == nil && readerIdentity != "" {
					break
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if readerIdentity == "" {
		t.Fatal("crash fixture did not initialize reader identity")
	}
	before, err := os.ReadFile(filepath.Join(dir, "reader-lifecycle.json"))
	if err != nil {
		t.Fatal("pre-spawn marker absent", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-joined:
		if err == nil {
			t.Fatal("SIGKILL fixture reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("crash did not join")
	}
	after, err := os.ReadFile(filepath.Join(dir, "reader-lifecycle.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("crash lost marker", err)
	}
	r := atm(t, root, nil, "dispatch", "--once", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeError || !containsReaderCode(r.res.Codes, wire.CodeQuiescenceUnproved) {
		t.Fatalf("crash restart admitted: %s", r.stdout)
	}
	if strings.Contains(output.String(), "left running for the next dispatcher") {
		t.Fatal("crash reported graceful handoff")
	}
}
