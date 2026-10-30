//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0150_StatusShowsDetachedRunStateReadOnly: dispatch status reports
// each tracked detached run's state from its marker and run record, reports
// a marker that does not decode as UNKNOWN, and writes nothing.
func TestCALV0150_StatusShowsDetachedRunStateReadOnly(t *testing.T) {
	root, a := attemptStore(t)
	base, err := cli.RunDirForTest(root, a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	self, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || self == "" {
		t.Fatalf("own identity: %q %v", self, err)
	}
	config, dir := readerCLIConfig(t, root, `printf '{}'`)
	markers := filepath.Join(dir, "detached")
	if err := os.MkdirAll(markers, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	marker := func(runID, identity string) {
		raw, _ := json.Marshal(map[string]any{"profile": "taskman-dispatch-detached-run/0", "runId": runID, "attemptId": a.AttemptID, "generation": gen(a),
			"ticket": "T-1", "role": "impl", "worker": "impl-1", "supervisorPid": os.Getpid(), "supervisorIdentity": identity, "phase": "DEFERRED",
			"deferredAt": now, "deferUntil": now.Add(time.Hour), "readyAt": time.Time{}, "exitStatus": nil, "result": "", "output": "", "successor": ""})
		fixture.Write(t, filepath.Join(markers, runID+".json"), raw)
	}
	want := map[string]string{"0000000000000001": "RUNNING", "0000000000000002": "SUPERVISOR_LOST", "0000000000000003": "MISSING", "0000000000000004": "REPLACED"}
	writeRecord(t, base, "0000000000000001", plantedRecord(a, "0000000000000001", "RUNNING", os.Getpid(), self))
	writeRecord(t, base, "0000000000000002", plantedRecord(a, "0000000000000002", "RUNNING", os.Getpid(), "forged"))
	writeRecord(t, base, "0000000000000004", plantedRecord(a, "0000000000000004", "RUNNING", os.Getpid(), self))
	marker("0000000000000001", self)
	marker("0000000000000002", "forged")
	marker("0000000000000003", self)
	marker("0000000000000004", "another")
	fixture.Write(t, filepath.Join(markers, "0000000000000005.json"), []byte(`{"profile":"taskman-dispatch-detached-run/0"}`))
	beforeState, beforeRuns := fixture.TreeSnapshot(t, dir), fixture.TreeSnapshot(t, base)

	r := atm(t, root, nil, "dispatch", "status", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("status: %s %s", r.stdout, r.stderr)
	}
	runs := field(r.res.Items[0], "detachedRuns").Arr
	if len(runs) != 5 {
		t.Fatalf("detachedRuns = %s", r.stdout)
	}
	for _, v := range runs {
		state := field(v, "state").Str
		if m, ok := v.Obj.Get("marker"); ok {
			if m.Str != "0000000000000005.json" || state != "UNKNOWN" {
				t.Fatalf("undecodable marker: %s", r.stdout)
			}
			continue
		}
		if id := field(v, "runId").Str; want[id] != state || field(v, "phase").Str != "DEFERRED" || field(v, "attempt").Str != a.AttemptID {
			t.Fatalf("run %s state %s, want %s: %s", id, state, want[id], r.stdout)
		}
	}
	if !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, dir)) || !fixture.SameTree(beforeRuns, fixture.TreeSnapshot(t, base)) {
		t.Fatal("dispatch status wrote dispatcher or run state")
	}
}
