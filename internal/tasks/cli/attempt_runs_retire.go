package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/store"
)

const (
	// maxRetainedEndedRuns is how many ended runs of terminal attempts stay
	// for inspection once they are quiet (ATR-V0-015).
	maxRetainedEndedRuns = 16
	// maxRunAttemptScan bounds one retirement pass over taskman-runs; a
	// larger backlog shrinks over later passes.
	maxRunAttemptScan = 1024
	// endedRunQuiet keeps a run whose files changed this recently, so an
	// attach that is reading a just-finished run still finds it.
	endedRunQuiet = time.Hour
	// retiredRunPrefix names a run directory moved out of its attempt's
	// directory before removal, so a reader never sees a half-removed run.
	retiredRunPrefix = ".retired-"
)

// retireEndedRuns removes the run directories of terminal attempts beyond the
// newest maxRetainedEndedRuns (ATR-V0-015). A supervisor calls it after its
// own run's FINISHED record is written. A run is removed only when its record
// decodes and names its own directory, it has ended (FINISHED, or both its
// supervisor and its recorded command are proven gone), it is quiet, and one
// audited snapshot holds its attempt in a terminal phase at the run's
// generation or a later one. Anything unproven is kept, and every failure is
// left for a later pass.
func (r *attemptRunner) retireEndedRuns() {
	root := filepath.Join(r.repo.CommonDir, "taskman-runs")
	f, err := os.Open(root)
	if err != nil {
		return
	}
	attempts, _ := f.ReadDir(maxRunAttemptScan)
	f.Close()
	type endedRun struct {
		dir, base, attemptID string
		generation           uint64
		mtime                time.Time
	}
	var ended []endedRun
	now := time.Now()
	for _, a := range attempts {
		if !a.IsDir() {
			continue
		}
		if strings.HasPrefix(a.Name(), retiredRunPrefix) {
			_ = os.RemoveAll(filepath.Join(root, a.Name())) // an earlier pass stopped mid-removal
			continue
		}
		base := filepath.Join(root, a.Name())
		runs, err := listRuns(base)
		if err != nil {
			continue
		}
		for _, e := range runs {
			if !e.IsDir() || !validRunID(e.Name()) {
				continue
			}
			dir := filepath.Join(base, e.Name())
			rec, err := readRunRecord(dir)
			if err != nil || rec.RunID != e.Name() || attemptRunsName(rec.AttemptID) != a.Name() || !runEnded(rec) {
				continue
			}
			generation, err := strconv.ParseUint(rec.Generation, 10, 64)
			if err != nil {
				continue
			}
			newest, ok := newestRunChange(dir)
			if !ok || now.Sub(newest) < endedRunQuiet {
				continue
			}
			ended = append(ended, endedRun{dir, base, rec.AttemptID, generation, newest})
		}
	}
	if len(ended) <= maxRetainedEndedRuns {
		return
	}
	seen := map[string]bool{}
	var ids []string
	for _, e := range ended {
		if !seen[e.attemptID] {
			seen[e.attemptID] = true
			ids = append(ids, e.attemptID)
		}
	}
	sort.Strings(ids)
	ctx, cancel := bounded()
	defer cancel()
	records, err := store.AttemptRecords(ctx, r.repo, ids)
	if err != nil {
		return
	}
	// A retry keeps the attempt ID and opens a newer generation, so a run is
	// removed only when the terminal record is at its generation or later.
	terminal := ended[:0]
	for _, e := range ended {
		if a := records[e.attemptID]; a != nil && !a.Live() && e.generation <= a.Generation.Uint64() {
			terminal = append(terminal, e)
		}
	}
	if len(terminal) <= maxRetainedEndedRuns {
		return
	}
	sort.Slice(terminal, func(i, j int) bool {
		if !terminal[i].mtime.Equal(terminal[j].mtime) {
			return terminal[i].mtime.After(terminal[j].mtime)
		}
		return terminal[i].dir > terminal[j].dir
	})
	for _, e := range terminal[maxRetainedEndedRuns:] {
		// The run leaves its attempt's directory in one rename, so attach
		// finds either the whole run or none of it.
		moved := filepath.Join(root, retiredRunPrefix+filepath.Base(e.base)+"-"+filepath.Base(e.dir))
		if os.Rename(e.dir, moved) != nil {
			continue
		}
		_ = os.RemoveAll(moved)
		_ = os.Remove(e.base) // only when no run is left
	}
}

// attemptRunsName is the attempt's directory name under taskman-runs.
func attemptRunsName(attemptID string) string {
	sum := sha256.Sum256([]byte(attemptID))
	return hex.EncodeToString(sum[:16])
}

// runEnded reports a run proven over: FINISHED, or neither its supervisor nor
// its recorded command is live. A run with no recorded command identity, or
// whose liveness cannot be read, is not proven over.
func runEnded(rec *runRecord) bool {
	if rec.State == runFinished {
		return true
	}
	if live, err := processLive(rec.SupervisorPid, rec.SupervisorIdentity); err != nil || live {
		return false
	}
	if rec.CommandPid == nil || rec.CommandIdentity == nil {
		return false
	}
	live, err := processLive(*rec.CommandPid, *rec.CommandIdentity)
	return err == nil && !live
}

// newestRunChange is the latest modification time of a run directory and its
// files; an unreadable directory reports false and is kept.
func newestRunChange(dir string) (time.Time, bool) {
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() {
		return time.Time{}, false
	}
	newest := st.ModTime()
	for _, name := range []string{runRecordName, runResultName, runOutputName, runOutputPrevName, runSupervisorLogName} {
		if st, err := os.Lstat(filepath.Join(dir, name)); err == nil && st.ModTime().After(newest) {
			newest = st.ModTime()
		}
	}
	return newest, true
}
