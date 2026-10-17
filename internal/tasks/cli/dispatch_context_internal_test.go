package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// CAL-V0-342: dispatch status shows each worker's context estimate, its age,
// the highest recorded crossing and the applied role's thresholds; an
// unobserved, unread or unknown estimate is UNKNOWN, never 0.
func TestCALV0342_DispatchStatusShowsContextEstimate(t *testing.T) {
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

// CAL-V0-349: status shows the thresholds the dispatcher applied, from the
// worker's record, not the configuration file's: an invalid file (which
// leaves only the state directory) or a valid edit awaiting reload changes
// nothing, and a worker without a record shows UNKNOWN thresholds.
func TestCALV0349_DispatchStatusShowsAppliedThresholds(t *testing.T) {
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
