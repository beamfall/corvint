package w

import (
	"os"
	"testing"

	"example.com/fix/lib"
)

func TestWrite(t *testing.T) { _ = os.WriteFile("state", []byte{byte(lib.A())}, 0o600) }

func TestOther(t *testing.T) { _ = lib.B() }
