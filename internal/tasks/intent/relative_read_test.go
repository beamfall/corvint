//go:build darwin || linux

package intent

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-225 (#900): a relative path that reaches the safe open is a
// path-syntax refusal, never UNSUPPORTED_FILESYSTEM. The file exists
// relative to the test's process directory, so only the open refuses.
func TestCALV0225_RelativeReadIsMalformed(t *testing.T) {
	for _, read := range []func() error{
		func() error { _, err := ReadFile("relative_read_test.go", 1<<20); return err },
		func() error { _, _, err := ReadFileWithInfo("./relative_read_test.go", 1<<20); return err },
	} {
		err := read()
		if wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("relative read: %v (%s)", err, wire.CodeOf(err))
		}
	}
}
