//go:build darwin || linux

package authority

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func admissionRestore(t *testing.T) {
	t.Helper()
	o, f, c, r, u, w, h := admissionOpen, admissionFlock, preparationCloseFile, preparationCloseRoot, preparationUnlock, admissionWrite, admissionReached
	g, gf := preparationOpen, flockNB
	t.Cleanup(func() {
		admissionOpen, admissionFlock, preparationCloseFile, preparationCloseRoot, preparationUnlock, admissionWrite, admissionReached = o, f, c, r, u, w, h
		preparationOpen, flockNB = g, gf
	})
}

func admissionBudget(repo *intent.Repository) *preparationBudget {
	return &preparationBudget{ctx: context.Background(), deadline: time.Now().Add(30 * time.Second), wait: 30 * time.Second, poll: time.Millisecond, path: filepath.Join(repo.CommonDir, preparationLockFileName), phase: "registry"}
}

func admissionScope(t *testing.T, repo *intent.Repository) (*preparationScope, *preparationFile, *preparationBudget) {
	t.Helper()
	b := admissionBudget(repo)
	s, e := openPreparationScope(repo, b)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.close(); e != nil {
			t.Error(e)
		}
	})
	r, e := s.open(preparationRegistryName, true, b)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := r.close(); e != nil {
			t.Error(e)
		}
	})
	return s, r, b
}

func admissionRecord(rank uint64) []byte {
	b := make([]byte, 16)
	copy(b, "CPA1")
	binary.BigEndian.PutUint64(b[8:], rank)
	return b
}

