package delta

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeltaCaptureBoundAndTransport(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("safe capture unavailable")
	}
	root := t.TempDir()
	put(t, root, "record", "1234")
	data, err := capture(root, "record", 4)
	if err != nil || !bytes.Equal(data, []byte("1234")) {
		t.Fatalf("exact bound %q %v", data, err)
	}
	if _, err := capture(root, "record", 3); err == nil {
		t.Fatal("size bound")
	}
	if err := os.Symlink("record", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link", ".", "mcp:secret", "command:echo secret", "https://example.invalid", "x\x00y"} {
		if _, err := capture(root, name, 10); err == nil {
			t.Fatalf("unsafe input %q admitted", name)
		}
	}
}
