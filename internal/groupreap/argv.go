//go:build darwin || linux

package groupreap

import (
	"bytes"
	"syscall"
)

// maxArguments bounds one process's argument read.
const maxArguments = 1 << 20

// GroupLeadersWithArgument returns the live, non-zombie processes that lead
// their own process group and whose argument vector holds marker exactly. It
// sends no signal. A process whose arguments cannot be read (it is exiting,
// between fork and exec, or another user's) is not returned, so absence is
// never proved here; the caller confirms it independently.
func GroupLeadersWithArgument(marker string) ([]Process, error) {
	table, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	want := []byte(marker)
	var leaders []Process
	for _, p := range table {
		if p.State == StateZombie || p.PID <= 1 || p.PGID != p.PID {
			continue
		}
		arguments, err := processArguments(p.PID)
		if err != nil {
			continue
		}
		for _, argument := range arguments {
			if bytes.Equal(argument, want) {
				leaders = append(leaders, p)
				break
			}
		}
	}
	return leaders, nil
}

// StopKillGroup stops p's process group under identity pinning (signalPinned),
// then kills it. Only this caller can end the stopped leader, so its PID, and
// with it the group ID, is not released before the SIGKILL; no unpinned group
// is signalled. PGO-V0-007 records signalPinned's residual Darwin window.
func StopKillGroup(p Process) error {
	if err := signalPinned(p, true, syscall.SIGSTOP); err != nil {
		return err
	}
	return syscall.Kill(-p.PID, syscall.SIGKILL)
}

// splitArguments splits NUL-terminated argument strings.
func splitArguments(raw []byte, count int) [][]byte {
	var arguments [][]byte
	for len(raw) > 0 && (count < 0 || len(arguments) < count) {
		i := bytes.IndexByte(raw, 0)
		if i < 0 {
			i = len(raw)
		}
		arguments = append(arguments, raw[:i])
		if i == len(raw) {
			break
		}
		raw = raw[i+1:]
	}
	return arguments
}
