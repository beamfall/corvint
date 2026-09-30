//go:build darwin || linux

package goplsclient

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// GCS-V0-002
func TestCrashAndSessionCancellationRetireDescendant(t *testing.T) {
	for _, mode := range []string{"crash", "wait-cancel"} {
		t.Run(mode, func(t *testing.T) {
			cfg, _ := fakeConfig(t, mode)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c, err := Start(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); <-c.Done() }()
			uri := fileURI(filepath.Join(cfg.Root, "main.go"))
			if err = c.Open(ctx, uri); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() { _, e := c.Definition(ctx, uri, Position{2, 8}); result <- e }()
			deadline := time.Now().Add(3 * time.Second)
			var pid int
			for time.Now().Before(deadline) {
				b, e := os.ReadFile(filepath.Join(cfg.Root, "child-pid"))
				if e == nil {
					pid, _ = strconv.Atoi(string(b))
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("child witness absent")
			}
			if mode == "wait-cancel" {
				cancel()
			}
			select {
			case <-c.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("session not retired")
			}
			if !c.CleanupProven() {
				t.Fatal("process group cleanup unproven")
			}
			if e := <-result; e == nil {
				t.Fatal("failed session returned success")
			}
			if err = syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("descendant still present: %v", err)
			}
		})
	}
}
