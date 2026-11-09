//go:build !darwin && !linux

package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// CEM-PILOT-033: without advisory locks, Lock refuses publish-failed with the
// platform refusal as the message's last clause, and creates nothing.
func TestLockRefusesUpFrontWithoutAdvisoryLocks(t *testing.T) {
	dir := t.TempDir()
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := root.Lock(".corvint/change.cem.json")
	if unlock != nil || cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("Lock = (%v, %v), want a publish-failed refusal", unlock != nil, err)
	}
	if want := ": " + LockRefusal().Error(); !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("refusal %q does not end with %q", err, want)
	}
	if _, statErr := os.Lstat(filepath.Join(dir, ".corvint")); !os.IsNotExist(statErr) {
		t.Fatalf("refusal created the lock directory: %v", statErr)
	}
}
