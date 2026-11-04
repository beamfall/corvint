package t3

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"example.com/fix/lib"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		lib.Child()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func shared() string { return filepath.Join(os.TempDir(), "typedreach-shared") }

func TestConst(t *testing.T) {
	if lib.CB != 2 {
		t.Fatal(lib.CB)
	}
}

func TestArr(t *testing.T) { var a lib.Arr; _ = len(a) }

func TestSelf(t *testing.T) {
	c := new(exec.Cmd)
	c.Path, c.Args = os.Args[0], []string{os.Args[0], "child"}
	_ = c.Run()
}

func TestSpawnA(t *testing.T) {
	_ = lib.A()
	var c exec.Cmd
	c.Path = os.Args[0]
	_ = c.Run()
}

func TestReadShared(t *testing.T) { _, _ = os.ReadFile(shared()) }

func TestIoutil(t *testing.T) { _ = ioutil.WriteFile(shared(), nil, 0o600) }

func TestRoot(t *testing.T) {
	if root, err := os.OpenRoot(os.TempDir()); err == nil {
		_ = root.Close()
	}
}

func TestStdoutA(t *testing.T) { _ = lib.A(); os.Stdout = os.Stderr }

func TestPrint(t *testing.T) { fmt.Println("x") }

func TestPlain(t *testing.T) { _ = lib.B() }
