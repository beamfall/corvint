package authority

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LockOptions bounds one acquisition.
type LockOptions struct {
	// Wait is the maximum time to wait for the lock. Zero or negative means
	// DefaultLockWait; anything above MaxLockWait is clamped to it (§1).
	Wait time.Duration
	// Poll is the interval between non-blocking attempts while contended.
	// Zero or negative means DefaultLockPoll.
	Poll time.Duration
}

// EffectiveWait is the wait AcquireLock actually uses for a requested
// value: the §1 default when unset, the §1 maximum when exceeded.
func EffectiveWait(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultLockWait
	}
	if d > MaxLockWait {
		return MaxLockWait
	}
	return d
}

// Lock is a held exclusive flock on `<git-common-dir>/taskman.lock`. It
// owns one open file description; the lock lives exactly as long as that
// descriptor and is released by Close (or by the OS when the process dies,
// §5.3 C12). A Lock proves mutual exclusion among corvint-tasks processes on this
// host and nothing else: not ownership, not an actor, not capacity.
type Lock struct {
	mu       sync.Mutex
	f        *os.File
	path     string
	acquired time.Time
	closed   bool
	observe  func(time.Duration)
}

// AcquireLock takes the exclusive flock on the repository authority's lock
// file (§5.2 first line; TM-V0-009) with a bounded, cancellable wait.
//
// Rules:
//   - The repository comes from intent.Resolve; it is re-resolved here and
//     any drift of the common dir refuses UNSUPPORTED_FILESYSTEM.
//   - The lock path must be `<CommonDir>/taskman.lock`, opened through an
//     os.Root confined to the common dir; a symlink or non-regular file at
//     that name, a symlink in the common-dir path, or a file whose identity
//     changes between stat, open and acquisition is refused.
//   - The file is created when absent (a mutating command may) and is
//     never truncated, written or read: its content is meaningless and is
//     no ownership proof. Read verbs never call this.
//   - Contention polls `flock(LOCK_EX|LOCK_NB)` every opts.Poll until
//     EffectiveWait(opts.Wait) elapses, then fails LOCK_TIMEOUT; a done ctx
//     returns ctx.Err() unchanged. On every failure the descriptor is
//     closed before returning, so a failed call never holds the lock.
//   - ENOLCK/EOPNOTSUPP/ENOTSUP fail UNSUPPORTED_FILESYSTEM (§5.1).
func AcquireLock(ctx context.Context, repo *intent.Repository, opts LockOptions) (*Lock, error) {
	return acquireLock(ctx, repo, opts, false)
}

