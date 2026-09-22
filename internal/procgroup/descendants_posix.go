//go:build darwin || linux

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func descendantSnapshot(ctx context.Context) (map[int]ObservedProcess, error) {
	command := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,lstart=,stat=")
	command.Env = []string{"LC_ALL=C", "TZ=UTC"}
	command.WaitDelay = 100 * time.Millisecond
	capture := newProcessCapture(4<<20, make(chan struct{}, 1), OverflowFail)
	command.Stdout = capture
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, errors.New("descendant snapshot unavailable")
	}
	data, overflow := capture.result()
	if overflow {
		return nil, errors.New("descendant snapshot exceeds bound")
	}
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
		rows[pid] = ObservedProcess{PID: pid, ParentPID: parent, Start: strings.Join(fields[2:7], " "), State: fields[7]}
	}
	return rows, nil
}

func signalObservedProcess(ctx context.Context, owned ObservedProcess) error {
	rows, err := descendantSnapshot(ctx)
	if err != nil {
		return err
	}
	current, present := rows[owned.PID]
	if !present || current.Start != owned.Start {
		return nil
	}
	if err := syscall.Kill(owned.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal observed descendant: %w", err)
	}
	return nil
}
