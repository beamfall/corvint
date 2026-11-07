// SPDX-License-Identifier: Apache-2.0
// Package cliconformance checks the public process boundary independently of
// each command's native serializers and in-process test helpers.
package cliconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/clierror"
)

// Failure launches one binary and requires one coded, explanatory stderr
// envelope, empty stdout, and the contract exit status.
func Failure(t *testing.T, binary string, args []string, wantExit int) clierror.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err := cmd.Run()
	exit := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("launch: %v", err)
		}
		exit = e.ExitCode()
	}
	if exit != wantExit || out.Len() != 0 {
		t.Fatalf("%v: exit=%d want=%d stdout=%q stderr=%q", args, exit, wantExit, out.String(), errout.String())
	}
	var envelope clierror.Envelope
	if err := json.Unmarshal(errout.Bytes(), &envelope); err != nil {
		t.Fatalf("%v is not one JSON envelope: %v %q", args, err, errout.String())
	}
	if err := clierror.Validate(envelope); err != nil {
		t.Fatalf("%v: %v: %s", args, err, errout.String())
	}
	return envelope
}
