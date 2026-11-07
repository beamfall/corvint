package s

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"example.com/fix/lib"
)

var once sync.Once

func TestEnvWrite(t *testing.T) { t.Setenv("X", strconv.Itoa(lib.A())); _ = os.Setenv("Y", "1") }

func TestEnvRead(t *testing.T) { get := os.Getenv; _ = get("Y") }

func TestStructWrite(t *testing.T) { alias := lib.Shared; *alias.P = lib.A() }

func TestStructRead(t *testing.T) { _ = *lib.Shared.P }

func TestOnceWrite(t *testing.T) {
	if lib.A() == 1 {
		do := once.Do
		do(func() {})
	}
}

func TestOnceRead(t *testing.T) { once.Do(func() {}) }

func TestFileWrite(t *testing.T) {
	path := "typedreach-" + strconv.Itoa(lib.A())
	_ = os.WriteFile(path, nil, 0o600)
}

func TestFileRead(t *testing.T) { _, _ = os.ReadFile("typedreach-1-x" + strconv.Itoa(2)) }

func TestPrivateFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(dir+"/x", []byte{byte(lib.B())}, 0o600)
}
