//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type psrQueue struct {
	fakeQueue
	calls  chan PoolSweepRequest
	invoke func(context.Context, PoolSweepRequest) (PoolSweepResult, error)
	count  atomic.Int32
}

func (q *psrQueue) PoolSweepActor() (string, string, error) { return "tester", "OPERATOR", nil }
func (q *psrQueue) PoolSweep(ctx context.Context, r PoolSweepRequest) (PoolSweepResult, error) {
	q.count.Add(1)
	q.calls <- r
	return q.invoke(ctx, r)
}
func psrDispatch(t *testing.T, invoke func(context.Context, PoolSweepRequest) (PoolSweepResult, error)) (*Dispatcher, *psrQueue) {
	t.Helper()
	c := testConfig(t, "/bin/sleep 60")
	c.PoolSweep = &PoolSweepConfig{TimeoutSeconds: 15, IntervalSeconds: 1}
	q := &psrQueue{calls: make(chan PoolSweepRequest, 16), invoke: invoke}
	q.obs.Members = []Member{{Pool: "db", Member: "a", State: "QUARANTINED", Queue: "queue:a:q", Allocation: strings.Repeat("a", 64), Definition: strings.Repeat("b", 64), SafeReuse: true}}
	d, e := Open("prog", c, q, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, q
}
func psrCall(t *testing.T, q *psrQueue) PoolSweepRequest {
	t.Helper()
	select {
	case r := <-q.calls:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("native call not reached")
		return PoolSweepRequest{}
	}
}
func psrCollect(t *testing.T, d *Dispatcher) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for d.sweepJob != nil && time.Now().Before(end) {
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if d.sweepJob != nil {
		t.Fatal("operation not joined/recorded")
	}
}
func psrRetireWorkers(t *testing.T, d *Dispatcher) {
	t.Helper()
	for _, w := range d.ledger.Workers {
		w.State = "KILLING"
		w.KillReason = "WALL"
	}
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		d.supervise()
		alive := false
		for _, w := range d.ledger.Workers {
			alive = alive || len(w.Members) > 0
		}
		if !alive {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("owned worker descendants not retired")
}
func TestPSRDispatchConfigAndLedger(t *testing.T) {
	c := testConfig(t, "exit 0")
	legacy, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(legacy, []byte("poolSweep")) {
		t.Fatal("legacy config changed")
	}
	c.PoolSweep = &PoolSweepConfig{TimeoutSeconds: 1800, IntervalSeconds: 3600}
	raw, _ := json.Marshal(c)
	if _, e := DecodeConfig(raw); e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{
		bytes.Replace(raw, []byte(`"poolSweep"`), []byte(`"POOLSWEEP"`), 1),
		bytes.Replace(raw, []byte(`"timeoutSeconds":1800`), []byte(`"TimeoutSeconds":1800`), 1),
		bytes.Replace(raw, []byte(`"timeoutSeconds":1800`), []byte(`"timeoutSeconds":1800,"timeoutSeconds":1`), 1),
		bytes.Replace(raw, []byte(`"timeoutSeconds":1800`), []byte(`"timeoutSeconds":null`), 1),
		bytes.Replace(raw, []byte(`"timeoutSeconds":1800`), []byte(`"timeoutSeconds":1801`), 1),
		bytes.Replace(raw, []byte(`"intervalSeconds":3600`), []byte(`"intervalSeconds":0`), 1),
		bytes.Replace(raw, []byte(`{"timeoutSeconds":1800,"intervalSeconds":3600}`), []byte(`null`), 1),
	} {
		if bytes.Equal(raw, bad) {
			t.Fatal("malformed config fixture not reached")
		}
		if _, e := DecodeConfig(bad); e == nil {
			t.Fatal("malformed config admitted", string(bad))
		}
	}
	d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
		return PoolSweepResult{Outcome: "COMPLETED"}, nil
	})
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	psrCall(t, q)
	psrCollect(t, d)
	saved, e := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := LoadLedger(d.dir, d.Program); e != nil {
		t.Fatal("valid ledger", e)
	}
	compact, _ := json.Marshal(d.ledger)
	for _, bad := range [][]byte{
		bytes.Replace(compact, []byte(`,"result":{"pending":false,"outcome":"COMPLETED"}`), nil, 1),
		bytes.Replace(saved, []byte(`"pending": false,`), nil, 1),
		bytes.Replace(saved, []byte(`"poolSweeps"`), []byte(`"PoolSweeps"`), 1),
		bytes.Replace(saved, []byte(`"pending": false`), []byte(`"pending": null`), 1),
		bytes.Replace(saved, []byte(`"phase": "TERMINAL"`), []byte(`"phase": "UNKNOWN","phase":"TERMINAL"`), 1),
	} {
		if bytes.Equal(saved, bad) || bytes.Equal(compact, bad) {
			t.Fatal("ledger mutation not reached")
		}
		if e := os.WriteFile(filepath.Join(d.dir, "state.json"), bad, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := LoadLedger(d.dir, d.Program); e == nil {
			t.Fatal("malformed ledger admitted")
		}
	}
	if e := os.WriteFile(filepath.Join(d.dir, "state.json"), saved, 0600); e != nil {
		t.Fatal(e)
	}
	// Whole candidate bytes include unrelated legacy state, not just sweep rows.
	r := *d.ledger.PoolSweeps[sweepKey(q.obs.Members[0].Queue, "db", "a")]
	d.ledger.Backoff["unrelated"] = &BackoffState{Fingerprint: strings.Repeat("x", maxLedger)}
	calls := q.count.Load()
	if e := d.commitPoolSweep(r); e == nil {
		t.Fatal("full ledger overflow admitted")
	}
	after, _ := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if !bytes.Equal(saved, after) || q.count.Load() != calls {
		t.Fatal("overflow published or invoked")
	}
	delete(d.ledger.Backoff, "unrelated")
}

// PSR-V0-009/010 (H1 retry): STARTING is durable before invocation; restart and
// interval retries reuse the identical original request, and a Pending original
// reaches TERMINAL only by replaying that request.
func TestPSRDispatchStartAndReplay(t *testing.T) {
	t.Run("save-before-invoke", func(t *testing.T) {
		d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) { return PoolSweepResult{}, nil })
		d.poolSweepSave = func(*Ledger, string) error { return errors.New("disk failure") }
		if e := d.Tick(context.Background()); e == nil {
			t.Fatal("save failure hidden")
		}
		if q.count.Load() != 0 || len(d.ledger.PoolSweeps) != 0 {
			t.Fatal("invoked before durable STARTING")
		}
	})
	t.Run("joined-publication-failure-restart", func(t *testing.T) {
		var commands atomic.Int32
		d, q := psrDispatch(t, func(_ context.Context, r PoolSweepRequest) (PoolSweepResult, error) {
			commands.CompareAndSwap(0, 1)
			return PoolSweepResult{Outcome: "COMPLETED", ReceiptSeq: "9", Evidence: strings.Repeat("c", 64)}, nil
		})
		d.poolSweepSave = func(l *Ledger, dir string) error {
			for _, r := range l.PoolSweeps {
				if r.Phase != "STARTING" {
					return errors.New("terminal disk failure")
				}
			}
			return l.save(dir)
		}
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		first := psrCall(t, q)
		end := time.Now().Add(2 * time.Second)
		failed := false
		for time.Now().Before(end) {
			if e := d.Tick(context.Background()); e != nil {
				failed = strings.Contains(e.Error(), "publication UNKNOWN")
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !failed || d.sweepJob == nil || d.sweepJob.returned == nil {
			t.Fatal("joined return-save failure not retained")
		}
		if e := d.Close(); e == nil || !strings.Contains(e.Error(), "publication UNKNOWN") {
			t.Fatal("Close claimed durable terminal", e)
		}
		l, e := LoadLedger(d.dir, d.Program)
		if e != nil {
			t.Fatal(e)
		}
		for _, r := range l.PoolSweeps {
			if r.Phase != "STARTING" {
				t.Fatal("lost last successful record")
			}
		}
		reopened, e := Open("prog", d.Config, q, io.Discard)
		if e != nil {
			t.Fatal(e)
		}
		defer reopened.Close()
		reopened.Config.PoolSweep = &PoolSweepConfig{TimeoutSeconds: 99, IntervalSeconds: 1}
		if e := reopened.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		again := psrCall(t, q)
		if first != again || commands.Load() != 1 {
			t.Fatal("restart changed original request", first, again)
		}
		psrCollect(t, reopened)
	})
	t.Run("saved-before-call-definition-drift", func(t *testing.T) {
		var current atomic.Value
		current.Store(strings.Repeat("b", 64))
		var commands atomic.Int32
		d, q := psrDispatch(t, func(_ context.Context, r PoolSweepRequest) (PoolSweepResult, error) {
			if r.Definition != current.Load().(string) {
				return PoolSweepResult{}, errors.New("FENCED")
			}
			commands.Add(1)
			return PoolSweepResult{}, nil
		})
		reached := false
		d.poolSweepSave = func(l *Ledger, dir string) error {
			if e := l.save(dir); e != nil {
				return e
			}
			if !reached {
				reached = true
				current.Store(strings.Repeat("c", 64))
			}
			return nil
		}
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		psrCall(t, q)
		psrCollect(t, d)
		if !reached || commands.Load() != 0 {
			t.Fatal("first native admission ignored recorded definition")
		}
		for _, r := range d.ledger.PoolSweeps {
			if r.Phase != "UNKNOWN" {
				t.Fatal(r)
			}
		}
	})
	t.Run("before-native-crash-reuses-starting", func(t *testing.T) {
		d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
			return PoolSweepResult{Pending: true}, nil
		})
		m := q.obs.Members[0]
		now := time.Now().UTC()
		r := PoolSweepRecord{PoolSweepRequest: PoolSweepRequest{WorkRoot: d.Config.WorkRoot, Program: d.Program, Queue: m.Queue, Pool: m.Pool, Member: m.Member, Allocation: m.Allocation, Definition: m.Definition, RequestID: sweepID(d.Program, m.Queue, m.Allocation), Actor: "tester", ActorRole: "OPERATOR", ConfigDigest: strings.Repeat("c", 64), TimeoutSeconds: 15}, Phase: "STARTING", Started: now, Observed: now}
		if e := d.commitPoolSweep(r); e != nil {
			t.Fatal(e)
		}
		if e := d.Close(); e != nil {
			t.Fatal(e)
		}
		next, e := Open("prog", d.Config, q, io.Discard)
		if e != nil {
			t.Fatal(e)
		}
		defer next.Close()
		if e := next.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		got := psrCall(t, q)
		if got != r.PoolSweepRequest {
			t.Fatal("before-call identity changed")
		}
		psrCollect(t, next)
		next.sweepNext = time.Now().Add(time.Hour)
		for i := 0; i < 3; i++ {
			if e := next.Tick(context.Background()); e != nil {
				t.Fatal(e)
			}
		}
		if q.count.Load() != 1 {
			t.Fatal("pending request spun within its interval")
		}
		// After the interval the same identity is reconciled again (replay never
		// re-executes), so explicit native recovery can end it without a restart.
		next.sweepNext = time.Now().Add(-time.Second)
		if e := next.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		if again := psrCall(t, q); again != r.PoolSweepRequest || q.count.Load() != 2 {
			t.Fatal("pending identity not retried after interval", again)
		}
		psrCollect(t, next)
		if got := next.ledger.PoolSweeps[sweepKey(m.Queue, m.Pool, m.Member)]; got == nil || got.Phase != "PENDING" || got.RequestID != r.RequestID {
			t.Fatal("pending record changed", got)
		}
	})
	t.Run("retry-reaches-terminal", func(t *testing.T) {
		var pending atomic.Bool
		pending.Store(true)
		d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
			return PoolSweepResult{Pending: pending.Load()}, nil
		})
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		first := psrCall(t, q)
		psrCollect(t, d)
		pending.Store(false)
		d.sweepNext = time.Now().Add(-time.Second)
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		if again := psrCall(t, q); again != first {
			t.Fatal("retry changed identity")
		}
		psrCollect(t, d)
		m := q.obs.Members[0]
		if got := d.ledger.PoolSweeps[sweepKey(m.Queue, m.Pool, m.Member)]; got == nil || got.Phase != "TERMINAL" || d.sweepLaneHeld(m.Pool, m.Member) {
			t.Fatal("released original did not end lane hold", got)
		}
	})
}

