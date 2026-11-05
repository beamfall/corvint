//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package supervisor

import (
	"errors"
	"testing"
)

func TestATRV07202_OwnerIdentityReadFailures(t *testing.T) {
	transient := errors.New("transient identity read")
	n := 0
	for _, step := range []struct {
		identity string
		err      error
		lost     bool
		errors   int
	}{
		{"", transient, false, 1},
		{"same", nil, false, 0},
		{"", transient, false, 1},
		{"", transient, false, 2},
		{"", transient, true, 3},
		{"same", nil, false, 0},
		{"", nil, true, 0},
		{"replacement", nil, true, 0},
	} {
		if lost := ownerIdentityLost("same", step.identity, step.err, &n); lost != step.lost || n != step.errors {
			t.Fatalf("read %q/%v: lost=%v errors=%d, want %v/%d", step.identity, step.err, lost, n, step.lost, step.errors)
		}
	}
}

func TestATRV07202_ReadablePID1Owner(t *testing.T) {
	identity, err := leaderOwnerIdentity(1, func(pid int) (string, error) {
		if pid != 1 {
			t.Fatalf("PID %d", pid)
		}
		return "init-start-identity", nil
	})
	if err != nil || identity != "init-start-identity" {
		t.Fatalf("PID 1: %q %v", identity, err)
	}
	for _, tc := range []struct {
		pid      int
		identity string
		err      error
	}{
		{0, "readable", nil}, {1, "", nil}, {1, "", errors.New("unreadable")},
	} {
		if _, err := leaderOwnerIdentity(tc.pid, func(int) (string, error) { return tc.identity, tc.err }); err == nil {
			t.Fatalf("unavailable owner accepted: %+v", tc)
		}
	}
}
