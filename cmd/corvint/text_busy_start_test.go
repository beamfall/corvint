// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// copiedBinaryTextBusyBudget bounds how long startCopiedBinary keeps retrying
// ETXTBSY. The product adapter keeps its short workTextBusyRetries schedule;
// a test start may wait much longer because on a heavily loaded host a
// sibling test's forked child can hold the copy's inherited write descriptor
// for far more than that schedule's ~375 ms before its own exec closes it
// (V1-3619, ASD-V0-927).
var copiedBinaryTextBusyBudget = 10 * time.Second

// startCopiedBinary starts a fresh command from build, retrying a start that
// fails with ETXTBSY until copiedBinaryTextBusyBudget elapses. Test helpers
// copy or build a binary and then exec it while other parallel tests fork;
// such a fork can hold the new file's write descriptor until its own exec
// closes it, so a start can fail "text file busy" on Linux although nothing
// started. Every test helper that execs a binary it just wrote starts it here
// (ASD-V0-106, V1-3175, ASD-V0-927).
func startCopiedBinary(build func() *exec.Cmd) (*exec.Cmd, error) {
	var command *exec.Cmd
	err := retryTextBusyStart(copiedBinaryTextBusyBudget, func() error {
		command = build()
		return command.Start()
	})
	return command, err
}

// retryTextBusyStart calls start until it returns anything but ETXTBSY or
// budget elapses, backing off from 5 ms to at most 200 ms between attempts.
func retryTextBusyStart(budget time.Duration, start func() error) error {
	deadline := time.Now().Add(budget)
	delay := 5 * time.Millisecond
	for {
		err := start()
		if err == nil || !errors.Is(err, syscall.ETXTBSY) || !time.Now().Add(delay).Before(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 200*time.Millisecond {
			delay *= 2
		}
	}
}

// TestASDV0927CopiedBinaryStartOutlastsLoadedFork holds the start "busy" for
// longer than the product adapter's whole retry schedule, as a slow fork on a
// loaded host can; the test start must keep retrying and succeed, must stop
// at the first non-ETXTBSY error, and must return ETXTBSY once its budget is
// spent (V1-3619).
func TestASDV0927CopiedBinaryStartOutlastsLoadedFork(t *testing.T) {
	var schedule time.Duration
	for _, delay := range workTextBusyRetries {
		schedule += delay
	}
	busyFor := schedule + 300*time.Millisecond
	started := time.Now()
	attempts := 0
	err := retryTextBusyStart(copiedBinaryTextBusyBudget, func() error {
		attempts++
		if time.Since(started) < busyFor {
			return &os.PathError{Op: "fork/exec", Path: "bin/corvint", Err: syscall.ETXTBSY}
		}
		return nil
	})
	if err != nil || attempts < 2 {
		t.Fatalf("start busy for %s past the %s adapter schedule: err=%v attempts=%d", busyFor, schedule, err, attempts)
	}
	other := errors.New("permission denied")
	attempts = 0
	if err := retryTextBusyStart(copiedBinaryTextBusyBudget, func() error { attempts++; return other }); err != other || attempts != 1 {
		t.Fatalf("non-ETXTBSY start: err=%v attempts=%d", err, attempts)
	}
	err = retryTextBusyStart(50*time.Millisecond, func() error { return syscall.ETXTBSY })
	if !errors.Is(err, syscall.ETXTBSY) {
		t.Fatalf("spent budget: %v", err)
	}
}
