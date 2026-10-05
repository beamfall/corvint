//go:build darwin || linux

package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Leader may run the runtime only after the exact immutable acknowledgment.
func Leader(ctx context.Context, dir, hash string) error {
	raw, e := ReadBounded(filepath.Join(dir, "capsule"), MaxCapsule)
	if e != nil {
		return e
	}
	if Digest(raw) != hash {
		return fmt.Errorf("capsule identity differs")
	}
	var c Capsule
	if e = decode(raw, &c); e != nil {
		return e
	}
	if e = ValidateCapsule(c); e != nil {
		return e
	}
	if syscall.Getpgrp() != os.Getpid() {
		return fmt.Errorf("leader is not isolated session group")
	}
	started, e := ProcessIdentity(os.Getpid())
	if e != nil || started == "" {
		return fmt.Errorf("leader identity unavailable: %v", e)
	}
	boot := Boot{Effect: c.Effect, PID: os.Getpid(), Started: started}
	if e = Publish(dir, "boot", boot); e != nil {
		return e
	}
	deadline, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var ack Ack
	if e = await(deadline, filepath.Join(dir, "ack"), &ack); e != nil {
		return e
	}
	if !ack.Accepted || ack.Boot != boot {
		return fmt.Errorf("ack not accepted for this leader")
	}
	cmd := exec.Command(c.Executable, c.Argv...)
	cmd.Dir = c.Directory
	cmd.Env = c.Env
	cmd.Stdin = strings.NewReader(c.Prompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	if e = Publish(dir, "exit", hostExit{Boot: boot, Passed: runErr == nil}); e != nil {
		return e
	}
	// Keep the ownership anchor alive until the supervisor drains the group.
	hold, cancelHold := context.WithTimeout(ctx, 90*time.Second)
	defer cancelHold()
	<-hold.Done()
	return hold.Err()
}

type boundedOutput struct {
	sync.Mutex
	b        []byte
	overflow bool
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	n := len(p)
	left := MaxHostOutput - len(w.b)
	if n > left {
		w.overflow = true
		p = p[:left]
	}
	w.b = append(w.b, p...)
	return n, nil
}
func groupMembers(group int) (map[int]string, error) {
	raw, e := exec.Command("/bin/ps", "-axo", "pid=,pgid=,stat=").Output()
	if e != nil {
		return nil, e
	}
	out := map[int]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || f[1] != strconv.Itoa(group) || strings.HasPrefix(f[2], "Z") {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			return nil, e
		}
		id, e := ProcessIdentity(pid)
		if e != nil {
			return nil, e
		}
		if id == "" {
			return nil, fmt.Errorf("group member changed during identity observation")
		}
		out[pid] = id
	}
	return out, nil
}
func signalMembers(m map[int]string, s syscall.Signal) error {
	for pid, id := range m {
		now, e := ProcessIdentity(pid)
		if e != nil {
			return e
		}
		if now == id {
			if e = syscall.Kill(pid, s); e != nil && e != syscall.ESRCH {
				return e
			}
		}
	}
	return nil
}

