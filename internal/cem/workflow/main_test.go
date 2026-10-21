package workflow

import (
	"os"
	"testing"
)

// fixtureTempBase is the temporary directory captured before any test runs,
// so a test that points TMPDIR into its repository cannot relocate the shared
// fixture template.
var fixtureTempBase string

func TestMain(m *testing.M) {
	fixtureTempBase = os.TempDir()
	code := m.Run()
	if repoTemplate.parent != "" {
		_ = os.RemoveAll(repoTemplate.parent)
	}
	os.Exit(code)
}
