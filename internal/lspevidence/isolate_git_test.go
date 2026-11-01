package lspevidence

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635). Run as a fake gopls
// (LSPEVIDENCE_FAKE_SERVE), it serves the scripted session instead.
func TestMain(m *testing.M) {
	if os.Getenv("LSPEVIDENCE_FAKE_SERVE") != "" {
		serveFakeGopls()
		os.Exit(0)
	}
	testsupport.IsolateGit()
	os.Exit(m.Run())
}
