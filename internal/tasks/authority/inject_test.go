//go:build darwin || linux

package authority

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Injection tests replace the package's OS hooks to force the failures a
// real mount cannot be asked for on demand (an unsupported full sync, a
// refused flock, a failed publication sync) and prove that every exit
// removes what it created and reports what happened.

func restoreHooks(t *testing.T) {
	t.Helper()
	sf, sd, fn, fr := syncFile, syncDirectory, flockNB, flockRelease
	t.Cleanup(func() { syncFile, syncDirectory, flockNB, flockRelease = sf, sd, fn, fr })
}

func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), probePrefix) || strings.Contains(e.Name(), ".tmp-") {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestTMV0010_QualifyReportsUnsupportedFullSyncAndCleansUp: an ENOTSUP from
// the full sync is reported as UNSUPPORTED_FILESYSTEM naming the sync, the
// probe file is removed, and no step after the failure is claimed.
func TestTMV0010_QualifyReportsUnsupportedFullSyncAndCleansUp(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	syncFile = func(*os.File) error { return syscall.ENOTSUP }
	q, err := Qualify(r.CommonDir)
	if q == nil {
		t.Fatalf("no result: %v", err)
	}
	if !q.Allowed {
		t.Skipf("host filesystem is not in the allowlist; the probe never runs here: %v", err)
	}
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "full sync is unsupported") {
		t.Fatalf("injected ENOTSUP reported as: %v", err)
	}
	if q.FullSync || q.Flock || q.DirSync {
		t.Errorf("steps claimed after the failure: %+v", q)
	}
	if q.ProbeName == "" {
		t.Errorf("probe file name not reported")
	}
	if left := tempFiles(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe left behind: %v", left)
	}
	// A generic I/O failure is reported as a failure, not as unsupported.
	syncFile = func(*os.File) error { return syscall.EIO }
	_, err = Qualify(r.CommonDir)
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "full sync failed") {
		t.Fatalf("injected EIO reported as: %v", err)
	}
	if left := tempFiles(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe left behind: %v", left)
	}
}

// TestTMV0010_QualifyReportsUnsupportedFlockAndCleansUp: ENOLCK, ENOTSUP
// and EOPNOTSUPP from flock refuse the mount (§5.1); the full sync that
// preceded them is still recorded as observed; the probe is removed.
func TestTMV0010_QualifyReportsUnsupportedFlockAndCleansUp(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	for _, errno := range []syscall.Errno{syscall.ENOLCK, syscall.ENOTSUP, syscall.EOPNOTSUPP} {
		flockNB = func(int) error { return errno }
		q, err := Qualify(r.CommonDir)
		if q != nil && !q.Allowed {
			t.Skipf("host filesystem is not in the allowlist: %v", err)
		}
		if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "flock is unsupported") {
			t.Fatalf("%v reported as: %v", errno, err)
		}
		if !q.FullSync || q.Flock || q.DirSync {
			t.Errorf("%v: steps misreported: %+v", errno, q)
		}
		if left := tempFiles(t, r.CommonDir); len(left) != 0 {
			t.Errorf("%v: probe left behind: %v", errno, left)
		}
	}
	// An unlock failure is a failure too.
	flockNB = flockExclusiveNB
	flockRelease = func(int) error { return syscall.EIO }
	q, err := Qualify(r.CommonDir)
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "LOCK_UN") || q.Flock {
		t.Fatalf("unlock failure reported as: %v %+v", err, q)
	}
	if left := tempFiles(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe left behind: %v", left)
	}
}

// TestTMV0010_QualifyReportsDirectorySyncFailure: the directory sync after
// the probe is removed is part of qualification; its failure refuses.
func TestTMV0010_QualifyReportsDirectorySyncFailure(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	calls := 0
	syncDirectory = func(f *os.File) error {
		calls++
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if st.IsDir() {
			return syscall.EIO
		}
		return fullSync(f)
	}
	q, err := Qualify(r.CommonDir)
	if q != nil && !q.Allowed {
		t.Skipf("host filesystem is not in the allowlist: %v", err)
	}
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "directory sync failed") {
		t.Fatalf("directory sync failure reported as: %v", err)
	}
	if !q.FullSync || !q.Flock || q.DirSync {
		t.Errorf("steps misreported: %+v", q)
	}
	if calls != 1 {
		t.Errorf("expected exactly one directory sync, saw %d sync calls", calls)
	}
	if left := tempFiles(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe left behind: %v", left)
	}
}

