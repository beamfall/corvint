// SPDX-License-Identifier: AGPL-3.0-or-later

package procfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"testing"
)

func TestUnsupportedHostIsNotObserved(t *testing.T) {
	if Supported {
		t.Skip("this build reads the Linux procfs profile")
	}
	_, captureErr := CaptureBirth(context.Background(), os.Getpid(), 1<<20)
	_, sweepErr := SweepProcesses(context.Background(), 16)
	for _, err := range []error{captureErr, sweepErr} {
		var unsupported UnsupportedError
		if !errors.As(err, &unsupported) || unsupported.Outcome() != "NOT_OBSERVED" || unsupported.Code() != "process-observation-unsupported" {
			t.Fatalf("unsupported host returned %v", err)
		}
	}
}

func TestCaptureOwnBirth(t *testing.T) {
	if !Supported {
		t.Skip("NOT_OBSERVED: this build has no Linux procfs profile")
	}
	pid := os.Getpid()
	c, err := CaptureBirth(context.Background(), pid, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte(strconv.Itoa(pid) + " (")
	if !bytes.HasPrefix(c.StatBefore, prefix) || !bytes.HasPrefix(c.StatAfter, prefix) || !bytes.HasSuffix(c.StatBefore, []byte("\n")) {
		t.Fatalf("stat bytes: %q", c.StatBefore)
	}
	if !bytes.HasPrefix(c.NamespaceLink, []byte("pid:[")) || len(c.BootID) == 0 || c.NamespaceInode == 0 {
		t.Fatalf("identity: boot %q namespace %q inode %d", c.BootID, c.NamespaceLink, c.NamespaceInode)
	}
	sum := sha256.Sum256(c.Executable)
	if len(c.Executable) == 0 || hex.EncodeToString(sum[:]) != c.ExecutableAfterSHA256 || int64(len(c.Executable)) != c.ExecutableAfterBytes {
		t.Fatal("executable bytes and their second digest differ")
	}
	if !bytes.Equal(c.Cmdline, c.CmdlineAfter) || len(c.Cmdline) == 0 || c.Cmdline[len(c.Cmdline)-1] != 0 {
		t.Fatalf("argv bytes: %q", c.Cmdline)
	}
	if _, err := CaptureBirth(context.Background(), pid, 1); err == nil {
		t.Fatal("executable bound was not enforced")
	}
}

func TestSweepListsOwnProcess(t *testing.T) {
	if !Supported {
		t.Skip("NOT_OBSERVED: this build has no Linux procfs profile")
	}
	s, err := SweepProcesses(context.Background(), 1<<16)
	if err != nil {
		t.Fatal(err)
	}
	if lines := bytes.Count(s.PIDDirectory, []byte("\n")); lines != len(s.Processes)+len(s.ReadFailures) {
		t.Fatalf("PID directory has %d entries for %d rows", lines, len(s.Processes)+len(s.ReadFailures))
	}
	found := false
	for _, row := range s.Processes {
		found = found || row.PID == os.Getpid()
	}
	if !found {
		t.Fatal("sweep omits the observer's own process")
	}
	if _, err := SweepProcesses(context.Background(), 1); err == nil {
		t.Fatal("process bound was not enforced")
	}
}
