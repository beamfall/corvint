package r

import (
	"os"
	"testing"

	"example.com/fix/lib"
)

var result int

func TestMain(m *testing.M) {
	if len(os.Args) > 0 {
		result = lib.A()
		os.Exit(m.Run())
	}
	os.Exit(m.Run())
}

func TestResult(t *testing.T) { _ = result }
