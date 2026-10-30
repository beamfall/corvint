package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// AHI-043: for an unchanged tree, worktree, hook input and session, the Claude SessionStart and
// UserPromptSubmit stdout is byte-identical across invocations whatever the wall clock reads, so
// an injected packet never busts the host's prompt cache by itself. synctest's fake clock moves
// 61 s and then 25 h between invocations, crossing minute, hour and day boundaries, without the
// test waiting for it.
func TestAHI043ClaudeContextPacketsAreByteStableAcrossTime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":          "# Agents\n\nRun `go test ./...` before every change to stability.go.\n",
		"stability.go":       "package stability\n\n// Stable returns the packet stability marker.\nfunc Stable() string { return \"stable\" }\n",
		"docs/specs/stab.md": "# Stability spec\n\nIntent status: accepted\n\n- STAB-001: Stable MUST return the same marker.\n",
		"stability_test.go":  "package stability\n\nimport \"testing\"\n\nfunc TestStable(t *testing.T) { _ = Stable() }\n",
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	// An unchanged dirty worktree is part of the unchanged state, not a reason to vary.
	if err := os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package stability\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		event   string
		payload map[string]any
	}{
		{"session-start", map[string]any{"session_id": "stable-session", "source": "startup"}},
		{"session-start", map[string]any{"session_id": "stable-session", "source": "resume"}},
		{"user-prompt", map[string]any{"session_id": "stable-session", "prompt": "change Stable in stability.go and keep STAB-001 passing"}},
	}
	for _, test := range cases {
		input, err := json.Marshal(test.payload)
		if err != nil {
			t.Fatal(err)
		}
		synctest.Test(t, func(t *testing.T) {
			ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root})
			invoke := func() []byte {
				var stdout bytes.Buffer
				if code := runHostAdapter(ctx, []string{"claude-code", test.event}, bytes.NewReader(input), &stdout); code != 0 {
					t.Fatalf("%s exited %d", test.event, code)
				}
				return stdout.Bytes()
			}
			label := test.event
			if source, ok := test.payload["source"].(string); ok {
				label += " " + source
			}
			first := invoke()
			for _, gap := range []time.Duration{61 * time.Second, 25 * time.Hour} {
				time.Sleep(gap)
				if later := invoke(); !bytes.Equal(first, later) {
					t.Fatalf("%s packet changed after %s on unchanged state:\nfirst %s\nlater %s", label, gap, first, later)
				}
			}
			// Two identical deadline or stale-index fallbacks would compare equal without proving the
			// packet is stable, so each case must carry the full repository envelope.
			text := string(first)
			if !strings.Contains(text, "BEGIN CORVINT REPOSITORY DATA") || strings.Contains(text, "dogfood-event-deadline") || strings.Contains(text, "index-snapshot-stale") {
				t.Fatalf("%s produced no full repository packet, so equality proves nothing: %s", label, first)
			}
			t.Logf("%s: %d identical bytes", label, len(first))
		})
	}
}
