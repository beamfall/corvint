//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// snapshotOutputDrain bounds how long a descendant snapshot waits for ps's
// output after ps exits (V1-3435).
var snapshotOutputDrain = 5 * time.Second

func descendantSnapshot(ctx context.Context) (map[int]ObservedProcess, error) {
	command := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,lstart=,stat=")
	return descendantSnapshotCommand(command)
}

// descendantSnapshotCommand accepts an unstarted command for controlled fixtures.
// Production always supplies /bin/ps with the original arguments above.
func descendantSnapshotCommand(command *exec.Cmd) (map[int]ObservedProcess, error) {
	command.Env = []string{"LC_ALL=C", "TZ=UTC"}
	// ps exits before its output copy finishes; on a loaded host that copy can
	// lag well past 100 ms, which made a successful snapshot read as
	// unavailable. 5 s matches the portable Git backend (CEM-V1-820, V1-3435).
	command.WaitDelay = snapshotOutputDrain
	capture := newProcessCapture(4<<20, make(chan struct{}, 1), OverflowFail)
	command.Stdout = capture
	// Drain excess diagnostics so a noisy ps does not block on a full pipe.
	// Overflow still invalidates the entire snapshot, even on command failure.
	stderr := newProcessCapture(16<<10, nil, OverflowTruncate)
	command.Stderr = stderr
	runErr := command.Run() // Wait joins both output-copy goroutines.
	data, overflow := capture.result()
	_, stderrOverflow := stderr.result()
	var failure error
	if runErr != nil {
		failure = errors.New("descendant snapshot unavailable")
	}
	if overflow {
		failure = errors.Join(failure, errors.New("descendant snapshot exceeds bound"))
	}
	if stderrOverflow {
		failure = errors.Join(failure, errors.New("descendant snapshot stderr exceeds bound"))
	}
	if failure != nil {
		return nil, failure
	}
	return parseDescendantSnapshot(data)
}

func parseDescendantSnapshot(data []byte) (map[int]ObservedProcess, error) {
	rows := map[int]ObservedProcess{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 8 {
			return nil, errors.New("invalid descendant snapshot row")
		}
		pid, e1 := strconv.Atoi(fields[0])
		parent, e2 := strconv.Atoi(fields[1])
		if e1 != nil || e2 != nil || pid <= 0 {
			return nil, errors.New("invalid descendant snapshot identity")
		}
		if _, exists := rows[pid]; exists {
			return nil, errors.New("duplicate descendant snapshot identity")
		}
		rows[pid] = ObservedProcess{PID: pid, ParentPID: parent, Start: strings.Join(fields[2:7], " "), State: fields[7]}
	}
	return rows, nil
}

// signalObservedProcess kills owned only while rows, the snapshot its caller
// just took, still shows the same start identity for that PID.
func signalObservedProcess(rows map[int]ObservedProcess, owned ObservedProcess) error {
	current, present := rows[owned.PID]
	if !present || current.Start != owned.Start {
		return nil
	}
	if err := syscall.Kill(owned.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal observed descendant: %w", err)
	}
	return nil
}
