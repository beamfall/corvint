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

const preparationLockFileName = "taskman.prepare.lock"

type preparationObserverKey struct{}

// PreparationObservation separates admission wait and preparation hold from the
// CAL-V0-026 native writer-lock hold. Acquired is false on failed acquisition;
// in that case Hold is zero. Successful holds are observed only after release.
type PreparationObservation struct {
	Wait, Hold time.Duration
	Acquired   bool
	// Released is true only when both the final gate and slot retired cleanly.
	Released bool
	Err      error
}

// WithPreparationObserver records preparation admission independently of
// WithLockObserver. The callback must not block. It confers no write authority.
func WithPreparationObserver(ctx context.Context, observe func(PreparationObservation)) context.Context {
	return context.WithValue(ctx, preparationObserverKey{}, observe)
}

// PreparationLock serializes cooperating lease preparations across processes.
// It is not a Lock and cannot authorize a transaction session. The compatibility
// gate remains inert; separate fixed slots carry only disposable scheduling rank.
// All these files live outside the journal and confer no evidence or authority.
// Close leaves the inert file in place so another caller cannot lock a new inode.
// Identity is checked through acquisition, not continuously while held.
type PreparationLock struct {
	lock    *Lock
	slot    *preparationFile
	mu      sync.Mutex
	closed  bool
	result  error
	wait    time.Duration
	observe func(PreparationObservation)
}

// AcquirePreparation uses the same bounded, cancellable acquisition and pinned
// identity checks as the writer lock, on the distinct taskman.prepare.lock file.
// Acquisition defaults to and is capped at 30 seconds total; an explicit
// opts.CallerWait (CAL-V0-111) replaces that bound, up to MaxCallerLockWait,
// and is spent across the same phases without restarting. Published live
// registrations enter in rank order; pre-registration scheduling and old clients
// have no ordering guarantee. The fixed capacity includes the serving holder.
// Callers must acquire preparation before any writer lock and release it after
// monitor teardown, never across external execution or a task's lease lifetime.
func AcquirePreparation(ctx context.Context, repo *intent.Repository, opts LockOptions) (*PreparationLock, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	wait := opts.wait()
	poll := opts.Poll
	if poll <= 0 {
		poll = DefaultLockPoll
	}
	b := &preparationBudget{ctx: ctx, deadline: started.Add(wait), wait: wait, poll: poll, phase: "registry"}
	if repo != nil {
		b.path = filepath.Join(repo.CommonDir, preparationLockFileName)
	}
	observe, _ := ctx.Value(preparationObserverKey{}).(func(PreparationObservation))
	l, err := acquireOrderedPreparation(repo, b, started)
	if err != nil {
		if wire.CodeOf(err) == wire.CodeLockTimeout {
			err = wire.Errorf(wire.CodeLockTimeout, b.path, "preparation acquisition exceeded %v; phase=%s registered=%t rank=%d; cause: %v", b.wait, b.phase, b.rank != 0, b.rank, err)
		}
		if observe != nil {
			observe(PreparationObservation{Wait: time.Since(started), Err: err})
		}
		return nil, err
	}
	l.observe = observe
	return l, nil
}

// Path reports the fixed admission filename, not a writer-session capability.
func (l *PreparationLock) Path() string { return l.lock.Path() }

// Close releases and closes this handle exactly once and leaves its file intact.
// As with Lock.Close, callers must retain any release/close error as unresolved.
func (l *PreparationLock) Close() error {
	l.mu.Lock()
	if l.closed {
		err := l.result
		l.mu.Unlock()
		return err
	}
	released, gateErr := closePreparationGate(l.lock)
	slotErr := l.slot.close()
	l.result = errors.Join(gateErr, slotErr)
	l.closed = true
	err := l.result
	o := PreparationObservation{Wait: l.wait, Hold: released.Sub(l.lock.acquired), Acquired: true, Released: err == nil, Err: err}
	observe := l.observe
	l.mu.Unlock()
	if observe != nil {
		observe(o)
	}
	return err
}

func closePreparationGate(l *Lock) (time.Time, error) {
	relErr := withFD(l.f, preparationUnlock)
	released := time.Now()
	closeErr := preparationCloseFile(l.f)
	if relErr != nil {
		released = time.Now()
	} // A failed unlock may retain ownership until close.
	return released, errors.Join(relErr, closeErr)
}

func acquireOrderedPreparation(repo *intent.Repository, b *preparationBudget, started time.Time) (result *PreparationLock, err error) {
	s, err := openPreparationScope(repo, b)
	if err != nil {
		return nil, err
	}
	var registry, slot *preparationFile
	var gate *Lock
	defer func() {
		// Temporary ownership must retire before a handle can be reported successful.
		err = withCleanup(err, errors.Join(registry.close(), s.close()))
		if err == nil {
			err = b.check()
		}
		if err != nil {
			if gate != nil {
				_, e := closePreparationGate(gate)
				err = withCleanup(err, e)
			}
			err = withCleanup(err, slot.close())
			result = nil
		}
	}()
	registry, err = s.open(preparationRegistryName, true, b)
	if err != nil {
		return nil, err
	}
	slot, err = s.register(registry, b)
	if err != nil {
		return nil, err
	}
	if err = s.awaitHead(registry, slot, b); err != nil {
		return nil, err
	}
	b.phase = "gate"
	gate, err = acquirePreparationGate(repo, b)
	if err != nil {
		return nil, err
	}
	if err = s.validate(slot); err != nil {
		return nil, err
	}
	if err = s.validate(registry); err != nil {
		return nil, err
	}
	rank, e := preparationRank(slot)
	if e != nil {
		return nil, e
	}
	if rank != b.rank {
		return nil, fsErr(slot.name, "own rank changed after gate acquisition")
	}
	if err = b.check(); err != nil {
		return nil, err
	}
	admissionReached("entered", b.rank)
	return &PreparationLock{lock: gate, slot: slot, wait: gate.acquired.Sub(started)}, nil
}

// Private OS seam for reached-boundary refusal tests. The product path uses the
// same no-follow, nonblocking pinned-root open as the writer lock.
var preparationOpen = safeopen.InRoot

// openPreparationFile distinguishes initial creation from an existing open.
// Only EEXIST permits a second open; ENOENT never authorizes recreation. The
// returned preimage must reach the caller's shared inode checks after a race.
func openPreparationFile(ctx context.Context, root *os.Root, pre os.FileInfo, path string, deadline time.Time, wait time.Duration) (*os.File, os.FileInfo, error) {
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(deadline) {
			return wire.Errorf(wire.CodeLockTimeout, path, "lock acquisition exceeded %v", wait)
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, nil, err
	}
	if pre == nil {
		f, err := preparationOpen(root, preparationLockFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644, false)
		if err == nil {
			return f, nil, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, nil, fsErr(path, "cannot exclusively create preparation lock: %v", err)
		}
		if err := check(); err != nil {
			return nil, nil, err
		}
		pre, err = root.Lstat(preparationLockFileName)
		if err != nil {
			return nil, nil, fsErr(path, "cannot stat existing preparation lock after create collision: %v", err)
		}
		if !pre.Mode().IsRegular() {
			return nil, nil, fsErr(path, "existing preparation lock after create collision is not a regular file (mode %v)", pre.Mode())
		}
		if err := check(); err != nil {
			return nil, nil, err
		}
	}
	f, err := preparationOpen(root, preparationLockFileName, os.O_RDWR, 0o644, false)
	if err != nil {
		return nil, nil, fsErr(path, "cannot open existing preparation lock: %v", err)
	}
	return f, pre, nil
}
