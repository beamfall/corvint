package main

import (
	"strings"
	"testing"
)

// FPK-V0-012, FPK-V0-021, FPK-V0-024, FPK-V0-031: bounded CLI inputs are regular files.
func TestReadBoundedFileRefusesDirectory(t *testing.T) {
	t.Parallel()
	if _, err := readBoundedFile(t.TempDir(), 1); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("readBoundedFile directory error = %v", err)
	}
}
