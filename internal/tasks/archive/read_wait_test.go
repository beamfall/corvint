package archive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0006_ArchiveExportUnderConcurrentWriter: `archive export` run
// repeatedly while a writer commits receipts, each with a short window
// between receipt link-in and head rename, waits the writer out instead of
// reporting REDO_PENDING or SNAPSHOT_MOVED, and every archive verifies.
func TestCTSV0006_ArchiveExportUnderConcurrentWriter(t *testing.T) {
	_, repo := repoWithStore(t)
	old := snapshot.DefaultPatience
	snapshot.DefaultPatience = 2 * time.Second
	defer func() { snapshot.DefaultPatience = old }()
	export := func() error {
		var out bytes.Buffer
		if _, err := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out}); err != nil {
			return err
		}
		_, err := Verify(bytes.NewReader(out.Bytes()))
		return err
	}
	started := time.Now()
	if err := export(); err != nil {
		t.Fatal(err)
	}
	t.Logf("quiet export: %s", time.Since(started).Round(time.Millisecond))
	// The writer commits at least 8 receipts and keeps committing until
	// minExports exports have finished while it ran, so the overlap does
	// not depend on host speed (V1-1532); the cap bounds a stalled host.
	const commits, minExports, maxCommits = 8, 5, 400
	var finished atomic.Int32
	writerErr := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		writerErr <- commitWithWindow(repo.StateDir, commits, 10*time.Millisecond, 150*time.Millisecond, func(i int) bool {
			return i < maxCommits && finished.Load() < minExports
		})
	}()
	var failures []string
	exports := 0
	for done := false; !done; {
		select {
		case <-stop:
			done = true
		default:
			exports++
			if err := export(); err != nil {
				failures = append(failures, err.Error())
			}
			finished.Add(1)
		}
	}
	if err := <-writerErr; err != nil {
		t.Fatalf("writer: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("%d of %d exports failed under a concurrent writer:\n%s", len(failures), exports, strings.Join(failures, "\n"))
	}
	if exports < minExports {
		t.Errorf("only %d exports ran; the writer finished before they exercised it", exports)
	}
}

// TestCTSV0006_ArchiveAttemptsShareOnePatience: the archive's four attempts
// draw on one patience deadline, so a store that keeps moving costs at most
// the budget in pauses in total, not the budget per attempt, and the final
// SNAPSHOT_MOVED names the wait and says the read is retryable.
func TestCTSV0006_ArchiveAttemptsShareOnePatience(t *testing.T) {
	r, repo := repoWithStore(t)
	const patience = 120 * time.Millisecond
	var total time.Duration
	pauses := 0
	rd := snapshot.Reader{StateDir: repo.StateDir, Patience: patience, Sleep: func(d time.Duration) {
		pauses++
		total += d
		time.Sleep(d)
	}}
	bodies := 0
	started := time.Now()
	_, err := readArchive(rd, func(*snapshot.Snapshot) error {
		bodies++
		fixture.Commit(t, r, "MUTATION")
		return nil
	})
	took := time.Since(started)
	if code(err) != wire.CodeSnapshotMoved || bodies < 4 {
		t.Fatalf("err=%v bodies=%d", err, bodies)
	}
	if pauses == 0 || total > patience {
		t.Errorf("%d pauses totalling %s exceed the %s budget (took %s)", pauses, total, patience, took)
	}
	msg := err.Error()
	if !strings.Contains(msg, "four attempts") || !strings.Contains(msg, fmt.Sprintf("after %d pauses", pauses)) || !strings.Contains(msg, "retryable") {
		t.Errorf("message does not name the wait: %v", err)
	}
	// Without patience the four attempts run unpaused and the message is
	// the plain TM-V0-008 refusal.
	rd = snapshot.Reader{StateDir: repo.StateDir, Patience: snapshot.NoPatience, Sleep: func(time.Duration) { t.Error("NoPatience must not sleep") }}
	bodies = 0
	_, err = readArchive(rd, func(*snapshot.Snapshot) error {
		bodies++
		fixture.Commit(t, r, "MUTATION")
		return nil
	})
	if code(err) != wire.CodeSnapshotMoved || bodies != 4 || strings.Contains(err.Error(), "retryable") {
		t.Errorf("NoPatience: err=%v bodies=%d", err, bodies)
	}
}

