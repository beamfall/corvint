package procgroup

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	descendantResidentLimit      = 4096
	descendantWitnessLimit       = descendantResidentLimit - 1
	descendantWitnessOmission    = "Process rows are a bounded witness sample; omitted historical or resident identities are not an exhaustive process list."
	descendantUnresolvedOmission = "Additional owned or potentially owned identities may be omitted; cleanup failure remains unresolved."
	descendantInterval           = 20 * time.Millisecond
	// A loaded host can starve one snapshot far past the sampling interval, so
	// each snapshot has its own bound, and the first snapshot and the final
	// sweep retry inside the settle window before they report it unavailable.
	// Under CPU contention on a darwin host (load 50-130), /bin/ps took over
	// 1 s in 14 of 80 samples, over 2.5 s in 4 and at most 5.5 s, with slow
	// samples isolated (V1-1368). Half the settle window lets a slow ps finish
	// and leaves room for a second attempt inside the window and the gap bound.
	descendantSnapshotTimeout = descendantSettle / 2
	descendantSettle          = 5 * time.Second
	// Lost periodic snapshots are tolerated only while consecutive successful
	// snapshots stay this close; a longer blind interval observed nothing.
	// Each periodic snapshot can spend its whole timeout before it is lost, so
	// the bound admits three consecutive lost snapshots and a slow fourth that
	// succeeds; a fourth lost snapshot exceeds it (PGO-V0-020, V1-1651). At one
	// settle window, two lost snapshots always exceeded it on a loaded host.
	descendantMaxGap = 4 * descendantSnapshotTimeout
)

type ObservedProcess struct {
	PID       int    `json:"pid"`
	ParentPID int    `json:"parent_pid"`
	Start     string `json:"start"`
	State     string `json:"state"`
}

// SnapshotGapFailurePrefix starts the failure that records a descendant-snapshot gap beyond the
// observation bound (AHI-340); a report whose every failure line starts with it saw no survivor.
const SnapshotGapFailurePrefix = "descendant snapshots were unavailable for "

type DescendantObservation struct {
	Scope       string            `json:"scope"`
	IntervalMS  int               `json:"interval_ms"`
	Processes   []ObservedProcess `json:"processes"`
	Absent      bool              `json:"absent"`
	Failures    []string          `json:"failures"`
	Limitations []string          `json:"limitations"`
}

type processIdentity struct {
	pid   int
	start string
}

func identity(row ObservedProcess) processIdentity { return processIdentity{row.PID, row.Start} }

func compareProcesses(a, b ObservedProcess) int {
	if n := cmp.Compare(a.PID, b.PID); n != 0 {
		return n
	}
	return strings.Compare(a.Start, b.Start)
}

