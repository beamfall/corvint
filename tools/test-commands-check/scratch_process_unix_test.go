// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// DTL-V0-018: a separate supervisor retains the group cleanup even if the
// invoking test binary is SIGKILLed. Its private stdin pipe is the lifeline;
// neither bash nor its descendants inherit the writing end.
func guardScratchBash(t *testing.T, c *exec.Cmd) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	script := c.Args[len(c.Args)-1]
	c.Path, c.Args = exe, []string{exe, "-test.run=^TestDTLV0018ScratchProcessHelper$"}
	c.Env = append(c.Env, "CORVINT_DFRACE_BASH="+script)
	c.Stdin = r
	c.Cancel = func() error { return w.Close() }
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
}

func TestDTLV0018ScratchProcessHelper(t *testing.T) {
	script, ok := os.LookupEnv("CORVINT_DFRACE_BASH")
	if !ok {
		return
	}
	c := exec.Command("bash", "-c", script)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.ExtraFiles = []*os.File{os.Stdout}
	result, _, err := superviseScratch(c, os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != groupreap.Released {
		t.Fatalf("scratch group retirement %s: %v", result.State, result.Err)
	}
	if result.WaitErr != nil {
		t.Fatal(result.WaitErr)
	}
}

// scratchHost is the supervisor's groupreap host; tests record its calls.
var scratchHost = groupreap.Primitives{}

// scratchRetireBound bounds retirement after bash's exit was observed.
const scratchRetireBound = 30 * time.Second

// superviseScratch owns bash's process group through its unreaped leader.
// Lifeline EOF requests Owner.Stop, which signals only while the leader is
// held unreaped; Finish retires the group, reaps once, and then sends only
// signal-0 probes, so no numeric signal can reach a reused PID or group ID.
// stopped closes once the lifeline's Stop request has returned.
func superviseScratch(c *exec.Cmd, lifeline io.Reader) (groupreap.Result, <-chan struct{}, error) {
	owner, err := groupreap.StartWith(c, scratchHost)
	if err != nil {
		return groupreap.Result{}, nil, err
	}
	stopped := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, lifeline); owner.Stop(); close(stopped) }()
	<-owner.Exited()
	bound, cancel := context.WithTimeout(context.Background(), scratchRetireBound)
	defer cancel()
	return owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done()}), stopped, nil
}

// DTL-V0-018: kill the actual invoking test process while bash and a bash
// command-substitution child are alive, then wait for both process handles to signal
// exit. No sleep/poll loop and no dependence on the host reaping zombies.
func TestDTLV0018ScratchParentSIGKILL(t *testing.T) {
	if os.Getenv("CORVINT_DFRACE_PARENT") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		// $$ works on Darwin's bundled bash 3.2 as well as newer bash. Exec
		// inside the substitution gives its bash child a distinct reported PID.
		c := exec.CommandContext(ctx, "bash", "-c", `printf '%s\n' "$$"; x=$(exec bash -c 'printf "%s\n" "$$" >&3; echo ready >&3; while :; do :; done')`)
		c.Env = os.Environ()
		c.ExtraFiles = []*os.File{os.Stdout}
		guardScratchBash(t, c)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command(exe, "-test.run=^TestDTLV0018ScratchParentSIGKILL$")
	c.Env = append(os.Environ(), "CORVINT_DFRACE_PARENT=1")
	out, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Process.Kill() })
	// Bound readiness reads as well as the command. Timeout kills the parent,
	// whose pipe closure makes the supervisor clean its bash group.
	timer := time.AfterFunc(10*time.Second, func() { _ = c.Process.Kill() })
	defer timer.Stop()
	var data strings.Builder
	buf := make([]byte, 1)
	for !strings.HasSuffix(data.String(), "ready\n") {
		if _, err := io.ReadFull(out, buf); err != nil {
			t.Fatalf("readiness: %v %q", err, data.String())
		}
		data.Write(buf)
	}
	var children []func()
	for _, word := range strings.Fields(strings.TrimSuffix(data.String(), "ready\n")) {
		pid, err := strconv.Atoi(word)
		if err != nil {
			t.Fatal(err)
		}
		// No numeric cleanup signal: once reaped these IDs may be reused.
		// Killing the parent closes the supervisor's lifeline instead.
		children = append(children, watchScratchExit(t, pid))
	}
	if len(children) != 2 {
		t.Fatalf("want two live bash processes, got %q", data.String())
	}
	if err := c.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = c.Wait()
	for _, wait := range children {
		wait()
	}
}

// scratchRecorder wraps the host primitives and records every real group
// signal, each signal-0 probe and the reap, in order.
type scratchRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *scratchRecorder) add(call string) {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
}

func (r *scratchRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

func (r *scratchRecorder) primitives() groupreap.Primitives {
	real := groupreap.HostPrimitives()
	return groupreap.Primitives{
		KillGroup: func(leader int) error { r.add("kill"); return real.KillGroup(leader) },
		SignalGroup: func(leader int, signal syscall.Signal) error {
			r.add("signal-" + signal.String())
			return real.SignalGroup(leader, signal)
		},
		QuietProof: real.QuietProof,
		ProbeGroup: func(leader int) (groupreap.Probe, error) { r.add("probe"); return real.ProbeGroup(leader) },
		Reap:       func(command *exec.Cmd) error { r.add("reap"); return command.Wait() },
	}
}

// DTL-V0-018: with an injected recording host, the supervisor's one group
// SIGKILL precedes the single reap and only signal-0 probes follow it, both
// when the lifeline closes while bash runs and when it closes after the reap.
func TestDTLV0018ScratchSupervisorNoSignalAfterReap(t *testing.T) {
	for name, tc := range map[string]struct {
		script       string
		lifelineLate bool
	}{
		"lifeline closes while bash runs": {script: "sleep 300 & wait"},
		"lifeline closes after the reap":  {script: "sleep 300 >/dev/null 2>&1 &", lifelineLate: true},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &scratchRecorder{}
			old := scratchHost
			scratchHost = rec.primitives()
			t.Cleanup(func() { scratchHost = old })
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
			if !tc.lifelineLate {
				_ = w.Close()
			}
			result, stopped, err := superviseScratch(exec.Command("bash", "-c", tc.script), r)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != groupreap.Released {
				t.Fatalf("retirement %s: %v", result.State, result.Err)
			}
			_ = w.Close()
			select {
			case <-stopped:
			case <-time.After(10 * time.Second):
				t.Fatal("lifeline Stop did not return")
			}
			calls := rec.seen()
			reap := slices.Index(calls, "reap")
			if reap < 0 || slices.Contains(calls[reap+1:], "reap") {
				t.Fatalf("calls = %v, want exactly one reap", calls)
			}
			if kills := slices.Index(calls, "kill"); kills < 0 || kills > reap || slices.Contains(calls[kills+1:], "kill") {
				t.Fatalf("calls = %v, want one kill before the reap", calls)
			}
			for _, call := range calls[reap+1:] {
				if call != "probe" {
					t.Fatalf("calls = %v: real signal %s after the reap", calls, call)
				}
			}
		})
	}
}
