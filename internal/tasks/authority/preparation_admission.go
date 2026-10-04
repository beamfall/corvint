package authority

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const preparationRegistryName = "taskman.prepare.registry.lock"
const preparationSlots = 64

// These seams belong only to preparation. Writer acquisition and its observers
// retain their existing behavior. Hooks are replaced only by sequential tests.
var (
	admissionOpen        = safeopen.InRoot
	admissionFlock       = flockExclusiveNB
	preparationCloseFile = func(f *os.File) error { return f.Close() }
	preparationCloseRoot = func(r *os.Root) error { return r.Close() }
	preparationUnlock    = flockUnlock
	admissionWrite       = func(f *os.File, b []byte) (int, error) { return f.WriteAt(b, 0) }
	admissionReached     = func(string, uint64) {}
)

type preparationBudget struct {
	ctx         context.Context
	deadline    time.Time
	wait, poll  time.Duration
	path, phase string
	rank        uint64
}

func (b *preparationBudget) check() error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if !time.Now().Before(b.deadline) {
		return wire.Errorf(wire.CodeLockTimeout, b.path, "preparation acquisition exceeded %v; phase=%s registered=%t rank=%d", b.wait, b.phase, b.rank != 0, b.rank)
	}
	return nil
}

func (b *preparationBudget) pause() error {
	if err := b.check(); err != nil {
		return err
	}
	d := b.poll
	if left := time.Until(b.deadline); left < d {
		d = left
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case <-t.C:
		return b.check()
	}
}

// A scope pins the Git common directory. It never conveys writer authority.
type preparationScope struct {
	root *os.Root
	path string
	info os.FileInfo
}

