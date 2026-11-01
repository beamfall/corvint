package main

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// WQO-V0-684 (V1-2586): an ADAPTER_FAILED error names its operation and a
// fixed cause on one stderr line, and the cause never echoes adapter bytes.
func TestWQOV0684AdapterFailedNamesItsCause(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("process residue"), "(process residue)"},
		{errors.New("process reap incomplete"), "not reaped within 1s of its kill (process reap incomplete)"},
		{errors.New("incomplete pipe capture"), "stayed open for 1s of awake drain time, or 1m0s of wall time"},
		{errors.New("missing private adapter environment"), "private adapter environment is missing"},
		{&fs.PathError{Op: "fork/exec", Path: "/secret/adapter", Err: fs.ErrNotExist}, "could not be started"},
		{errors.New("adapter printed SECRET-BYTES"), "failed before its output was captured"},
	}
	for _, tc := range cases {
		receipt := workqueue.AdapterReceipt{Operation: "list"}
		runner := &workAdapterRunner{failure: tc.err, failureReceipt: receipt}
		var stderr bytes.Buffer
		if code := runner.commandError(tc.err, &stderr); code != "ADAPTER_FAILED" {
			t.Fatalf("%v: code %s", tc.err, code)
		}
		line := stderr.String()
		if !strings.HasPrefix(line, "corvint work: ADAPTER_FAILED: list: ") || !strings.Contains(line, tc.want) || strings.Count(line, "\n") != 1 {
			t.Fatalf("%v: stderr %q, want cause %q", tc.err, line, tc.want)
		}
		if strings.Contains(line, "SECRET") || strings.Contains(line, "/secret/") {
			t.Fatalf("cause echoed error bytes: %q", line)
		}
	}
	// An error that is not the recorded failure names no operation.
	runner := &workAdapterRunner{}
	var stderr bytes.Buffer
	runner.commandError(errors.New("other"), &stderr)
	if !strings.HasPrefix(stderr.String(), "corvint work: ADAPTER_FAILED: adapter: ") {
		t.Fatalf("stderr %q", &stderr)
	}
}
