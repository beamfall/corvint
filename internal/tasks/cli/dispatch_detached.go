package cli

import (
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// AttemptRuns reads an attempt's detached run records for the dispatcher
// (CAL-V0-145): at most maxRunsPerAttempt+1 entries, each record strictly
// decoded (ATR-V0-010) and kept only when it names this attempt, its own
// directory and a valid launch time. A missing run directory is no runs.
func (q dispatchQueue) AttemptRuns(attemptID string) ([]dispatch.DetachedRun, error) {
	repo, err := intent.Resolve(q.env.Cwd)
	if err != nil {
		return nil, err
	}
	base := runsDir(repo, attemptID)
	entries, err := listRuns(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []dispatch.DetachedRun
	for _, e := range entries {
		if !e.IsDir() || !validRunID(e.Name()) {
			continue
		}
		if run, ok := detachedRun(base, attemptID, e.Name()); ok {
			out = append(out, run)
		}
	}
	return out, nil
}

// AttemptRun reads one run record; a missing record is nil, and one that
// does not decode or names another run is an error.
func (q dispatchQueue) AttemptRun(attemptID, runID string) (*dispatch.DetachedRun, error) {
	if !validRunID(runID) {
		return nil, errors.New("invalid run id")
	}
	repo, err := intent.Resolve(q.env.Cwd)
	if err != nil {
		return nil, err
	}
	base := runsDir(repo, attemptID)
	if _, err := readRunRecord(filepath.Join(base, runID)); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	run, ok := detachedRun(base, attemptID, runID)
	if !ok {
		return nil, errors.New("run record " + runID + " does not name its attempt, run and launch time")
	}
	return &run, nil
}

func detachedRun(base, attemptID, runID string) (dispatch.DetachedRun, bool) {
	dir := filepath.Join(base, runID)
	rec, err := readRunRecord(dir)
	if err != nil || rec.AttemptID != attemptID || rec.RunID != runID {
		return dispatch.DetachedRun{}, false
	}
	at, err := time.Parse(time.RFC3339, rec.LaunchedAt)
	if err != nil {
		return dispatch.DetachedRun{}, false
	}
	run := dispatch.DetachedRun{RunID: rec.RunID, AttemptID: rec.AttemptID, Generation: rec.Generation, State: rec.State, SupervisorPID: rec.SupervisorPid, SupervisorIdentity: rec.SupervisorIdentity, TimeoutSeconds: rec.TimeoutSeconds, LaunchedAt: at, ExitStatus: rec.ExitStatus, Dir: dir, Output: filepath.Join(dir, runOutputName)}
	if rec.State == runFinished {
		run.Result = filepath.Join(dir, runResultName)
	}
	return run, true
}

// dispatchDetachedRuns is the CAL-V0-150 status view of the detached runs a
// dispatcher tracks: one bounded read of its markers and of each run's
// record. It reports false when there is nothing to show.
func dispatchDetachedRuns(env Env, dir string) (wire.Value, bool) {
	str := wire.String
	views, bad, err := dispatch.DetachedMarkers(dir)
	if err != nil {
		o := wire.NewObject().Set("state", str(dispatch.StateUnknown)).Set("reason", str(prose(err.Error())))
		return wire.Array(wire.ObjectValue(o)), true
	}
	if len(views) == 0 && len(bad) == 0 {
		return wire.Value{}, false
	}
	q := dispatchQueue{env: env}
	out := []wire.Value{}
	for _, v := range views {
		run, rerr := q.AttemptRun(v.AttemptID, v.RunID)
		o := wire.NewObject().Set("runId", str(v.RunID)).Set("attempt", str(v.AttemptID)).Set("ticket", str(v.Ticket)).Set("role", str(v.Role)).Set("worker", str(v.Worker)).Set("phase", str(v.Phase)).Set("state", str(dispatch.RunState(v, run, rerr))).Set("deferUntil", str(v.DeferUntil.UTC().Format(time.RFC3339)))
		if v.ExitStatus != nil {
			o.Set("exitStatus", str(strconv.Itoa(*v.ExitStatus)))
		}
		if v.Successor != "" {
			o.Set("successor", str(v.Successor))
		}
		out = append(out, wire.ObjectValue(o))
	}
	for _, name := range slices.Sorted(maps.Keys(bad)) {
		o := wire.NewObject().Set("marker", str(name)).Set("state", str(dispatch.StateUnknown)).Set("reason", str(prose(bad[name].Error())))
		out = append(out, wire.ObjectValue(o))
	}
	return wire.Array(out...), true
}
