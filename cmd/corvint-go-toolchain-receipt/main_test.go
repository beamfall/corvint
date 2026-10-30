package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEmitsReceipt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("go1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output, errors bytes.Buffer
	if code := run([]string{root}, &output, &errors); code != 0 {
		t.Fatalf("code=%d errors=%s", code, errors.String())
	}
	if !strings.Contains(output.String(), `"Entries":1`) || !strings.Contains(output.String(), `"SHA256":"`) {
		t.Fatalf("output=%s", output.String())
	}
}
