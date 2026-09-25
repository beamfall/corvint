package fixture

import (
	"os"
	"testing"
)

// Main runs a test binary with its temp dirs on a filesystem the §5.1
// qualification accepts, so store tests exercise the real primitives on
// hosts whose default temp dir is refused. An unsupported host still fails.
func Main(m *testing.M) {
	if dir := qualifiedTempDir(); dir != "" {
		os.Setenv("TMPDIR", dir)
	}
	os.Exit(m.Run())
}
