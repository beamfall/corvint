package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
)

// maxRetainedEndedRuns is how many ended runs of terminal attempts stay for
// inspection once they are quiet (ATR-V0-015). maxRunAttemptScan bounds the
// attempt directories one retirement pass examines: a window that resumes
// after the previous pass's, plus the attempts of the retained runs carried
// from it (ATR-V0-154). maxRunAttemptNames bounds the names a pass lists to
// find its window; listing reads names only. Tests lower all three.
var (
	maxRetainedEndedRuns = 16
	maxRunAttemptScan    = 1024
	maxRunAttemptNames   = 64 * 1024
)

const (
	// runRetireCursorName is the ATR-V0-154 cursor beside the attempt
	// directories: where the next window starts and the carried attempts.
	runRetireCursorName = ".retire-cursor"
	// endedRunQuiet keeps a run whose files changed this recently, so an
	// attach that is reading a just-finished run still finds it.
	endedRunQuiet = time.Hour
	// retiredRunPrefix names a run directory moved out of its attempt's
	// directory before removal, so a reader never sees a half-removed run.
	retiredRunPrefix = ".retired-"
	// retiringRunPrefix names a run moved out of its attempt's directory
	// while the pass reads the terminal records again (ATR-V0-171). Any pass
	// that finds one restores it, so a pass that stopped keeps the run.
	retiringRunPrefix = ".retiring-"
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
	retireEndedRunsIn(filepath.Join(r.repo.CommonDir, "taskman-runs"), func(ctx context.Context, ids []string) (map[string]*snapshot.Attempt, error) {
		return store.AttemptRecords(ctx, r.repo, ids)
	})
}

