package p

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"example.com/fix/lib"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		lib.B()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestA(t *testing.T) { _ = lib.A() }

func TestB(t *testing.T) { _ = lib.B() }

func TestStringer(t *testing.T) { _ = fmt.Sprint(lib.T{}) }

func TestRegistry(t *testing.T) { _ = lib.Registry["a"]() }

func TestBump(t *testing.T) { lib.Bump(); _ = lib.A() }

func TestRead(t *testing.T) { _ = lib.Read() }

func TestExec(t *testing.T) { _ = lib.Run("x") }

func TestGit(t *testing.T) { _ = exec.Command("git", "version").Run() }
