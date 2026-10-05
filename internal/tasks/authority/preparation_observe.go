package authority

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// PreparationQueue is a racy, lock-free observation of published preparation
// registrations (CAL-V0-074). It never registers, flocks, creates, truncates
// or writes. Liveness comes from a platform lock query that acquires nothing;
// a slot whose owner lock is not observed is stale scheduling bytes and is
// ignored. Each slot is observed at a different instant, so counts can
// include a registry-held scan probe or miss a concurrent publication.
type PreparationQueue struct {
	// Method names the lock query; empty when NotObserved is set.
	Method string
	// NotObserved names why liveness could not be read; empty otherwise.
	NotObserved string
	// Registered counts live slots carrying a valid published record,
	// including a currently serving holder, whose slot stays live until Close.
	Registered int
	// Unpublished counts live slots without a readable valid record.
	Unpublished int
	// MaxRank is the largest observed live rank; zero when none is live.
	MaxRank uint64
	// RegistryActive reports the registry lock held at the start or end of
	// the sweep, when a registrant may be publishing or probing slots.
	RegistryActive bool
}

// WouldBeRank is the rank a registration published now would receive,
// known only when every observed live slot carried a valid record.
func (q PreparationQueue) WouldBeRank() (uint64, bool) {
	if q.NotObserved != "" || q.Unpublished != 0 || q.MaxRank == math.MaxUint64 {
		return 0, false
	}
	return q.MaxRank + 1, true
}

// Private seam: tests replace it to prove the observer reports, rather than
// guesses, an unavailable lock query.
var preparationLockViewLoad = loadPreparationLockView

// ObservePreparationQueue reads the fixed slot namespace under the pinned Git
// common directory without taking any lock and without writing.
func ObservePreparationQueue(repo *intent.Repository) PreparationQueue {
	q, err := observePreparationQueue(repo)
	if err != nil {
		return PreparationQueue{NotObserved: err.Error()}
	}
	return q
}

func observePreparationQueue(repo *intent.Repository) (PreparationQueue, error) {
	q := PreparationQueue{}
	if !supportedPlatform || repo == nil {
		return q, errors.New("no supported repository authority")
	}
	view, err := preparationLockViewLoad()
	if err != nil {
		return q, err
	}
	q.Method = view.method()
	if err = intent.CheckNoSymlink(repo.CommonDir); err != nil {
		return q, errors.New("common directory is not observable")
	}
	intended, err := os.Lstat(repo.CommonDir)
	if err != nil {
		return q, errors.New("common directory is not observable")
	}
	root, err := openRoot(repo.CommonDir)
	if err != nil {
		return q, errors.New("common directory is not observable")
	}
	defer root.Close()
	if opened, e := root.Stat("."); e != nil || !os.SameFile(intended, opened) {
		return q, errors.New("common directory identity drift")
	}
	active, err := observeRegistry(root, view)
	if err != nil {
		return q, err
	}
	seen := false
	for i := 0; i < preparationSlots; i++ {
		live, rank, e := observeSlot(root, preparationSlotName(i), view)
		if e != nil {
			return q, e
		}
		if !live {
			continue
		}
		seen = true
		if rank == 0 {
			q.Unpublished++
			continue
		}
		q.Registered++
		if rank > q.MaxRank {
			q.MaxRank = rank
		}
	}
	// Probes happen only under registry ownership; a quiet registry at the
	// end matters only when something was seen live.
	end := false
	if seen || active {
		if end, err = observeRegistry(root, view); err != nil {
			return q, err
		}
	}
	q.RegistryActive = active || end
	return q, nil
}

// observeInert opens an existing fixed coordination file read-only. It
// returns nil for an absent file and refuses a non-regular or drifting one.
func observeInert(root *os.Root, name string) (*os.File, os.FileInfo, error) {
	pre, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil || !pre.Mode().IsRegular() {
		return nil, nil, errors.New("preparation file " + name + " is not an observable regular file")
	}
	f, err := safeopen.InRoot(root, name, os.O_RDONLY, 0, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, errors.New("preparation file " + name + " is not readable")
	}
	st, err := f.Stat()
	if err != nil || !os.SameFile(pre, st) {
		f.Close()
		return nil, nil, errors.New("preparation file " + name + " identity drift")
	}
	return f, st, nil
}

func observeRegistry(root *os.Root, view preparationLockView) (bool, error) {
	f, st, err := observeInert(root, preparationRegistryName)
	if f == nil || err != nil {
		return false, err
	}
	defer f.Close()
	return view.held(f, st)
}

// observeSlot reports liveness and, for a live slot, its rank, or zero when
// the live record is absent, partial or malformed at the read instant.
func observeSlot(root *os.Root, name string, view preparationLockView) (bool, uint64, error) {
	f, st, err := observeInert(root, name)
	if f == nil || err != nil {
		return false, 0, err
	}
	defer f.Close()
	live, err := view.held(f, st)
	if err != nil || !live {
		return false, 0, err
	}
	var raw [17]byte
	n, err := f.ReadAt(raw[:], 0)
	if err != nil && err != io.EOF {
		return true, 0, nil
	}
	if n != 16 || string(raw[:4]) != "CPA1" || binary.BigEndian.Uint32(raw[4:8]) != 0 {
		return true, 0, nil
	}
	return true, binary.BigEndian.Uint64(raw[8:16]), nil
}

// preparationLockView answers whether another open file description holds a
// lock on an open file, without acquiring one.
type preparationLockView interface {
	method() string
	held(f *os.File, st os.FileInfo) (bool, error)
}
