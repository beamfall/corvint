//go:build darwin || linux

package dispatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns runs a worker that
// writes four segments to each stream and then blocks. One tick truncates
// both live logs in place and keeps each stream's newest bytes, behind a
// marker line, in a rotated segment no larger than the cap. The worker keeps
// appending to the truncated file, and the finished summary reads its final
// message across the two segments.
func TestCALV0143_WorkerLogsAreCappedWhileTheWorkerRuns(t *testing.T) {
	defer func(n int64) { workerLogSegmentBytes = n }(workerLogSegmentBytes)
	workerLogSegmentBytes = 64 << 10
	c := testConfig(t, `i=0
while [ $i -lt 640 ]; do
  printf '{"type":"step_start","pad":"%0400d"}\n' $i
  printf 'stderr line %0400d\n' $i >&2
  i=$((i+1))
done
while [ ! -e go.flag ]; do sleep 0.05; done
printf '{"type":"text","part":{"text":"capped done"}}\n'`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	w := d.ledger.Workers[0]
	dir := d.workerDir(w.ID)
	size := func(name string) int64 {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return -1
		}
		return st.Size()
	}
	for i := 0; size("stderr.log") < 640*413 || size("stdout.log") < 640*431; i++ {
		if i == 400 {
			t.Fatalf("worker output did not complete: %d, %d", size("stdout.log"), size("stderr.log"))
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("supervising tick: %v running %d", err, d.Running())
	}
	for _, name := range []string{"stdout.log", "stderr.log"} {
		if got := size(name); got != 0 {
			t.Fatalf("%s holds %d bytes after the cap", name, got)
		}
		rotated, err := os.ReadFile(filepath.Join(dir, name+".1"))
		if err != nil || int64(len(rotated)) > workerLogSegmentBytes || int64(len(rotated)) < workerLogSegmentBytes-1024 {
			t.Fatalf("%s.1 = %d bytes (%v), want at most %d", name, len(rotated), err, workerLogSegmentBytes)
		}
		head, _, _ := strings.Cut(string(rotated), "\n")
		if !strings.HasPrefix(head, "corvint-tasks dispatch: "+name+" reached ") {
			t.Fatalf("%s.1 marker = %q", name, head)
		}
		if !strings.Contains(string(rotated), fmt.Sprintf("%0400d", 639)) {
			t.Fatalf("%s.1 lost the newest line", name)
		}
	}
	if w.LogBytes != 0 {
		t.Fatalf("recorded log bytes %d after the cap", w.LogBytes)
	}
	if err := os.WriteFile(filepath.Join(c.WorkRoot, "go.flag"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := size("stdout.log"); got <= 0 || got > 64 {
		t.Fatalf("the worker's append after truncation left stdout.log at %d bytes", got)
	}
	events, _ := ReadEvents(d.dir, 1000)
	found := false
	for _, e := range events {
		if e.Kind == "finished" {
			found = true
			if !strings.HasSuffix(e.Message, ": capped done") {
				t.Fatalf("finished summary = %q", e.Message)
			}
		}
	}
	if !found {
		t.Fatal("no finished event")
	}
}

// TestCALV0144_FinishedWorkerDirsAreRetired fills workers/ with 40 quiet
// finished directories and one that changed within the quiet window. When a
// real worker finishes, only the 32 newest quiet finished directories stay
// beside the recent one and the just-finished worker's own; a directory an
// infrastructure retry names is kept however old it is.
func TestCALV0144_FinishedWorkerDirsAreRetired(t *testing.T) {
	c := testConfig(t, `printf '{"type":"text","part":{"text":"done"}}\n'`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	plant := func(name string, at time.Time) {
		dir := d.workerDir(name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"stdout.log", "stderr.log"} {
			p := filepath.Join(dir, f)
			if err := os.WriteFile(p, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			os.Chtimes(p, at, at)
		}
		os.Chtimes(dir, at, at)
	}
	for i := 0; i < 40; i++ {
		plant(fmt.Sprintf("old-%02d", i), now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	plant("recent", now.Add(-5*time.Minute))
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	id := d.ledger.Workers[0].ID
	// A running worker's tick retires nothing: no worker has finished.
	if _, err := os.Stat(d.workerDir("old-39")); err != nil {
		t.Fatalf("retired before any worker finished: %v", err)
	}
	waitEnded(t, d)
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !has(kinds(t, d), "finished") {
		t.Fatal("the worker did not finish")
	}
	for i := 0; i < 40; i++ {
		_, err := os.Stat(d.workerDir(fmt.Sprintf("old-%02d", i)))
		if kept := err == nil; kept != (i < maxRetainedWorkerDirs) {
			t.Errorf("old-%02d kept=%v", i, kept)
		}
	}
	for _, name := range []string{"recent", id} {
		if _, err := os.Stat(filepath.Join(d.workerDir(name), "stdout.log")); err != nil {
			t.Errorf("%s was retired: %v", name, err)
		}
	}
	// The oldest directory stays while a reserved infrastructure retry names
	// it, since its absence would prove that launch never spawned.
	plant("infra-held", now.Add(-10*time.Hour))
	d.ledger.InfraRetry = map[string]*InfraEpisode{"ticket:a:q:other": {State: InfraReserved, Launch: "infra-held"}}
	d.retireWorkerDirs()
	if _, err := os.Stat(d.workerDir("infra-held")); err != nil {
		t.Fatalf("a directory named by an infrastructure retry was retired: %v", err)
	}
	d.ledger.InfraRetry = nil
	d.retireWorkerDirs()
	if _, err := os.Stat(d.workerDir("infra-held")); !os.IsNotExist(err) {
		t.Fatalf("an unprotected oldest directory was kept: %v", err)
	}
}
