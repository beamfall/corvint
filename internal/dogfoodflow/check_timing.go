package dogfoodflow

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// checkTimings is the file, in the evidence directory, that keeps one row per
// phase of a dogfood check or seal, and checkBuildTimings the prefix of the
// file script/dogfood-check.sh leaves there for the check it execs, named
// checkBuildTimings + PID + ".tsv" (DCW-V0-942).
const (
	checkTimings      = "dogfood-check.timings.tsv"
	checkBuildTimings = "dogfood-check.build."
	checkBuildProfile = "corvint-dogfood-check-build/0"
	// maxBuildTimingsBytes bounds the wrapper file read; a larger file is ignored.
	maxBuildTimingsBytes = 4 << 10
)

type phaseRow struct {
	phase, detail string
	seconds       float64
}

// phaseClock splits one run into contiguous phases: every instant belongs to
// exactly one running phase, so the rows sum to the run's wall time
// (DCW-V0-942). Rows with the same phase and detail add up, in first-seen
// order. A nil clock times nothing.
type phaseClock struct {
	mu            sync.Mutex
	start, mark   time.Time
	phase, detail string
	rows          []phaseRow
	index         map[string]int
}

func newPhaseClock(now time.Time) *phaseClock {
	return &phaseClock{start: now, mark: now, phase: "startup", detail: "-", index: map[string]int{}}
}

func (p *phaseClock) closeLocked(now time.Time) {
	key := p.phase + "\t" + p.detail
	i, ok := p.index[key]
	if !ok {
		i = len(p.rows)
		p.index[key] = i
		p.rows = append(p.rows, phaseRow{phase: p.phase, detail: p.detail})
	}
	p.rows[i].seconds += now.Sub(p.mark).Seconds()
	p.mark = now
}

// enter ends the running phase and starts phase; the returned function ends
// it and resumes the phase that was running before.
func (p *phaseClock) enter(phase, detail string) func() {
	if p == nil {
		return func() {}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked(time.Now())
	previous, previousDetail := p.phase, p.detail
	p.phase, p.detail = phase, detail
	return func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.closeLocked(time.Now())
		p.phase, p.detail = previous, previousDetail
	}
}

// to ends the running phase and starts phase, with nothing to resume.
func (p *phaseClock) to(phase string) {
	p.enter(phase, "-")
}

// prepend adds the wrapper's rows before the clock's own and moves the
// clock's start back to where they began, so the total stays the wall time
// since the wrapper started. A clock still in its first phase starts that
// phase where the wrapper's rows end, so the exec gap is counted.
func (p *phaseClock) prepend(rows []phaseRow, started time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	end := started
	for _, row := range rows {
		end = end.Add(time.Duration(row.seconds * float64(time.Second)))
	}
	if len(p.rows) == 0 && end.Before(p.mark) {
		p.mark = end
	}
	rows = append(rows, p.rows...)
	p.rows, p.index, p.start = nil, map[string]int{}, started
	for _, row := range rows {
		p.index[row.phase+"\t"+row.detail] = len(p.rows)
		p.rows = append(p.rows, row)
	}
}

// tsv ends the running phase and renders the rows and their wall total.
func (p *phaseClock) tsv() ([]byte, float64, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.closeLocked(now)
	var out bytes.Buffer
	out.WriteString("phase\tdetail\tseconds\n")
	for _, row := range p.rows {
		fmt.Fprintf(&out, "%s\t%s\t%.3f\n", row.phase, row.detail, row.seconds)
	}
	return out.Bytes(), now.Sub(p.start).Seconds(), len(p.rows)
}

// readBuildTimings reads the wrapper's rows when the file names pid, the
// process the wrapper exec'd into, and removes the file; any other file is
// left out, so a stale one from another run is never counted.
func readBuildTimings(path string, pid int) ([]phaseRow, time.Time, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBuildTimingsBytes {
		return nil, time.Time{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) < 3 || lines[0] != checkBuildProfile || lines[1] != "pid "+strconv.Itoa(pid) || !strings.HasPrefix(lines[2], "started ") {
		return nil, time.Time{}, false
	}
	epoch, err := strconv.ParseFloat(strings.TrimPrefix(lines[2], "started "), 64)
	if err != nil || epoch <= 0 || math.IsInf(epoch, 0) {
		return nil, time.Time{}, false
	}
	seconds, fraction := math.Modf(epoch)
	started := time.Unix(int64(seconds), int64(fraction*1e9))
	var rows []phaseRow
	for _, line := range lines[3:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" {
			return nil, time.Time{}, false
		}
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || value < 0 || math.IsInf(value, 0) {
			return nil, time.Time{}, false
		}
		rows = append(rows, phaseRow{fields[0], fields[1], value})
	}
	_ = os.Remove(path)
	return rows, started, true
}

// adoptBuildTimings prepends the wrapper's rows once the evidence directory
// is known; the time from the wrapper's last row to now is the check's
// startup.
func (c *check) adoptBuildTimings() {
	rows, started, ok := readBuildTimings(buildTimingsPath(c.evidence, os.Getpid()), os.Getpid())
	if !ok || c.clock == nil {
		return
	}
	c.clock.prepend(rows, started)
}

// buildTimingsPath names the wrapper's build file for process pid.
func buildTimingsPath(evidence string, pid int) string {
	return evidence + "/" + checkBuildTimings + strconv.Itoa(pid) + ".tsv"
}

// isTimingFile reports whether an evidence-directory entry is a timing file,
// which the DCW-V0-347 inputs digest leaves out.
func isTimingFile(name string) bool {
	return name == checkTimings || strings.HasPrefix(name, checkBuildTimings)
}

// writeTimings keeps the phase rows, however the run ended, and returns the
// wall total, the row count and the file written ("" when it was not). Only a
// run holding the operation lock writes, so a check refused for a busy lock
// leaves the previous file; a run that never learned its evidence directory
// writes nothing. Nothing is printed: the check's output stays what it was
// (DCW-V0-942).
func (c *check) writeTimings() (float64, int, string) {
	if c.clock == nil || c.evidence == "" || c.operationUnlock == nil {
		return 0, 0, ""
	}
	data, total, phases := c.clock.tsv()
	name := c.evidence + "/" + checkTimings
	if os.MkdirAll(c.evidence, 0o700) != nil || removeFile(name) != nil || writePrivate(name, data) != nil {
		return total, phases, ""
	}
	return total, phases, name
}

// verifierPhase names the verifier a runner is, for its timing row.
func (c *check) verifierPhase(runner Runner) string {
	switch {
	case runner.Path == c.options.BaseVerifier.Path:
		return "verifier-base"
	case runner.Path == c.options.TreeVerifier.Path:
		return "verifier-tree"
	default:
		return "verifier-override"
	}
}
