// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// PRB-V0-8000: opt-in wall-stage profiling on V1-2240's exact 3,000 tracked,
// 700 modified and 767 untracked file shape. Built-in go test -trace captures
// the named regions without adding a runtime knob or changing a receipt.
func TestPRBV08000HookStageProfile(t *testing.T) {
	t.Run("PRB-V0-8000 profile", func(t *testing.T) {
		parent := os.Getenv("CORVINT_PERFCORPUS_HOOK_DIR")
		if parent == "" {
			t.Skip("set CORVINT_PERFCORPUS_HOOK_DIR and use go test -trace")
		}
		if !trace.IsEnabled() {
			t.Fatal("stage profile requires go test -trace=FILE")
		}
		root := filepath.Join(parent, "repo")
		if err := os.MkdirAll(parent, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		write := func(name, text string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
		}
		write("go.mod", "module example.test/hookprofile\n\ngo 1.27.0\n")
		for i := range 2999 {
			write(fmt.Sprintf("f%04d.go", i), fmt.Sprintf("package p\nfunc Ledger%d() int { return %d }\n", i, i))
		}
		cemGit(t, root, "init", "-q")
		cemGit(t, root, "add", ".")
		cemGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
		for i := range 700 {
			write(fmt.Sprintf("f%04d.go", i), fmt.Sprintf("package p\nfunc Ledger%d() int { return %d }\n", i, i+1))
		}
		for i := range 767 {
			write(fmt.Sprintf("untracked%04d.go", i), "package p\n")
		}
		t.Setenv("CLAUDE_PROJECT_DIR", root)
		t.Setenv("CORVINT_INDEX_STORE", filepath.Join(parent, "test-index"))
		index, err := contextindex.Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = contextindex.WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		index.Release()
		if count := strings.Count(cemGit(t, root, "status", "--porcelain=v1", "-z", "--untracked-files=all"), "\x00"); count != 1467 {
			t.Fatalf("dirty paths %d != 1467", count)
		}
		load := func() {
			if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
				t.Logf("host_load=%s", strings.TrimSpace(string(raw)))
			} else {
				t.Log("host_load=NOT_OBSERVED")
			}
		}
		load()
		for _, event := range []string{"user-prompt", "post-tool"} {
			payload := map[string]any{"session_id": "perfcorpus-hook", "prompt": "Ledger123", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, "f0123.go")}}
			raw, _ := json.Marshal(payload)
			var first []byte
			for i := range 20 {
				ctx, task := trace.NewTask(context.Background(), "perfcorpus."+event)
				ctx, cancel := adapterHostKillContext(ctx, []string{"adapter", "claude-code", event}, time.Now())
				var out bytes.Buffer
				start := time.Now()
				code := runHostAdapter(ctx, []string{"claude-code", event}, bytes.NewReader(raw), &out)
				elapsed := time.Since(start)
				cancel()
				task.End()
				if code != 0 {
					t.Fatalf("%s: code %d: %s", event, code, out.Bytes())
				}
				if i == 0 {
					first = bytes.Clone(out.Bytes())
				} else if !bytes.Equal(first, out.Bytes()) {
					t.Fatalf("%s output changed", event)
				}
				t.Logf("sample event=%s n=%d elapsed_ns=%d stdout_sha256=%s", event, i, elapsed.Nanoseconds(), contextDigestForProfile(out.Bytes()))
			}
		}
		load()
	})
}

func contextDigestForProfile(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
