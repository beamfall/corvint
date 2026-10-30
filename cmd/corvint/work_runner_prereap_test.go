//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// V1-1036 / WQO-V0-032: the work runner's residue result is the pre-reap
// observation made while the exited leader is held unreaped, never a group
// signal sent after the reap. The observation is inverted here: a group with
// a live member is reported quiet, and a quiet group is reported as residue.
// A runner that still decided residue by a post-reap group SIGKILL would
// report the opposite in both cases.
func TestV11036WorkRunnerResidueIsObservedBeforeReap(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		observed     bool
	}{
		{"live member reported quiet", "#!/bin/sh\nsleep 30 &\nprintf '%s' $! > child-pid\nexit 0\n", false},
		{"quiet group reported residue", "#!/bin/sh\nexit 0\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := workWaitResidue
			t.Cleanup(func() { workWaitResidue = previous })
			calls := 0
			workWaitResidue = func(command *exec.Cmd) (bool, error) {
				calls++
				_, err := previous(command)
				return tc.observed, err
			}
			runner := workTestRunner(t, context.Background(), tc.script)
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if tc.observed {
				if err == nil || receipt.State != "INCOMPLETE" || !strings.Contains(err.Error(), "residue") {
					t.Fatalf("observed residue not reported: %+v %v", receipt, err)
				}
			} else {
				if err != nil {
					t.Fatalf("post-reap signal decided residue: %+v %v", receipt, err)
				}
				// The live member was still killed, before the reap.
				workAssertHelperGone(t, filepath.Join(runner.root, "child-pid"))
			}
			if calls != 1 {
				t.Fatalf("pre-reap residue observation calls = %d, want 1", calls)
			}
		})
	}
}

// V1-2433: an adapter start refused "text file busy", as a concurrent fork
// holding materialization's write descriptor makes it on Linux, is retried
// with a fresh command instead of failing the capture as ADAPTER_FAILED; a
// start that stays busy past the bounded retries still fails, and any other
// start error is not retried. A cancellation or deadline that ends the context
// during the retry wait stops the retries and reports the context's cause
// (CANCELLED or INPUT_LIMIT), never the busy start's ADAPTER_FAILED.
func TestV12433WorkRunnerRetriesTextBusyAdapterStart(t *testing.T) {
	previous := workStartLive
	t.Cleanup(func() { workStartLive = previous })
	for _, tc := range []struct {
		name       string
		busy       int
		cause      error
		interrupt  string
		wantStarts int
		wantErr    error
		wantCode   string
	}{
		{"busy then started", 2, syscall.ETXTBSY, "", 3, nil, ""},
		{"busy past the retries", 100, syscall.ETXTBSY, "", 1 + len(workTextBusyRetries), syscall.ETXTBSY, "ADAPTER_FAILED"},
		{"other start error", 100, syscall.EACCES, "", 1, syscall.EACCES, "ADAPTER_FAILED"},
		{"cancelled during the retry", 100, syscall.ETXTBSY, "cancelled", 1, context.Canceled, "CANCELLED"},
		{"deadline during the retry", 100, syscall.ETXTBSY, "deadline", 1, context.DeadlineExceeded, "INPUT_LIMIT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, interrupt := context.Context(context.Background()), func() {}
			if tc.interrupt != "" {
				ctx, interrupt = workFinalInterruption(tc.interrupt)
			}
			defer interrupt()
			starts := 0
			commands := map[*exec.Cmd]bool{}
			workStartLive = func(command *exec.Cmd) error {
				starts++
				if commands[command] {
					t.Error("a command was started twice")
				}
				commands[command] = true
				if starts <= tc.busy {
					// The context ends after the first refused start, while
					// the runner decides whether to retry.
					interrupt()
					return &fs.PathError{Op: "fork/exec", Path: command.Path, Err: tc.cause}
				}
				return previous(command)
			}
			runner := workTestRunner(t, ctx, "#!/bin/sh\nexit 0\n")
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if starts != tc.wantStarts || (err == nil) != (tc.wantErr == nil) {
				t.Fatalf("starts=%d err=%v receipt=%+v; want %d starts, error %v", starts, err, receipt, tc.wantStarts, tc.wantErr)
			}
			if tc.wantErr != nil && (!errors.Is(err, tc.wantErr) || runner.commandError(err, io.Discard) != tc.wantCode) {
				t.Fatalf("failed start = %v (%s), want %v (%s)", err, runner.commandError(err, io.Discard), tc.wantErr, tc.wantCode)
			}
		})
	}
}

// PGO-V0-009 (V1-1056): the work adapter's process group is recorded for the
// exit kill when it starts, and released by the pre-reap wait, so an exiting
// corvint retires a running adapter's group and never names a reaped one.
func TestPGOV0009WorkRunnerRecordsItsGroupForTheExitKill(t *testing.T) {
	previousStart, previousWait := workStartLive, workWaitResidue
	t.Cleanup(func() { workStartLive, workWaitResidue = previousStart, previousWait })
	started, waited := 0, 0
	workStartLive = func(command *exec.Cmd) error {
		started++
		return previousStart(command)
	}
	workWaitResidue = func(command *exec.Cmd) (bool, error) {
		if started != 1 {
			t.Errorf("adapter waited before it was recorded (starts %d)", started)
		}
		waited++
		return previousWait(command)
	}
	runner := workTestRunner(t, context.Background(), "#!/bin/sh\nexit 0\n")
	if _, receipt, err := runner.run("snapshot", []string{}, 100); err != nil {
		t.Fatalf("run: %+v %v", receipt, err)
	}
	if started != 1 || waited != 1 {
		t.Fatalf("recorded starts = %d, pre-reap waits = %d; want 1 and 1", started, waited)
	}
}
