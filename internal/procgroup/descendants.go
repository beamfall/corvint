package procgroup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	descendantInterval = 20 * time.Millisecond
	// A loaded host can starve one snapshot far past the sampling interval, so
	// each snapshot has its own bound and the final sweep retries inside the
	// settle window before it reports cleanup as incomplete.
	descendantSnapshotTimeout = time.Second
	descendantSettle          = 5 * time.Second
	// Lost periodic snapshots are tolerated only while consecutive successful
	// snapshots stay this close; a longer blind interval observed nothing.
	descendantMaxGap = descendantSettle
)

type ObservedProcess struct {
	PID       int    `json:"pid"`
	ParentPID int    `json:"parent_pid"`
	Start     string `json:"start"`
	State     string `json:"state"`
}

type DescendantObservation struct {
	Scope       string            `json:"scope"`
	IntervalMS  int               `json:"interval_ms"`
	Processes   []ObservedProcess `json:"processes"`
	Absent      bool              `json:"absent"`
	Failures    []string          `json:"failures"`
	Limitations []string          `json:"limitations"`
}

type descendantObserver struct {
	mu      sync.Mutex
	known   map[int]ObservedProcess
	root    int
	stop    chan struct{}
	done    chan struct{}
	failure error
	// missed counts periodic snapshots that were unavailable. A lost sample is
	// not a survivor: the final sweep still has to prove every known identity gone.
	missed int
	// seen is the last successful snapshot; gap is the longest interval between
	// two successful snapshots while the observer ran.
	seen time.Time
	gap  time.Duration
	// cancel abandons a periodic snapshot still in flight when finish begins.
	cancel context.CancelFunc
	// snap replaces descendantSnapshot in tests.
	snap func(context.Context) (map[int]ObservedProcess, error)
}

func (o *descendantObserver) snapshot(parent context.Context) (map[int]ObservedProcess, error) {
	ctx, cancel := context.WithTimeout(parent, descendantSnapshotTimeout)
	defer cancel()
	if o.snap != nil {
		return o.snap(ctx)
	}
	return descendantSnapshot(ctx)
}

func startDescendantObserver(pid int) (*descendantObserver, error) {
	return startObserver(&descendantObserver{}, pid)
}

func startObserver(o *descendantObserver, pid int) (*descendantObserver, error) {
	o.root, o.stop, o.done = pid, make(chan struct{}), make(chan struct{})
	rows, err := o.snapshot(context.Background())
	if err != nil {
		return nil, err
	}
	root, ok := rows[pid]
	if !ok {
		return nil, errors.New("descendant observer could not bind leader identity")
	}
	o.known = map[int]ObservedProcess{pid: root}
	o.expand(rows)
	o.seen = time.Now()
	var periodic context.Context
	periodic, o.cancel = context.WithCancel(context.Background())
	go func() {
		defer close(o.done)
		ticker := time.NewTicker(descendantInterval)
		defer ticker.Stop()
		for {
			select {
			case <-o.stop:
				return
			case <-ticker.C:
				rows, err := o.snapshot(periodic)
				if periodic.Err() != nil {
					return
				}
				o.mu.Lock()
				if err != nil {
					o.missed++
				} else {
					o.observed()
					o.expand(rows)
				}
				o.mu.Unlock()
			}
		}
	}()
	return o, nil
}

// observed records a successful snapshot and the blind interval it ended.
func (o *descendantObserver) observed() {
	now := time.Now()
	if !o.seen.IsZero() {
		o.gap = max(o.gap, now.Sub(o.seen))
	}
	o.seen = now
}

// expand admits children only while the observed parent's start identity agrees.
func (o *descendantObserver) expand(rows map[int]ObservedProcess) {
	for pass := 0; pass < len(rows); pass++ {
		added := false
		for pid, row := range rows {
			if previous, exists := o.known[pid]; exists && previous.Start == row.Start {
				continue
			}
			parent, owned := o.known[row.ParentPID]
			current, present := rows[row.ParentPID]
			if owned && present && parent.Start == current.Start {
				if len(o.known) >= 4096 {
					o.failure = errors.New("descendant observation exceeds 4096-process bound")
					return
				}
				o.known[pid] = row
				added = true
			}
		}
		if !added {
			return
		}
	}
}

func (o *descendantObserver) finish() (*DescendantObservation, error) {
	close(o.stop)
	if o.cancel != nil {
		o.cancel()
	}
	<-o.done
	report := &DescendantObservation{Scope: "observed-pid-start-identities", IntervalMS: int(descendantInterval / time.Millisecond), Processes: []ObservedProcess{}, Failures: []string{}, Limitations: []string{"fast detachment and reparenting between snapshots can remain unobserved; this is not full OS containment", "PID start identity resolution is platform-dependent; no universal adversarial containment claim"}}
	ctx, cancel := context.WithTimeout(context.Background(), descendantSettle)
	defer cancel()
	// remains and signalFailure describe the last round that had a snapshot;
	// unavailable is why the latest round had none.
	var remains bool
	var signalFailure error
	for {
		rows, unavailable := o.snapshot(ctx)
		if unavailable == nil {
			o.observed()
			o.expand(rows)
			remains, signalFailure = false, nil
			for pid, owned := range o.known {
				if pid == o.root {
					continue
				}
				current, present := rows[pid]
				// A zombie has exited; only its parent can finish reaping it.
				if !present || current.Start != owned.Start || strings.HasPrefix(current.State, "Z") {
					continue
				}
				remains = true
				signalFailure = errors.Join(signalFailure, signalObservedProcess(rows, owned))
			}
			if !remains {
				break
			}
		}
		select {
		case <-ctx.Done():
			if remains {
				o.failure = errors.Join(o.failure, errors.New("observed descendant remains after cleanup"), signalFailure)
			}
			o.failure = errors.Join(o.failure, unavailable)
		case <-time.After(descendantInterval):
			continue
		}
		break
	}
	if o.gap > descendantMaxGap {
		o.failure = errors.Join(o.failure, fmt.Errorf("descendant snapshots were unavailable for %s, beyond the %s observation bound", o.gap.Round(time.Millisecond), descendantMaxGap))
	}
	report.Absent = o.failure == nil
	if o.missed > 0 {
		report.Limitations = append(report.Limitations, fmt.Sprintf("%d periodic snapshots were unavailable; a descendant alive only during those intervals can remain unobserved", o.missed))
	}
	for pid, row := range o.known {
		if pid != o.root {
			report.Processes = append(report.Processes, row)
		}
	}
	slices.SortFunc(report.Processes, func(a, b ObservedProcess) int { return a.PID - b.PID })
	if o.failure != nil {
		report.Failures = append(report.Failures, o.failure.Error())
	}
	return report, o.failure
}
