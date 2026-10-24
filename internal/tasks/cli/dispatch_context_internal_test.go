package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-032: dispatch status shows each worker's context estimate, its age,
// the highest recorded crossing and the applied role's thresholds; an
// unobserved, unread or unknown estimate is UNKNOWN, never 0.
func TestATRV0032_DispatchStatusShowsContextEstimate(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	record := func(worker string, raw []byte) {
		t.Helper()
		path := dispatch.ContextRecordPath(dir, worker)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := dispatch.ContextRecord{Profile: dispatch.ContextProfile, Worker: "w1", Tokens: "150000", ObservedAt: now.Add(-42 * time.Second).Format(time.RFC3339),
		WarnTokens: "100000", HandoffTokens: "NONE", HardTokens: "NONE", Level: dispatch.ContextWarn,
		Crossings: []dispatch.ContextCrossing{{Level: dispatch.ContextWarn, Threshold: "100000", Tokens: "150000", At: now.Add(-42 * time.Second).Format(time.RFC3339)}}}
	raw, _ := json.Marshal(rec)
	record("w1", append(raw, '\n'))
	unknown := dispatch.ContextRecord{Profile: dispatch.ContextProfile, Worker: "w3", Tokens: dispatch.UsageUnknown, ObservedAt: now.Add(-5 * time.Second).Format(time.RFC3339),
		WarnTokens: "NONE", HandoffTokens: "NONE", HardTokens: "NONE", Level: dispatch.ContextNone, Crossings: []dispatch.ContextCrossing{}}
	raw, _ = json.Marshal(unknown)
	record("w3", append(raw, '\n'))
	record("w4", []byte(`{"profile":"taskman-dispatch-context/0","tokens":0}`))

	c := &dispatch.Config{Roles: []dispatch.Role{
		{Name: "impl", UsageFormat: "claude-code", Context: &dispatch.ContextLimits{WarnTokens: 100000, HardTokens: 180000}},
		{Name: "plain"},
	}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: "w1", Role: "impl", Key: "ticket:a:q:t1", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
		{ID: "w2", Role: "impl", Key: "ticket:a:q:t2", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
		{ID: "w3", Role: "impl", Key: "ticket:a:q:t3", Usage: &dispatch.WorkerUsage{Format: "opencode"}},
		{ID: "w4", Role: "impl", Key: "ticket:a:q:t4", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
		{ID: "w5", Role: "plain", Key: "ticket:a:q:t5"},
	}}
	workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, now), "workers")
	want := []map[string]string{
		// The recorded (applied) thresholds and crossing, with the age: the
		// file's not-yet-applied hardTokens 180000 is not shown.
		{"tokens": "150000", "observedAt": "2026-10-17T11:59:18Z", "ageSeconds": "42", "level": "WARN", "warnTokens": "100000", "handoffTokens": "NONE", "hardTokens": "NONE"},
		// No record yet: nothing observed, no applied thresholds known.
		{"tokens": "UNKNOWN", "observedAt": "UNKNOWN", "ageSeconds": "UNKNOWN", "level": "NONE", "warnTokens": "UNKNOWN", "handoffTokens": "UNKNOWN", "hardTokens": "UNKNOWN"},
		// An unknown estimate keeps its observation time, never 0 tokens.
		{"tokens": "UNKNOWN", "observedAt": "2026-10-17T11:59:55Z", "ageSeconds": "5", "level": "NONE", "warnTokens": "NONE", "hardTokens": "NONE"},
		// An unreadable record is UNKNOWN throughout.
		{"tokens": "UNKNOWN", "observedAt": "UNKNOWN", "ageSeconds": "UNKNOWN", "level": "UNKNOWN", "warnTokens": "UNKNOWN", "hardTokens": "UNKNOWN"},
		// A role without a context-capable usage format.
		{"tokens": "UNKNOWN", "ageSeconds": "UNKNOWN", "level": "NONE", "warnTokens": "NONE", "handoffTokens": "NONE", "hardTokens": "NONE"},
	}
	for i, w := range want {
		ctx, ok := statusField(t, workers.Arr[i], "context")
		if !ok {
			t.Fatalf("worker %d has no context", i)
		}
		statusStrings(t, ctx, w)
	}
}

// ATR-V0-039: status shows the thresholds the dispatcher applied, from the
// worker's record, not the configuration file's: an invalid file (which
// leaves only the state directory) or a valid edit awaiting reload changes
// nothing, and a worker without a record shows UNKNOWN thresholds.
func TestATRV0039_DispatchStatusShowsAppliedThresholds(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	rec := dispatch.ContextRecord{Profile: dispatch.ContextProfile, Worker: "w1", Tokens: "150000", ObservedAt: now.Format(time.RFC3339),
		WarnTokens: "100000", HandoffTokens: "NONE", HardTokens: "200000", Level: dispatch.ContextWarn,
		Crossings: []dispatch.ContextCrossing{{Level: dispatch.ContextWarn, Threshold: "100000", Tokens: "150000", At: now.Format(time.RFC3339)}}}
	raw, _ := json.Marshal(rec)
	path := dispatch.ContextRecordPath(dir, "w1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: "w1", Role: "impl", Key: "ticket:a:q:t1", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
		{ID: "w2", Role: "impl", Key: "ticket:a:q:t2", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
	}}
	edited := &dispatch.Config{Roles: []dispatch.Role{{Name: "impl", UsageFormat: "claude-code", Context: &dispatch.ContextLimits{WarnTokens: 5}}}}
	for name, c := range map[string]*dispatch.Config{"invalid file": {}, "edit awaiting reload": edited} {
		workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, now), "workers")
		for i, want := range []map[string]string{
			{"warnTokens": "100000", "handoffTokens": "NONE", "hardTokens": "200000", "level": "WARN"},
			{"warnTokens": "UNKNOWN", "handoffTokens": "UNKNOWN", "hardTokens": "UNKNOWN", "level": "NONE"},
		} {
			ctx, ok := statusField(t, workers.Arr[i], "context")
			if !ok {
				t.Fatalf("%s: worker %d has no context", name, i)
			}
			statusStrings(t, ctx, want)
		}
	}
}

