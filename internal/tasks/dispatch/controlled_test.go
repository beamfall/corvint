//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
)

// SERVICE500-003: a controlled dispatcher takes its fence before each new
// worker launch and releases it only after the launched worker is in the
// saved ledger; a refusing fence launches nothing.
func TestSERVICE500_OpenControlledFencesLaunches(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	if _, err := OpenControlled("prog", c, q, io.Discard, nil); err == nil {
		t.Fatal("a controlled dispatcher opened without a fence")
	}
	if OwnerState(ProgramDir(c, "prog")) != "NOT_RUNNING" {
		t.Fatal("a refused OpenControlled took dispatcher ownership")
	}
	admit, taken := false, 0
	var recorded []int
	fence := func() (func(), error) {
		taken++
		if !admit {
			return nil, errors.New("refused")
		}
		return func() {
			l, err := LoadLedger(ProgramDir(c, "prog"), "prog")
			if err != nil {
				t.Error(err)
				return
			}
			recorded = append(recorded, len(l.Workers))
		}, nil
	}
	d, err := OpenControlled("prog", c, q, io.Discard, fence)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if st, pid := OwnerProcess(d.dir); st != "RUNNING" || pid == 0 {
		t.Fatalf("owner %s pid %d", st, pid)
	}
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 0 || taken != 1 {
		t.Fatalf("refused fence: %v running %d taken %d", err, d.Running(), taken)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || !Settled(l) {
		t.Fatalf("refused launch left a record: %v", err)
	}
	admit = true
	if err := d.Tick(ctx); err != nil || d.Running() != 2 || taken != 3 {
		t.Fatalf("admitting fence: %v running %d taken %d", err, d.Running(), taken)
	}
	if !reflect.DeepEqual(recorded, []int{1, 2}) {
		t.Fatalf("fence released before the worker was saved: %v", recorded)
	}
	if l, err := LoadLedger(d.dir, "prog"); err != nil || Settled(l) {
		t.Fatalf("recorded workers reported settled: %v", err)
	}
	waitEnded(t, d)
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
	admit, released := false, 0
	d.fence = func() (func(), error) {
		if !admit {
			return nil, errors.New("refused")
		}
		return func() { released++ }, nil
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.count.Load() != 0 || len(d.ledger.PoolSweeps) != 0 {
		t.Fatal("pool sweep started through a refusing fence")
	}
	admit = true
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	psrCall(t, q)
	if released != 1 {
		t.Fatalf("fence released %d times", released)
	}
	if l, err := LoadLedger(d.dir, d.Program); err != nil || Settled(l) {
		t.Fatalf("a STARTING pool sweep reported settled: %v", err)
	}
	close(release)
}
