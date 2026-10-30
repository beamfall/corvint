//go:build darwin || linux

package publish

import "testing"

// CEM-PILOT-033: a platform with advisory locks has no lock refusal.
func TestLockRefusalIsNilWithAdvisoryLocks(t *testing.T) {
	if err := LockRefusal(); err != nil {
		t.Fatalf("LockRefusal() = %v on a locking platform", err)
	}
}