// This preflight runs before same-process slot filling; platform qualification
// does not retroactively stand in for independent-open ownership here.
func admissionSeparateOpen(t *testing.T, path string) {
	t.Helper()
	a, e := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := os.OpenFile(path, os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if e = withFD(a, flockExclusiveNB); e != nil {
		t.Fatal(e)
	}
	if e = withFD(b, flockExclusiveNB); !isWouldBlock(e) {
		t.Fatalf("independent open did not exclude: %v", e)
	}
	// Never unlock the unsuccessful probe.
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	if e = withFD(b, flockExclusiveNB); e != nil {
		t.Fatal(e)
	}
}

func TestGH494PreparationAdmission(t *testing.T) {
	t.Run("capacity64-and-reclaim", func(t *testing.T) {
		_, repo := preparationOpenFixture(t)
		admissionSeparateOpen(t, filepath.Join(repo.CommonDir, preparationSlotName(0)))
		s, r, b := admissionScope(t, repo)
		var slots []*preparationFile
		t.Cleanup(func() {
			for _, p := range slots {
				if e := p.close(); e != nil {
					t.Error(e)
				}
			}
		})
		for i := 0; i < preparationSlots; i++ {
			p, e := s.register(r, b)
			if e != nil {
				t.Fatal(i, e)
			}
			slots = append(slots, p)
			if b.rank != uint64(i+1) {
				t.Fatal(b.rank)
			}
		}
		p, e := s.register(r, b)
		if p != nil || wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("65th %v %v", p, e)
		}
		if e = slots[20].close(); e != nil {
			t.Fatal(e)
		}
		p, e = s.register(r, b)
		if e != nil {
			t.Fatal(e)
		}
		slots = append(slots, p)
		if b.rank != 65 || p.name != preparationSlotName(20) {
			t.Fatalf("reuse rank%d %s", b.rank, p.name)
		}
		names, e := os.ReadDir(repo.CommonDir)
		if e != nil {
			t.Fatal(e)
		}
		count := 0
		for _, n := range names {
			if strings.HasPrefix(n.Name(), "taskman.prepare.slot.") {
				count++
			}
		}
		if count != 64 {
			t.Fatal("unbounded namespace", count)
		}
	})
	t.Run("live-record-refusals-and-unowned-partial", func(t *testing.T) {
		for _, kind := range []string{"magic", "reserved", "zero", "short", "oversize", "overflow", "duplicate"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				s, r, b := admissionScope(t, repo)
				p, e := s.register(r, b)
				if e != nil {
					t.Fatal(e)
				}
				defer p.close()
				raw := admissionRecord(1)
				switch kind {
				case "magic":
					raw[0] = 'X'
				case "reserved":
					raw[4] = 1
				case "zero":
					raw = admissionRecord(0)
				case "short":
					raw = raw[:7]
				case "oversize":
					raw = append(raw, 0)
				case "overflow":
					raw = admissionRecord(math.MaxUint64)
				}
				if kind == "duplicate" {
					q, e := s.register(r, b)
					if e != nil {
						t.Fatal(e)
					}
					defer q.close()
					if _, e = q.f.WriteAt(raw, 0); e != nil {
						t.Fatal(e)
					}
				}
				if e = p.f.Truncate(0); e != nil {
					t.Fatal(e)
				}
				if _, e = p.f.WriteAt(raw, 0); e != nil {
					t.Fatal(e)
				}
				q, e := s.register(r, b)
				want := wire.CodeUnsupportedFilesystem
				if kind == "overflow" {
					want = wire.CodeLimitExceeded
				}
				if q != nil || wire.CodeOf(e) != want {
					t.Fatalf("live %s: %v %v", kind, q, e)
				}
				if e = p.close(); e != nil {
					t.Fatal(e)
				}
				if kind == "duplicate" {
					return
				}
				q, e = s.register(r, b)
				if e != nil {
					t.Fatal("unowned bytes retained authority", e)
				}
				defer q.close()
			})
		}
	})
	t.Run("fixed-name-and-unsafe-objects", func(t *testing.T) {
		for _, kind := range []string{"registry-symlink", "registry-directory", "slot-symlink", "slot-directory"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				name := preparationRegistryName
				if strings.HasPrefix(kind, "slot") {
					name = preparationSlotName(31)
				}
				p := filepath.Join(repo.CommonDir, name)
				var e error
				if strings.HasSuffix(kind, "symlink") {
					e = os.Symlink("HEAD", p)
				} else {
					e = os.Mkdir(p, 0700)
				}
				if e != nil {
					t.Fatal(e)
				}
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{})
				preparationOpenRefused(t, l, e, wire.CodeUnsupportedFilesystem)
			})
		}
		_, repo := preparationOpenFixture(t)
		s, _, b := admissionScope(t, repo)
		if p, e := s.open("taskman.prepare.slot.64", true, b); p != nil || wire.CodeOf(e) != wire.CodeUnsupportedFilesystem {
			t.Fatal(p, e)
		}
	})
	t.Run("slot-and-registry-identity-boundaries", func(t *testing.T) {
		for _, kind := range []string{"slot-before-publication", "slot-after-publication", "registry-after-publication", "slot-after-gate"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				admissionRestore(t)
				reached := false
				replace := func() {
					reached = true
					name := preparationSlotName(0)
					if kind == "registry-after-publication" {
						name = preparationRegistryName
					}
					path := filepath.Join(repo.CommonDir, name)
					if e := os.Rename(path, path+".old"); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(path, nil, 0600); e != nil {
						t.Fatal(e)
					}
				}
				admissionReached = func(phase string, _ uint64) {
					if !reached && ((kind == "slot-before-publication" && phase == "slot-owned") || ((kind == "slot-after-publication" || kind == "registry-after-publication") && phase == "published")) {
						replace()
					}
				}
				old := flockNB
				flockNB = func(fd int) error {
					e := old(fd)
					if e == nil && kind == "slot-after-gate" {
						replace()
					}
					return e
				}
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{})
				preparationOpenRefused(t, l, e, wire.CodeUnsupportedFilesystem)
				if !reached {
					t.Fatal("identity injection not reached")
				}
			})
		}
	})
	t.Run("total-budget-and-phase-cancellation", func(t *testing.T) {
		for _, phase := range []string{"registry", "queue", "gate"} {
			t.Run(phase, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				admissionRestore(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				reached := false
				if phase == "registry" {
					old := admissionFlock
					admissionFlock = func(fd int) error { reached = true; cancel(); return old(fd) }
				}
				if phase == "queue" {
					admissionReached = func(p string, _ uint64) {
						if p == "published" {
							reached = true
							cancel()
						}
					}
				}
				if phase == "gate" {
					old := flockNB
					flockNB = func(fd int) error { reached = true; cancel(); return old(fd) }
				}
				l, e := AcquirePreparation(ctx, repo, LockOptions{})
				if l != nil || !errors.Is(e, context.Canceled) || !reached {
					t.Fatalf("cancel %s %v %v reached%t", phase, l, e, reached)
				}
			})
		}
		_, repo := preparationOpenFixture(t)
		admissionRestore(t)
		// Two separately reached delays consume one original budget. A new timer
		// at publication or final-gate acquisition would incorrectly succeed.
		start := time.Now()
		published, gate := false, false
		admissionReached = func(p string, _ uint64) {
			if p == "published" {
				published = true
				time.Sleep(60 * time.Millisecond)
			}
		}
		old := preparationOpen
		preparationOpen = func(r *os.Root, n string, f int, m os.FileMode, d bool) (*os.File, error) {
			gate = true
			time.Sleep(60 * time.Millisecond)
			return old(r, n, f, m, d)
		}
		l, e := AcquirePreparation(context.Background(), repo, LockOptions{Wait: 100 * time.Millisecond, Poll: time.Millisecond})
		if l != nil || wire.CodeOf(e) != wire.CodeLockTimeout || !published || !gate || !strings.Contains(e.Error(), "phase=gate") || time.Since(start) > time.Second {
			t.Fatalf("one budget %v %v %t %t", l, e, published, gate)
		}
		if EffectiveWait(0) != 30*time.Second || EffectiveWait(time.Hour) != 30*time.Second {
			t.Fatal("default/max changed")
		}
	})
	t.Run("locally-owned-cleanup-errors", func(t *testing.T) {
		for _, kind := range []string{"registry", "probe", "gate-root", "admission-root", "primary-and-slot"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				admissionRestore(t)
				injected := errors.New("reached preparation cleanup failure")
				primary := errors.New("reached primary write failure")
				reached := false
				var actual *os.File
				fd := -1
				if kind == "probe" {
					for _, i := range []int{1, 2} {
						if e := os.WriteFile(filepath.Join(repo.CommonDir, preparationSlotName(i)), nil, 0600); e != nil {
							t.Fatal(e)
						}
					}
				}
				old := preparationCloseFile
				preparationCloseFile = func(f *os.File) error {
					name := filepath.Base(f.Name())
					hit := (kind == "registry" && name == preparationRegistryName) || (kind == "probe" && name == preparationSlotName(2)) || (kind == "primary-and-slot" && strings.HasPrefix(name, "taskman.prepare.slot."))
					if hit && !reached {
						reached = true
						actual = f
						fd = int(f.Fd())
						return errors.Join(old(f), injected)
					}
					return old(f)
				}
				roots := 0
				closeRoot := preparationCloseRoot
				preparationCloseRoot = func(r *os.Root) error {
					roots++
					e := closeRoot(r)
					if (kind == "gate-root" && roots == 1) || (kind == "admission-root" && roots == 2) {
						reached = true
						return errors.Join(e, injected)
					}
					return e
				}
				if kind == "primary-and-slot" {
					admissionWrite = func(*os.File, []byte) (int, error) { return 0, primary }
				}
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{})
				if l != nil || !errors.Is(e, injected) || !reached {
					t.Fatalf("cleanup%s %v %v reached%t", kind, l, e, reached)
				}
				if kind == "primary-and-slot" && !errors.Is(e, primary) {
					t.Fatal("lost primary", e)
				}
				if actual != nil {
					preparationOpenClosed(t, actual, fd)
				}
				// Disable only the reached injection, then prove no retained slot/gate.
				preparationCloseFile = old
				preparationCloseRoot = closeRoot
				admissionWrite = func(f *os.File, b []byte) (int, error) { return f.WriteAt(b, 0) }
				next, e := AcquirePreparation(context.Background(), repo, LockOptions{Wait: time.Second})
				if e != nil {
					t.Fatal("retained ownership", e)
				}
				if e = next.Close(); e != nil {
					t.Fatal(e)
				}
			})
		}
	})
	t.Run("unsupported-probe-and-short-write", func(t *testing.T) {
		for _, kind := range []string{"flock", "write"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := preparationOpenFixture(t)
				admissionRestore(t)
				reached := false
				if kind == "flock" {
					admissionFlock = func(int) error { reached = true; return syscall.ENOLCK }
				} else {
					admissionWrite = func(f *os.File, b []byte) (int, error) { reached = true; return f.WriteAt(b[:7], 0) }
				}
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{})
				if l != nil || e == nil || !reached {
					t.Fatal(l, e, reached)
				}
			})
		}
	})
}

