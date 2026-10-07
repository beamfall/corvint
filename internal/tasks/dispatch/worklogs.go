package dispatch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// workerLogSegmentBytes is the CAL-V0-143 cap on each worker log stream's
// live file and on its one rotated segment, matching a detached run's
// 8 MiB output segments. Tests lower it.
var workerLogSegmentBytes int64 = 8 << 20

const (
	// maxRetainedWorkerDirs is how many finished worker directories outside
	// the quiet window are kept (CAL-V0-144).
	maxRetainedWorkerDirs = 32
	// workerDirQuiet keeps any worker directory whose directory or logs
	// changed this recently, so a tree the ledger does not record (a crash
	// between launch and the ledger save) is not removed while it writes.
	workerDirQuiet = time.Hour
	// maxWorkerDirScan bounds one retirement pass; a larger backlog shrinks
	// over later passes.
	maxWorkerDirScan = 4096
	workerLeaderName = "leader"
	// workerRetireMarks holds one candidate mark per worker directory a pass
	// found removable. It sits beside the worker directories, never inside
	// one, so a mark changes neither a directory's age nor its contents.
	workerRetireMarks = ".retiring"
	// workerRetireConfirm is how long a mark must stand before a later pass
	// that finds the directory removable again may remove it.
	workerRetireConfirm = time.Minute
	// workerRetireMarkMaxAge, or three ticks if longer, is the oldest mark a
	// pass may confirm; an older one is stale and confirmation starts over.
	workerRetireMarkMaxAge = 10 * time.Minute
	// maxRetireMarks bounds the marks that exist at once.
	maxRetireMarks    = 4096
	workerLogMarkerOf = "corvint-tasks dispatch: %s reached %d bytes and was truncated; this segment keeps its last %d bytes\n"
)

// capWorkerLogs keeps each of a worker's log streams within one live segment
// and one rotated segment (CAL-V0-143). The worker writes through its own
// append-mode descriptors and may outlive the dispatcher, so the file cannot
// be renamed away: the newest part of an oversized stream is copied to
// <name>.1, replacing the previous segment, and the live file is truncated in
// place; the worker's next append lands at its new end. It reports whether
// any stream was cut, and whether stdout.log was (CAL-V0-157).
func capWorkerLogs(dir string) (cut, stdout bool) {
	stdout = capWorkerLog(dir, "stdout.log")
	return capWorkerLog(dir, "stderr.log") || stdout, stdout
}

func capWorkerLog(dir, name string) bool {
	path := filepath.Join(dir, name)
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() <= workerLogSegmentBytes {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !os.SameFile(lst, st) || st.Size() <= workerLogSegmentBytes {
		return false
	}
	size := st.Size()
	marker := fmt.Sprintf(workerLogMarkerOf, name, size, workerLogSegmentBytes)
	keep := max(workerLogSegmentBytes-int64(len(marker)), 0)
	// The segment starts at a line boundary when one is near, so a reader of
	// its tail never meets a partial first line behind the marker.
	head := make([]byte, min(keep, 64<<10))
	if n, _ := f.ReadAt(head, size-keep); n > 0 {
		if i := bytes.IndexByte(head[:n], '\n'); i >= 0 {
			keep -= int64(i + 1)
		}
	}
	marker = fmt.Sprintf(workerLogMarkerOf, name, size, keep)
	if tmp, err := os.CreateTemp(dir, name+".tmp-*"); err == nil {
		_, err = io.WriteString(tmp, marker)
		if err == nil {
			_, err = io.Copy(tmp, io.NewSectionReader(f, size-keep, keep))
		}
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmp.Name(), path+".1")
		}
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}
	// The cap holds even when the copy failed; bytes appended after the stat
	// are lost with the truncation.
	return f.Truncate(0) == nil
}

