// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fillLedger writes n records into a fresh private ledger directory, each a minute older than the
// next, and returns the ledger.
func fillLedger(tb testing.TB, n int) *ledger {
	tb.Helper()
	dir := filepath.Join(tb.TempDir(), "ledger")
	if err := os.Mkdir(dir, 0o700); err != nil {
		tb.Fatal(err)
	}
	l := &ledger{dir: dir, stdout: io.Discard, stderr: io.Discard}
	start := time.Now().Add(-time.Duration(n) * time.Minute)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%064x", i)
		path := l.recordPath(key)
		if err := os.WriteFile(path, []byte(`{"schema":"gate-ledger/1","key":"`+key+"\"}\n"), 0o600); err != nil {
			tb.Fatal(err)
		}
		at := start.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, at, at); err != nil {
			tb.Fatal(err)
		}
	}
	return l
}

// BenchmarkRecordFullLedger measures one record write into a ledger already holding n records,
// the steady state of a busy host (V1-4241: 4096 records were reached in about 11 hours).
func BenchmarkRecordFullLedger(b *testing.B) {
	for _, n := range []int{4096, 32768} {
		b.Run(fmt.Sprintf("records=%d", n), func(b *testing.B) {
			l := fillLedger(b, n)
			i := 0
			for b.Loop() {
				i++
				if err := l.record(record{Schema: schema, Step: "bench", Key: fmt.Sprintf("%064x", n+i), RecordedAt: "2026-11-01T00:00:00Z"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// countSuffix returns how many entries of dir end in suffix.
func countSuffix(tb testing.TB, dir, suffix string) int {
	tb.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		tb.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			n++
		}
	}
	return n
}

// needLocks skips where the platform has no advisory file locks (platform_other.go).
func needLocks(t *testing.T) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := lockExclusive(f, true); err != nil {
		t.Skipf("no advisory file locks here: %v", err)
	}
	unlock(f)
}

// ageStamp sets prune.stamp's modification time to age ago, creating it if needed.
func ageStamp(t *testing.T, l *ledger, age time.Duration) {
	t.Helper()
	stamp := filepath.Join(l.dir, pruneStamp)
	if err := os.WriteFile(stamp, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(stamp, at, at); err != nil {
		t.Fatal(err)
	}
}

// GL-V0-910: a scan trims a ledger over the bound to exactly the newest maxRecords records.
func TestPruneKeepsTheNewestRecords(t *testing.T) {
	t.Parallel() // GL-V0-9500: private directory; serial lock hooks have finished.
	const over = 50
	l := fillLedger(t, maxRecords+over)
	if err := l.prune(); err != nil {
		t.Fatal(err)
	}
	if got := countSuffix(t, l.dir, ".json"); got != maxRecords {
		t.Fatalf("%d records after prune, want %d", got, maxRecords)
	}
	for i := 0; i < over; i++ {
		if _, err := os.Lstat(l.recordPath(fmt.Sprintf("%064x", i))); !os.IsNotExist(err) {
			t.Fatalf("oldest record %d survived: %v", i, err)
		}
	}
	if _, err := os.Lstat(l.recordPath(fmt.Sprintf("%064x", over))); err != nil {
		t.Fatalf("newest records must stay: %v", err)
	}
}

// GL-V0-911: a fresh stamp skips the scan; a stale or missing one scans and is touched.
func TestPruneScansOncePerInterval(t *testing.T) {
	needLocks(t)
	l := fillLedger(t, 3)
	dir, err := l.locksDir()
	if err != nil {
		t.Fatal(err)
	}
	free := filepath.Join(dir, fmt.Sprintf("%064x.lock", 99))
	put := func() {
		if err := os.WriteFile(free, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put()
	ageStamp(t, l, time.Minute)
	if err := l.prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(free); err != nil {
		t.Fatalf("a fresh stamp must skip the scan: %v", err)
	}
	ageStamp(t, l, pruneInterval+time.Minute)
	if err := l.prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(free); !os.IsNotExist(err) {
		t.Fatalf("a stale stamp must scan: %v", err)
	}
	info, err := os.Lstat(filepath.Join(l.dir, pruneStamp))
	if err != nil || time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("the scan must touch the stamp: %v %v", info, err)
	}
	put()
	if err := os.Remove(filepath.Join(l.dir, pruneStamp)); err != nil {
		t.Fatal(err)
	}
	if err := l.prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(free); !os.IsNotExist(err) {
		t.Fatalf("a missing stamp must scan: %v", err)
	}
	if got := countSuffix(t, l.dir, ".json"); got != 3 {
		t.Fatalf("a ledger under the bound lost records: %d", got)
	}
}

// GL-V0-912: the holder removes its lock file, and a waiter that locked the removed file re-checks
// the path and reopens it instead of running beside a newer holder. lockHook pins the order: the
// waiter provably holds the original descriptor, locks it once the first holder releases, and a
// replacement holder takes the path before the waiter re-checks. Without the re-check the waiter
// would return while the replacement holds; with it, it reopens. Exactly one of the two happens.
func TestLockRemovesItsFileAndExcludesARacer(t *testing.T) {
	needLocks(t)
	l := fillLedger(t, 0)
	key := fmt.Sprintf("%064x", 7)
	first, err := l.lock(key, "first")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := l.locksDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, key+".lock")
	original, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("a held lock has a file: %v", err)
	}
	opened := make(chan struct{}, maxLockAttempts)
	flocked := make(chan struct{})
	resume := make(chan struct{})
	flocks := 0
	lockHook = func(step, event string) {
		if step != "waiter" {
			return
		}
		switch event {
		case "open":
			opened <- struct{}{}
		case "flock":
			flocks++
			if flocks == 1 {
				flocked <- struct{}{}
				<-resume
			}
		}
	}
	t.Cleanup(func() { lockHook = nil })
	acquired := make(chan func(), 1)
	go func() {
		release, err := l.lock(key, "waiter")
		if err != nil {
			t.Error(err)
			close(acquired)
			return
		}
		acquired <- release
	}()
	<-opened // the waiter holds a descriptor of the first holder's file
	first()  // removes that file and releases it
	<-flocked
	replacement, err := l.lock(key, "replacement")
	if err != nil {
		t.Fatal(err)
	}
	if now, err := os.Lstat(path); err != nil || os.SameFile(now, original) {
		t.Fatalf("the replacement holder must hold a new file: %v", err)
	}
	close(resume)
	select {
	case <-acquired:
		t.Fatal("the waiter ran beside the replacement holder: no inode re-check")
	case <-opened:
	}
	replacement()
	waiter, ok := <-acquired
	if !ok {
		t.Fatal("the waiter never acquired the lock")
	}
	// The waiter holds the file the path now names: a third acquirer cannot take it.
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("the waiter holds no file at %s: %v", path, err)
	}
	if lockExclusive(f, true) == nil {
		t.Error("a third acquirer took the lock the waiter holds")
	}
	f.Close()
	waiter()
	if got := countSuffix(t, dir, ".lock"); got != 0 {
		t.Fatalf("%d lock files outlive their runs", got)
	}
}

// GL-V0-912: a lock file that is a symlink never matches the re-check, so the lock becomes
// unavailable after the bounded attempts instead of looping, and runStep runs unrecorded.
func TestLockGivesUpOnASymlink(t *testing.T) {
	needLocks(t)
	l := fillLedger(t, 0)
	dir, err := l.locksDir()
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("%064x", 8)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, key+".lock")); err != nil {
		t.Fatal(err)
	}
	opens := 0
	lockHook = func(step, event string) {
		if event == "open" {
			opens++
		}
	}
	t.Cleanup(func() { lockHook = nil })
	release, err := l.lock(key, "symlink")
	if err == nil {
		release()
		t.Fatal("a symlinked lock file was accepted")
	}
	if opens != maxLockAttempts {
		t.Fatalf("%d attempts, want %d", opens, maxLockAttempts)
	}
}

// GL-V0-913: a scan reclaims free lock files of the namespace, leaves a held one alone, and never
// unlinks a top-level KEY.lock, which an older acquirer may hold.
func TestPruneReclaimsOnlyFreeLockFiles(t *testing.T) {
	needLocks(t)
	l := fillLedger(t, 1)
	dir, err := l.locksDir()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%064x.lock", 100+i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(l.dir, fmt.Sprintf("%064x.lock", 300+i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	held := fmt.Sprintf("%064x", 200)
	release, err := l.lock(held, "held")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := l.prune(); err != nil {
		t.Fatal(err)
	}
	if got := countSuffix(t, dir, ".lock"); got != 1 {
		t.Fatalf("%d namespace lock files after the scan, want only the held one", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, held+".lock")); err != nil {
		t.Fatalf("the held lock file was removed: %v", err)
	}
	if got := countSuffix(t, l.dir, ".lock"); got != 3 {
		t.Fatalf("%d top-level legacy lock files after the scan, want all 3 kept", got)
	}
}

// GL-V0-914: a record's bytes are the record's JSON and a newline, field for field as before.
func TestRecordBytesUnchanged(t *testing.T) {
	l := fillLedger(t, 0)
	key := strings.Repeat("ab", 32)
	rec := record{Schema: schema, Step: "go-test-package", Key: key, Tree: "t", RecordedAt: "2026-11-01T00:00:00Z", DurationMS: 5, Host: "h", Packages: []string{"p"}, Bound: "b", Scope: []string{"./x"}}
	if err := l.record(rec); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(l.recordPath(key))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"gate-ledger/1","step":"go-test-package","key":"` + key + `","tree":"t","recorded_at":"2026-11-01T00:00:00Z","duration_ms":5,"host":"h","packages":["p"],"bound":"b","scope":["./x"]}` + "\n"
	if string(got) != want {
		t.Fatalf("record bytes changed:\n got %s\nwant %s", got, want)
	}
	if back, hit := l.lookup(key); !hit || back.Key != key {
		t.Fatalf("lookup of a recorded key: %+v %v", back, hit)
	}
}