// ATR-V0-054: a context record another build wrote shows UNKNOWN throughout,
// never a guessed level.
func TestATRV0054_DispatchStatusShowsNewerRecordUnknown(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := dispatch.ContextRecordPath(dir, "w1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"profile":"taskman-dispatch-context/1","worker":"w1","level":"HARD"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &dispatch.Config{Roles: []dispatch.Role{{Name: "impl", UsageFormat: "claude-code"}}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: "w1", Role: "impl", Key: "ticket:a:q:t1", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
	}}
	workers, _ := statusField(t, dispatchStatusValue(c, dir, l, nil, now), "workers")
	ctx, ok := statusField(t, workers.Arr[0], "context")
	if !ok {
		t.Fatal("no context")
	}
	statusStrings(t, ctx, map[string]string{"tokens": "UNKNOWN", "observedAt": "UNKNOWN", "level": "UNKNOWN", "warnTokens": "UNKNOWN", "handoffTokens": "UNKNOWN", "hardTokens": "UNKNOWN"})
}

// ATR-V0-056: `dispatch status --worker ID` shows that worker's context
// notice read from its record, writes nothing, and refuses an ID the
// ledger does not name.
func TestATRV0056_DispatchStatusWorkerShowsContextNotice(t *testing.T) {
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(stateDir, "prog")
	at := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	rec := dispatch.ContextRecord{Profile: dispatch.ContextProfile, Worker: "prog.impl.1.ab-1", Tokens: "150000", ObservedAt: at,
		WarnTokens: "100000", HandoffTokens: "140000", HardTokens: "200000", Level: dispatch.ContextHandoff,
		Crossings: []dispatch.ContextCrossing{{Level: dispatch.ContextWarn, Threshold: "100000", Tokens: "120000", At: at}, {Level: dispatch.ContextHandoff, Threshold: "140000", Tokens: "150000", At: at}}}
	raw, _ := json.Marshal(rec)
	path := dispatch.ContextRecordPath(dir, rec.Worker)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: rec.Worker, Role: "impl", Key: "ticket:a:q:t1", Usage: &dispatch.WorkerUsage{Format: "claude-code"}},
	}}
	if raw, err = json.Marshal(l); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(stateDir, "dispatch.json")
	if err := os.WriteFile(config, []byte(`{"profile":"nope","stateDir":`+jsonQuote(stateDir)+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cli := func(args ...string) *wire.Result {
		t.Helper()
		var out, errb bytes.Buffer
		// Off darwin and linux Tasks refuses the command up front (CTS-V0-025).
		fixture.SkipUnsupportedPlatform(t)
		Run(Env{Cwd: stateDir, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return res
	}
	before, _ := os.ReadFile(path)
	res := cli("dispatch", "status", "--program", "prog", "--config", config, "--worker", rec.Worker)
	// ATR-V0-134: the config file here is INVALID, so status keeps the
	// view and fails the command.
	if res.Outcome != wire.OutcomeError || len(res.Items) != 1 {
		t.Fatalf("status --worker: %+v", res)
	}
	notice, ok := statusField(t, res.Items[0], "contextNotice")
	if !ok {
		t.Fatalf("no contextNotice: %+v", res.Items[0])
	}
	statusStrings(t, notice, map[string]string{"worker": rec.Worker, "file": path, "tokens": "150000", "level": "HANDOFF", "warnTokens": "100000", "handoffTokens": "140000", "hardTokens": "200000", "observedAt": at})
	crossings, _ := statusField(t, notice, "crossings")
	if len(crossings.Arr) != 2 {
		t.Fatalf("crossings %+v", crossings)
	}
	statusStrings(t, crossings.Arr[1], map[string]string{"level": "HANDOFF", "threshold": "140000", "tokens": "150000", "at": at})
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("status --worker rewrote the record")
	}
	if res := cli("dispatch", "status", "--program", "prog", "--config", config); res.Outcome != wire.OutcomeError || len(res.Items) != 1 {
		t.Fatalf("status: %+v", res)
	} else if _, ok := statusField(t, res.Items[0], "contextNotice"); ok {
		t.Fatal("contextNotice without --worker")
	}
	if res := cli("dispatch", "status", "--program", "prog", "--config", config, "--worker", "../other"); res.Outcome == wire.OutcomeOK {
		t.Fatalf("unknown worker accepted: %+v", res)
	}
}