func TestGH494PreparationCompositeClose(t *testing.T) {
	for _, kind := range []string{"success", "gate-error", "slot-error", "both-errors"} {
		t.Run(kind, func(t *testing.T) {
			_, repo := preparationOpenFixture(t)
			admissionRestore(t)
			var l *PreparationLock
			var observed atomic.Int32
			var observation PreparationObservation
			ctx := WithPreparationObserver(context.Background(), func(o PreparationObservation) {
				observation = o
				observed.Add(1)
				if e := l.Close(); !errors.Is(e, o.Err) {
					t.Error("reentrant close changed result", e, o.Err)
				}
			})
			var e error
			l, e = AcquirePreparation(ctx, repo, LockOptions{})
			if e != nil {
				t.Fatal(e)
			}
			gateFD, slotFD := int(l.lock.f.Fd()), int(l.slot.f.Fd())
			slotFile := l.slot.f
			gateError, slotError := errors.New("gate close injected"), errors.New("slot close injected")
			var order []string
			var gateEnd time.Time
			old := preparationCloseFile
			preparationCloseFile = func(f *os.File) error {
				which := "slot"
				if f == l.lock.f {
					which = "gate"
				}
				order = append(order, which)
				e := old(f)
				if which == "gate" {
					gateEnd = time.Now()
				}
				if which == "gate" && (kind == "gate-error" || kind == "both-errors") {
					e = errors.Join(e, gateError)
				}
				if which == "slot" && (kind == "slot-error" || kind == "both-errors") {
					time.Sleep(20 * time.Millisecond)
					e = errors.Join(e, slotError)
				}
				return e
			}
			var wg sync.WaitGroup
			results := make(chan error, 8)
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); results <- l.Close() }()
			}
			wg.Wait()
			close(results)
			for e := range results {
				if (kind == "gate-error" || kind == "both-errors") != errors.Is(e, gateError) || (kind == "slot-error" || kind == "both-errors") != errors.Is(e, slotError) {
					t.Fatal(kind, e)
				}
			}
			if observed.Load() != 1 || strings.Join(order, ",") != "gate,slot" || !observation.Acquired || observation.Released != (kind == "success") || observation.Hold <= 0 {
				t.Fatalf("close %+v order%v count%d", observation, order, observed.Load())
			}
			if observation.Hold > gateEnd.Sub(l.lock.acquired) {
				t.Fatal("hold includes later slot retirement", observation.Hold, gateEnd.Sub(l.lock.acquired))
			}
			preparationOpenClosed(t, l.lock.f, gateFD)
			preparationOpenClosed(t, slotFile, slotFD)
		})
	}
}
