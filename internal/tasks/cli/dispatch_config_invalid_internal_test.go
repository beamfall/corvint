package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-134: `dispatch status` exits 1 with outcome ERROR when the file
// on disk does not validate, naming the reason, and keeps the status view.
// ATR-V0-135: a restart on that file refuses before it reads or writes the
// ledger, even though the ledger records an applied configuration.
func TestATRV0134_StatusFailsOnInvalidConfigFile(t *testing.T) {
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{},
		Config: &dispatch.ConfigRecord{AppliedSha256: strings.Repeat("a", 64), AppliedAt: now}}
	ledger, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "prog", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, ledger, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "dispatch.json")
	config := func(argv string) string {
		return `{"profile":"taskman-dispatch/0","stateDir":` + jsonQuote(stateDir) + `,"workRoot":` + jsonQuote(stateDir) +
			`,"tickSeconds":1,"globalCap":1,"killGraceSeconds":1,"hosts":{"h":{"argv":` + argv + `}}` +
			`,"roles":[{"name":"impl","host":"h","cap":1,"match":{},"prompt":"work {ticketLocal}","idleSeconds":30,"wallSeconds":60,"carryOver":{}}]` +
			`,"backoff":{"cooldownSeconds":0,"parkAfter":1},"heal":{"handoff":true,"reap":true}}`
	}
	run := func(args ...string) (int, *wire.Result) {
		t.Helper()
		var out, errb bytes.Buffer
		// Off darwin and linux Tasks refuses the command up front (CTS-V0-025).
		fixture.SkipUnsupportedPlatform(t)
		rc := Run(Env{Cwd: stateDir, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return rc, res
	}
	status := []string{"dispatch", "status", "--program", "prog", "--config", path}

	// A templated session title next to a whole {prompt} validates.
	if err := os.WriteFile(path, []byte(config(`["/bin/echo","--title","x {role} {ticketLocal} {worker}","{prompt}"]`)), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc, res := run(status...); rc != 0 || res.Outcome != wire.OutcomeOK {
		t.Fatalf("valid file: rc %d %+v", rc, res)
	}

	// {prompt} spliced into another element does not.
	if err := os.WriteFile(path, []byte(config(`["/bin/echo","--prompt={prompt}"]`)), 0o600); err != nil {
		t.Fatal(err)
	}
	const reason = "splices {prompt} into argv element 1"
	rc, res := run(status...)
	if rc != 1 || res.Outcome != wire.OutcomeError || !slices.Equal(res.Codes, []string{wire.CodeMalformed}) || len(res.Items) != 1 {
		t.Fatalf("invalid file: rc %d %+v", rc, res)
	}
	var named bool
	for _, w := range res.Warnings {
		named = named || strings.Contains(w, "INVALID") && strings.Contains(w, reason)
	}
	file, ok := statusField(t, res.Items[0], "configFile")
	if !named || !ok || pressureField(t, file, "state").Str != "INVALID" || !strings.Contains(pressureField(t, file, "reason").Str, reason) {
		t.Fatalf("reason not named: warnings %q, configFile %+v", res.Warnings, file)
	}

	// ATR-V0-135: the restart drops to no dispatcher.
	rc, res = run("dispatch", "--once", "--program", "prog", "--config", path)
	if rc != 1 || res.Outcome != wire.OutcomeError || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], reason) {
		t.Fatalf("restart: rc %d %+v", rc, res)
	}
	if after, err := os.ReadFile(statePath); err != nil || !bytes.Equal(after, ledger) {
		t.Fatalf("restart touched the ledger: %v", err)
	}
}