func openPreparationScope(repo *intent.Repository, b *preparationBudget) (s *preparationScope, err error) {
	if err = b.check(); err != nil {
		return nil, err
	}
	if !supportedPlatform || repo == nil {
		return nil, fsErr("", "no supported repository authority")
	}
	if repo.LockPath != filepath.Join(repo.CommonDir, LockFileName) {
		return nil, fsErr(repo.LockPath, "repository lock tuple drift")
	}
	again, err := intent.Resolve(repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	if again.CommonDir != repo.CommonDir || again.LockPath != repo.LockPath {
		return nil, fsErr(repo.CommonDir, "repository identity drift")
	}
	if err = intent.CheckNoSymlink(repo.CommonDir); err != nil {
		return nil, err
	}
	intended, err := os.Lstat(repo.CommonDir)
	if err != nil {
		return nil, fsErr(repo.CommonDir, "cannot stat common directory: %v", err)
	}
	r, err := openRoot(repo.CommonDir)
	if err != nil {
		return nil, fsErr(repo.CommonDir, "cannot open common directory: %v", err)
	}
	s = &preparationScope{root: r, path: repo.CommonDir, info: intended}
	if err = s.check(); err == nil {
		err = b.check()
	}
	if err != nil {
		return nil, withCleanup(err, s.close())
	}
	return s, nil
}

func (s *preparationScope) check() error {
	opened, e := s.root.Stat(".")
	current, f := os.Lstat(s.path)
	if e != nil || f != nil || !os.SameFile(s.info, opened) || !os.SameFile(s.info, current) || current.Mode()&os.ModeSymlink != 0 {
		return fsErr(s.path, "common directory identity drift")
	}
	return intent.CheckNoSymlink(s.path)
}

func (s *preparationScope) close() error { return preparationCloseRoot(s.root) }

type preparationFile struct {
	f        *os.File
	name     string
	info     os.FileInfo
	held     bool
	acquired time.Time
}

func (p *preparationFile) close() error {
	if p == nil || p.f == nil {
		return nil
	}
	f := p.f
	p.f = nil
	var err error
	if p.held {
		err = withFD(f, preparationUnlock)
	}
	p.held = false
	return errors.Join(err, preparationCloseFile(f))
}

func (s *preparationScope) validate(p *preparationFile) error {
	if err := s.check(); err != nil {
		return err
	}
	st, err := p.f.Stat()
	cur, e := s.root.Lstat(p.name)
	if err != nil || e != nil || !cur.Mode().IsRegular() || !os.SameFile(p.info, st) || !os.SameFile(st, cur) {
		return fsErr(filepath.Join(s.path, p.name), "preparation file identity drift")
	}
	return nil
}

func preparationSlotName(i int) string { return fmt.Sprintf("taskman.prepare.slot.%02d", i) }

// Fixed allowlist, finite absent/create/EEXIST/open protocol. No open truncates.
func (s *preparationScope) open(name string, create bool, b *preparationBudget) (*preparationFile, error) {
	allowed := name == preparationRegistryName || name == preparationLockFileName
	for i := 0; i < preparationSlots && !allowed; i++ {
		allowed = name == preparationSlotName(i)
	}
	if !allowed {
		return nil, fsErr(name, "unsupported preparation filename")
	}
	if err := b.check(); err != nil {
		return nil, err
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	pre, err := s.root.Lstat(name)
	if err != nil && !os.IsNotExist(err) {
		return nil, fsErr(name, "cannot stat preparation file: %v", err)
	}
	if os.IsNotExist(err) && !create {
		return nil, nil
	}
	if pre != nil && !pre.Mode().IsRegular() {
		return nil, fsErr(name, "preparation file is not regular")
	}
	var f *os.File
	if name == preparationLockFileName {
		f, pre, err = openPreparationFile(b.ctx, s.root, pre, filepath.Join(s.path, name), b.deadline, b.wait)
	} else {
		if pre == nil {
			f, err = admissionOpen(s.root, name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644, false)
			if errors.Is(err, os.ErrExist) {
				if err = b.check(); err != nil {
					return nil, err
				}
				pre, err = s.root.Lstat(name)
				if err != nil || !pre.Mode().IsRegular() {
					return nil, fsErr(name, "invalid create collision: %v", err)
				}
			} else if err != nil {
				return nil, fsErr(name, "cannot create preparation file: %v", err)
			}
		}
		if f == nil {
			if err = b.check(); err != nil {
				return nil, err
			}
			f, err = admissionOpen(s.root, name, os.O_RDWR, 0644, false)
		}
	}
	if err != nil {
		if name == preparationLockFileName {
			return nil, err
		}
		return nil, fsErr(name, "cannot open preparation file: %v", err)
	}
	st, err := f.Stat()
	p := &preparationFile{f: f, name: name, info: st}
	if err != nil || !st.Mode().IsRegular() || (pre != nil && !os.SameFile(pre, st)) {
		return nil, withCleanup(fsErr(name, "opened preparation file identity drift"), p.close())
	}
	if err = s.validate(p); err == nil {
		err = b.check()
	}
	if err != nil {
		return nil, withCleanup(err, p.close())
	}
	return p, nil
}

func preparationTake(p *preparationFile, b *preparationBudget, flock func(int) error) error {
	for {
		if err := b.check(); err != nil {
			return err
		}
		err := withFD(p.f, flock)
		if err == nil {
			p.acquired = time.Now()
			p.held = true
			return b.check()
		}
		if isInterrupted(err) {
			continue
		}
		if !isWouldBlock(err) {
			return fsErr(p.name, "preparation flock failed: %v", err)
		}
		if err := b.pause(); err != nil {
			return err
		}
	}
}

func preparationRank(p *preparationFile) (uint64, error) {
	st, err := p.f.Stat()
	if err != nil || st.Size() != 16 {
		return 0, fsErr(p.name, "invalid live preparation record length: %v", err)
	}
	var raw [16]byte
	n, err := p.f.ReadAt(raw[:], 0)
	if err != nil || n != 16 || string(raw[:4]) != "CPA1" || binary.BigEndian.Uint32(raw[4:8]) != 0 {
		return 0, fsErr(p.name, "invalid live preparation record: %v", err)
	}
	rank := binary.BigEndian.Uint64(raw[8:])
	if rank == 0 {
		return 0, fsErr(p.name, "zero live preparation rank")
	}
	return rank, nil
}

// Called only with registry ownership. Successful probes are unowned scratch;
// a failed probe is never unlocked. The own slot uses its retained descriptor.
func (s *preparationScope) scan(registry, own *preparationFile, b *preparationBudget, register bool) (free *preparationFile, minimum, maximum uint64, err error) {
	seen := map[uint64]bool{}
	freeName := ""
	defer func() {
		if err != nil {
			err = withCleanup(err, free.close())
			free = nil
		}
	}()
	for i := 0; i < preparationSlots; i++ {
		if err = b.check(); err != nil {
			return
		}
		name := preparationSlotName(i)
		p := own
		if own == nil || own.name != name {
			p, err = s.open(name, false, b)
			if err != nil {
				return
			}
			if p == nil {
				if freeName == "" {
					freeName = name
				}
				continue
			}
			e := withFD(p.f, admissionFlock)
			if e == nil {
				p.held = true
				if register && free == nil {
					free = p
				} else {
					err = p.close()
					if err != nil {
						return
					}
				}
				continue
			}
			if !isWouldBlock(e) {
				err = withCleanup(fsErr(name, "slot probe failed: %v", e), p.close())
				return
			}
		}
		var rank uint64
		rank, err = preparationRank(p)
		if err == nil {
			err = s.validate(p)
		}
		if p != own {
			err = withCleanup(err, p.close())
		}
		if err != nil {
			return
		}
		if seen[rank] {
			err = fsErr(name, "duplicate live preparation rank")
			return
		}
		seen[rank] = true
		if minimum == 0 || rank < minimum {
			minimum = rank
		}
		if rank > maximum {
			maximum = rank
		}
	}
	if err = s.validate(registry); err != nil {
		return
	}
	if own != nil {
		var rank uint64
		rank, err = preparationRank(own)
		if err == nil && rank != b.rank {
			err = fsErr(own.name, "own preparation rank changed")
		}
		if err == nil {
			err = s.validate(own)
		}
		return
	}
	if !register {
		return
	}
	if free == nil && freeName != "" {
		free, err = s.open(freeName, true, b)
		if err != nil {
			return
		}
		// Under registry ownership a new free slot must be immediately available.
		err = withFD(free.f, admissionFlock)
		if err == nil {
			free.held = true
		} else {
			err = fsErr(free.name, "free slot lock failed: %v", err)
			return
		}
	}
	if free == nil || maximum == math.MaxUint64 {
		err = wire.Errorf(wire.CodeLimitExceeded, b.path, "preparation admission capacity or rank exhausted")
		return
	}
	return
}

func (s *preparationScope) withRegistry(registry *preparationFile, b *preparationBudget, fn func() error) (err error) {
	b.phase = "registry"
	if err = preparationTake(registry, b, admissionFlock); err != nil {
		return
	}
	defer func() {
		e := withFD(registry.f, preparationUnlock)
		// On unlock error retain held=true so terminal close attempts retirement.
		if e == nil {
			registry.held = false
		}
		err = withCleanup(err, e)
	}()
	if err = s.validate(registry); err != nil {
		return
	}
	admissionReached("registry", b.rank)
	return fn()
}

func (s *preparationScope) register(registry *preparationFile, b *preparationBudget) (slot *preparationFile, err error) {
	err = s.withRegistry(registry, b, func() error {
		var max uint64
		var e error
		slot, _, max, e = s.scan(registry, nil, b, true)
		if e != nil {
			return e
		}
		admissionReached("slot-owned", 0)
		var record [16]byte
		copy(record[:4], "CPA1")
		binary.BigEndian.PutUint64(record[8:], max+1)
		if e = s.validate(slot); e != nil {
			return e
		}
		if e = b.check(); e != nil {
			return e
		}
		n, e := admissionWrite(slot.f, record[:])
		if e == nil && n != len(record) {
			e = io.ErrShortWrite
		}
		if e != nil {
			return e
		}
		if e = slot.f.Truncate(16); e != nil {
			return e
		}
		got, e := preparationRank(slot)
		if e != nil {
			return e
		}
		if got != max+1 {
			return fsErr(slot.name, "registration readback differs")
		}
		if e = s.validate(slot); e != nil {
			return e
		}
		if e = s.validate(registry); e != nil {
			return e
		}
		if e = b.check(); e != nil {
			return e
		}
		b.rank = got
		return nil
	})
	if err != nil {
		return nil, withCleanup(err, slot.close())
	}
	// Published acknowledgement never executes under registry ownership.
	admissionReached("published", b.rank)
	return slot, nil
}

func (s *preparationScope) awaitHead(registry, slot *preparationFile, b *preparationBudget) error {
	for {
		var min uint64
		err := s.withRegistry(registry, b, func() error {
			var e error
			_, min, _, e = s.scan(registry, slot, b, false)
			return e
		})
		if err != nil {
			return err
		}
		b.phase = "queue"
		if err = b.check(); err != nil {
			return err
		}
		if min == b.rank {
			return nil
		}
		admissionReached("queued", b.rank)
		if err = b.pause(); err != nil {
			return err
		}
	}
}