// PSR-V0-010 rollback: disabling the opt-in releases lanes held only by retained
// records, which stay intact for a later enable; an in-flight job still holds.
func TestPSRDispatchDisableReleasesLane(t *testing.T) {
	d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
		return PoolSweepResult{Pending: true}, nil
	})
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	psrCall(t, q)
	psrCollect(t, d)
	m := q.obs.Members[0]
	key := sweepKey(m.Queue, m.Pool, m.Member)
	if got := d.ledger.PoolSweeps[key]; got == nil || got.Phase != "PENDING" || !d.sweepLaneHeld(m.Pool, m.Member) {
		t.Fatal("enabled pending record does not hold lane", got)
	}
	config := d.Config.PoolSweep
	d.Config.PoolSweep = nil
	if d.sweepLaneHeld(m.Pool, m.Member) {
		t.Fatal("disabled opt-in still holds lane")
	}
	d.sweepNext = time.Time{}
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	if q.count.Load() != 1 || d.ledger.PoolSweeps[key] == nil || d.ledger.PoolSweeps[key].Phase != "PENDING" {
		t.Fatal("disable invoked or dropped retained record")
	}
	d.sweepJob = &poolSweepJob{record: *d.ledger.PoolSweeps[key]}
	if !d.sweepLaneHeld(m.Pool, m.Member) {
		t.Fatal("in-flight job released lane")
	}
	d.sweepJob = nil
	d.Config.PoolSweep = config
	if !d.sweepLaneHeld(m.Pool, m.Member) {
		t.Fatal("re-enable lost retained hold")
	}
}
func TestPSRDispatchAsyncSupervision(t *testing.T) {
	release := make(chan struct{})
	d, q := psrDispatch(t, func(ctx context.Context, _ PoolSweepRequest) (PoolSweepResult, error) {
		select {
		case <-ctx.Done():
			return PoolSweepResult{Pending: true}, ctx.Err()
		case <-release:
			return PoolSweepResult{Outcome: "COMPLETED"}, nil
		}
	})
	defer psrRetireWorkers(t, d)
	q.obs.Tickets = []Ticket{ticket("one", "P1", 1)}
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	psrCall(t, q)
	if d.Running() != 1 || d.sweepJob == nil {
		t.Fatal("sweep required idle dispatcher")
	}
	q.obs.Tickets = append(q.obs.Tickets, ticket("two", "P1", 2))
	start := time.Now()
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	if time.Since(start) > time.Second || d.Running() != 2 {
		t.Fatal("blocked sweep prevented other admission")
	}
	q.obs.Tickets = nil
	future := time.Now().Add(61 * time.Second)
	d.Now = func() time.Time { return future }
	if e := d.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	if d.Running() != 0 || !has(kinds(t, d), "killed") {
		t.Fatal("active sweep hid worker wall enforcement")
	}
	if d.sweepJob == nil {
		t.Fatal("blocked sweep ended unexpectedly")
	}
	close(release)
	psrCollect(t, d)
	for i := 0; i < 3; i++ {
		future = future.Add(2 * time.Second)
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if q.count.Load() != 1 {
		t.Fatal("same allocation swept again")
	}
}
func TestPSRDispatchCancellationJoin(t *testing.T) {
	t.Run("Close-retains-lock-until-joined", func(t *testing.T) {
		canceled := make(chan struct{})
		release := make(chan struct{})
		d, q := psrDispatch(t, func(ctx context.Context, _ PoolSweepRequest) (PoolSweepResult, error) {
			<-ctx.Done()
			close(canceled)
			<-release
			return PoolSweepResult{Pending: true}, ctx.Err()
		})
		if e := d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		psrCall(t, q)
		done := make(chan error, 1)
		go func() { done <- d.Close() }()
		select {
		case <-canceled:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("Close did not cancel")
		}
		other, e := Open("prog", d.Config, q, io.Discard)
		if e == nil {
			other.Close()
			close(release)
			t.Fatal("lock released before actual join")
		}
		select {
		case <-done:
			close(release)
			t.Fatal("Close returned before join")
		default:
		}
		close(release)
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Close failed to join")
		}
		if e := d.Close(); e != nil {
			t.Fatal("Close not idempotent", e)
		}
	})
	for _, mode := range []string{"finite-run", "config-disable", "run-cancel"} {
		t.Run(mode, func(t *testing.T) {
			joined := make(chan struct{})
			d, q := psrDispatch(t, func(ctx context.Context, _ PoolSweepRequest) (PoolSweepResult, error) {
				<-ctx.Done()
				close(joined)
				return PoolSweepResult{Pending: true}, ctx.Err()
			})
			switch mode {
			case "finite-run":
				if e := d.Run(context.Background(), 1); e != nil {
					t.Fatal(e)
				}
				psrCall(t, q)
			case "config-disable":
				if e := d.Tick(context.Background()); e != nil {
					t.Fatal(e)
				}
				psrCall(t, q)
				d.Config.PoolSweep = nil
				psrCollect(t, d)
			case "run-cancel":
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- d.Run(ctx, 0) }()
				psrCall(t, q)
				cancel()
				select {
				case e := <-done:
					if e != nil {
						t.Fatal(e)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("Run failed to join")
				}
			}
			select {
			case <-joined:
			default:
				t.Fatal("return preceded operation join")
			}
			if d.sweepJob != nil {
				t.Fatal("retained running operation")
			}
		})
	}
}

