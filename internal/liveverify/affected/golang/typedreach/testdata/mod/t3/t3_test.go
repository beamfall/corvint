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

const CrossConst = lib.CB + 1

type CrossArray [CrossConst]int

func TestCrossConst(t *testing.T) { _ = CrossConst }

func TestCrossArray(t *testing.T) { var a CrossArray; _ = len(a) }

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

func TestIoutilRead(t *testing.T) { _, _ = ioutil.ReadFile(shared()) }

func TestIoutilReadDir(t *testing.T) { _, _ = ioutil.ReadDir(shared()) }

func TestIoutilTempFile(t *testing.T) { _, _ = ioutil.TempFile(shared(), "entry-") }

func TestIoutilTempDir(t *testing.T) { _, _ = ioutil.TempDir(shared(), "entry-") }

func TestIoutilTempFileValue(t *testing.T) { f := ioutil.TempFile; _, _ = f("", "entry-") }

func TestOSCreateTemp(t *testing.T) { _, _ = os.CreateTemp(shared(), "entry-") }

func TestOSMkdirTemp(t *testing.T) { _, _ = os.MkdirTemp(shared(), "entry-") }

func TestPrivateTemp(t *testing.T) { _, _ = ioutil.TempDir(t.TempDir(), "entry-") }

func TestCopyFS(t *testing.T) { _ = os.CopyFS(shared(), os.DirFS(shared())) }

func TestOpenInRoot(t *testing.T) { _, _ = os.OpenInRoot(shared(), "entry") }

func TestRootRead(t *testing.T) { var r *os.Root; _, _ = r.ReadFile("entry") }

func TestRootWrite(t *testing.T) { var r *os.Root; _ = r.WriteFile("entry", nil, 0o600) }

func TestRoot(t *testing.T) {
	if root, err := os.OpenRoot(os.TempDir()); err == nil {
		_ = root.Close()
	}
}

func TestStdoutA(t *testing.T) { _ = lib.A(); os.Stdout = os.Stderr }

func TestPrint(t *testing.T) { fmt.Println("x") }

func TestPlain(t *testing.T) { _ = lib.B() }
