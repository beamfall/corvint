//go:build darwin || linux

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestDSGV0007_UpgradeCheck covers GitHub #1815: `dispatch upgrade --check
// NEW_BUILD` reports, without changing state, whether a restart on NEW_BUILD
// adopts the live workers. NEW_BUILD is a stand-in that replays the result a
// real `dispatch status` gives for each ledger: the same format with a live
// worker is ROLLING, a previous version with a live worker is DRAIN_REQUIRED,
// a refused configuration is REFUSED, and a build that prints no result is
// UNKNOWN.
func TestDSGV0007_UpgradeCheck(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(stateDir, "dispatch.json")
	cfg := `{"profile":"taskman-dispatch/0","stateDir":` + jsonQuote(stateDir) + `,"workRoot":` + jsonQuote(stateDir) +
		`,"tickSeconds":1,"globalCap":1,"killGraceSeconds":1,"hosts":{"h":{"argv":["/bin/echo","{prompt}"]}}` +
		`,"roles":[{"name":"impl","host":"h","cap":1,"match":{},"prompt":"work {ticketLocal}","idleSeconds":30,"wallSeconds":60,"carryOver":{}}]` +
		`,"backoff":{"cooldownSeconds":0,"parkAfter":1},"heal":{"handoff":true,"reap":true}}`
	if err := os.WriteFile(config, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "prog", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	ledger := func(profile string) []byte {
		l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
			Workers: []*dispatch.Worker{{ID: "w1", Role: "impl", Host: "h", Key: "ticket:a:q:t1", Ticket: "ticket:a:q:t1", PID: 1, Started: now}}}
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		return bytes.Replace(raw, []byte(`"`+dispatch.StateProfile+`"`), []byte(`"`+profile+`"`), 1)
	}
	run := func(args ...string) *wire.Result {
		t.Helper()
		var out bytes.Buffer
		Run(Env{Cwd: stateDir, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &bytes.Buffer{}})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return res
	}
	// build is a stand-in NEW_BUILD that prints what this build's own
	// `dispatch status` prints for the state as it is now.
	build := filepath.Join(stateDir, "new-build")
	replay := func(body string) {
		t.Helper()
		if body == "" {
			var out bytes.Buffer
			Run(Env{Cwd: stateDir, Args: []string{"dispatch", "status", "--program", "prog", "--config", config, "--events", "0"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &bytes.Buffer{}})
			body = out.String()
		}
		if err := os.WriteFile(filepath.Join(stateDir, "replay.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\n[ \"$1 $2 $7 $8\" = \"dispatch status --events 0\" ] || exit 2\ncat " + filepath.Join(stateDir, "replay.json") + "\n"
		if err := os.WriteFile(build, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	check := func() map[string]string {
		t.Helper()
		before, _ := os.ReadFile(statePath)
		res := run("dispatch", "upgrade", "--program", "prog", "--config", config, "--check", build)
		if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
			t.Fatalf("check: %+v", res)
		}
		if after, _ := os.ReadFile(statePath); !bytes.Equal(before, after) {
			t.Fatal("the check changed the ledger")
		}
		got := map[string]string{}
		for _, k := range []string{"verdict", "reason", "liveWorkers", "detail"} {
			v, _ := res.Items[0].Obj.Get(k)
			got[k] = v.Str
		}
		return got
	}

	if err := os.WriteFile(statePath, ledger(dispatch.StateProfile), 0o600); err != nil {
		t.Fatal(err)
	}
	replay("")
	if got := check(); got["verdict"] != UpgradeRolling || got["liveWorkers"] != "1" || !strings.Contains(got["detail"], "adopts the 1 live worker") {
		t.Fatalf("same format: %+v", got)
	}

	// The new build reads only a drained copy of this ledger: here the
	// current build plays it against a version 1 ledger that records a worker.
	if err := os.WriteFile(statePath, ledger("taskman-dispatch-state/1"), 0o600); err != nil {
		t.Fatal(err)
	}
	replay("")
	if err := os.WriteFile(statePath, ledger(dispatch.StateProfile), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := check(); got["verdict"] != UpgradeDrainRequired || got["reason"] != "STATE_FORMAT" || !strings.Contains(got["detail"], "records a worker") {
		t.Fatalf("drain: %+v", got)
	}

	// A configuration the new build refuses: replayed from a status run on
	// a file this build refuses too.
	bad := strings.Replace(cfg, `"/bin/echo","{prompt}"`, `"/bin/echo","--prompt={prompt}"`, 1)
	if err := os.WriteFile(config, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	replay("")
	if err := os.WriteFile(config, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := check(); got["verdict"] != UpgradeRefused || got["reason"] != "CONFIG_INVALID" {
		t.Fatalf("config: %+v", got)
	}

	replay("not a result\n")
	if got := check(); got["verdict"] != UpgradeUnknown || got["reason"] != "PROBE_FAILED" {
		t.Fatalf("unreadable: %+v", got)
	}

	if res := run("dispatch", "upgrade", "--program", "prog", "--config", config, "--check", config); res.Outcome != wire.OutcomeError {
		t.Fatalf("a non-executable NEW_BUILD: %+v", res)
	}
	if res := run("dispatch", "upgrade", "--program", "prog", "--config", config); res.Outcome != wire.OutcomeError {
		t.Fatalf("no --check: %+v", res)
	}
}

// TestDSGV0011_UpgradeApplyRequestsOrRefuses covers `dispatch upgrade
// --apply NEW_BUILD` (GitHub #1815): after a ROLLING probe it writes one
// handover request for a running dispatcher that accepts handover, and
// refuses, writing nothing, a build without the handover protocol, a
// program with no running dispatcher, a dispatcher that recorded no
// handover support, a handover already executing, a fourth distinct live
// build, and a second request while one is pending.
func TestDSGV0011_UpgradeApplyRequestsOrRefuses(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(stateDir, "dispatch.json")
	cfg := `{"profile":"taskman-dispatch/0","stateDir":` + jsonQuote(stateDir) + `,"workRoot":` + jsonQuote(stateDir) +
		`,"tickSeconds":1,"globalCap":2,"killGraceSeconds":1,"hosts":{"h":{"argv":["/bin/echo","{prompt}"]}}` +
		`,"roles":[{"name":"impl","host":"h","cap":2,"match":{},"prompt":"work {ticketLocal}","idleSeconds":30,"wallSeconds":60}]` +
		`,"backoff":{"cooldownSeconds":0,"parkAfter":1},"heal":{"handoff":true,"reap":true}}`
	if err := os.WriteFile(config, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(stateDir, "prog")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Workers: []*dispatch.Worker{
			{ID: "w1", Role: "impl", Host: "h", Key: "ticket:a:q:t1", Ticket: "ticket:a:q:t1", PID: 1, Started: now},
			{ID: "w2", Role: "impl", Host: "h", Key: "ticket:a:q:t2", Ticket: "ticket:a:q:t2", PID: 2, Started: now}}}
	raw, _ := json.Marshal(l)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	sidecar := func(worker, sum string) {
		t.Helper()
		p := filepath.Join(dir, "workers", worker, "build.json")
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"profile":"taskman-dispatch-worker-build/0","worker":"`+worker+`","executable":"/old/corvint-tasks","sha256":"`+sum+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	upgradeState := func(phase string) {
		t.Helper()
		body := `{"profile":"taskman-dispatch-upgrade-state/0","phase":"` + phase + `","pid":` + strconv.Itoa(os.Getpid()) + `,"identity":"x","build":"/cur/corvint-tasks","sha256":"` + strings.Repeat("c", 64) + `","at":"2026-10-26T00:00:00Z"}`
		if err := os.WriteFile(filepath.Join(dir, "upgrade.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var status bytes.Buffer
	Run(Env{Cwd: stateDir, Args: []string{"dispatch", "status", "--program", "prog", "--config", config, "--events", "0"}, Stdin: bytes.NewReader(nil), Stdout: &status, Stderr: &bytes.Buffer{}})
	if !strings.Contains(status.String(), dispatch.HandoverProtocol) {
		t.Fatalf("status does not advertise %s: %s", dispatch.HandoverProtocol, status.String())
	}
	build := filepath.Join(stateDir, "new-build")
	replay := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(stateDir, "replay.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\n[ \"$1 $2 $7 $8\" = \"dispatch status --events 0\" ] || exit 2\ncat " + filepath.Join(stateDir, "replay.json") + "\n"
		if err := os.WriteFile(build, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	apply := func() (string, string) {
		t.Helper()
		var out bytes.Buffer
		Run(Env{Cwd: stateDir, Args: []string{"dispatch", "upgrade", "--program", "prog", "--config", config, "--apply", build}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &bytes.Buffer{}})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
			t.Fatalf("apply: %v %s", err, out.Bytes())
		}
		v, _ := res.Items[0].Obj.Get("verdict")
		r, _ := res.Items[0].Obj.Get("reason")
		return v.Str, r.Str
	}
	pending := func() bool {
		_, err := os.Lstat(dispatch.UpgradeRequestPath(dir))
		return err == nil
	}
	expect := func(verdict, reason string) {
		t.Helper()
		if v, r := apply(); v != verdict || r != reason {
			t.Fatalf("apply = %s/%s, want %s/%s", v, r, verdict, reason)
		}
		if reason != "HANDOVER_REQUESTED" && reason != "REQUEST_PENDING" && pending() {
			t.Fatalf("refusal %s wrote a request", reason)
		}
	}

	replay(status.String())
	expect(UpgradeRefused, "NOT_RUNNING")
	id, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lock"), []byte(strconv.Itoa(os.Getpid())+" "+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(UpgradeRefused, "NO_HANDOVER") // the running dispatcher recorded no handover support
	upgradeState(dispatch.UpgradeRunning)
	replay(strings.ReplaceAll(status.String(), dispatch.HandoverProtocol, "taskman-dispatch-handover/none"))
	expect(UpgradeRefused, "NO_HANDOVER") // the new build does not accept one
	replay(status.String())
	sidecar("w1", strings.Repeat("a", 64))
	sidecar("w2", strings.Repeat("b", 64))
	expect(UpgradeRefused, "MIXED_BUILD_LIMIT") // a, b, the running c and the new build
	sidecar("w2", strings.Repeat("a", 64))
	upgradeState(dispatch.UpgradeExec)
	expect(UpgradeRefused, "UPGRADE_IN_PROGRESS")
	upgradeState(dispatch.UpgradeRunning)
	expect(UpgradeRequested, "HANDOVER_REQUESTED")
	if !pending() {
		t.Fatal("no request written")
	}
	expect(UpgradeRefused, "REQUEST_PENDING")

	var out bytes.Buffer
	Run(Env{Cwd: stateDir, Args: []string{"dispatch", "upgrade", "--program", "prog", "--config", config, "--apply", build, "--check", build}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &bytes.Buffer{}})
	if res, err := wire.DecodeResult(out.Bytes()); err != nil || res.Outcome != wire.OutcomeError {
		t.Fatalf("--check with --apply: %s", out.Bytes())
	}
}
