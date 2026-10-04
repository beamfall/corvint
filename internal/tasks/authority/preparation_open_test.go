//go:build darwin || linux

package authority

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Canonical encoding and a full genesis audit must succeed before any injection.
// The journal/intent checks run separately from lock-file fixture side effects.
func preparationOpenFixture(t *testing.T) (*fixture.Repo, *intent.Repository) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r)
	fixture.WriteState(t, r)
	audit := func() {
		t.Helper()
		q, err := wire.ParseQueueID("", fixture.QueueID)
		if err != nil {
			t.Fatal(err)
		}
		a, err := (journal.Reader{Source: journal.Native{StateDir: r.StateDir, PrimaryWorktree: r.Root}, QueueID: q, PrimaryWorktree: r.Root}).Audit("intent/queue.json")
		if err != nil || a == nil || a.StructuralConsistency != "CONSISTENT" || a.ProjectionAgreement != "AGREES" {
			t.Fatalf("fixture audit before/after injection: %+v %v", a, err)
		}
	}
	audit()
	state, intended := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	t.Cleanup(func() {
		audit()
		if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intended, fixture.TreeSnapshot(t, r.IntentDir)) {
			t.Error("lock acquisition changed canonical journal or intent")
		}
	})
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	return r, repo
}

func preparationOpenRestore(t *testing.T) {
	t.Helper()
	open, flock := preparationOpen, flockNB
	t.Cleanup(func() { preparationOpen, flockNB = open, flock })
}

func preparationOpenRefused(t *testing.T, lock *PreparationLock, err error, code string) {
	t.Helper()
	if lock != nil {
		_ = lock.Close()
		t.Fatal("unexpected successful preparation")
	}
	if wire.CodeOf(err) != code {
		t.Fatalf("refusal: %v, want %s", err, code)
	}
}

func preparationOpenClosed(t *testing.T, f *os.File, fd int) {
	t.Helper()
	if f == nil || fd < 0 {
		t.Fatal("opened-FD boundary was not reached")
	}
	if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file object was not closed: %v", err)
	}
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != syscall.EBADF {
		t.Fatalf("actual descriptor%d still live: %v", fd, err)
	}
}

// A guard callback lets a deterministic test act at a specific helper guard,
// after the preceding real filesystem operation. It never changes ctx results.
type preparationGuardContext struct {
	context.Context
	calls int
	at    int
	do    func()
}

func (c *preparationGuardContext) Err() error {
	c.calls++
	if c.calls == c.at {
		c.do()
	}
	return c.Context.Err()
}

