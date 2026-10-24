//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/worksource"
)

// WQO-V0-052: when retirement is not confirmed, Close keeps the run root and
// its records and returns the error instead of removing scratch under a live run.
//
// The held record is a flock-protected Git group record, which the source
// package keeps only on darwin and linux (internal/worksource/process_unix.go),
// so this file carries the same constraint; the `unix` tag alone admits the
// PRS-V1-019 cross-vet targets without syscall.Flock (solaris).
func TestWorkCloseKeepsScratchWhenRetirementIsUnconfirmed(t *testing.T) {
	caller := materializationFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	run, err := newWorkMaterialization(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(run.root)
	bound := workGitRetirementBound
	workGitRetirementBound = 100 * time.Millisecond
	defer func() { workGitRetirementBound = bound }()
	directory := filepath.Join(run.tmp, "corvint-work-source-held", "git-groups")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	// This test holds the record's lock, standing in for a Git run that never exits.
	record := filepath.Join(directory, "group-"+strings.Repeat("cd", 16))
	holder, err := os.OpenFile(record, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := run.Close(); err == nil || !strings.Contains(err.Error(), run.root) {
		t.Fatalf("unconfirmed retirement: %v", err)
	}
	if _, err := os.Stat(record); err != nil {
		t.Fatalf("run scratch removed under a live record: %v", err)
	}
}