// retireWorkerDirs removes finished worker directories beyond the newest
// maxRetainedWorkerDirs (CAL-V0-144). A directory is never removed while the
// ledger records its worker, its exit is still awaited, an infrastructure
// retry names it as its launch (its absence proves no spawn), it changed
// within workerDirQuiet, or its recorded leader's tree may still run; such a
// directory takes no retention slot. Removal needs two passes: the first that
// finds a directory removable only marks it, and a pass at least
// workerRetireConfirm later removes it only if it is still removable with the
// same age. A pass that finds it otherwise clears the mark. The pass runs once
// per tick, so the two are always in different ticks. Failures are left for
// the next pass.
func (d *Dispatcher) retireWorkerDirs() {
	// A pass that cannot read what it needs leaves any marks in place. When a
	// confirmation was scheduled, it schedules the next one
	// workerRetireConfirm later, so a transient failure retries at most once
	// a minute and never cancels confirmation.
	armed := !d.retireConfirm.IsZero()
	d.retireConfirm = time.Time{}
	retry := func() {
		if armed {
			d.retireConfirm = d.Now().Add(workerRetireConfirm)
		}
	}
	root := filepath.Join(d.dir, "workers")
	f, err := os.Open(root)
	if err != nil {
		retry()
		return
	}
	entries, err := f.ReadDir(maxWorkerDirScan)
	f.Close()
	if err != nil && err != io.EOF {
		retry()
		return
	}
	keep := map[string]bool{}
	for _, w := range d.ledger.Workers {
		keep[w.ID] = true
	}
	for id := range d.exits {
		keep[id] = true
	}
	for _, e := range d.ledger.InfraRetry {
		if e != nil && e.Launch != "" {
			keep[e.Launch] = true
		}
	}
	type dir struct {
		name  string
		mtime time.Time
	}
	var finished []dir
	now := d.Now()
	for _, e := range entries {
		if !e.IsDir() || e.Name() == workerRetireMarks || keep[e.Name()] {
			continue
		}
		path := filepath.Join(root, e.Name())
		newest, ok := newestMtime(path)
		if !ok || now.Sub(newest) < workerDirQuiet {
			continue
		}
		finished = append(finished, dir{e.Name(), newest})
	}
	removable := map[string]time.Time{}
	// A directory kept only because the process table could not be read
	// keeps its mark, and its confirmation is retried.
	unsure := map[string]bool{}
	if len(finished) > maxRetainedWorkerDirs {
		sort.Slice(finished, func(i, j int) bool {
			if !finished[i].mtime.Equal(finished[j].mtime) {
				return finished[i].mtime.After(finished[j].mtime)
			}
			return finished[i].name > finished[j].name
		})
		// A directory whose recorded tree may still run is kept and takes no
		// retention slot, so the newest maxRetainedWorkerDirs are counted
		// among the removable ones only.
		var groups, sessions map[int]bool
		read, usable, retained := false, false, 0
		for _, x := range finished {
			pid, id, err := readLeader(filepath.Join(root, x.name))
			switch {
			case errors.Is(err, fs.ErrNotExist):
				// A directory from a build that recorded no leader: the quiet
				// window is its only evidence.
			case err != nil:
				continue
			default:
				if !read {
					read = true
					groups, sessions, usable = liveTrees()
				}
				if !usable {
					unsure[x.name] = true
					continue
				}
				if treeMayRun(pid, id, groups, sessions) {
					continue
				}
			}
			if retained < maxRetainedWorkerDirs {
				retained++
				continue
			}
			removable[x.name] = x.mtime
		}
	}
	// Marks are pruned from their own directory, not from the worker scan: a
	// mark whose directory this pass did not find removable is cleared. At
	// most maxRetireMarks exist; a pass that reads that many writes none.
	marks := filepath.Join(root, workerRetireMarks)
	// A pass that leaves a mark schedules its own confirming pass: a tick
	// workerRetireConfirm from now, and at most one tick later, is inside
	// every mark's maximum age, so confirmation never waits on another
	// worker finishing.
	pending := false
	writeMark := func(mark string, age time.Time) {
		if os.MkdirAll(marks, 0o700) == nil && os.WriteFile(mark, []byte(fmt.Sprintf("%d %d\n", now.UnixNano(), age.UnixNano())), 0o600) == nil {
			pending = true
		}
	}
	defer func() {
		if pending {
			d.retireConfirm = now.Add(workerRetireConfirm)
		}
	}()
	held, full := 0, false
	if f, err := os.Open(marks); err != nil && !errors.Is(err, fs.ErrNotExist) {
		retry()
		return
	} else if err == nil {
		names, err := f.Readdirnames(maxRetireMarks)
		f.Close()
		if err != nil && err != io.EOF {
			retry()
			return
		}
		full = len(names) == maxRetireMarks
		for _, name := range names {
			if _, ok := removable[name]; ok {
				held++
			} else if unsure[name] {
				held++
				pending = true
			} else {
				_ = os.Remove(filepath.Join(marks, name))
			}
		}
	}
	markAge := max(workerRetireMarkMaxAge, 3*time.Duration(d.Config.TickSeconds)*time.Second)
	for name, newest := range removable {
		mark := filepath.Join(marks, name)
		markedAt, age, err := readRetireMark(mark)
		since := now.Sub(markedAt)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if !full && held < maxRetireMarks {
				held++
				writeMark(mark, newest)
			} else {
				pending = true // marked by the pass after the prune
			}
		case err != nil || since < 0 || since > markAge:
			writeMark(mark, newest) // malformed or stale: confirmation starts over
		case !age.Equal(newest):
			_ = os.Remove(mark) // its logs changed since it was marked
		case since >= workerRetireConfirm:
			// The scan's age may be stale by now: a directory that changed
			// since keeps its logs, and its mark is cleared.
			path := filepath.Join(root, name)
			if fresh, ok := newestMtime(path); !ok || !fresh.Equal(age) {
				_ = os.Remove(mark)
				continue
			}
			if os.RemoveAll(path) == nil {
				_ = os.Remove(mark)
			} else {
				pending = true
			}
		default:
			pending = true // marked under workerRetireConfirm ago
		}
	}
}

