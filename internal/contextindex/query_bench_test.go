package contextindex

import (
	"os"
	"path/filepath"
	"testing"
)

func benchmarkWriteFile(b testing.TB, root, relative, content string) {
	b.Helper()
	file := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
