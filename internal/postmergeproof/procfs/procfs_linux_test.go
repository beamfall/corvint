//go:build linux && (amd64 || arm64)

// SPDX-License-Identifier: Apache-2.0

package procfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"syscall"
	"testing"
)

// PMR-V2-011, V1-2570: a PID whose /proc entry vanished between enumeration
// and its stat read (ENOENT or ESRCH, and the entry confirmed gone on a
// re-check) exited and is dropped from the inventory; every other read error,
// and a vanish-shaped error whose entry is still present or cannot be
// re-checked, stays a read failure listed in the PID directory.
func TestPMRV2011SweepDropsOnlyConfirmedVanishedPIDs(t *testing.T) {
	gone := fmt.Errorf("procfs read: %w", fs.ErrNotExist)
	exited := fmt.Errorf("procfs read: %w", syscall.ESRCH)
	denied := fmt.Errorf("procfs read: %w", fs.ErrPermission)
	reads := map[string]error{"20": gone, "30": exited, "40": denied, "50": gone, "60": gone}
	rechecks := map[string]error{"20": fs.ErrNotExist, "30": syscall.ENOENT, "40": fs.ErrNotExist, "50": nil, "60": fs.ErrPermission}
	readStat := func(name string) ([]byte, error) {
		if err := reads[name]; err != nil {
			return nil, err
		}
		return []byte(name + " (p) S 1"), nil
	}
	checked := []string{}
	lstat := func(name string) error {
		checked = append(checked, name)
		return rechecks[name]
	}
	var s Sweep
	if err := sweepNames(context.Background(), &s, []string{"10", "20", "self", "30", "40", "50", "60", "70"}, 16, readStat, lstat); err != nil {
		t.Fatal(err)
	}
	if got, want := string(s.PIDDirectory), "10\n40\n50\n60\n70\n"; got != want {
		t.Fatalf("PID directory = %q, want %q", got, want)
	}
	pids := []int{}
	for _, process := range s.Processes {
		pids = append(pids, process.PID)
	}
	if !slices.Equal(pids, []int{10, 70}) {
		t.Fatalf("swept PIDs = %v, want [10 70]", pids)
	}
	if want := []string{"40: stat unreadable", "50: stat unreadable", "60: stat unreadable"}; !slices.Equal(s.ReadFailures, want) {
		t.Fatalf("read failures = %q, want %q", s.ReadFailures, want)
	}
	if want := []string{"20", "30", "50", "60"}; !slices.Equal(checked, want) {
		t.Fatalf("re-checked %v, want only vanish-shaped reads %v", checked, want)
	}
}

// PMR-V2-011: a dropped PID does not count toward the process bound, and the
// bound still refuses an inventory over it.
func TestPMRV2011SweepBoundCountsOnlyListedEntries(t *testing.T) {
	readStat := func(name string) ([]byte, error) {
		if name == "2" {
			return nil, fmt.Errorf("procfs read: %w", syscall.ESRCH)
		}
		return []byte(name + " (p) S 1"), nil
	}
	lstat := func(string) error { return fs.ErrNotExist }
	var s Sweep
	if err := sweepNames(context.Background(), &s, []string{"1", "2", "3"}, 2, readStat, lstat); err != nil {
		t.Fatalf("bound 2 over two listed PIDs: %v", err)
	}
	s = Sweep{}
	if err := sweepNames(context.Background(), &s, []string{"1", "3", "2"}, 2, readStat, lstat); err != nil {
		t.Fatalf("bound 2 with a trailing vanished PID: %v", err)
	}
	s = Sweep{}
	if err := sweepNames(context.Background(), &s, []string{"1", "2", "3", "4"}, 2, readStat, lstat); err == nil || !errors.Is(err, errSweepBound) {
		t.Fatalf("bound 2 over three listed PIDs = %v, want the bound refusal", err)
	}
}
