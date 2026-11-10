package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635), and builds the corvint-docs app the
// docs tools reach through their envelope onto PATH (decision 0594 item 3).
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	cleanup, err := testsupport.BuildOnPATH("github.com/Beamfall/corvint/cmd/corvint-docs", "corvint-docs")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
