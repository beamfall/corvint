package cli

import (
	"os"
	"testing"
)

// TestMain runs the tests under the platform's process-group checks
// (testMain) and removes the shared compiled executable afterwards.
func TestMain(m *testing.M) {
	code := testMain(m)
	removeCompiledTasks()
	os.Exit(code)
}
