package q

import (
	"fmt"
	"os/exec"
	"testing"

	"example.com/fix/lib"
)

func TestBoxed(t *testing.T) {
	x, _ := lib.Box()
	_ = fmt.Sprint(x)
}

func TestIndirectExec(t *testing.T) {
	command := exec.Command
	_ = command("x").Run()
}

func TestAliasWrite(t *testing.T) {
	alias := lib.Values
	alias[0] = lib.A()
}

func TestAliasRead(t *testing.T) { _ = lib.Values[0] }

func TestLength(t *testing.T) { _ = len(lib.Values) + lib.B() }
