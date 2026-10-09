package groupreap

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
)

// ErrExiting refuses a child start after KillLive: the process is exiting and
// a child started now would outlive it.
var ErrExiting = errors.New("groupreap: process is exiting")

// liveRegistry records the process groups this process leads children into,
// from start until the leader is about to be reaped, so an exit path that
// skips deferred cleanup can still retire them (AHI-048).
type liveRegistry struct {
	// gate is held shared across a start and its record, and across a
	// release; kill holds it exclusively, so it never signals a group whose
	// leader a concurrent Wait has reaped, and never misses a started one.
	gate   sync.RWMutex
	closed bool // written under the exclusive gate
	mu     sync.Mutex
	groups map[int]struct{}
	signal func(int, syscall.Signal) error
}

var liveGroups = &liveRegistry{groups: map[int]struct{}{}, signal: signalLiveGroup}

// StartLive starts command and, where Wait signals the group before reaping
// its leader (waitid), records a group the command leads until Wait releases
// it. A command that leads its own group must then be reaped by Wait, never by
// command.Wait alone, or the record would outlive the reap. Drain and Start
// record and release the same way.
func StartLive(command *exec.Cmd) error { return liveGroups.start(command) }

// KillLive SIGKILLs every recorded process group synchronously and refuses
// later StartLive calls. It is for a process exit that skips deferred
// cleanup, such as os.Exit after an abandoned read; it waits for no reap.
func KillLive() { liveGroups.kill() }

func (r *liveRegistry) start(command *exec.Cmd) error {
	r.gate.RLock()
	defer r.gate.RUnlock()
	if r.closed {
		return ErrExiting
	}
	if err := command.Start(); err != nil {
		return err
	}
	if waitidAvailable && leadsOwnGroup(command) {
		r.mu.Lock()
		r.groups[command.Process.Pid] = struct{}{}
		r.mu.Unlock()
	}
	return nil
}

// release forgets a group before its leader is reaped.
func (r *liveRegistry) release(leader int) {
	r.gate.RLock()
	r.mu.Lock()
	delete(r.groups, leader)
	r.mu.Unlock()
	r.gate.RUnlock()
}

func (r *liveRegistry) kill() {
	r.gate.Lock()
	defer r.gate.Unlock()
	r.closed = true
	for leader := range r.groups {
		_ = r.signal(-leader, syscall.SIGKILL)
	}
	clear(r.groups)
}

func leadsOwnGroup(command *exec.Cmd) bool {
	return command.SysProcAttr != nil && setpgid(command.SysProcAttr) && command.Process != nil
}
