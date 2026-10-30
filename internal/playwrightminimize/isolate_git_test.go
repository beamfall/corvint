package playwrightminimize

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635).
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	os.Exit(m.Run())
}
