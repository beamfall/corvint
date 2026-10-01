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
	go func() {
		defer close(o.done)
		ticker := time.NewTicker(descendantInterval)
		defer ticker.Stop()
		for {
			select {
			case <-o.stop:
				return
			case <-ticker.C:
				rows, err := o.snapshot(context.Background())
				o.mu.Lock()
				if err != nil {
					o.missed++
				} else {
					o.expand(rows)
				}
				o.mu.Unlock()
			}
		}
	}()
	return o, nil
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
	<-o.done
	report := &DescendantObservation{Scope: "observed-pid-start-identities", IntervalMS: int(descendantInterval / time.Millisecond), Processes: []ObservedProcess{}, Failures: []string{}, Limitations: []string{"fast detachment and reparenting between snapshots can remain unobserved; this is not full OS containment", "PID start identity resolution is platform-dependent; no universal adversarial containment claim"}}
	ctx, cancel := context.WithTimeout(context.Background(), descendantSettle)
	defer cancel()
	for {
		// unavailable is the reason this round proved nothing; only a round
		// that still proves nothing when the settle window closes is a failure.
		rows, unavailable := o.snapshot(ctx)
		if unavailable == nil {
			o.expand(rows)
			live := false
			for pid, owned := range o.known {
				if pid == o.root {
					continue
				}
				current, present := rows[pid]
				// A zombie has exited; only its parent can finish reaping it.
				if !present || current.Start != owned.Start || strings.HasPrefix(current.State, "Z") {
					continue
				}
				live = true
				if err := signalObservedProcess(rows, owned); err != nil {
					o.failure = errors.Join(o.failure, err)
				}
			}
			if !live {
				report.Absent = o.failure == nil
				break
			}
		}
		select {
		case <-ctx.Done():
			if unavailable == nil {
				unavailable = errors.New("observed descendant remains after cleanup")
			}
			o.failure = errors.Join(o.failure, unavailable)
		case <-time.After(descendantInterval):
			continue
		}
		break
	}
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
