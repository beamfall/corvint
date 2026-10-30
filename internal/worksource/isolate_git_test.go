package worksource

import (
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2635) and keeps acquisition
// scratch under the test temporary root instead of the fixed /tmp (V1-2768).
func TestMain(m *testing.M) {
	testsupport.IsolateGit()
	removeRunParent, err := testsupport.PrivateWorkRunParent()
	if err != nil {
		fmt.Fprintln(os.Stderr, "work run parent:", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := removeRunParent(); err != nil {
		fmt.Fprintln(os.Stderr, "remove work run parent:", err)
		code = 1
	}
	os.Exit(code)
}
