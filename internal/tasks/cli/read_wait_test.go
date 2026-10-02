package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// withReadPatience runs body with the production read patience restored
// (the fixture package disables it for every other test).
func withReadPatience(t *testing.T, d time.Duration, body func()) {
	t.Helper()
	old := snapshot.DefaultPatience
	snapshot.DefaultPatience = d
	defer func() { snapshot.DefaultPatience = old }()
	body()
}

// readResult runs one read verb and decodes its envelope.
func readResult(t *testing.T, cwd string, args ...string) *wire.Result {
	t.Helper()
	var out bytes.Buffer
	Run(Env{Cwd: cwd, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &out})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
	}
	return res
}

// TestCTSV0006_ReadVerbsWaitForWriterToApplyReceipt: `queue status` and
// `plan preview` started while a writer sits between its receipt link-in
// and its head rename succeed on the new head once the writer finishes,
// instead of reporting NOT_RUN/REDO_PENDING (issue #433).
func TestCTSV0006_ReadVerbsWaitForWriterToApplyReceipt(t *testing.T) {
	for _, args := range [][]string{{"queue", "status"}, {"plan", "preview"}} {
		t.Run(args[0], func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.PlantReceipt(t, r, 2)
			if res := readResult(t, r.Root, args...); res.Outcome != wire.OutcomeNotRun || res.Codes[0] != wire.CodeRedoPending {
				t.Fatalf("fixture reads report the pending redo at once: %+v", res)
			}
			done := make(chan error, 1)
			go func() {
				time.Sleep(40 * time.Millisecond)
				done <- applyHead(r, 2)
			}()
			withReadPatience(t, 2*time.Second, func() {
				res := readResult(t, r.Root, args...)
				if res.Outcome != wire.OutcomeOK || res.Snapshot == nil || res.Snapshot.HeadSeq == nil || string(*res.Snapshot.HeadSeq) != "2" || res.Snapshot.PendingRedo {
					t.Fatalf("read did not wait for the writer: %+v", res)
				}
			})
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestCTSV0006_ReadVerbsUnderConcurrentWriter reproduces issue #433: a
// writer committing receipts back to back, each with a short window between
// receipt link-in and head rename, while `queue status` and `plan preview`
// run repeatedly. Every read must succeed; none may report REDO_PENDING or
// SNAPSHOT_MOVED, and the store must end exactly as the writer left it.
func TestCTSV0006_ReadVerbsUnderConcurrentWriter(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"), fixture.Ticket("B"))
	verbs := [][]string{{"queue", "status"}, {"plan", "preview"}}
	// A quiet read's cost bounds how often a read straddles a commit; the
	// writer's gap below is several times the slower verb on a quiet host.
	for _, args := range verbs {
		started := time.Now()
		readResult(t, r.Root, args...)
		t.Logf("quiet %v: %s", args, time.Since(started).Round(time.Millisecond))
	}
	const commits = 12
	writerErr := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		writerErr <- simulateWriter(r, commits, 10*time.Millisecond, 150*time.Millisecond)
	}()
	type tally struct {
		reads      int
		total, max time.Duration
	}
	var mu sync.Mutex
	var failures []string
	reads := map[string]*tally{}
	var wg sync.WaitGroup
	withReadPatience(t, 2*time.Second, func() {
		for _, args := range verbs {
			wg.Add(1)
			reads[args[0]] = &tally{}
			go func(args []string) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					var out bytes.Buffer
					started := time.Now()
					Run(Env{Cwd: r.Root, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &out})
					took := time.Since(started)
					res, err := wire.DecodeResult(out.Bytes())
					mu.Lock()
					ty := reads[args[0]]
					ty.reads++
					ty.total += took
					if took > ty.max {
						ty.max = took
					}
					if err != nil {
						failures = append(failures, fmt.Sprintf("%v: %v", args, err))
					} else if res.Outcome != wire.OutcomeOK {
						failures = append(failures, fmt.Sprintf("%v: %s %v %v", args, res.Outcome, res.Codes, res.Warnings))
					}
					mu.Unlock()
				}
			}(args)
		}
		wg.Wait()
	})
	if err := <-writerErr; err != nil {
		t.Fatalf("writer: %v", err)
	}
	for name, ty := range reads {
		t.Logf("%s under the writer: %d reads, mean %s, max %s", name, ty.reads, (ty.total / time.Duration(max(ty.reads, 1))).Round(time.Millisecond), ty.max.Round(time.Millisecond))
	}
	if len(failures) != 0 {
		t.Fatalf("%d reads failed under a concurrent writer:\n%s", len(failures), joinLines(failures))
	}
	for name, ty := range reads {
		if ty.reads < 5 {
			t.Errorf("%s ran only %d times; the writer finished before the reads exercised it", name, ty.reads)
		}
	}
	res := readResult(t, r.Root, "queue", "status")
	if res.Outcome != wire.OutcomeOK || res.Snapshot.HeadSeq == nil || string(*res.Snapshot.HeadSeq) != fmt.Sprint(commits+1) {
		t.Fatalf("final head: %+v", res)
	}
}

// simulateWriter commits n receipts the way the §5.2 writer does from a
// reader's point of view: receipt linked in (REDO_PENDING for window), then
// head renamed into place, then a gap before the next commit. Every file
// lands by rename so a reader never sees a partial file.
func simulateWriter(r *fixture.Repo, n int, window, gap time.Duration) error {
	for i := 0; i < n; i++ {
		h, err := readHead(r)
		if err != nil {
			return err
		}
		seq := h.LastSeq.Uint64() + 1
		rc := wire.EncodeFile(fixture.ReceiptValue(seq, h.LastReceiptSha256, "MUTATION", h.Generation.Uint64()))
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		if err := placeFile(r, filepath.Join(r.StateDir, "receipts", name), rc); err != nil {
			return err
		}
		time.Sleep(window)
		if err := applyHead(r, seq); err != nil {
			return err
		}
		time.Sleep(gap)
	}
	return nil
}

// applyHead is the writer's head rename for a receipt already linked in at
// seq (lastSeq+1), safe to call from a goroutine.
func applyHead(r *fixture.Repo, seq uint64) error {
	h, err := readHead(r)
	if err != nil {
		return err
	}
	if seq != h.LastSeq.Uint64()+1 {
		return fmt.Errorf("applyHead %d: head.lastSeq is %s", seq, h.LastSeq)
	}
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return err
	}
	rc, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", name))
	if err != nil {
		return err
	}
	return placeFile(r, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(fixture.HeadValue(h.PrimaryWorktree, seq, wire.Sum(rc), h.Generation.Uint64(), h.InitSha256)))
}

func readHead(r *fixture.Repo) (*snapshot.Head, error) {
	hraw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if err != nil {
		return nil, err
	}
	return snapshot.DecodeHead(hraw)
}

// placeFile writes data beside the state dir and renames it into place, as
// the §5.2 writer does, so a reader never sees a partial file.
func placeFile(r *fixture.Repo, path string, data []byte) error {
	tmp := filepath.Join(r.CommonDir, filepath.Base(path)+".writer")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func joinLines(s []string) string {
	var b bytes.Buffer
	for _, l := range s {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}
