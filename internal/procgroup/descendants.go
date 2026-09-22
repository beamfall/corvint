package procgroup

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

const descendantInterval = 20 * time.Millisecond

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
}

func startDescendantObserver(pid int) (*descendantObserver, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	rows, err := descendantSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	root, ok := rows[pid]
	if !ok {
		return nil, errors.New("descendant observer could not bind leader identity")
	}
	o := &descendantObserver{known: map[int]ObservedProcess{pid: root}, root: pid, stop: make(chan struct{}), done: make(chan struct{})}
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
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				rows, err := descendantSnapshot(ctx)
				cancel()
				o.mu.Lock()
				if o.failure == nil {
					o.failure = err
				}
				if err == nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		rows, err := descendantSnapshot(ctx)
		if err != nil {
			o.failure = errors.Join(o.failure, err)
			break
		}
		o.expand(rows)
		live := false
		for pid, owned := range o.known {
			if pid == o.root {
				continue
			}
			current, present := rows[pid]
			if !present || current.Start != owned.Start {
				continue
			}
			live = true
			if err := signalObservedProcess(ctx, owned); err != nil {
				o.failure = errors.Join(o.failure, err)
			}
		}
		if !live {
			report.Absent = o.failure == nil
			break
		}
		select {
		case <-ctx.Done():
			o.failure = errors.Join(o.failure, errors.New("observed descendant remains after cleanup"))
		case <-time.After(descendantInterval):
			continue
		}
		break
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
