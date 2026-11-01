package lspevidence

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635). Run as a fake gopls
// (testsupport.FakeGoplsExecutable), it serves the scripted session instead.
func TestMain(m *testing.M) {
	testsupport.ServeFakeGoplsIfRequested()
	testsupport.IsolateGit()
	os.Exit(m.Run())
}
