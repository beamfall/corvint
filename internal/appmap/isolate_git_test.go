package appmap

import (
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635).
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	var err error
	fixtureSeeds.dir, err = os.MkdirTemp("", "appmap-fixture-seeds-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(fixtureSeeds.dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
