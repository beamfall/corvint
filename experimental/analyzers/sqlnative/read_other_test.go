//go:build !darwin && !linux && !windows

package sqlnative

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// PRS-V1-004: unsupported platforms cannot acquire SQL input, even when it exists.
func TestReadNoFollowOtherRefusesRegularFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.sql"), []byte("SELECT 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	value, err := ReadNoFollow(context.Background(), root, "input.sql", 64)
	if value != nil || err == nil || err.Error() != "unsupported" {
		t.Fatalf("unsupported acquisition returned value=%q err=%v", value, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := ReadNoFollow(ctx, root, "input.sql", 64); value != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled acquisition returned value=%q err=%v", value, err)
	}
	if value, err := ReadNoFollow(nil, root, "input.sql", 64); value != nil || err == nil || err.Error() != "nil context" {
		t.Fatalf("nil-context acquisition returned value=%q err=%v", value, err)
	}
}
