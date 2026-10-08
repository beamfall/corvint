//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// fakeControl is a LaunchControl whose admissions and boundaries a test
// scripts and records.
type fakeControl struct {
	mu       sync.Mutex
	admit    bool
	gate     chan struct{} // when set, Admit blocks until it is closed
	waiting  chan struct{} // closed once Admit is blocked on gate
	intents  []string
	released []bool
	onAdmit  func()
	bounds   [][2]bool
	stop     bool
}

func (f *fakeControl) Admit(intent string) (func(bool), error) {
	if f.gate != nil {
		close(f.waiting)
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.intents = append(f.intents, intent)
	if !f.admit {
		return nil, errors.New("refused")
	}
	if f.onAdmit != nil {
		f.onAdmit()
	}
	return func(recorded bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.released = append(f.released, recorded)
	}, nil
}

func (f *fakeControl) Boundary(recorded, settled bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bounds = append(f.bounds, [2]bool{recorded, settled})
	return f.stop && settled
}

// SERVICE500-003: a controlled dispatcher takes its fence before each new
// worker launch and releases it only after the launched worker is in the
// saved ledger; a refusing fence launches nothing.
func TestSERVICE500_OpenControlledFencesLaunches(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	if _, err := OpenControlled("prog", c, q, io.Discard, nil); err == nil {
		t.Fatal("a controlled dispatcher opened without a launch control")
	}
	if OwnerState(ProgramDir(c, "prog")) != "NOT_RUNNING" {
		t.Fatal("a refused OpenControlled took dispatcher ownership")
	}
	f := &fakeControl{}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if st, pid := OwnerProcess(d.dir); st != "RUNNING" || pid == 0 {
		t.Fatalf("owner %s pid %d", st, pid)
	}
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 0 || len(f.intents) != 1 {
		t.Fatalf("refused fence: %v running %d intents %v", err, d.Running(), f.intents)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Settled(l) || Records(l, f.intents[0]) {
		t.Fatalf("refused launch left a record: %v", err)
	}
	f.admit = true
	if err := d.Tick(ctx); err != nil || d.Running() != 2 || len(f.intents) != 3 {
		t.Fatalf("admitting fence: %v running %d intents %v", err, d.Running(), f.intents)
	}
	if !reflect.DeepEqual(f.released, []bool{true, true}) {
		t.Fatalf("releases %v", f.released)
	}
	l, err := LoadLedger(d.dir, "prog")
	if err != nil || Settled(l) || !Records(l, f.intents[1]) || !Records(l, f.intents[2]) {
		t.Fatalf("admitted launches not recorded under their intents: %v %v", err, f.intents)
	}
	waitEnded(t, d)
}

// SERVICE500-003: a worker whose ledger save failed after spawn releases
// its admission unrecorded, and the tick boundary reports it unrecorded and
// unsettled until a later save succeeds.
func TestSERVICE500_UnsavedLaunchStaysUnrecorded(t *testing.T) {
	c := testConfig(t, "/bin/sleep 60")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &fakeControl{admit: true}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	// A directory at state.json makes every ledger rename fail.
	state := filepath.Join(d.dir, "state.json")
	f.onAdmit = func() {
		_ = os.Remove(state)
		if err := os.MkdirAll(filepath.Join(state, "block"), 0o700); err != nil {
			t.Error(err)
		}
	}
	// CAL-V0-192: the run reports the failed final save.
	if err := d.Run(context.Background(), 1); !errors.Is(err, ErrLedgerUnsaved) {
		t.Fatalf("unsaved launch run: %v", err)
	}
	f.mu.Lock()
	if !reflect.DeepEqual(f.released, []bool{false}) || !reflect.DeepEqual(f.bounds, [][2]bool{{false, false}}) {
		f.mu.Unlock()
		t.Fatalf("unsaved launch: releases %v bounds %v", f.released, f.bounds)
	}
	f.onAdmit, f.admit = nil, false
	f.mu.Unlock()
	if err := os.RemoveAll(state); err != nil {
		t.Fatal(err)
	}
	if err := d.Run(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if got := f.bounds[len(f.bounds)-1]; got != [2]bool{true, false} {
		t.Fatalf("saved live worker boundary %v", got)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Records(l, f.intents[0]) {
		t.Fatalf("later save did not record the unsaved worker: %v", err)
	}
	for _, w := range d.ledger.Workers {
		_, _ = killTree(w, time.Second, nil)
	}
}

// SERVICE500-003: a cancel that arrives while the fence is awaited releases
// the admission and starts nothing.
func TestSERVICE500_CancelDuringFenceWaitLaunchesNothing(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &fakeControl{admit: true, gate: make(chan struct{}), waiting: make(chan struct{})}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Tick(ctx) }()
	<-f.waiting
	cancel()
	close(f.gate)
	<-done
	if d.Running() != 0 || !reflect.DeepEqual(f.released, []bool{true}) {
		t.Fatalf("cancelled admission: running %d releases %v", d.Running(), f.released)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Settled(l) {
		t.Fatalf("cancelled admission left a record: %v", err)
	}
}

// SERVICE500-003: a settled boundary the control accepts ends Run with
// ErrSettled; a boundary is never settled while a worker is recorded.
func TestSERVICE500_BoundarySettlesRun(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &fakeControl{admit: true, stop: true}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Tick(context.Background()); err != nil || d.Running() != 1 {
		t.Fatalf("launch: %v running %d", err, d.Running())
	}
	f.admit = false
	waitEnded(t, d)
	err = d.Run(context.Background(), 20)
	if !errors.Is(err, ErrSettled) || d.Running() != 0 {
		t.Fatalf("run %v running %d bounds %v", err, d.Running(), f.bounds)
	}
	for i, b := range f.bounds[:len(f.bounds)-1] {
		if b[1] {
			t.Fatalf("boundary %d settled before the last", i)
		}
	}
	if got := f.bounds[len(f.bounds)-1]; got != [2]bool{true, true} {
		t.Fatalf("final boundary %v", got)
	}
}

// SERVICE500-003: a new pool sweep starts only under the fence, and its
// durable STARTING record keeps a drain unsettled.
func TestSERVICE500_OpenControlledFencesPoolSweepStart(t *testing.T) {
	release := make(chan struct{})
	d, q := psrDispatch(t, func(ctx context.Context, _ PoolSweepRequest) (PoolSweepResult, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return PoolSweepResult{}, ctx.Err()
	})
	f := &fakeControl{}
	d.control = f
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.count.Load() != 0 || len(d.ledger.PoolSweeps) != 0 {
		t.Fatal("pool sweep started through a refusing fence")
	}
	f.admit = true
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := psrCall(t, q)
	if !reflect.DeepEqual(f.released, []bool{true}) || f.intents[len(f.intents)-1] != r.RequestID {
		t.Fatalf("releases %v intents %v", f.released, f.intents)
	}
	if l, err := LoadLedger(d.dir, d.Program); err != nil || Settled(l) || !Records(l, r.RequestID) {
		t.Fatalf("a STARTING pool sweep reported settled: %v", err)
	}
	close(release)
}

// SERVICE500-003: a retained STARTING record has no proven native
// admission, so a controlled dispatcher replays it only under the fence.
func TestSERVICE500_RetainedStartingSweepNeedsFence(t *testing.T) {
	d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
		return PoolSweepResult{}, nil
	})
	m := d.Queue.(*psrQueue).obs.Members[0]
	now := time.Now().UTC()
	r := PoolSweepRecord{PoolSweepRequest: PoolSweepRequest{WorkRoot: d.Config.WorkRoot, Program: d.Program, Queue: m.Queue, Pool: m.Pool, Member: m.Member, Allocation: m.Allocation, Definition: m.Definition, RequestID: sweepID(d.Program, m.Queue, m.Allocation), Actor: "tester", ActorRole: "OPERATOR", ConfigDigest: strings.Repeat("c", 64), TimeoutSeconds: 15}, Phase: "STARTING", Started: now, Observed: now}
	if e := d.commitPoolSweep(r); e != nil {
		t.Fatal(e)
	}
	if e := d.Close(); e != nil {
		t.Fatal(e)
	}
	f := &fakeControl{}
	next, e := OpenControlled("prog", d.Config, q, io.Discard, f)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	for i := 0; i < 2; i++ {
		if e := next.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if q.count.Load() != 0 || len(f.intents) == 0 || f.intents[0] != r.RequestID {
		t.Fatalf("retained STARTING replayed through a refusing fence: calls %d intents %v", q.count.Load(), f.intents)
	}
	if l, err := LoadLedger(next.dir, next.Program); err != nil || Settled(l) {
		t.Fatalf("refused retained sweep reported settled: %v", err)
	}
	f.admit = true
	if e := next.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	if got := psrCall(t, q); got != r.PoolSweepRequest || !reflect.DeepEqual(f.released, []bool{true}) {
		t.Fatalf("admitted replay %v releases %v", got, f.released)
	}
	psrCollect(t, next)
}

// SERVICE500-003: a cancel after the progress save removed an ended,
// progress-granted worker but before its accounting (finish) reaches no
// tick boundary, so the control never settles a drain from that partial
// tick even though the saved ledger already records no worker.
func TestSERVICE500_CancelBeforeAccountingReachesNoBoundary(t *testing.T) {
	d, _, source := progressDispatcher(t)
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	key := "ticket:a:q:t"
	base := baseFingerprint(progressObservation(""), key)
	d.ledger.Workers = []*Worker{{ID: "ended", Key: key, Ticket: key, BaseFingerprint: base,
		ProgressDigest: progressDigest("A"), Fingerprint: progressFingerprint(base, progressDigest("A")), Started: time.Now()}}
	if err := d.ledger.save(d.dir); err != nil {
		t.Fatal(err)
	}
	f := &fakeControl{stop: true}
	d.control = f
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	saves := 0
	d.progressSave = func(l *Ledger, dir string) error {
		saves++
		err := l.save(dir)
		cancel()
		return err
	}
	writeProgress(t, source, "B")
	if err := d.Run(ctx, 5); err != nil {
		t.Fatalf("cancelled run: %v", err)
	}
	if saves != 1 || len(f.bounds) != 0 {
		t.Fatalf("progress saves %d boundaries %v", saves, f.bounds)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Settled(l) {
		t.Fatalf("the progress save did not publish the worker's removal: %v", err)
	}
	// The next uncancelled tick completes accounting and only then settles.
	d.progressSave = nil
	if err := d.Run(context.Background(), 1); !errors.Is(err, ErrSettled) || len(f.bounds) != 1 {
		t.Fatalf("completed tick: %v boundaries %v", err, f.bounds)
	}
}

// SERVICE500-003: a ledger save counts as durable only after its directory
// sync, so a rename that succeeded but whose sync failed keeps the launch
// intent unrecorded and the boundary unrecorded until a later synced save.
func TestSERVICE500_UnsyncedLedgerRenameStaysUnrecorded(t *testing.T) {
	c := testConfig(t, "/bin/sleep 60")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &fakeControl{admit: true}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Cleanup(func() { syncDir = syncDirectory })
	f.onAdmit = func() { syncDir = func(string) error { return errors.New("injected directory sync failure") } }
	// CAL-V0-192: the run reports the unsynced final save with its cause.
	if err := d.Run(context.Background(), 1); !errors.Is(err, ErrLedgerUnsaved) || !strings.Contains(err.Error(), "injected directory sync failure") || !strings.Contains(err.Error(), "not confirmed durable") {
		t.Fatalf("unsynced save run: %v", err)
	}
	f.mu.Lock()
	if !reflect.DeepEqual(f.released, []bool{false}) || !reflect.DeepEqual(f.bounds, [][2]bool{{false, false}}) {
		f.mu.Unlock()
		t.Fatalf("unsynced save: releases %v bounds %v", f.released, f.bounds)
	}
	f.onAdmit, f.admit = nil, false
	f.mu.Unlock()
	// The rename itself landed: only its durability is unproven.
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Records(l, f.intents[0]) {
		t.Fatalf("renamed ledger does not hold the worker: %v", err)
	}
	syncDir = syncDirectory
	if err := d.Run(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if got := f.bounds[len(f.bounds)-1]; got != [2]bool{true, false} {
		t.Fatalf("synced live worker boundary %v", got)
	}
	for _, w := range d.ledger.Workers {
		_, _ = killTree(w, time.Second, nil)
	}
}

// SERVICE500-003: a launch that started but whose identity could not be
// read is uncertain, not failed: the dispatcher keeps its pid and wait
// channel, releases the admission unrecorded, and never again reports a
// recorded or settled boundary even though its later saves succeed.
func TestSERVICE500_PostStartIdentityFailureStaysUnresolved(t *testing.T) {
	c := testConfig(t, "/bin/sleep 60")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &fakeControl{admit: true, stop: true}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Cleanup(func() { processIdentity = supervisor.ProcessIdentity })
	f.onAdmit = func() {
		processIdentity = func(int) (string, error) { return "", errors.New("injected identity failure") }
	}
	if err := d.Run(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	processIdentity = supervisor.ProcessIdentity
	f.mu.Lock()
	if d.Running() != 0 || !reflect.DeepEqual(f.released, []bool{false}) || !reflect.DeepEqual(f.bounds, [][2]bool{{false, false}}) {
		f.mu.Unlock()
		t.Fatalf("post-start failure: running %d releases %v bounds %v", d.Running(), f.released, f.bounds)
	}
	f.onAdmit, f.admit = nil, false
	f.mu.Unlock()
	if len(d.uncertain) != 1 || d.uncertain[0].pid <= 0 || d.uncertain[0].exit == nil {
		t.Fatalf("started launch not retained: %+v", d.uncertain)
	}
	select {
	case <-d.uncertain[0].exit:
	case <-time.After(10 * time.Second):
		t.Fatal("the started leader was not killed")
	}
	if err := d.Run(context.Background(), 2); err != nil {
		t.Fatalf("an uncertain launch settled the run: %v", err)
	}
	l, err := LoadLedger(d.dir, "prog")
	if err != nil || !Settled(l) {
		t.Fatalf("later saves failed: %v", err)
	}
	for i, b := range f.bounds {
		if b != [2]bool{false, false} {
			t.Fatalf("boundary %d %v cleared an uncertain launch", i, b)
		}
	}
}
