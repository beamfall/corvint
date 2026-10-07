package dispatch

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	maxWorkerDirScan  = 4096
	workerLogMarkerOf = "corvint-tasks dispatch: %s reached %d bytes and was truncated; this segment keeps its last %d bytes\n"
)

// capWorkerLogs keeps each of a worker's log streams within one live segment
// and one rotated segment (CAL-V0-143). The worker writes through its own
// append-mode descriptors and may outlive the dispatcher, so the file cannot
// be renamed away: the newest part of an oversized stream is copied to
// <name>.1, replacing the previous segment, and the live file is truncated in
// place; the worker's next append lands at its new end.
func capWorkerLogs(dir string) {
	for _, name := range []string{"stdout.log", "stderr.log"} {
		capWorkerLog(dir, name)
	}
}

func capWorkerLog(dir, name string) {
	path := filepath.Join(dir, name)
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() <= workerLogSegmentBytes {
		return
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !os.SameFile(lst, st) || st.Size() <= workerLogSegmentBytes {
		return
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
	_ = f.Truncate(0)
}

// retireWorkerDirs removes finished worker directories beyond the newest
// maxRetainedWorkerDirs (CAL-V0-144). A directory is never removed while the
// ledger records its worker, its exit is still awaited, an infrastructure
// retry names it as its launch (its absence proves no spawn), or it changed
// within workerDirQuiet. Failures are left for the next pass.
func (d *Dispatcher) retireWorkerDirs() {
	root := filepath.Join(d.dir, "workers")
	f, err := os.Open(root)
	if err != nil {
		return
	}
	entries, _ := f.ReadDir(maxWorkerDirScan)
	f.Close()
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
		if !e.IsDir() || keep[e.Name()] {
			continue
		}
		path := filepath.Join(root, e.Name())
		newest, ok := newestMtime(path)
		if !ok || now.Sub(newest) < workerDirQuiet {
			continue
		}
		finished = append(finished, dir{e.Name(), newest})
	}
	if len(finished) <= maxRetainedWorkerDirs {
		return
	}
	sort.Slice(finished, func(i, j int) bool {
		if !finished[i].mtime.Equal(finished[j].mtime) {
			return finished[i].mtime.After(finished[j].mtime)
		}
		return finished[i].name > finished[j].name
	})
	for _, x := range finished[maxRetainedWorkerDirs:] {
		_ = os.RemoveAll(filepath.Join(root, x.name))
	}
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