// PSR-V0-010: under a reader HOLD, Run and Close still cancel and join an
// in-flight sweep before returning, publish no ledger or event (the record
// stays STARTING for the next dispatcher), and return readerErr unchanged.
func TestPSRDispatchReaderHoldJoinsSweep(t *testing.T) {
	for _, mode := range []string{"Close", "Run"} {
		t.Run(mode, func(t *testing.T) {
			var joined atomic.Bool
			d, q := psrDispatch(t, func(ctx context.Context, _ PoolSweepRequest) (PoolSweepResult, error) {
				<-ctx.Done()
				// A join that does not wait for the actual return is observable.
				time.Sleep(100 * time.Millisecond)
				joined.Store(true)
				return PoolSweepResult{Pending: true}, ctx.Err()
			})
			if e := d.Tick(context.Background()); e != nil {
				t.Fatal(e)
			}
			psrCall(t, q)
			if d.sweepJob == nil {
				t.Fatal("sweep not in flight")
			}
			files := func() (ledger, events []byte) {
				var e error
				if ledger, e = os.ReadFile(filepath.Join(d.dir, "state.json")); e != nil {
					t.Fatal(e)
				}
				if events, e = os.ReadFile(filepath.Join(d.dir, "events.jsonl")); e != nil {
					t.Fatal(e)
				}
				return ledger, events
			}
			ledgerBefore, eventsBefore := files()
			readerErr := d.poisonReader(errors.New("injected reader hold"))
			if mode == "Run" {
				if got := d.Run(context.Background(), 1); got != readerErr {
					t.Fatalf("Run returned %v, want readerErr unchanged", got)
				}
				if !joined.Load() || d.sweepJob != nil {
					t.Fatal("Run returned before the sweep was joined")
				}
			}
			if got := d.Close(); got != readerErr {
				t.Fatalf("Close returned %v, want readerErr unchanged", got)
			}
			if !joined.Load() || d.sweepJob != nil {
				t.Fatal("Close returned before the sweep was joined")
			}
			ledgerAfter, eventsAfter := files()
			if !bytes.Equal(ledgerBefore, ledgerAfter) || !bytes.Equal(eventsBefore, eventsAfter) {
				t.Fatal("HOLD published ledger or event bytes")
			}
			l, e := LoadLedger(d.dir, "prog")
			if e != nil {
				t.Fatal(e)
			}
			if len(l.PoolSweeps) != 1 {
				t.Fatal("sweep record absent", l.PoolSweeps)
			}
			for _, r := range l.PoolSweeps {
				if r.Phase != "STARTING" {
					t.Fatal("record not left STARTING", r.Phase)
				}
			}
		})
	}
}
