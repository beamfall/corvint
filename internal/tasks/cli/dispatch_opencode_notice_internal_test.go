//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// openCodeUsage is an OpenCode step_finish event whose context estimate is
// tokens.
func openCodeUsage(tokens int) string {
	return `{"type":"step_finish","timestamp":2,"sessionID":"ses_1","part":{"type":"step-finish","reason":"tool-calls","tokens":{"input":` + strconv.Itoa(tokens-3) + `,"output":2,"reasoning":0,"cache":{"read":1,"write":0}}}}`
}

// TestDSGV0008_OpenCodeSessionReceivesHandoffInSession covers GitHub #1821
// end to end: a dispatcher worker on an opencode host that declares
// contextNotices "plugin" is one node process running the shipped
// integrations/opencode/src/dispatch-notice.js the way the Corvint OpenCode
// plugin does before each model request. Its tools are the compiled
// corvint-tasks the dispatcher names in CORVINT_TASKS_BIN. When the session's
// usage crosses warnTokens and then handoffTokens, the next model request's
// system context carries the WARN and then the HANDOFF notice, each
// delivered once.
func TestDSGV0008_OpenCodeSessionReceivesHandoffInSession(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the OpenCode plugin module cannot run")
	}
	plugin, err := filepath.Abs("../../../integrations/opencode/src/dispatch-notice.js")
	if err != nil {
		t.Fatal(err)
	}
	bin := CompiledTasksBinary(t)
	ReapWorkersOnCleanup(t)
	root := t.TempDir()
	out, done, pidFile := filepath.Join(root, "requests.jsonl"), filepath.Join(root, "done"), filepath.Join(root, "pid")
	driver := filepath.Join(root, "session.mjs")
	// Each loop turn is one model request of the session.
	if err := os.WriteFile(driver, []byte(`import { createDispatchNotices } from `+strconv.Quote(plugin)+`
import { appendFileSync, writeFileSync } from "node:fs"
const [out, done, warn, handoff] = process.argv.slice(2)
const notices = createDispatchNotices({ environment: process.env, tasksBinary: process.env.CORVINT_TASKS_BIN })
if (!notices) { appendFileSync(out, "NO_NOTICE_SOURCE\n"); process.exit(1) }
const pause = ms => new Promise(r => setTimeout(r, ms))
const until = async level => {
  for (let i = 0; i < 300; i++) {
    const request = { system: [{ type: "text", text: "base prompt" }] }
    await notices.inject(request)
    if (request.system.some(s => s.text.includes("its " + level + " threshold"))) {
      appendFileSync(out, JSON.stringify(request.system.map(s => s.text)) + "\n")
      return
    }
    await pause(50)
  }
  appendFileSync(out, "NO_" + level + "\n")
  process.exit(2)
}
process.stdout.write(warn + "\n")
await until("WARN")
process.stdout.write(handoff + "\n")
await until("HANDOFF")
const last = { system: [] }
await notices.inject(last)
appendFileSync(out, JSON.stringify(last.system.map(s => s.text)) + "\n")
writeFileSync(done, "")
setInterval(() => {}, 1000)
`), 0o600); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "host.sh")
	script := "echo $$ > '" + pidFile + "'\nexec " + strconv.Quote(node) + " " + strconv.Quote(driver) + " '" + out + "' '" + done + "' '" + openCodeUsage(1500) + "' '" + openCodeUsage(2500) + "'"
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	c := &dispatch.Config{Profile: dispatch.ConfigProfile, StateDir: filepath.Join(root, "state"), WorkRoot: root, TickSeconds: 1, GlobalCap: 1, KillGraceSeconds: 1,
		Hosts: map[string]dispatch.Host{"opencode": {Argv: []string{host, "{prompt}"}, ContextNotices: dispatch.ContextNoticesPlugin}},
		Roles: []dispatch.Role{{Name: "impl", Host: "opencode", Cap: 1, Match: &dispatch.Match{}, Prompt: "work on {ticketLocal}", IdleSeconds: 3600, WallSeconds: 7200,
			UsageFormat: "opencode", Context: &dispatch.ContextLimits{WarnTokens: 1000, HandoffTokens: 2000}}},
		Backoff: dispatch.Backoff{CooldownSeconds: 3600, ParkAfter: 2},
		Heal:    dispatch.Heal{Handoff: true, Reap: true}}
	q := &issue502Queue{obs: dispatch.Observation{Tickets: []dispatch.Ticket{{ID: "ticket:a:q:t1", Local: "t1", Status: "OPEN", Priority: "P1", Kind: "TASK", Revision: "1", Order: 1, State: dispatch.StateNone}}}}
	d, err := dispatch.Open("prog", c, q, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.HookCommand = []string{bin, "dispatch", "context-hook"}
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 1 {
				syscall.Kill(-pid, syscall.SIGKILL)
				syscall.Kill(pid, syscall.SIGKILL)
				// V1-3832: SIGKILL is asynchronous. On a loaded host the
				// session's members (node and any context-hook child) can
				// still be running when ReapWorkersOnCleanup reads the
				// process table, which it reports as a leaked group.
				awaitGroupExit(t, pid, 30*time.Second)
			}
		}
	})
	for i := 0; i < 1200; i++ {
		if _, err := os.Stat(done); err == nil {
			break
		}
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	raw, _ := os.ReadFile(out)
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("the session never saw both notices in-session: %q", raw)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("model requests %q", raw)
	}
	var warn, handoff, after []string
	for i, v := range []*[]string{&warn, &handoff, &after} {
		if err := json.Unmarshal([]byte(lines[i]), v); err != nil {
			t.Fatalf("request %d %q: %v", i, lines[i], err)
		}
	}
	count := func(system []string, level string) int {
		n := 0
		for _, s := range system {
			if strings.Contains(s, "its "+level+" threshold") {
				n++
			}
		}
		return n
	}
	if count(warn, "WARN") != 1 || count(warn, "HANDOFF") != 0 || warn[0] != "base prompt" {
		t.Fatalf("request after WARN: %q", warn)
	}
	if count(handoff, "HANDOFF") != 1 || !strings.Contains(strings.Join(handoff, "\n"), "CORVINT_DISPATCH_EVIDENCE_DIR") {
		t.Fatalf("request after HANDOFF: %q", handoff)
	}
	// A later request of the same session keeps the notice once.
	if count(after, "HANDOFF") != 1 || count(after, "WARN") != 1 {
		t.Fatalf("a later request: %q", after)
	}
	dir := dispatch.ProgramDir(c, "prog")
	l, err := dispatch.LoadLedger(dir, "prog")
	if err != nil || len(l.Workers) != 1 {
		t.Fatalf("ledger %+v %v", l, err)
	}
	workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, time.Now()), "workers")
	delivery, _ := statusField(t, workers.Arr[0], "contextDelivery")
	if got := string(wire.Encode(delivery)); got != `{"delivered":["WARN","HANDOFF"],"mode":"HOOK"}` {
		t.Fatalf("contextDelivery %s", got)
	}
}

