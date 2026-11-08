package t4

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"example.com/fix/lib"
)

type box struct{ N int }

var shared any = &box{}

func TestIfaceWrite(t *testing.T) { shared.(*box).N = lib.A() }

func TestIfaceRead(t *testing.T) { _ = shared.(*box).N }

func sharedPath(_ string) string { return filepath.Join(os.TempDir(), "typedreach-t4") }

func TestFakePrivateWrite(t *testing.T) {
	p := sharedPath(t.TempDir())
	_ = os.WriteFile(p, []byte{byte(lib.A())}, 0o600)
}

func TestFileReader(t *testing.T) { _, _ = os.ReadFile(sharedPath("")) }

var output, outputErr = os.Create(filepath.Join(os.TempDir(), "typedreach-t4-handle"))

func TestHandleWrite(t *testing.T) { _, _ = output.Write([]byte{byte(lib.A())}) }

func TestHTTPWrite(t *testing.T) {
	if lib.A() != 0 {
		http.DefaultClient = nil
	}
}

func TestHTTPRead(t *testing.T) { _, _ = http.Get("unsupported-scheme://host") }

func TestListenA(t *testing.T) {
	if lib.A() == 0 {
		return
	}
	if l, err := net.Listen("tcp", "127.0.0.1:48637"); err == nil {
		_ = l
	}
}

func TestListenB(t *testing.T) {
	if l, err := net.Listen("tcp", "127.0.0.1:48637"); err == nil {
		l.Close()
	}
}

func TestPrivateJoin(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "x"), nil, 0o600)
}

func TestPure(t *testing.T) { _ = lib.B() }