// acquireLock admits only the two fixed native coordination filenames. A
// preparation handle stays private to its distinct wrapper and cannot authorize
// a writer session. Both paths retain the repository writer-lock tuple checks.
func acquireLock(ctx context.Context, repo *intent.Repository, opts LockOptions, preparation bool) (*Lock, error) {
	name := LockFileName
	if preparation {
		name = preparationLockFileName
	}
	if !supportedPlatform {
		return nil, fsErr("", "unsupported platform")
	}
	start := time.Now()
	if ctx == nil {
		ctx = context.Background()
	}
	if repo == nil {
		return nil, fsErr("", "no repository authority was resolved")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wantWriterLock := filepath.Join(repo.CommonDir, LockFileName)
	wantLock := filepath.Join(repo.CommonDir, name)
	if repo.LockPath != wantWriterLock {
		return nil, fsErr(repo.LockPath, "lock path is not %s (repository identity drift)", wantWriterLock)
	}
	again, err := intent.Resolve(repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	if again.CommonDir != repo.CommonDir || again.LockPath != repo.LockPath {
		return nil, fsErr(repo.CommonDir, "common dir re-resolved to %s (repository identity drift)", again.CommonDir)
	}
	if err := intent.CheckNoSymlink(repo.CommonDir); err != nil {
		return nil, err
	}
	intended, err := os.Lstat(repo.CommonDir)
	if err != nil {
		return nil, fsErr(repo.CommonDir, "cannot stat: %v", err)
	}
	root, err := openRoot(repo.CommonDir)
	if err != nil {
		return nil, fsErr(repo.CommonDir, "cannot open the common dir: %v", err)
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(intended, opened) {
		return nil, fsErr(repo.CommonDir, "common directory identity drift")
	}

	pre, err := root.Lstat(name)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, fsErr(wantLock, "cannot stat: %v", err)
	}
	if exists {
		if pre.Mode()&os.ModeSymlink != 0 {
			return nil, fsErr(wantLock, "%s is a symlink", name)
		}
		if !pre.Mode().IsRegular() {
			return nil, fsErr(wantLock, "%s is not a regular file (mode %v)", name, pre.Mode())
		}
	}
	wait := EffectiveWait(opts.Wait)
	deadline := start.Add(wait)
	// No O_TRUNC, no O_APPEND: the file's bytes are never touched.
	var f *os.File
	if preparation {
		f, pre, err = openPreparationFile(ctx, root, pre, wantLock, deadline, wait)
		if err != nil {
			return nil, err
		}
		exists = pre != nil
	} else {
		f, err = safeopen.InRoot(root, name, os.O_RDWR|os.O_CREATE, 0o644, false)
		if err != nil {
			return nil, fsErr(wantLock, "cannot open or create: %v", err)
		}
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fsErr(wantLock, "cannot stat the opened lock file: %v", err)
	}
	if !st.Mode().IsRegular() {
		f.Close()
		return nil, fsErr(wantLock, "opened lock file is not a regular file (mode %v)", st.Mode())
	}
	if exists && !os.SameFile(pre, st) {
		f.Close()
		return nil, fsErr(wantLock, "lock file replaced between stat and open (identity drift)")
	}
	post, err := root.Lstat(name)
	if err != nil || post.Mode()&os.ModeSymlink != 0 || !os.SameFile(post, st) {
		f.Close()
		return nil, fsErr(wantLock, "lock file replaced after open (identity drift)")
	}

	poll := opts.Poll
	if poll <= 0 {
		poll = DefaultLockPoll
	}
	var acquired time.Time
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, wire.Errorf(wire.CodeLockTimeout, wantLock, "lock acquisition exceeded %v", wait)
		}
		err := withFD(f, flockNB)
		if err == nil {
			acquired = time.Now()
			break
		}
		if isInterrupted(err) {
			continue
		}
		if !isWouldBlock(err) {
			f.Close()
			if isLockUnsupported(err) {
				return nil, fsErr(wantLock, "flock is unsupported on this filesystem: %v", err)
			}
			return nil, fsErr(wantLock, "flock failed: %v", err)
		}
		now := time.Now()
		if !now.Before(deadline) {
			f.Close()
			return nil, wire.Errorf(wire.CodeLockTimeout, wantLock, "%s is held by another process; waited %v", name, wait)
		}
		if err := ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		sleep := poll
		if rem := deadline.Sub(now); rem < sleep {
			sleep = rem
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	if err := ctx.Err(); err != nil {
		f.Close()
		return nil, err
	}
	if !time.Now().Before(deadline) {
		f.Close()
		return nil, wire.Errorf(wire.CodeLockTimeout, wantLock, "lock acquisition exceeded %v", wait)
	}
	// The flock is on the open file description; if the directory entry
	// was unlinked or replaced while waiting, the lock guards an orphan
	// and a concurrent process could hold the new file. Refuse.
	cur, err := root.Lstat(name)
	if err != nil || cur.Mode()&os.ModeSymlink != 0 || !os.SameFile(cur, st) {
		relErr := withFD(f, flockRelease)
		closeErr := f.Close()
		return nil, withCleanup(fsErr(wantLock, "lock file was removed or replaced during acquisition (identity drift)"), errors.Join(relErr, closeErr))
	}
	currentDir, err := os.Lstat(repo.CommonDir)
	if err != nil || !os.SameFile(opened, currentDir) {
		f.Close()
		return nil, fsErr(repo.CommonDir, "common directory moved during lock acquisition")
	}
	if err := ctx.Err(); err != nil {
		f.Close()
		return nil, err
	}
	if !time.Now().Before(deadline) {
		f.Close()
		return nil, wire.Errorf(wire.CodeLockTimeout, wantLock, "lock acquisition exceeded %v", wait)
	}
	var observe func(time.Duration)
	if !preparation {
		observe, _ = ctx.Value(lockObserverKey{}).(func(time.Duration))
	}
	return &Lock{f: f, path: wantLock, acquired: acquired, observe: observe}, nil
}

// Path is the lock file path.
func (l *Lock) Path() string { return l.path }

// HeldSince is when the flock was granted.
func (l *Lock) HeldSince() time.Time { return l.acquired }

// Close releases the flock and closes the descriptor. It is idempotent;
// the first call reports any unlock or close failure, so a lock that could
// not be released is never reported as released. A Lock must be closed by
// the code that acquired it; no finalizer stands in.
func (l *Lock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	relErr := withFD(l.f, flockRelease)
	released := time.Now()
	closeErr := l.f.Close()
	if relErr != nil {
		return withCleanup(fsErr(l.path, "flock(LOCK_UN) failed: %v", relErr), closeErr)
	}
	if closeErr != nil {
		return fsErr(l.path, "closing the lock descriptor failed: %v", closeErr)
	}
	if l.observe != nil {
		l.observe(released.Sub(l.acquired))
	}
	return nil
}

// acquirePreparationGate spends the original admission deadline. It never feeds
// a remaining duration through EffectiveWait, which would reset an expired wait.
// Public writer acquisition above keeps its original behavior and cleanup scope.
func acquirePreparationGate(repo *intent.Repository, b *preparationBudget) (lock *Lock, err error) {
	s, err := openPreparationScope(repo, b)
	if err != nil {
		return nil, err
	}
	var p *preparationFile
	defer func() {
		err = withCleanup(err, s.close())
		if err != nil {
			err = withCleanup(err, p.close())
			lock = nil
		}
	}()
	p, err = s.open(preparationLockFileName, true, b)
	if err != nil {
		return nil, err
	}
	if err = preparationTake(p, b, flockNB); err != nil {
		return nil, err
	}
	acquired := p.acquired
	if err = s.validate(p); err != nil {
		return nil, err
	}
	if err = b.check(); err != nil {
		return nil, err
	}
	return &Lock{f: p.f, path: filepath.Join(repo.CommonDir, preparationLockFileName), acquired: acquired}, nil
}
