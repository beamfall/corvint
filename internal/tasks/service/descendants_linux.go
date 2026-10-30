//go:build linux

package service

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// Linux helper descendant ownership. The wrapper makes itself a child
// subreaper, so an orphaned descendant of a helper, including one that
// left the helper's process group or session, is reparented to the wrapper
// rather than to init. The helper leader is owned by #464 groupreap
// (unreaped-leader group signal, reap and post-reap probe); a concurrent
// sweeper kills and reaps every other child of the wrapper. A non-leader
// child is reaped only by this sweeper, so a listed PID cannot be reused
// before it is signalled. Retirement is proved when, after the leader is
// released, wait4 reports ECHILD: the wrapper has no child, so no
// descendant remains (any live descendant's nearest live ancestor below
// the wrapper is the wrapper's child). A process a helper asked another
// manager to start is not a descendant and is not covered.

const (
	prSetChildSubreaper = 36
	prGetChildSubreaper = 37
	sweepInterval       = 5 * time.Millisecond
)

func prctl(op, arg uintptr) (uintptr, error) {
	r, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, op, arg, 0, 0, 0, 0)
	if errno != 0 {
		return r, errno
	}
	return r, nil
}

func subreaperSet() bool {
	var v int32
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prGetChildSubreaper, uintptr(unsafe.Pointer(&v)), 0, 0, 0, 0)
	return errno == 0 && v == 1
}

type linuxSpawner struct{}

// helperRuntime makes this process a verified child subreaper.
func helperRuntime() (helperSpawner, error) {
	if _, err := prctl(prSetChildSubreaper, 1); err != nil {
		return nil, errors.Join(errors.New("child subreaper unavailable"), err)
	}
	if !subreaperSet() {
		return nil, errors.New("child subreaper was not set")
	}
	return linuxSpawner{}, nil
}

// Quiet proves the wrapper has no child: zombies are reaped, a live child
// is an error.
func (linuxSpawner) Quiet() error {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		switch {
		case errors.Is(err, syscall.ECHILD):
			return nil
		case errors.Is(err, syscall.EINTR):
		case err != nil:
			return err
		case pid == 0:
			return errors.New("a live child of this wrapper remains")
		}
	}
}

func (linuxSpawner) Start(cmd *exec.Cmd) (helperProc, error) {
	o, err := groupreap.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &linuxProc{owner: o, pid: cmd.Process.Pid}, nil
}

type linuxProc struct {
	owner *groupreap.Owner
	pid   int
}

func (p *linuxProc) PID() int                { return p.pid }
func (p *linuxProc) Exited() <-chan struct{} { return p.owner.Exited() }

func (p *linuxProc) Verify() (string, error) {
	if !subreaperSet() {
		return "", errors.New("wrapper is no longer a child subreaper")
	}
	if pg, err := syscall.Getpgid(p.pid); err != nil || pg != p.pid {
		return "", errors.New("helper leader does not lead its own process group")
	}
	if ppid, _, err := procParent(p.pid); err != nil || ppid != os.Getpid() {
		return "", errors.New("helper leader is not this wrapper's child")
	}
	ident, err := supervisor.ProcessIdentity(p.pid)
	if err != nil || ident == "" {
		return "", errors.New("helper leader start identity is unreadable")
	}
	return ident, nil
}

func (p *linuxProc) Retire(bound time.Duration) (string, error) {
	deadline := time.Now().Add(bound)
	expired := func() bool { return !time.Now().Before(deadline) }
	stop := make(chan struct{})
	swept := make(chan error, 1)
	go func() { swept <- sweep(p.pid, stop) }()
	p.owner.Stop()
	res := p.owner.FinishBounded(groupreap.RetirementBound{Expired: expired})
	close(stop)
	sweepErr := <-swept
	exit := "exit unobserved"
	if res.State == groupreap.Released {
		exit = "exited"
		if res.WaitErr != nil {
			exit = res.WaitErr.Error()
		}
	}
	if res.State != groupreap.Released {
		return exit, errors.Join(errors.New("helper group retirement unproved"), res.Err, sweepErr)
	}
	if sweepErr != nil {
		return exit, sweepErr
	}
	// The leader is reaped: every remaining child is a descendant.
	for {
		if err := killChildren(0); err != nil {
			return exit, err
		}
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		switch {
		case errors.Is(err, syscall.ECHILD):
			return exit, nil
		case errors.Is(err, syscall.EINTR), err == nil && pid > 0:
			continue
		case err != nil:
			return exit, err
		}
		if expired() {
			return exit, errors.New("helper descendants remained at the retirement bound")
		}
		time.Sleep(sweepInterval)
	}
}

// sweep kills and reaps every child of the wrapper except the leader until
// stop is closed.
func sweep(leader int, stop <-chan struct{}) error {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		if err := killChildren(leader); err != nil {
			return err
		}
		select {
		case <-stop:
			return nil
		case <-t.C:
		}
	}
}

// killChildren sends SIGKILL to every child of this process other than
// leader and reaps those already dead.
func killChildren(leader int) error {
	kids, err := children()
	if err != nil {
		return err
	}
	for _, pid := range kids {
		if pid == leader {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		var ws syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil); err != nil && !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
	return nil
}

// children lists the processes whose parent is this process.
func children() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	out := []int{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		ppid, _, err := procParent(pid)
		if err != nil {
			continue // exited between the listing and the read
		}
		if ppid == self {
			out = append(out, pid)
		}
	}
	return out, nil
}

// procParent reads the parent PID and process group from /proc/PID/stat.
func procParent(pid int) (int, int, error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0, err
	}
	end := strings.LastIndex(string(raw), ") ")
	if end < 0 {
		return 0, 0, errors.New("invalid process stat")
	}
	f := strings.Fields(string(raw[end+2:]))
	if len(f) < 3 {
		return 0, 0, errors.New("short process stat")
	}
	ppid, err := strconv.Atoi(f[1])
	if err != nil {
		return 0, 0, err
	}
	pg, err := strconv.Atoi(f[2])
	return ppid, pg, err
}

// bootClock is the boot identity and monotonic uptime in seconds.
func bootClock() (string, uint64, bool) {
	raw, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	boot := strings.TrimSpace(string(raw))
	if err != nil || boot == "" || len(boot) > 64 {
		return "", 0, false
	}
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil || si.Uptime < 0 {
		return boot, 0, false
	}
	return boot, uint64(si.Uptime), true
}