// All tests are sequential: the private OS seams are restored per subtest.
func TestGH494PreparationOpen(t *testing.T) {
	const exclusive = os.O_RDWR | os.O_CREATE | os.O_EXCL
	t.Run("absent-and-present-flags-identity-and-content", func(t *testing.T) {
		r, repo := preparationOpenFixture(t)
		preparationOpenRestore(t)
		original := preparationOpen
		var flags []int
		var opened *os.File
		fd := -1
		preparationOpen = func(root *os.Root, name string, flag int, mode os.FileMode, dir bool) (*os.File, error) {
			if name != preparationLockFileName || mode != 0644 || dir {
				t.Fatalf("unexpected open arguments: %q %d %v", name, mode, dir)
			}
			flags = append(flags, flag)
			f, err := original(root, name, flag, mode, dir)
			if err == nil {
				opened, fd = f, int(f.Fd())
			}
			return f, err
		}
		lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		preparationOpenClosed(t, opened, fd)
		path := filepath.Join(r.CommonDir, preparationLockFileName)
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, path, []byte("inert existing bytes"))
		lock, err = AcquirePreparation(context.Background(), repo, LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		preparationOpenClosed(t, opened, fd)
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(before, after) {
			t.Fatal("existing inode changed", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != "inert existing bytes" || !reflect.DeepEqual(flags, []int{exclusive, os.O_RDWR}) {
			t.Fatalf("content/flags: %q %v %v", raw, flags, err)
		}
	})

	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprintf("EEXIST-winner-wrapped-%t", wrapped), func(t *testing.T) {
			r, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			original := preparationOpen
			var flags []int
			var winner os.FileInfo
			preparationOpen = func(root *os.Root, name string, flag int, mode os.FileMode, dir bool) (*os.File, error) {
				flags = append(flags, flag)
				if len(flags) == 1 {
					fixture.Write(t, filepath.Join(r.CommonDir, name), []byte("winner"))
					var err error
					winner, err = root.Lstat(name)
					if err != nil {
						t.Fatal(err)
					}
					if wrapped {
						return nil, fmt.Errorf("wrapped collision: %w", &os.PathError{Op: "open", Path: name, Err: syscall.EEXIST})
					}
				}
				return original(root, name, flag, mode, dir)
			}
			lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			got, err := lock.lock.f.Stat()
			if err != nil || winner == nil || !os.SameFile(winner, got) || !reflect.DeepEqual(flags, []int{exclusive, os.O_RDWR}) {
				t.Fatalf("winner/call sequence: %v %v", flags, err)
			}
		})
	}

	for _, injected := range []error{syscall.ENOENT, syscall.EIO, syscall.EACCES, syscall.ENOSPC, syscall.EINTR, &os.PathError{Op: "open", Path: "test", Err: errors.New("noncollision")}} {
		t.Run("terminal-create-"+injected.Error(), func(t *testing.T) {
			_, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			calls := 0
			preparationOpen = func(_ *os.Root, _ string, flag int, _ os.FileMode, _ bool) (*os.File, error) {
				calls++
				if flag != exclusive {
					t.Fatal("unexpected create flags", flag)
				}
				return nil, injected
			}
			lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
			preparationOpenRefused(t, lock, err, wire.CodeUnsupportedFilesystem)
			if calls != 1 || !strings.Contains(err.Error(), injected.Error()) {
				t.Fatalf("injection not reached exactly once: calls%d err%v", calls, err)
			}
		})
	}

	for _, kind := range []string{"absent", "symlink", "directory", "fifo", "closed-root"} {
		t.Run("collision-restat-"+kind, func(t *testing.T) {
			r, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			calls := 0
			preparationOpen = func(root *os.Root, name string, flag int, _ os.FileMode, _ bool) (*os.File, error) {
				calls++
				if calls != 1 || flag != exclusive {
					t.Fatal("unexpected fallback", calls, flag)
				}
				path := filepath.Join(r.CommonDir, name)
				var err error
				switch kind {
				case "closed-root":
					err = root.Close()
				case "symlink":
					err = os.Symlink("HEAD", path)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "fifo":
					err = syscall.Mkfifo(path, 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				return nil, syscall.EEXIST
			}
			lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
			preparationOpenRefused(t, lock, err, wire.CodeUnsupportedFilesystem)
			if calls != 1 {
				t.Fatal("injected collision not reached", calls)
			}
		})
	}

	for _, fromCollision := range []bool{false, true} {
		for _, action := range []string{"remove", "replace", "symlink", "fifo"} {
			t.Run(fmt.Sprintf("before-existing-open-collision-%t-%s", fromCollision, action), func(t *testing.T) {
				r, repo := preparationOpenFixture(t)
				preparationOpenRestore(t)
				path := filepath.Join(r.CommonDir, preparationLockFileName)
				if !fromCollision {
					fixture.Write(t, path, nil)
				}
				original := preparationOpen
				var flags []int
				var opened *os.File
				fd := -1
				reached := false
				preparationOpen = func(root *os.Root, name string, flag int, mode os.FileMode, dir bool) (*os.File, error) {
					flags = append(flags, flag)
					if flag == exclusive {
						fixture.Write(t, path, nil)
						return original(root, name, flag, mode, dir)
					}
					reached = true
					// Retain the old inode so replacement cannot reuse its number.
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
					var err error
					switch action {
					case "replace":
						fixture.Write(t, path, nil)
					case "symlink":
						err = os.Symlink("HEAD", path)
					case "fifo":
						err = syscall.Mkfifo(path, 0600)
					}
					if err != nil {
						t.Fatal(err)
					}
					f, err := original(root, name, flag, mode, dir)
					if err == nil {
						opened, fd = f, int(f.Fd())
					}
					return f, err
				}
				lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
				preparationOpenRefused(t, lock, err, wire.CodeUnsupportedFilesystem)
				want := []int{os.O_RDWR}
				if fromCollision {
					want = []int{exclusive, os.O_RDWR}
				}
				if !reached || !reflect.DeepEqual(flags, want) {
					t.Fatalf("existing boundary not reached: %v", flags)
				}
				if action == "replace" || action == "fifo" {
					preparationOpenClosed(t, opened, fd)
				}
			})
		}
	}

	for _, injected := range []error{syscall.ENOENT, syscall.EIO, syscall.EACCES, syscall.ENOSPC} {
		t.Run("terminal-fallback-"+injected.Error(), func(t *testing.T) {
			r, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			calls := 0
			preparationOpen = func(_ *os.Root, name string, flag int, _ os.FileMode, _ bool) (*os.File, error) {
				calls++
				if calls == 1 {
					if flag != exclusive {
						t.Fatal(flag)
					}
					fixture.Write(t, filepath.Join(r.CommonDir, name), nil)
					return nil, syscall.EEXIST
				}
				if flag != os.O_RDWR {
					t.Fatal("fallback may not create", flag)
				}
				return nil, injected
			}
			lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
			preparationOpenRefused(t, lock, err, wire.CodeUnsupportedFilesystem)
			if calls != 2 || !strings.Contains(err.Error(), injected.Error()) {
				t.Fatalf("fallback not terminal: %d %v", calls, err)
			}
		})
	}

	for _, when := range []string{"before-flock", "after-flock", "parent-after-open"} {
		t.Run("identity-"+when, func(t *testing.T) {
			r, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			path := filepath.Join(r.CommonDir, preparationLockFileName)
			original, originalFlock := preparationOpen, flockNB
			var opened *os.File
			fd, flockCalls := -1, 0
			reached := false
			replace := func() {
				reached = true
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
				fixture.Write(t, path, nil)
			}
			preparationOpen = func(root *os.Root, name string, flag int, mode os.FileMode, dir bool) (*os.File, error) {
				f, err := original(root, name, flag, mode, dir)
				if err == nil {
					opened, fd = f, int(f.Fd())
					if when == "before-flock" {
						replace()
					}
					if when == "parent-after-open" {
						if err := os.Rename(r.CommonDir, r.CommonDir+".old"); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if err := os.Remove(r.CommonDir); err != nil {
								t.Error(err)
							}
							if err := os.Rename(r.CommonDir+".old", r.CommonDir); err != nil {
								t.Error(err)
							}
						})
						if err := os.Mkdir(r.CommonDir, 0700); err != nil {
							t.Fatal(err)
						}
						reached = true
					}
				}
				return f, err
			}
			flockNB = func(fd int) error {
				flockCalls++
				err := originalFlock(fd)
				if err == nil && when == "after-flock" {
					replace()
				}
				return err
			}
			lock, err := AcquirePreparation(context.Background(), repo, LockOptions{})
			preparationOpenRefused(t, lock, err, wire.CodeUnsupportedFilesystem)
			preparationOpenClosed(t, opened, fd)
			wantFlock := 1
			if when == "before-flock" || when == "parent-after-open" {
				wantFlock = 0
			}
			if !reached || flockCalls != wantFlock {
				t.Fatalf("boundary not reached: %t flock%d", reached, flockCalls)
			}
		})
	}

	// Direct helper calls pinpoint guards without tying public acquisition tests
	// to an incidental ctx.Err call count in the surrounding shared caller.
	for _, at := range []int{1, 2, 3} {
		for _, cancelInstead := range []bool{true, false} {
			t.Run(fmt.Sprintf("guard-%d-cancel-%t", at, cancelInstead), func(t *testing.T) {
				r, _ := preparationOpenFixture(t)
				preparationOpenRestore(t)
				root, err := openRoot(r.CommonDir)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				deadline := time.Now().Add(100 * time.Millisecond)
				ctx := &preparationGuardContext{Context: base, at: at, do: func() {
					if cancelInstead {
						cancel()
					} else {
						time.Sleep(time.Until(deadline) + time.Millisecond)
					}
				}}
				calls := 0
				preparationOpen = func(_ *os.Root, name string, flag int, _ os.FileMode, _ bool) (*os.File, error) {
					calls++
					if calls != 1 || flag != exclusive {
						t.Fatal("unexpected second open", calls, flag)
					}
					fixture.Write(t, filepath.Join(r.CommonDir, name), nil)
					return nil, syscall.EEXIST
				}
				f, _, err := openPreparationFile(ctx, root, nil, filepath.Join(r.CommonDir, preparationLockFileName), deadline, 100*time.Millisecond)
				if f != nil {
					f.Close()
					t.Fatal("guard allowed open")
				}
				if cancelInstead {
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				} else if wire.CodeOf(err) != wire.CodeLockTimeout {
					t.Fatal(err)
				}
				wantCalls := 1
				if at == 1 {
					wantCalls = 0
				}
				if ctx.calls != at || calls != wantCalls {
					t.Fatalf("guard not reached: ctx%d open%d", ctx.calls, calls)
				}
			})
		}
	}

	for _, cancelInstead := range []bool{true, false} {
		t.Run(fmt.Sprintf("original-budget-after-open-cancel-%t", cancelInstead), func(t *testing.T) {
			_, repo := preparationOpenFixture(t)
			preparationOpenRestore(t)
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := preparationOpen
			var opened *os.File
			fd, calls := -1, 0
			preparationOpen = func(root *os.Root, name string, flag int, mode os.FileMode, dir bool) (*os.File, error) {
				calls++
				f, err := original(root, name, flag, mode, dir)
				if err == nil {
					opened, fd = f, int(f.Fd())
					if cancelInstead {
						cancel()
					} else {
						time.Sleep(150 * time.Millisecond)
					}
				}
				return f, err
			}
			lock, err := AcquirePreparation(base, repo, LockOptions{Wait: 100 * time.Millisecond})
			if lock != nil {
				lock.Close()
				t.Fatal("expired acquisition succeeded")
			}
			if cancelInstead {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			} else if wire.CodeOf(err) != wire.CodeLockTimeout {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatal("open not reached", calls)
			}
			preparationOpenClosed(t, opened, fd)
		})
	}

	t.Run("writer-bypasses-preparation-open", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		preparationOpenRestore(t)
		calls := 0
		preparationOpen = func(*os.Root, string, int, os.FileMode, bool) (*os.File, error) { calls++; return nil, syscall.EIO }
		lock, err := AcquireLock(context.Background(), repo, LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatal("writer used preparation path", calls)
		}
	})
	t.Run("gate-replaced-while-waiting", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		preparationOpenRestore(t)
		path := filepath.Join(repo.CommonDir, preparationLockFileName)
		holder, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			t.Fatal(e)
		}
		defer holder.Close()
		if e = withFD(holder, flockExclusiveNB); e != nil {
			t.Fatal(e)
		}
		opened := make(chan struct{})
		old := preparationOpen
		preparationOpen = func(r *os.Root, n string, f int, m os.FileMode, d bool) (*os.File, error) {
			p, e := old(r, n, f, m, d)
			if e == nil {
				close(opened)
			}
			return p, e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		result := make(chan error, 1)
		go func() {
			l, e := AcquirePreparation(ctx, repo, LockOptions{Poll: time.Millisecond})
			if l != nil {
				l.Close()
			}
			result <- e
		}()
		joined := false
		defer func() {
			cancel()
			holder.Close()
			if !joined {
				<-result
			}
		}()
		select {
		case <-opened:
		case <-ctx.Done():
			t.Fatal("actual gate open not reached")
		}
		if e = os.Rename(path, path+".old"); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, nil, 0600); e != nil {
			t.Fatal(e)
		}
		if e = holder.Close(); e != nil {
			t.Fatal(e)
		}
		e = <-result
		joined = true
		if wire.CodeOf(e) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("replacement after actual gate open: %v", e)
		}
	})
}