// commitWithWindow commits n receipts as the §5.2 writer appears to a
// reader: receipt linked in (REDO_PENDING for window), head renamed into
// place, then a gap. Files land by rename; it is safe in a goroutine. After n
// receipts it keeps committing while more(i), when set, reports true.
func commitWithWindow(stateDir string, n int, window, gap time.Duration, more func(i int) bool) error {
	place := func(path string, data []byte) error {
		tmp := filepath.Join(filepath.Dir(stateDir), filepath.Base(path)+".writer")
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	}
	for i := 0; i < n || (more != nil && more(i)); i++ {
		hraw, err := os.ReadFile(filepath.Join(stateDir, "head.json"))
		if err != nil {
			return err
		}
		h, err := snapshot.DecodeHead(hraw)
		if err != nil {
			return err
		}
		seq := h.LastSeq.Uint64() + 1
		rc := wire.EncodeFile(fixture.ReceiptValue(seq, h.LastReceiptSha256, "MUTATION", h.Generation.Uint64()))
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		if err := place(filepath.Join(stateDir, "receipts", name), rc); err != nil {
			return err
		}
		time.Sleep(window)
		if err := place(filepath.Join(stateDir, "head.json"), wire.EncodeFile(fixture.HeadValue(h.PrimaryWorktree, seq, wire.Sum(rc), h.Generation.Uint64(), h.InitSha256))); err != nil {
			return err
		}
		time.Sleep(gap)
	}
	return nil
}

// TestCTSV0022_SlowExportStillWaitsOutWriter: the CTS-V0-006 budget bounds
// pauses, not the export body. An export whose body alone outlasts the
// budget (a 10k-ticket store takes seconds) and that meets a writer between
// receipt link-in and head rename at its re-probe must still pause, see the
// writer finish and re-read, instead of failing REDO_PENDING at once
// (V1-0903). Before the fix the body spent the wall-clock deadline, so the
// export exited 1 whenever its re-probe met the writer's window.
func TestCTSV0022_SlowExportStillWaitsOutWriter(t *testing.T) {
	_, repo := repoWithStore(t)
	const patience = 300 * time.Millisecond
	old := snapshot.DefaultPatience
	snapshot.DefaultPatience = patience
	defer func() { snapshot.DefaultPatience = old }()
	bodies := 0
	writer := make(chan error, 1)
	var out bytes.Buffer
	res, err := Export(ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &out, afterProbe: func() {
		bodies++
		if bodies != 1 {
			return
		}
		time.Sleep(patience + 100*time.Millisecond)
		hraw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
		if err != nil {
			t.Fatal(err)
		}
		h, err := snapshot.DecodeHead(hraw)
		if err != nil {
			t.Fatal(err)
		}
		next, _ := snapshot.ReceiptName(h.LastSeq.Uint64() + 1)
		go func() { writer <- commitWithWindow(repo.StateDir, 1, 150*time.Millisecond, 0, nil) }()
		for start := time.Now(); time.Since(start) < 5*time.Second; time.Sleep(time.Millisecond) {
			if _, err := os.Lstat(filepath.Join(repo.StateDir, "receipts", next)); err == nil {
				return
			}
		}
		t.Fatal("writer never linked its receipt in")
	}})
	if werr := <-writer; werr != nil {
		t.Fatalf("writer: %v", werr)
	}
	if err != nil {
		t.Fatalf("slow export meeting a writer in flight: %v (bodies %d)", err, bodies)
	}
	if bodies < 2 || res.Manifest.ExportedAtSeq != "4" {
		t.Fatalf("export ran %d bodies at seq %s, want a re-read at the writer's receipt 4", bodies, res.Manifest.ExportedAtSeq)
	}
	if _, err := Verify(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatal(err)
	}
}
