//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const contextHookHelperEnv = "CORVINT_TEST_CONTEXT_HOOK_HELPER"

// TestATRV0101_ContextHookHelper is the hook command the fake host runs:
// this test binary re-executed as `corvint-tasks dispatch context-hook`.
func TestATRV0101_ContextHookHelper(t *testing.T) {
	if os.Getenv(contextHookHelperEnv) == "" {
		t.Skip("helper process for the context hook tests")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	os.Exit(Run(Env{Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}

// claudeUsage is a main-session Claude Code assistant event whose context
// estimate is tokens.
func claudeUsage(tokens int) string {
	return `{"type":"assistant","message":{"id":"m","type":"message","role":"assistant","content":[],"usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":` + strconv.Itoa(tokens-30) + `,"output_tokens":0,"service_tier":"standard"}},"parent_tool_use_id":null,"session_id":"s"}`
}

// ATR-V0-101, ATR-V0-102 and ATR-V0-103: a worker that never runs a
// dispatcher verb receives the WARN and then the HANDOFF notice, each once,
// through the Claude Code hook its host registers from
// {contextHookSettings}; the fake host calls the registered command after
// every "tool use" as Claude Code would.
func TestATRV0101_WorkerWithoutDispatcherVerbReceivesNoticesThroughHook(t *testing.T) {
	ReapWorkersOnCleanup(t)
	t.Setenv(contextHookHelperEnv, "1")
	root := t.TempDir()
	out, done, pidFile := filepath.Join(root, "hook.out"), filepath.Join(root, "done"), filepath.Join(root, "pid")
	// Each loop is the session's tool calls: the hook runs after each one.
	script := `echo $$ > '` + pidFile + `'
cmd=$(sed -n 's/.*"command":"\([^"]*\)".*/\1/p' "$1")
hook() { printf '{"hook_event_name":"PostToolUse","tool_name":"Bash"}' | eval "$cmd" >> '` + out + `' 2>> '` + out + `.err'; }
printf '%s\n' '` + claudeUsage(1500) + `'
i=0; while [ $i -lt 300 ] && ! grep -q 'its WARN threshold' '` + out + `' 2>/dev/null; do hook; i=$((i+1)); sleep 0.05; done
printf '%s\n' '` + claudeUsage(2500) + `'
i=0; while [ $i -lt 300 ] && ! grep -q 'its HANDOFF threshold' '` + out + `' 2>/dev/null; do hook; i=$((i+1)); sleep 0.05; done
hook; hook
: > '` + done + `'
exec sleep 300`
	host := filepath.Join(root, "host.sh")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	c := &dispatch.Config{Profile: dispatch.ConfigProfile, StateDir: filepath.Join(root, "state"), WorkRoot: root, TickSeconds: 1, GlobalCap: 1, KillGraceSeconds: 1,
		Hosts: map[string]dispatch.Host{"claude": {Argv: []string{host, "{contextHookSettings}", "{prompt}"}}},
		Roles: []dispatch.Role{{Name: "impl", Host: "claude", Cap: 1, Match: &dispatch.Match{}, Prompt: "work on {ticketLocal}", IdleSeconds: 3600, WallSeconds: 7200,
			UsageFormat: "claude-code", Context: &dispatch.ContextLimits{WarnTokens: 1000, HandoffTokens: 2000}}},
		Backoff: dispatch.Backoff{CooldownSeconds: 3600, ParkAfter: 2},
		Heal:    dispatch.Heal{Handoff: true, Reap: true}}
	q := &issue502Queue{obs: dispatch.Observation{Tickets: []dispatch.Ticket{{ID: "ticket:a:q:t1", Local: "t1", Status: "OPEN", Priority: "P1", Kind: "TASK", Revision: "1", Order: 1, State: dispatch.StateNone}}}}
	d, err := dispatch.Open("prog", c, q, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.HookCommand = []string{os.Args[0], "-test.run=^TestATRV0101_ContextHookHelper$", "--", "dispatch", "context-hook"}
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 1 {
				syscall.Kill(-pid, syscall.SIGKILL)
				syscall.Kill(pid, syscall.SIGKILL)
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
	errs, _ := os.ReadFile(out + ".err")
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("the session never saw both notices; hook output %q, stderr %q", raw, errs)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("hook output %d lines, want WARN then HANDOFF once each: %q", len(lines), raw)
	}
	for i, level := range []string{"WARN", "HANDOFF"} {
		var v struct {
			HookSpecificOutput map[string]string `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &v); err != nil || v.HookSpecificOutput["hookEventName"] != "PostToolUse" || !strings.Contains(v.HookSpecificOutput["additionalContext"], "its "+level+" threshold") {
			t.Fatalf("notice %d = %q (%v)", i, lines[i], err)
		}
	}
	if !strings.Contains(lines[1], "CORVINT_DISPATCH_EVIDENCE_DIR") {
		t.Fatalf("the HANDOFF notice does not name the evidence directory: %q", lines[1])
	}
	dir := dispatch.ProgramDir(c, "prog")
	l, err := dispatch.LoadLedger(dir, "prog")
	if err != nil || len(l.Workers) != 1 {
		t.Fatalf("ledger %+v %v", l, err)
	}
	workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, time.Now()), "workers")
	delivery, ok := statusField(t, workers.Arr[0], "contextDelivery")
	if !ok || string(wire.Encode(delivery)) != `{"delivered":["WARN","HANDOFF"],"mode":"HOOK"}` {
		t.Fatalf("contextDelivery %s", wire.Encode(delivery))
	}
}

// ATR-V0-101: the hook never blocks the session: every failure exits 0
// with nothing on stdout and one stderr line, and a crossing is delivered
// once.
func TestATRV0101_ContextHookFailsQuietly(t *testing.T) {
	dir := t.TempDir()
	write := func(worker string, rec dispatch.ContextRecord) string {
		t.Helper()
		path := dispatch.ContextRecordPath(dir, worker)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(rec)
		if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	at := "2026-10-18T09:00:00Z"
	warn := dispatch.ContextRecord{Profile: dispatch.ContextProfile, Worker: "w1", Tokens: "1500", ObservedAt: at, WarnTokens: "1000", HandoffTokens: "2000", HardTokens: "NONE", Level: dispatch.ContextWarn,
		Crossings: []dispatch.ContextCrossing{{Level: dispatch.ContextWarn, Threshold: "1000", Tokens: "1500", At: at}}}
	good := write("w1", warn)
	newer := filepath.Join(dir, "workers", "w2", "context.json")
	if err := os.MkdirAll(filepath.Dir(newer), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte(`{"profile":"taskman-dispatch-context/9","worker":"w2"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w3 := warn
	w3.Worker = "w3"
	blocked := write("w3", w3)
	if err := os.Chmod(filepath.Dir(blocked), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(blocked), 0o700) })

	run := func(path string, args ...string) (int, string, string) {
		t.Helper()
		t.Setenv("CORVINT_DISPATCH_CONTEXT_FILE", path)
		var stdout, stderr bytes.Buffer
		code := Run(Env{Args: append([]string{"dispatch", "context-hook"}, args...), Stdin: strings.NewReader("not json"), Stdout: &stdout, Stderr: &stderr})
		return code, stdout.String(), stderr.String()
	}
	for name, tc := range map[string]struct {
		path string
		args []string
	}{"unset": {"", nil}, "newer record": {newer, nil}, "extra argument": {good, []string{"--now"}}, "unwritable marker": {blocked, nil}, "not a record": {filepath.Join(dir, "other.json"), nil}} {
		if os.Geteuid() == 0 && name == "unwritable marker" {
			continue
		}
		code, stdout, stderr := run(tc.path, tc.args...)
		if code != 0 || stdout != "" || strings.Count(stderr, "\n") != 1 {
			t.Errorf("%s: exit %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}
	code, stdout, stderr := run(good)
	var v struct {
		HookSpecificOutput map[string]string `json:"hookSpecificOutput"`
	}
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &v) != nil || v.HookSpecificOutput["hookEventName"] != "PostToolUse" || !strings.Contains(v.HookSpecificOutput["additionalContext"], "its WARN threshold of 1000 tokens") {
		t.Fatalf("first delivery: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(good); code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("second delivery: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// ATR-V0-103: dispatch status shows each context-capable worker's delivery
// mode and delivered levels: HOOK where the host registers the hook,
// UNKNOWN for another claude-code host and UNSUPPORTED for opencode.
func TestATRV0103_StatusReportsContextDelivery(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "workers", "w1", "context-delivered-WARN")
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	limits := &dispatch.ContextLimits{WarnTokens: 1000, HardTokens: 4000}
	c := &dispatch.Config{
		Hosts: map[string]dispatch.Host{"hook": {Argv: []string{"claude", "--settings", "{contextHookSettings}"}}, "plain": {Argv: []string{"claude"}}},
		Roles: []dispatch.Role{
			{Name: "impl", UsageFormat: "claude-code", Context: limits},
			{Name: "oc", UsageFormat: "opencode", Context: limits},
			{Name: "cx", UsageFormat: "codex"},
		}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: "w1", Role: "impl", Host: "hook", Key: "ticket:a:q:t1"},
		{ID: "w2", Role: "impl", Host: "plain", Key: "ticket:a:q:t2"},
		{ID: "w3", Role: "oc", Host: "plain", Key: "ticket:a:q:t3"},
		{ID: "w4", Role: "cx", Host: "plain", Key: "ticket:a:q:t4"},
	}}
	workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, time.Now()), "workers")
	want := []string{`{"delivered":["WARN"],"mode":"HOOK"}`, `{"delivered":[],"mode":"UNKNOWN"}`, `{"delivered":[],"mode":"UNSUPPORTED"}`, ""}
	for i, w := range want {
		got, ok := statusField(t, workers.Arr[i], "contextDelivery")
		if w == "" {
			if ok {
				t.Errorf("worker %d shows contextDelivery %s", i, wire.Encode(got))
			}
			continue
		}
		if !ok || string(wire.Encode(got)) != w {
			t.Errorf("worker %d contextDelivery %s, want %s", i, wire.Encode(got), w)
		}
	}
}

// ATR-V0-107: dispatch status shows each ticket and role's hard-stop run
// with its state, and UNKNOWN with the reason for a record that does not
// read; no record shows nothing.
func TestATRV0107_DispatchStatusShowsContextExhausted(t *testing.T) {
	dir := t.TempDir()
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, dir, l, nil, time.Now()), "contextExhausted"); ok {
		t.Fatal("contextExhausted shown without a record")
	}
	rec := dispatch.ContextExhaustion{Profile: dispatch.ContextExhaustedProfile, Program: "prog", Entries: []dispatch.HardStopStreak{
		{Key: "ticket:a:q:t1", Role: "impl", HardStops: 3, State: dispatch.ContextExhausted, LastWorker: "w3", LastAt: "2026-10-18T09:00:00Z"},
		{Key: "ticket:a:q:t2", Role: "impl", HardStops: 1, State: dispatch.ContextCounting, LastWorker: "w4", LastAt: "2026-10-18T09:05:00Z"},
	}}
	raw, _ := json.Marshal(rec)
	if err := os.WriteFile(dispatch.ContextExhaustedPath(dir), append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, dir, l, nil, time.Now()), "contextExhausted")
	want := `[{"hardStops":"3","key":"ticket:a:q:t1","lastAt":"2026-10-18T09:00:00Z","lastWorker":"w3","role":"impl","state":"CONTEXT_EXHAUSTED"},` +
		`{"hardStops":"1","key":"ticket:a:q:t2","lastAt":"2026-10-18T09:05:00Z","lastWorker":"w4","role":"impl","state":"COUNTING"}]`
	if !ok || string(wire.Encode(v)) != want {
		t.Fatalf("contextExhausted %s", wire.Encode(v))
	}
	if err := os.WriteFile(dispatch.ContextExhaustedPath(dir), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, _ = statusField(t, dispatchStatusValue(&dispatch.Config{}, dir, l, nil, time.Now()), "contextExhausted")
	if state, _ := statusField(t, v, "state"); state.Str != "UNKNOWN" {
		t.Fatalf("damaged record %s", wire.Encode(v))
	}
	if reason, ok := statusField(t, v, "reason"); !ok || reason.Str == "" {
		t.Fatalf("damaged record has no reason: %s", wire.Encode(v))
	}
}