// Run commits SPAWNING before fork and RUNNING before immutable ack:true.
func Run(ctx context.Context, self, dir string, c Capsule, journal Journal) (out Outcome, err error) {
	if err = ValidateCapsule(c); err != nil {
		return
	}
	if err = Publish(dir, "capsule", c); err != nil {
		return
	}
	raw, e := ReadBounded(filepath.Join(dir, "capsule"), MaxCapsule)
	if e != nil {
		return out, e
	}
	if err = journal("SPAWNING", Boot{}, nil); err != nil {
		return
	}
	spawned := false
	defer func() {
		if !spawned {
			out.Clean = true
			out.Class = "NO_EXEC"
			out.OutputSHA256 = Digest(nil)
			_ = journal("STOPPING", Boot{}, &out)
			if e := journal("FINISHED", Boot{}, &out); err == nil {
				err = e
			}
		}
	}()
	cmd := exec.Command(self, "lane-leader", "--directory", dir, "--capsule", Digest(raw))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = os.Environ()
	stdout, stderr := &boundedOutput{}, &boundedOutput{}
	r1, w1, e := os.Pipe()
	if e != nil {
		return out, e
	}
	defer r1.Close()
	defer w1.Close()
	r2, w2, e := os.Pipe()
	if e != nil {
		return out, e
	}
	defer r2.Close()
	defer w2.Close()
	cmd.Stdout = w1
	cmd.Stderr = w2
	if err = cmd.Start(); err != nil {
		return
	}
	spawned = true
	start, startErr := ProcessIdentity(cmd.Process.Pid)
	owned := newOwnedGroup(Boot{PID: cmd.Process.Pid, Started: start})
	w1.Close()
	w2.Close()
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); io.Copy(stdout, r1) }()
	go func() { defer readers.Done(); io.Copy(stderr, r2) }()
	waited := false
	stopped := make(chan error, 1)
	go func() { stopped <- cmd.Wait() }()
	// A detached host's escaped groups are observed while the host runs,
	// since an escape orphaned by its parent's exit is no longer reachable.
	vocabulary, _ := HostVocabulary(c.Host)
	escaped := newEscapes(Boot{PID: cmd.Process.Pid, Started: start})
	// Always retire the owned group, including failed boot/ack paths.
	defer func() {
		if vocabulary.Detached {
			escaped.scan()
		}
		out.Clean = owned.drain()
		if vocabulary.Detached && !escaped.drain() {
			out.Clean = false
		}
		if startErr != nil {
			out.Clean = false
		}
		if !waited {
			select {
			case <-stopped:
				waited = true
			case <-time.After(time.Second):
				out.Clean = false
			}
		}
		// For a detached host, end of file on both pipes is the proof that no
		// process outside the drained groups still holds the host output.
		quiesce := time.Second
		if vocabulary.Detached {
			quiesce = detachedQuiesce
		}
		readDone := make(chan struct{})
		go func() { readers.Wait(); close(readDone) }()
		select {
		case <-readDone:
		case <-time.After(quiesce):
			out.Clean = false
			r1.Close()
			r2.Close()
			<-readDone
		}
		out.Stdout = stdout.b
		out.Stderr = stderr.b
		out.OutputSHA256 = Digest(out.Stdout)
		out.SessionID = vocabulary.Session(out.Stdout)
		if stdout.overflow || stderr.overflow {
			out.Class = "OUTPUT_LIMIT"
			if err == nil {
				err = fmt.Errorf("host output truncated")
			}
		}
		if out.Class == "EXIT_ZERO" {
			var parseErr error
			out.SessionID, out.Result, parseErr = vocabulary.Decode(out.Stdout)
			if parseErr != nil {
				out.Class = "INVALID_RESULT"
				if err == nil {
					err = parseErr
				}
			}
		}
		if out.Clean {
			if je := journal("FINISHED", out.Boot, &out); je != nil && err == nil {
				err = je
			}
		} else {
			if je := journal("BLOCKED_RECOVERY", out.Boot, &out); je != nil && err == nil {
				err = je
			}
		}
	}()
	deadline, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err = await(deadline, filepath.Join(dir, "boot"), &out.Boot); err != nil {
		out.Class = "BOOT_FAILED"
		_ = journal("STOPPING", out.Boot, &out)
		return
	}
	if out.Boot.PID != cmd.Process.Pid || out.Boot.Effect != c.Effect {
		err = fmt.Errorf("boot child binding")
		out.Class = "BOOT_FAILED"
		_ = journal("STOPPING", out.Boot, &out)
		return
	}
	identity, e := ProcessIdentity(out.Boot.PID)
	if e != nil || identity != out.Boot.Started {
		err = fmt.Errorf("boot identity changed")
		_ = journal("STOPPING", out.Boot, &out)
		return
	}
	if err = journal("RUNNING", out.Boot, nil); err != nil {
		return
	}
	if err = Publish(dir, "ack", Ack{Boot: out.Boot, Accepted: true}); err != nil {
		_ = journal("STOPPING", out.Boot, &out)
		return
	}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	lastScan := time.Time{}
waitHost:
	for {
		select {
		case e = <-stopped:
			waited = true
			out.Class = "LEADER_EXITED"
			err = fmt.Errorf("leader exited without supervisor stop: %v", e)
			break waitHost
		case <-ctx.Done():
			out.Class = "INTERRUPTED"
			err = ctx.Err()
			break waitHost
		case <-tick.C:
			if vocabulary.Detached && time.Since(lastScan) >= escapeScanEvery {
				escaped.scan()
				lastScan = time.Now()
			}
			raw, re := ReadBounded(filepath.Join(dir, "exit"), MaxCapsule)
			if os.IsNotExist(re) {
				continue
			}
			var result hostExit
			if re != nil || decode(raw, &result) != nil || result.Boot != out.Boot {
				out.Class = "INVALID_RESULT"
				err = fmt.Errorf("host exit binding differs")
				break waitHost
			}
			out.Class = "EXIT_ZERO"
			if !result.Passed {
				out.Class = "EXIT_NONZERO"
				err = fmt.Errorf("host exited nonzero")
			}
			break waitHost
		}
	}
	if e = journal("STOPPING", out.Boot, &out); e != nil && err == nil {
		err = e
	}
	return
}

type hostExit struct {
	Boot   Boot `json:"boot"`
	Passed bool `json:"passed"`
}
