package extevidence

import (
	"os"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635). A provider helper
// re-run of this binary keeps the exact environment the transport gave it,
// which the EEP-TR-003 containment check inspects.
func TestMain(m *testing.M) {
	if !slices.Contains(os.Args, helperMarker) {
		testsupport.IsolateGit()
	}
	os.Exit(m.Run())
}
