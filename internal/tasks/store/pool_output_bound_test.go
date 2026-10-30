//go:build unix

package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-033: combined pool output is capped at 64 KiB, independently of gates.
func TestPoolOutputBound(t *testing.T) {
	for _, n := range []int{64 * wire.KiB, 64*wire.KiB + 1} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			root := t.TempDir()
			data := bytes.Repeat([]byte("x"), n)
			path := filepath.Join(root, "output")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			// A single process isolates the byte boundary from descendant cleanup.
			def := &intent.PoolCommand{Argv: []string{"/bin/cat", path}, TimeoutSeconds: "5"}
			class, clean, digest := executePoolGuarded(context.Background(), def, root, nil, nil)
			want := "EXIT_ZERO"
			if n > 64*wire.KiB {
				want = "OUTPUT_LIMIT"
				data = data[:64*wire.KiB]
			}
			if class != want || !clean || digest != wire.Sum(data) {
				t.Fatalf("bytes=%d class=%s want=%s clean=%v digest matches capped output=%v", n, class, want, clean, digest == wire.Sum(data))
			}
		})
	}
}

func TestPoolOutputBoundGateUnchanged(t *testing.T) {
	c := &cappedOutput{kill: func() { t.Fatal("gate killed below 16 MiB") }}
	data := bytes.Repeat([]byte("g"), 64*wire.KiB+1)
	if _, err := c.Write(data); err != nil || c.overflow || c.buf.Len() != len(data) || wire.MaxGateOutputBytes != 16*wire.MiB {
		t.Fatalf("gate output cap changed: len=%d overflow=%v err=%v", c.buf.Len(), c.overflow, err)
	}
}

// Overflow must retire the owned group, including a live child holding pipes.
func TestPoolOutputBoundRetiresGroup(t *testing.T) {
	root := t.TempDir()
	path, pids := filepath.Join(root, "output"), filepath.Join(root, "pids")
	data := bytes.Repeat([]byte("x"), 64*wire.KiB+1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	clean := false
	t.Cleanup(func() {
		if clean {
			return
		}
		// Retire this fixture's group even when observation cannot prove cleanup.
		raw, err := os.ReadFile(pids)
		if err != nil {
			return
		}
		fields := strings.Fields(string(raw))
		if len(fields) != 2 {
			return
		}
		if group, err := strconv.Atoi(fields[0]); err == nil && group > 1 {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
	})
	def := &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", `sleep 60 & child=$!; echo "$$ $child" > "$1"; cat "$2"; wait "$child"`, "sh", pids, path}, TimeoutSeconds: "5"}
	class, clean, digest := executePoolGuarded(ctx, def, root, []string{"PATH=/usr/bin:/bin"}, nil)
	if class != "OUTPUT_LIMIT" || !clean || digest != wire.Sum(data[:64*wire.KiB]) {
		t.Fatalf("overflow retirement: class=%s clean=%v cappedDigest=%v", class, clean, digest == wire.Sum(data[:64*wire.KiB]))
	}
	raw, err := os.ReadFile(pids)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		t.Fatalf("child start not reached: %q", raw)
	}
	group, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	if live, err := poolGroupLive(group); err != nil || live {
		t.Fatalf("owned group remains: live=%v err=%v", live, err)
	}
}