// TestTMV0009_LinkInCleansUpOnInjectedSyncFailure: a publication whose
// full sync fails leaves neither the temp nor the destination, and reports
// the failure with its code; a post-link directory sync failure is
// reported as *PostLinkError because the destination now exists.
func TestTMV0009_LinkInCleansUpOnInjectedSyncFailure(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.StateDir, "receipts"), 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := OpenDir(repo, "receipts")
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	defer d.Close()

	syncFile = func(*os.File) error { return syscall.ENOTSUP }
	err = d.LinkIn("1.json", []byte("{}\n"))
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "full sync is unsupported") {
		t.Fatalf("injected ENOTSUP reported as: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(d.Path(), "1.json")); !os.IsNotExist(err) {
		t.Errorf("destination exists after a failed publication: %v", err)
	}
	if left := tempFiles(t, d.Path()); len(left) != 0 {
		t.Errorf("temp left behind: %v", left)
	}
	entries, _ := os.ReadDir(d.Path())
	if len(entries) != 0 {
		t.Errorf("receipts dir not empty after failure: %d entries", len(entries))
	}

	// Directory sync fails only after the link: the file is published, the
	// error says so, and a retry sees ErrExists rather than a second copy.
	syncFile = fullSync
	syncDirectory = func(f *os.File) error {
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if st.IsDir() {
			return syscall.EIO
		}
		return fullSync(f)
	}
	err = d.LinkIn("2.json", []byte("{}\n"))
	var post *PostLinkError
	if !errors.As(err, &post) || post.Name != "2.json" || wire.CodeOf(post.Err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("post-link failure reported as: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(d.Path(), "2.json")); err != nil || string(raw) != "{}\n" {
		t.Fatalf("published file after post-link failure: %q %v", raw, err)
	}
	if left := tempFiles(t, d.Path()); len(left) != 0 {
		t.Errorf("temp left behind: %v", left)
	}
	syncFile = fullSync
	if err := d.LinkIn("2.json", []byte("{}\n")); !errors.Is(err, ErrExists) {
		t.Fatalf("retry after post-link failure: %v", err)
	}
}

// TestTMV0009_LockReportsUnsupportedFlockWithoutHolding: ENOLCK from flock
// refuses UNSUPPORTED_FILESYSTEM and holds nothing; a generic errno is a
// failure, not a wait; with the real flock the lock is then acquirable.
func TestTMV0009_LockReportsUnsupportedFlockWithoutHolding(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	flockNB = func(int) error { return syscall.ENOLCK }
	l, err := AcquireLock(context.Background(), repo, LockOptions{})
	if l != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "flock is unsupported") {
		t.Fatalf("ENOLCK: lock=%v err=%v", l, err)
	}
	flockNB = func(int) error { return syscall.EIO }
	l, err = AcquireLock(context.Background(), repo, LockOptions{})
	if l != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "flock failed") {
		t.Fatalf("EIO: lock=%v err=%v", l, err)
	}
	// EINTR is retried, then the real call succeeds.
	calls := 0
	flockNB = func(fd int) error {
		calls++
		if calls == 1 {
			return syscall.EINTR
		}
		return flockExclusiveNB(fd)
	}
	l, err = AcquireLock(context.Background(), repo, LockOptions{})
	if err != nil || calls != 2 {
		t.Fatalf("EINTR retry: err=%v calls=%d", err, calls)
	}
	// A release failure is reported, never swallowed.
	flockRelease = func(int) error { return syscall.EIO }
	if err := l.Close(); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "LOCK_UN") {
		t.Fatalf("release failure reported as: %v", err)
	}
	flockNB, flockRelease = flockExclusiveNB, flockUnlock
	// The descriptor was closed by Close even though the unlock call
	// failed, so the OS dropped the lock: a fresh acquisition succeeds.
	l2, err := AcquireLock(context.Background(), repo, LockOptions{Wait: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("acquire after a reported release failure: %v", err)
	}
	if err := l2.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestTMV0001_LockRefusesIdentityDriftDuringAcquisition: when the lock file
// is unlinked and recreated while a caller waits, the flock the caller
// finally obtains is on an orphan inode; the acquisition is refused and
// nothing is held.
func TestTMV0001_LockRefusesIdentityDriftDuringAcquisition(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	flockNB = func(fd int) error {
		calls++
		if calls == 1 {
			// Simulate the swap between two attempts.
			if err := os.Remove(repo.LockPath); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(repo.LockPath, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return syscall.EWOULDBLOCK
		}
		return flockExclusiveNB(fd)
	}
	l, err := AcquireLock(context.Background(), repo, LockOptions{Wait: 2 * time.Second, Poll: time.Millisecond})
	if l != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("drift: lock=%v err=%v", l, err)
	}
	flockNB = flockExclusiveNB
	l, err = AcquireLock(context.Background(), repo, LockOptions{Wait: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("acquire after refused drift: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTMV0009_AS10_LockInterruptedDeadlineAndCancellation(t *testing.T) {
	restoreHooks(t)
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"deadline", "cancel-EINTR", "cancel-acquired"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			flockNB = func(fd int) error {
				calls++
				// Bounded even on the old broken loop; no helper processes or stranded goroutines.
				if calls > 2 {
					return syscall.EIO
				}
				if scenario == "deadline" {
					time.Sleep(250 * time.Millisecond)
					return syscall.EINTR
				}
				cancel()
				if scenario == "cancel-acquired" {
					return flockExclusiveNB(fd)
				}
				return syscall.EINTR
			}
			wait := time.Second
			if scenario == "deadline" {
				wait = 200 * time.Millisecond
			}
			l, err := AcquireLock(ctx, repo, LockOptions{Wait: wait})
			if l != nil {
				l.Close()
				t.Fatal("returned an acquired lock after cancellation")
			}
			if scenario == "deadline" && wire.CodeOf(err) != wire.CodeLockTimeout {
				t.Fatalf("deadline: %v", err)
			}
			if scenario != "deadline" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel: %v", err)
			}
			if calls != 1 {
				t.Fatalf("expected one interrupted/acquired syscall, got %d", calls)
			}
			flockNB = flockExclusiveNB
			l, err = AcquireLock(context.Background(), repo, LockOptions{Wait: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if err = l.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTMV0001_AS10_AuthorityDirectoryReplacement(t *testing.T) {
	for _, api := range []string{"publish-state", "publish-child", "qualify", "lock"} {
		for _, kind := range []string{"symlink", "fifo"} {
			t.Run(api+"-"+kind, func(t *testing.T) {
				r := fixture.TempRepo(t)
				if err := os.MkdirAll(filepath.Join(r.StateDir, "receipts"), 0755); err != nil {
					t.Fatal(err)
				}
				repo, err := intent.Resolve(r.Root)
				if err != nil {
					t.Fatal(err)
				}
				outside := fixture.TempDirOutside(t)
				before := fixture.TreeSnapshot(t, outside)
				oldRoot, oldSub := openRoot, openSubRoot
				t.Cleanup(func() { openRoot, openSubRoot = oldRoot, oldSub })
				swap := r.CommonDir
				if api == "publish-child" {
					swap = filepath.Join(r.StateDir, "receipts")
				}
				replace := func() {
					if err := os.Rename(swap, swap+"-original"); err != nil {
						t.Fatal(err)
					}
					if kind == "fifo" {
						if err := syscall.Mkfifo(swap, 0600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.Symlink(outside, swap); err != nil {
						t.Fatal(err)
					}
				}
				openRoot = func(path string) (*os.Root, error) { replace(); return oldRoot(path) }
				if api == "publish-child" {
					openRoot = oldRoot
					openSubRoot = func(root *os.Root, rel string) (*os.Root, error) { replace(); return oldSub(root, rel) }
				}
				done := make(chan error, 1)
				go func() {
					switch api {
					case "publish-state", "publish-child":
						d, err := OpenDir(repo, "receipts")
						if d != nil {
							defer d.Close()
							if err == nil {
								err = d.LinkIn("1.json", []byte("{}\n"))
							}
						}
						done <- err
					case "qualify":
						_, err := Qualify(r.CommonDir)
						done <- err
					case "lock":
						l, err := AcquireLock(context.Background(), repo, LockOptions{})
						if l != nil {
							l.Close()
						}
						done <- err
					}
				}()
				select {
				case err = <-done:
				case <-time.After(100 * time.Millisecond):
					f, e := os.OpenFile(swap, os.O_RDWR|syscall.O_NONBLOCK, 0)
					if e != nil {
						t.Fatal(e)
					}
					<-done
					f.Close()
					t.Fatal("authority directory open blocked")
				}
				if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
					t.Fatalf("replacement accepted: %v", err)
				}
				if !fixture.SameTree(before, fixture.TreeSnapshot(t, outside)) {
					t.Fatal("authority redirected effects")
				}
			})
		}
	}
}
