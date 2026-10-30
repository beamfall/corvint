//go:build darwin || linux

package jstestprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// V1-3020: identity collection runs under the operation context. A node
// --version wrapper that stalls returns an error within the timeout, one that
// floods output leaves the version unobserved, and file hashing stops on a
// cancelled context.
func TestIdentityCollectionIsBoundedByTheOperation(t *testing.T) {
	dir := t.TempDir()
	write := func(sub, script string) string {
		bin := filepath.Join(dir, sub)
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(bin, "node")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	stalled := write("stalled", "exec sleep 60")
	flooding := write("flooding", "yes v24.11.1 | head -c 1048576")
	t.Setenv("npm_node_execpath", "")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := Config{}.identity(ctx, []string{stalled, "cli.js"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stalled identity = %v, want the operation deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("stalled identity took %v", elapsed)
	}

	if got := nodeVersionFor(context.Background(), []string{flooding}); got != "" {
		t.Fatalf("flooding version = %d bytes, want unobserved", len(got))
	}

	config := filepath.Join(dir, "config.cjs")
	if err := os.WriteFile(config, []byte(strings.Repeat("x", 3*digestChunk)), 0o644); err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := digestFileContext(cancelled, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled digest = %v, want context.Canceled", err)
	}
	whole, err := digestFileContext(context.Background(), config)
	if want, _ := digestFile(config); err != nil || whole != want || whole == "" {
		t.Fatalf("digest = %q %v, want %q", whole, err, want)
	}
}
