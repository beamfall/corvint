package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderKitChecker(t *testing.T) {
	revision := strings.Repeat("a", 40)
	data := []byte(`{"schema":"external-evidence-provider/0","provider":{"id":"kit-example","revision":"0.1.0"},"repository":{"revision":"` + revision + `"},"entities":[],"relations":[]}`)
	file := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--file", file, "--profile", "external-evidence-provider/0", "--provider-id", "kit-example", "--provider-version", "0.1.0", "--revision", revision}
	var out bytes.Buffer
	if err := run(context.Background(), args, &out); err != nil || !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("check: %v %s", err, out.Bytes())
	}
	out.Reset()
	if err := run(context.Background(), append(args, "--provider-version", "0.2.0"), &out); err == nil || out.Len() != 0 {
		t.Fatal("mismatch emitted bytes")
	}
	if err := run(context.Background(), append(args, "--command", `["/bin/false"]`), &out); err == nil || out.Len() != 0 {
		t.Fatal("mixed sources accepted")
	}
}