// retireEndedRunsIn is one ATR-V0-015 pass over root's window (ATR-V0-154).
// The cursor advances only after a pass that read every terminal record it
// needed, so a failed lookup retries the same window.
func retireEndedRunsIn(root string, records func(context.Context, []string) (map[string]*snapshot.Attempt, error)) {
	window, next, ok := runRetireWindow(root)
	if !ok {
		return
	}
	type endedRun struct {
		dir, base, attemptID string
		generation           uint64
		mtime                time.Time
	}
	var ended []endedRun
	now := time.Now()
	for _, name := range window {
		if st, err := os.Lstat(filepath.Join(root, name)); err != nil || !st.IsDir() {
			continue
		}
		if strings.HasPrefix(name, retiredRunPrefix) {
			_ = os.RemoveAll(filepath.Join(root, name)) // an earlier pass stopped mid-removal
			continue
		}
		if strings.HasPrefix(name, retiringRunPrefix) {
			restoreRetiringRun(root, name) // a pass stopped, or is still checking
			continue
		}
		base := filepath.Join(root, name)
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
			if err != nil || rec.RunID != e.Name() || attemptRunsName(rec.AttemptID) != name || !runEnded(rec) {
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
	newestFirst := func(runs []endedRun) {
		sort.Slice(runs, func(i, j int) bool {
			if !runs[i].mtime.Equal(runs[j].mtime) {
				return runs[i].mtime.After(runs[j].mtime)
			}
			return runs[i].dir > runs[j].dir
		})
	}
	// The attempts of the retained runs are carried to the next window, so
	// one sweep leaves only the newest of all, not the newest of each window.
	carry := func(runs []endedRun) {
		seen := map[string]bool{}
		for _, e := range runs[:min(len(runs), maxRetainedEndedRuns)] {
			if name := filepath.Base(e.base); !seen[name] {
				seen[name] = true
				next.Carry = append(next.Carry, name)
			}
		}
		writeRunRetireCursor(root, next)
	}
	if len(ended) <= maxRetainedEndedRuns {
		carry(ended)
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
	terminalOf, err := records(ctx, ids)
	if err != nil {
		return
	}
	// A retry keeps the attempt ID and opens a newer generation, so a run is
	// removed only when the terminal record is at its generation or later.
	terminal := ended[:0]
	for _, e := range ended {
		if a := terminalOf[e.attemptID]; a != nil && !a.Live() && e.generation <= a.Generation.Uint64() {
			terminal = append(terminal, e)
		}
	}
	newestFirst(terminal)
	carry(terminal)
	if len(terminal) <= maxRetainedEndedRuns {
		return
	}
	// The run leaves its attempt's directory in one rename, so attach finds
	// either the whole run or none of it. A retry can revive the attempt
	// after the read above, so the runs are hidden first and removed only
	// when a second read, begun after every one left, still finds the
	// attempt terminal at the run's generation (ATR-V0-171).
	var moved []endedRun
	ids = nil
	seen = map[string]bool{}
	for _, e := range terminal[maxRetainedEndedRuns:] {
		if os.Rename(e.dir, filepath.Join(root, retiringRunName(e.base, e.dir))) != nil {
			continue
		}
		moved = append(moved, e)
		if !seen[e.attemptID] {
			seen[e.attemptID] = true
			ids = append(ids, e.attemptID)
		}
	}
	if len(moved) == 0 {
		return
	}
	sort.Strings(ids)
	again, err := records(ctx, ids)
	for _, e := range moved {
		name := retiringRunName(e.base, e.dir)
		if a := again[e.attemptID]; err != nil || a == nil || a.Live() || e.generation > a.Generation.Uint64() {
			restoreRetiringRun(root, name)
			continue
		}
		retired := filepath.Join(root, retiredRunPrefix+strings.TrimPrefix(name, retiringRunPrefix))
		if os.Rename(filepath.Join(root, name), retired) != nil {
			continue // another pass restored it
		}
		_ = os.RemoveAll(retired)
		_ = os.Remove(e.base) // only when no run is left
	}
}

// retiringRunName is the root entry that holds the run at dir of the attempt
// directory base while a pass rechecks it (ATR-V0-171).
func retiringRunName(base, dir string) string {
	return retiringRunPrefix + filepath.Base(base) + "-" + filepath.Base(dir)
}

// restoreRetiringRun moves a hidden run back into its attempt's directory.
// A name that does not parse, or a run that cannot be moved back, is left;
// no pass removes a .retiring- entry.
func restoreRetiringRun(root, name string) {
	attempt, run, ok := strings.Cut(strings.TrimPrefix(name, retiringRunPrefix), "-")
	if !ok || !runAttemptName(attempt) || !validRunID(run) {
		return
	}
	base := filepath.Join(root, attempt)
	if os.MkdirAll(base, 0o700) != nil {
		return
	}
	_ = os.Rename(filepath.Join(root, name), filepath.Join(base, run))
}

// runRetireCursor is where the next ATR-V0-154 window starts: after the
// attempt directory name After ("" starts over), with the attempts of the
// retained runs carried into it.
type runRetireCursor struct {
	After string   `json:"after"`
	Carry []string `json:"carry"`
}

// runRetireWindow lists at most maxRunAttemptNames names of root and returns
// the carried attempts plus, in name order, the window names (runWindowName)
// after the cursor, at most maxRunAttemptScan in all. Other names are passed
// over, so the cursor always ends on a name it can read back. A window that
// reaches the last name starts the next one over. A cursor that does not
// read back is ignored, and the sweep starts over.
func runRetireWindow(root string) (window []string, next runRetireCursor, ok bool) {
	f, err := os.Open(root)
	if err != nil {
		return nil, next, false
	}
	names, err := f.Readdirnames(maxRunAttemptNames)
	f.Close()
	if err != nil && err != io.EOF {
		return nil, next, false
	}
	cur := readRunRetireCursor(root)
	sort.Strings(names)
	window = append(window, cur.Carry...)
	carried := map[string]bool{}
	for _, name := range cur.Carry {
		carried[name] = true
	}
	i := sort.SearchStrings(names, cur.After)
	if i < len(names) && names[i] == cur.After {
		i++
	}
	for ; i < len(names) && len(window) < maxRunAttemptScan; i++ {
		if runWindowName(names[i]) && !carried[names[i]] {
			window = append(window, names[i])
			next.After = names[i]
		}
	}
	if i == len(names) {
		next.After = ""
	}
	return window, next, true
}

func readRunRetireCursor(root string) runRetireCursor {
	var cur runRetireCursor
	raw, err := readBounded(filepath.Join(root, runRetireCursorName), 64<<10)
	if err != nil || json.Unmarshal(raw, &cur) != nil || (cur.After != "" && !runWindowName(cur.After)) || len(cur.Carry) > maxRetainedEndedRuns {
		return runRetireCursor{}
	}
	seen := map[string]bool{}
	for _, name := range cur.Carry {
		// A repeated attempt would count its runs again and could push a
		// distinct retained run out of the newest set.
		if !runAttemptName(name) || seen[name] {
			return runRetireCursor{}
		}
		seen[name] = true
	}
	return cur
}

// writeRunRetireCursor replaces the cursor in one rename of a temporary
// file of its own, so concurrent writers never share an inode and the last
// rename wins whole; a failed write leaves the old one, and the next pass
// repeats its window.
func writeRunRetireCursor(root string, cur runRetireCursor) {
	raw, err := json.Marshal(cur)
	if err != nil {
		return
	}
	f, err := os.CreateTemp(root, runRetireCursorName+"-*.tmp")
	if err != nil {
		return
	}
	tmp := f.Name()
	_, err = f.Write(append(raw, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || os.Rename(tmp, filepath.Join(root, runRetireCursorName)) != nil {
		_ = os.Remove(tmp)
	}
}

// runWindowName reports a root entry a retirement window examines, and so a
// name the cursor may resume after: an attempt directory name, a retired run
// an earlier pass did not finish removing, or a run a pass hid to recheck.
func runWindowName(s string) bool {
	return runAttemptName(s) || ((strings.HasPrefix(s, retiredRunPrefix) || strings.HasPrefix(s, retiringRunPrefix)) && len(s) <= 255 && !strings.ContainsAny(s, "/\x00"))
}

// runAttemptName reports a name attemptRunsName can produce.
func runAttemptName(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
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