type descendantObserver struct {
	mu    sync.Mutex
	known map[int]ObservedProcess
	root  int
	// rootIdentity is immutable and reserves one resident slot, even after exit.
	// known holds only currently resident descendants, including visible zombies.
	rootIdentity     ObservedProcess
	witnesses        map[processIdentity]ObservedProcess
	witnessOmitted   bool
	residentOverflow bool
	stop             chan struct{}
	done             chan struct{}
	failure          error
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

func startDescendantObserver(ctx context.Context, pid int) (*descendantObserver, error) {
	return startObserver(ctx, &descendantObserver{}, pid)
}

func startObserver(ctx context.Context, o *descendantObserver, pid int) (*descendantObserver, error) {
	o.root, o.stop, o.done = pid, make(chan struct{}), make(chan struct{})
	rows, err := o.firstSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	root, ok := rows[pid]
	if !ok {
		return nil, errors.New("descendant observer could not bind leader identity")
	}
	o.rootIdentity = root
	o.known = make(map[int]ObservedProcess)
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

// firstSnapshot retries an unavailable snapshot inside the settle window, as
// the final sweep does, and returns the last failure when the window closes.
// Caller cancellation stops further retries but not the attempt in flight, so
// a cancelled start waits at most one snapshot bound and its outcome is the
// one a single attempt would have produced.
func (o *descendantObserver) firstSnapshot(caller context.Context) (map[int]ObservedProcess, error) {
	window, cancel := context.WithTimeout(context.Background(), descendantSettle)
	defer cancel()
	for {
		rows, err := o.snapshot(window)
		if err == nil {
			return rows, nil
		}
		select {
		case <-window.Done():
			return nil, err
		case <-caller.Done():
			return nil, err
		case <-time.After(descendantInterval):
		}
	}
}

// observed records a successful snapshot and the blind interval it ended.
func (o *descendantObserver) observed() {
	now := time.Now()
	if !o.seen.IsZero() {
		o.gap = max(o.gap, now.Sub(o.seen))
	}
	o.seen = now
}

// expand runs only on a complete, valid snapshot. Historical witnesses never
// confer ownership; a still-visible zombie does, until absence or PID reuse.
func (o *descendantObserver) expand(rows map[int]ObservedProcess) {
	for pid, previous := range o.known {
		current, present := rows[pid]
		if !present || current.Start != previous.Start {
			delete(o.known, pid)
		} else {
			o.known[pid] = current
		}
	}
	keys := make([]int, 0, len(rows))
	for pid := range rows {
		keys = append(keys, pid)
	}
	slices.Sort(keys)
	for pass := 0; pass < len(keys); pass++ {
		added := false
		for _, pid := range keys {
			row := rows[pid]
			if identity(row) == identity(o.rootIdentity) {
				continue
			}
			if previous, exists := o.known[pid]; exists && previous.Start == row.Start {
				continue
			}
			current, present := rows[row.ParentPID]
			if !present {
				continue
			}
			parent, owned := o.known[row.ParentPID]
			owned = owned && parent.Start == current.Start
			if identity(current) == identity(o.rootIdentity) {
				owned = true
			}
			if !owned {
				continue
			}
			if len(o.known) >= descendantResidentLimit-1 {
				if !o.residentOverflow {
					o.failure = errors.Join(o.failure, errors.New("descendant observation exceeds 4096-process bound"))
					o.residentOverflow = true
				}
				return
			}
			o.known[pid] = row
			o.remember(row)
			added = true
		}
		if !added {
			return
		}
	}
}

func (o *descendantObserver) remember(row ObservedProcess) {
	if o.witnesses == nil {
		o.witnesses = make(map[processIdentity]ObservedProcess)
	}
	key := identity(row)
	if _, exists := o.witnesses[key]; exists {
		return
	}
	if len(o.witnesses) == descendantWitnessLimit {
		o.witnessOmitted = true
		return
	}
	o.witnesses[key] = row
}

func (o *descendantObserver) displayProcesses(report *DescendantObservation) {
	residents := make([]ObservedProcess, 0, len(o.known))
	for _, row := range o.known {
		residents = append(residents, row)
	}
	// Unknown/live residents take priority over exited residents and history.
	slices.SortFunc(residents, func(a, b ObservedProcess) int {
		az, bz := strings.HasPrefix(a.State, "Z"), strings.HasPrefix(b.State, "Z")
		if az != bz {
			if az {
				return 1
			}
			return -1
		}
		return compareProcesses(a, b)
	})
	historical := make([]ObservedProcess, 0, len(o.witnesses))
	for _, row := range o.witnesses {
		historical = append(historical, row)
	}
	slices.SortFunc(historical, compareProcesses)
	selected := make(map[processIdentity]bool)
	for _, rows := range [][]ObservedProcess{residents, historical} {
		for _, row := range rows {
			key := identity(row)
			if selected[key] {
				continue
			}
			if len(report.Processes) == descendantWitnessLimit {
				o.witnessOmitted = true
				continue
			}
			selected[key] = true
			report.Processes = append(report.Processes, row)
		}
	}
	slices.SortFunc(report.Processes, compareProcesses)
	if o.witnessOmitted {
		report.Limitations = append(report.Limitations, descendantWitnessOmission)
	}
	if o.residentOverflow {
		report.Limitations = append(report.Limitations, descendantUnresolvedOmission)
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
		o.failure = errors.Join(o.failure, fmt.Errorf("%s%s, beyond the %s observation bound", SnapshotGapFailurePrefix, o.gap.Round(time.Millisecond), descendantMaxGap))
	}
	report.Absent = o.failure == nil
	if o.missed > 0 {
		report.Limitations = append(report.Limitations, fmt.Sprintf("%d periodic snapshots were unavailable; a descendant alive only during those intervals can remain unobserved", o.missed))
	}
	o.displayProcesses(report)
	if o.failure != nil {
		report.Failures = append(report.Failures, o.failure.Error())
	}
	return report, o.failure
}
