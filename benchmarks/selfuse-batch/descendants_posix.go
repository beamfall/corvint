//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type processRow struct {
	parent       int
	birth, state string
}
type descendantCleanup struct {
	cleaned, leaked []int
	status          string
	failure         error
	snapshot        func(context.Context) (map[int]processRow, error)
	owned           map[int]string
	leaderBirth     string
	monitorCancel   context.CancelFunc
	monitorDone     chan struct{}
	observationErr  error
}

func newDescendantCleanup() *descendantCleanup {
	return &descendantCleanup{cleaned: []int{}, leaked: []int{}, status: "NOT_OBSERVED", snapshot: processSnapshot, owned: map[int]string{}}
}

func (d *descendantCleanup) remember(rows map[int]processRow, pid int) {
	roots := map[int]bool{}
	if sameProcess(rows[pid], d.leaderBirth) {
		roots[pid] = true
	}
	for child, birth := range d.owned {
		if sameProcess(rows[child], birth) {
			roots[child] = true
		}
	}
	for ancestor := range roots {
		for child, birth := range descendants(rows, ancestor) {
			d.owned[child] = birth
		}
	}
}

// descendantMonitorTick paces the background /bin/ps probe used to catch a
// descendant that starts and exits entirely between polls, while the child is
// still running. stop() takes its own snapshot before signalling (see below),
// so this tick buys nothing beyond narrowing that start-and-exit race window;
// widening it from 20ms to 250ms cuts probe volume by 12.5x (roughly 40 forks
// instead of 500 over a ten-second child) at the cost of that window growing
// to 250ms. It has no effect on the stop path's own timing: stop cancels this
// ticker before doing anything else, and the 350ms SIGTERM grace and its
// retries below use their own fixed 20ms cadence, independent of this value.
const descendantMonitorTick = 250 * time.Millisecond

func (d *descendantCleanup) start(ctx context.Context, pid int) error {
	rows, err := d.snapshot(ctx)
	if err != nil {
		d.observationErr = err
		return err
	}
	d.leaderBirth = rows[pid].birth
	d.remember(rows, pid)
	monitorContext, cancel := context.WithCancel(context.Background())
	d.monitorCancel, d.monitorDone = cancel, make(chan struct{})
	go func() {
		defer close(d.monitorDone)
		ticker := time.NewTicker(descendantMonitorTick)
		defer ticker.Stop()
		for {
			select {
			case <-monitorContext.Done():
				return
			case <-ticker.C:
				rows, err := d.snapshot(monitorContext)
				if err != nil {
					if monitorContext.Err() == nil {
						d.observationErr = err
					}
					return
				}
				d.remember(rows, pid)
			}
		}
	}()
	return nil
}

type processBuffer struct{ bytes.Buffer }

func (b *processBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 4*1024*1024 {
		return 0, errors.New("process snapshot output limit")
	}
	return b.Buffer.Write(p)
}

// processProbeBudget bounds one /bin/ps probe. It is a hang detector, not a
// performance budget (decision 0082). The probe measures 20 ms on a quiet host,
// and the previous 100 ms left a 5x margin that a saturated host consumed: the
// monitor below probes every descendantMonitorTick for the whole child
// lifetime, so one overrun anywhere in that many probes sets observationErr
// and fails the run. The larger value costs nothing in the cleanup path, where
// the caller's context binds first -- procgroup allows BeforeStop and
// AfterStart min(1s, ShutdownTimeout/2), internal/procgroup/process.go:291 --
// and stop cancels the monitor before joining it, so an in-flight probe ends
// on cancellation.
const processProbeBudget = 2 * time.Second