// awaitGroupExit waits, bounded, until no live process is in group pgid, so
// a group the test killed has exited before ReapWorkersOnCleanup judges it
// (V1-3832). A group still live at the deadline is left to that check.
func awaitGroupExit(t *testing.T, pgid int, within time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		table, err := groupreap.ReadProcessTable()
		if err != nil {
			return
		}
		if !groupLive(table, pgid) {
			return
		}
	}
}

// groupLive reports a live (non-zombie) member of group pgid in table.
func groupLive(table map[int]groupreap.Process, pgid int) bool {
	for _, p := range table {
		if p.PGID == pgid && live(p) {
			return true
		}
	}
	return false
}

// TestAwaitGroupExitWaitsForKilledMembers (V1-3832): a group with a leader
// and a member, killed with SIGKILL, has no live member once awaitGroupExit
// returns, so ReapWorkersOnCleanup finds nothing to report.
func TestAwaitGroupExitWaitsForKilledMembers(t *testing.T) {
	ReapWorkersOnCleanup(t)
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & exec sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	pgid := cmd.Process.Pid
	for i := 0; i < 250; i++ {
		table, err := groupreap.ReadProcessTable()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range table {
			if p.PGID == pgid && live(p) {
				n++
			}
		}
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	syscall.Kill(-pgid, syscall.SIGKILL)
	awaitGroupExit(t, pgid, 30*time.Second)
	table, err := groupreap.ReadProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	if groupLive(table, pgid) {
		t.Fatalf("group %d has a live member after awaitGroupExit", pgid)
	}
	<-waited
}
