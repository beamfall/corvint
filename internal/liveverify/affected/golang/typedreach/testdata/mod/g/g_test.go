package g

import (
	"os"
	"testing"

	"example.com/fix/lib"
)

func TestMain(m *testing.M) {
	if lib.Guard() && len(os.Args) > 1 && os.Args[1] == "child" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestState(t *testing.T) { _ = lib.State }
