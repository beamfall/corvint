//go:build !(darwin || linux)

package taskman

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635); the darwin and linux
// TestMain in cem_verdict_portable_unix_test.go does the same.
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	os.Exit(m.Run())
}