// readRetireMark reads when a worker directory was marked removable and the
// age it had then.
func readRetireMark(path string) (markedAt, age time.Time, err error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	at, seen, ok := strings.Cut(strings.TrimSuffix(string(raw), "\n"), " ")
	a, aerr := strconv.ParseInt(at, 10, 64)
	b, berr := strconv.ParseInt(seen, 10, 64)
	if !ok || aerr != nil || berr != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("malformed retirement mark %s", path)
	}
	return time.Unix(0, a), time.Unix(0, b), nil
}

// readLeader reads the leader PID and start identity launch kept in a
// worker directory.
func readLeader(dir string) (int, string, error) {
	f, err := os.Open(filepath.Join(dir, workerLeaderName))
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 256))
	if err != nil {
		return 0, "", err
	}
	pidText, id, ok := strings.Cut(strings.TrimSuffix(string(raw), "\n"), " ")
	pid, perr := strconv.Atoi(pidText)
	if !ok || perr != nil || pid <= 0 || id == "" {
		return 0, "", fmt.Errorf("malformed worker leader in %s", dir)
	}
	return pid, id, nil
}

// newestMtime is the latest change time of a worker directory and its log
// segments; an unreadable directory reports false and is kept.
func newestMtime(path string) (time.Time, bool) {
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() {
		return time.Time{}, false
	}
	newest := st.ModTime()
	for _, name := range []string{"stdout.log", "stderr.log", "stdout.log.1", "stderr.log.1"} {
		if st, err := os.Lstat(filepath.Join(path, name)); err == nil && st.ModTime().After(newest) {
			newest = st.ModTime()
		}
	}
	return newest, true
}

// workerLogBytes is the combined size of a worker's live log files.
func workerLogBytes(dir string) int64 {
	var size int64
	for _, name := range []string{"stdout.log", "stderr.log"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
			size += st.Size()
		}
	}
	return size
}