func processSnapshot(parent context.Context) (map[int]processRow, error) {
	ctx, cancel := context.WithTimeout(parent, processProbeBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,stat=,lstart=")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	var out processBuffer
	cmd.Stdout = &out
	cmd.WaitDelay = processProbeBudget
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	rows := map[int]processRow{}
	for _, line := range strings.Split(out.String(), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		if len(parts) != 8 {
			return nil, errors.New("malformed process snapshot")
		}
		pid, e1 := strconv.Atoi(parts[0])
		parent, e2 := strconv.Atoi(parts[1])
		if e1 != nil || e2 != nil || pid <= 0 {
			return nil, errors.New("malformed process identity")
		}
		rows[pid] = processRow{parent: parent, state: parts[2], birth: strings.Join(parts[3:], " ")}
	}
	return rows, nil
}
func sameProcess(row processRow, birth string) bool {
	return row.birth != "" && row.birth == birth && !strings.HasPrefix(row.state, "Z")
}
func descendants(rows map[int]processRow, ancestor int) map[int]string {
	children := make(map[int][]int, len(rows))
	for pid, row := range rows {
		children[row.parent] = append(children[row.parent], pid)
	}
	// BFS out from ancestor over the parent-to-children adjacency built above,
	// which is linear in len(rows) to build and to walk (each pid is visited
	// once). roots guards re-enqueueing, so a ppid cycle reachable from
	// ancestor -- including ancestor being its own ancestor -- still
	// terminates instead of looping the way a naive parent-chain walk would.
	roots := map[int]bool{ancestor: true}
	queue := []int{ancestor}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if !roots[child] {
				roots[child] = true
				queue = append(queue, child)
			}
		}
	}
	out := map[int]string{}
	for pid := range roots {
		if pid != ancestor && sameProcess(rows[pid], rows[pid].birth) {
			out[pid] = rows[pid].birth
		}
	}
	return out
}
func survivors(rows map[int]processRow, owned map[int]string) []int {
	out := []int{}
	for pid, birth := range owned {
		if sameProcess(rows[pid], birth) {
			out = append(out, pid)
		}
	}
	sort.Ints(out)
	return out
}
func (d *descendantCleanup) stop(ctx context.Context, pid int) (failure error) {
	defer func() { d.failure = failure }()
	if d.monitorCancel != nil {
		d.monitorCancel()
		// The cancel above ends any in-flight probe. Join the monitor so no process
		// identity can be read or signalled after this invocation returns.
		<-d.monitorDone
	}
	defer func() { failure = errors.Join(failure, d.observationErr) }()
	// This snapshot, not the monitor's tick rate, fixes the set stop can act on:
	// taken after the monitor is joined and before any signal, it extends owned
	// with every descendant still reachable from the leader.
	rows, err := d.snapshot(ctx)
	if err != nil {
		d.status = "NOT_OBSERVED: snapshot-failed"
		return err
	}
	if d.leaderBirth == "" {
		d.leaderBirth = rows[pid].birth
	}
	d.remember(rows, pid)
	owned := d.owned
	d.status = "OBSERVED_TRUSTED_DESCENDANTS_ONLY"
	if rows[pid].birth == "" || strings.HasPrefix(rows[pid].state, "Z") {
		d.status = "NOT_OBSERVED: leader-exit-race"
	}
	for child := range owned {
		d.cleaned = append(d.cleaned, child)
	}
	sort.Ints(d.cleaned)
	if len(owned) == 0 {
		return nil
	}
	signalMatching := func(sig syscall.Signal) error {
		current, err := d.snapshot(ctx)
		if err != nil {
			return err
		}
		for _, child := range survivors(current, owned) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := syscall.Kill(child, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
				return err
			}
		}
		return nil
	}
	if err := signalMatching(syscall.SIGTERM); err != nil {
		return err
	}
	termDeadline := time.Now().Add(350 * time.Millisecond)
	for time.Now().Before(termDeadline) {
		rows, err = d.snapshot(ctx)
		if err != nil {
			return err
		}
		if len(survivors(rows, owned)) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := signalMatching(syscall.SIGKILL); err != nil {
		return err
	}
	for {
		rows, err = d.snapshot(ctx)
		if err != nil {
			return err
		}
		d.leaked = survivors(rows, owned)
		if len(d.leaked) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("observed descendants survived cleanup: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
