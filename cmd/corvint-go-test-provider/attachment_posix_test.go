//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestReadParentCapabilityHelper(t *testing.T) {
	if os.Getenv("CORVINT_CAPABILITY_TEST_HELPER") != "1" {
		return
	}
	capability, ok := readParentCapability()
	if !ok {
		os.Exit(2)
	}
	if !bytes.Equal(capability, bytes.Repeat([]byte{7}, 32)) {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestParentCapabilityAdmissionIsBounded(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		size int
		open bool
		exit int
	}{
		{"GLTP-V0-026-empty-open", 0, true, 2},
		{"GLTP-V0-026-complete-open", 32, true, 2},
		{"GLTP-V0-040-short-closed", 31, false, 2},
		{"GLTP-V0-040-complete-closed", 32, false, 0},
		{"GLTP-V0-040-long-closed", 33, false, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if _, err := writer.Write(bytes.Repeat([]byte{7}, test.size)); err != nil {
				t.Fatal(err)
			}
			if !test.open {
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestReadParentCapabilityHelper$")
			command.Env = []string{"CORVINT_CAPABILITY_TEST_HELPER=1"}
			command.ExtraFiles = []*os.File{reader}
			command.WaitDelay = time.Second
			output, err := command.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("capability admission waited for an open writer: %v", ctx.Err())
			}
			exit := 0
			if err != nil {
				var failure *exec.ExitError
				if !errors.As(err, &failure) {
					t.Fatal(err)
				}
				exit = failure.ExitCode()
			}
			if exit != test.exit || len(output) != 0 {
				t.Fatalf("exit=%d output=%q; want silent exit %d", exit, output, test.exit)
			}
			assertDirectCommandProcessGone(t, command.Process.Pid)
		})
	}
}
