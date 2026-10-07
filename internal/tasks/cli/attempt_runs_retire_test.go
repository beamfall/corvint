//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts plants quiet
// runs under two released attempts and one live attempt, then finishes a
// detached run of the live attempt. Its supervisor keeps the newest 16 ended
// runs of the terminal attempts and removes the rest, including a run whose
// supervisor and command are both gone, and an attempt directory left empty.
// A run whose supervisor is live, a malformed run, a misplaced run and every
// run of the live attempt are kept.
func TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("launches a detached run")
	}
	root, claimed := leaseCLIStore(t, 3, time.Now().UTC().Add(-12*time.Minute))
	a, b, c := claimed[0], claimed[1], claimed[2]
	detachedEnv(t)
	baseOf := func(id string) string {
		base, err := cli.RunDirForTest(root, id)
		if err != nil {
			t.Fatal(err)
		}
		return base
	}
	baseA, baseB, baseC := baseOf(a.AttemptID), baseOf(b.AttemptID), baseOf(c.AttemptID)
	self, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || self == "" {
		t.Fatalf("own identity: %q %v", self, err)
	}
	now := time.Now()
	age := func(dir string, at time.Time) {
		t.Helper()
		for _, p := range []string{filepath.Join(dir, "record.json"), dir} {
			if err := os.Chtimes(p, at, at); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
	finished := func(base string, owner any, runID string, at time.Time) string {
		rec := plantedRecord(a, runID, "FINISHED", os.Getpid(), self)
		rec["attemptId"] = owner
		rec["endedAt"], rec["exitStatus"], rec["resultSha256"] = "2026-10-06T00:00:01Z", 0, "0000000000000000000000000000000000000000000000000000000000000000"
		dir := writeRecord(t, base, runID, rec)
		age(dir, at)
		return dir
	}
	old := now.Add(-2 * time.Hour)
	var kept, retired []string
	for i := 0; i < 18; i++ {
		dir := finished(baseA, a.AttemptID, fmt.Sprintf("00000000000001%02x", i), old.Add(-time.Duration(i)*time.Minute))
		if i < 16 {
			kept = append(kept, dir)
		} else {
			retired = append(retired, dir)
		}
	}
	// Both the supervisor and the command of this run are gone: it ended.
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	dead := plantedRecord(a, "00000000000002aa", "RUNNING", gone.Process.Pid, "gone-supervisor")
	dead["commandIdentity"] = "gone-command"
	deadDir := writeRecord(t, baseA, "00000000000002aa", dead)
	age(deadDir, old.Add(-30*time.Minute))
	retired = append(retired, deadDir)
	liveRun := plantedRecord(a, "00000000000003aa", "RUNNING", os.Getpid(), self)
	liveRun["commandIdentity"] = self
	liveDir := writeRecord(t, baseA, "00000000000003aa", liveRun)
	age(liveDir, old.Add(-5*time.Hour))
	malformed := filepath.Join(baseA, "00000000000004aa")
	if err := os.MkdirAll(malformed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, "record.json"), []byte("{\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	age(malformed, old.Add(-5*time.Hour))
	cDir := finished(baseC, c.AttemptID, "00000000000005aa", old.Add(-10*time.Hour))
	retired = append(retired, cDir)
	var liveAttemptRuns []string
	for i := 0; i < 3; i++ {
		liveAttemptRuns = append(liveAttemptRuns, finished(baseB, b.AttemptID, fmt.Sprintf("00000000000006%02x", i), old.Add(-20*time.Hour)))
	}
	// A run recorded at a generation newer than the terminal record is kept:
	// a retry reuses the attempt ID at a newer generation.
	newer := finished(baseA, a.AttemptID, "00000000000008aa", old.Add(-30*time.Hour))
	rewriteGeneration(t, newer, "999")
	leftover := filepath.Join(filepath.Dir(baseA), ".retired-"+filepath.Base(baseA)+"-00000000000009aa")
	if err := os.MkdirAll(leftover, 0o700); err != nil {
		t.Fatal(err)
	}
	misplaced := finished(filepath.Join(filepath.Dir(baseA), "0123456789abcdef0123456789abcdef"), a.AttemptID, "00000000000007aa", old.Add(-20*time.Hour))
	releaseAttempt(t, root, a, "retire-release-a")
	releaseAttempt(t, root, c, "retire-release-c")
	// 20 ended quiet runs of terminal attempts: a's 18 FINISHED, its dead
	// tree and c's one run. The newest 16 stay; c's run is the oldest and the
	// last removed, which also removes c's emptied directory.

	r := runAttempt(t, root, b, gen(b), []string{"--timeout", "30", "--detach"}, "/bin/sh", "-c", "exit 0")
	if len(r.res.Items) != 1 {
		t.Fatalf("launch: code %d %s", r.code, r.stdout)
	}
	runID := field(r.res.Items[0], "runId").Str
	if r := attach(t, root, b, "--run", runID, "--wait", "60"); r.code != 0 || r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach: code %d %s", r.code, r.stdout)
	}
	for i := 0; ; i++ {
		if _, err := os.Stat(baseC); os.IsNotExist(err) {
			break
		}
		if i == 400 {
			t.Fatal("the supervisor did not retire the ended runs")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("a run left mid-removal was kept: %v", err)
	}
	for _, dir := range retired {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s was kept: %v", dir, err)
		}
	}
	keptAll := append(append(append([]string{}, kept...), liveDir, malformed, misplaced, newer, filepath.Join(baseB, runID)), liveAttemptRuns...)
	for _, dir := range keptAll {
		if _, err := os.Stat(filepath.Join(dir, "record.json")); err != nil {
			t.Errorf("%s was retired: %v", dir, err)
		}
	}
}

func rewriteGeneration(t *testing.T, dir, generation string) {
	t.Helper()
	p := filepath.Join(dir, "record.json")
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	raw, err := os.ReadFile(p)
	if err == nil {
		err = json.Unmarshal(raw, &rec)
	}
	if err != nil {
		t.Fatal(err)
	}
	rec["generation"] = generation
	if raw, err = json.Marshal(rec); err == nil {
		err = os.WriteFile(p, append(raw, '\n'), 0o600)
	}
	if err == nil {
		err = os.Chtimes(p, st.ModTime(), st.ModTime())
	}
	if err != nil {
		t.Fatal(err)
	}
}
