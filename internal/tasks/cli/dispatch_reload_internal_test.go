package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-125: status reports the derived CPU utilization, UNKNOWN until a
// tick has a previous sample to compare with.
func TestCALV0125_DispatchStatusCPUUtilization(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog", Pressure: &dispatch.PressureRecord{
		Sample: dispatch.PressureSample{SampledAt: now, Source: "linux-proc-host", CPUUtilization: .625, CPUUtilizationKnown: true},
		Held:   []dispatch.HeldLaunch{},
	}}
	p := pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure")
	if got := pressureField(t, p, "cpuUtilization").Str; got != "0.625" {
		t.Fatalf("cpuUtilization = %q", got)
	}
	l.Pressure.Sample.CPUUtilizationKnown = false
	p = pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure")
	if got := pressureField(t, p, "cpuUtilization").Str; got != "UNKNOWN" {
		t.Fatalf("unknown cpuUtilization = %q", got)
	}
}

// CAL-V0-127: status reports this run's reload record, the refused change
// (UNKNOWN digest when unreadable) or NONE, and omits it before any change.
func TestCALV0127_DispatchStatusConfigRecord(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog"}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config"); ok {
		t.Fatal("status reported a config record before any change")
	}
	sum := strings.Repeat("b", 64)
	l.Config = &dispatch.ConfigRecord{AppliedSha256: sum, AppliedAt: now}
	v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config")
	if !ok || pressureField(t, v, "appliedSha256").Str != sum || pressureField(t, v, "appliedAt").Str != "2026-10-06T00:00:00Z" || pressureField(t, v, "refused").Str != "NONE" {
		t.Fatalf("config = %+v", v)
	}
	l.Config.Refused = &dispatch.ConfigRefusal{At: now, Reason: "read: gone"}
	v, _ = statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config")
	r := pressureField(t, v, "refused")
	if pressureField(t, r, "sha256").Str != "UNKNOWN" || pressureField(t, r, "reason").Str != "read: gone" || pressureField(t, r, "at").Str != "2026-10-06T00:00:00Z" {
		t.Fatalf("refused = %+v", r)
	}
}

// CAL-V0-127: `dispatch status` reads the ledger and shows the recorded
// reload refusal even when the configuration file no longer decodes; the
// file is reported INVALID and the retry policy UNKNOWN. Unpark stays strict.
func TestCALV0127_DispatchStatusWithInvalidConfigFile(t *testing.T) {
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{},
		InfraRetry: map[string]*dispatch.InfraEpisode{"ticket:a:q:t1": {AcceptanceRevision: "1", State: dispatch.InfraRunning, Sessions: 1, Charged: 1, Limit: 3, CooldownUntil: now, Launch: "prog.impl.1.ab-3"}},
		Config:     &dispatch.ConfigRecord{AppliedSha256: strings.Repeat("a", 64), AppliedAt: now, Refused: &dispatch.ConfigRefusal{Sha256: strings.Repeat("b", 64), At: now, Reason: "dispatch config: bad"}}}
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "prog"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "prog", "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "dispatch.json")
	cli := func(args ...string) *wire.Result {
		t.Helper()
		var out, errb bytes.Buffer
		Run(Env{Cwd: stateDir, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return res
	}
	for name, file := range map[string]string{
		"malformed": `{"profile":"taskman-dispatch/0","stateDir":` + jsonQuote(stateDir) + `,"roles":[{`,
		"invalid":   `{"profile":"nope","stateDir":` + jsonQuote(stateDir) + `}`,
	} {
		if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
		res := cli("dispatch", "status", "--program", "prog", "--config", path)
		if res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: status refused: %+v", name, res)
		}
		st := res.Items[0]
		cfg, ok := statusField(t, st, "config")
		if !ok || pressureField(t, pressureField(t, cfg, "refused"), "reason").Str != "dispatch config: bad" {
			t.Fatalf("%s: recorded refusal not shown: %+v", name, st)
		}
		file, ok := statusField(t, st, "configFile")
		if !ok || pressureField(t, file, "state").Str != "INVALID" || len(pressureField(t, file, "sha256").Str) != 64 || pressureField(t, file, "reason").Str == "" {
			t.Fatalf("%s: configFile %+v", name, file)
		}
		if ir, _ := statusField(t, st, "infrastructureRetry"); pressureField(t, ir, "policy").Str != "UNKNOWN" {
			t.Fatalf("%s: retry policy %+v", name, ir)
		}
		if res := cli("dispatch", "unpark", "--program", "prog", "--config", path, "--key", "ticket:a:q:t1"); res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s: unpark accepted an invalid configuration", name)
		}
	}
	for name, file := range map[string]string{
		"no stateDir":  `{"profile":"nope"`,
		"relative":     `{"stateDir":"rel"}`,
		"duplicate":    `{"stateDir":` + jsonQuote(stateDir) + `,"stateDir":` + jsonQuote(stateDir) + `}`,
		"alias":        `{"StateDir":` + jsonQuote(stateDir) + `}`,
		"after error":  `{"profile":}"stateDir":` + jsonQuote(stateDir) + `}`,
		"not a string": `{"stateDir":["/x"]}`,
	} {
		if dir, ok := dispatch.ConfigStateDir([]byte(file)); ok {
			t.Fatalf("%s: recovered %q", name, dir)
		}
	}
}

func jsonQuote(s string) string { b, _ := json.Marshal(s); return string(b) }
