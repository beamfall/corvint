package flowdocs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// DMF-V1-002, DMF-V1-007: an output directory that cannot be read because of an inaccessible
// parent is an I/O error, not a flow-docs refusal, so a composing check fails instead of reporting
// NOT_CHECKED.
func TestDMFV1007UnreadableOutputDirectoryIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced for this user")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	var result Check
	err := compareRecordedOutput("", filepath.Join(locked, "out"), "", nil, &result)
	if err == nil || Refused(err) {
		t.Fatalf("want a non-refusal I/O error, got %v", err)
	}
}
