package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// corvintShapedRepository is a committed repository shaped like this one for the
// Claude Code prompt hook: the Corvint module line, a CLAUDE.md that imports
// AGENTS.md, a spec with a requirement ID and one source file.
func corvintShapedRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                     corvintModuleLine + "\n\ngo 1.27\n",
		"CLAUDE.md":                  "@AGENTS.md\n",
		"AGENTS.md":                  "# Corvint agent contract\n\nRead only the task's applicable context route.\n",
		"docs/specs/hooks-v0.md":     "# Hooks\n\n## Requirements\n\n- `HKR-V0-001`: The prompt hook resolves named anchors.\n",
		"internal/hooks/envelope.go": "package hooks\n\n// Envelope renders the prompt hook envelope.\nfunc Envelope() string { return \"\" }\n",
	}
	for relative, contents := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"add", "."}, {"commit", "-qm", "corvint-shaped fixture"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

// claudeHookContext runs the real Claude Code hook entrypoint for one event and
// returns its additionalContext ("" when the hook stays silent).
func claudeHookContext(t *testing.T, root, event string, fields map[string]any) string {
	t.Helper()
	native := map[string]string{"user-prompt": "UserPromptSubmit", "session-start": "SessionStart"}[event]
	input := map[string]any{
		"session_id": "native-session", "transcript_path": "/Users/dev/.claude/projects/p/native-session.jsonl",
		"cwd": root, "permission_mode": "default", "hook_event_name": native,
	}
	for key, value := range fields {
		input[key] = value
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	ctx := adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
	if status := runHostAdapter(ctx, []string{"claude-code", event}, bytes.NewReader(payload), &stdout); status != 0 {
		t.Fatalf("%s: status=%d output=%s", event, status, &stdout)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		return ""
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("%s: output %s: %v", event, &stdout, err)
	}
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	text, _ := hook["additionalContext"].(string)
	return text
}

// hookEnvelopeBytes bounds one Claude Code envelope that names a single
// evidence row: the trusted frame, the untrusted-data markers and one row.
const hookEnvelopeBytes = 1024

// V1-2762 (LCP-V0-039, AHI-216, AHI-047): on a repository shaped like this one,
// whose CLAUDE.md imports AGENTS.md, a prompt that names an existing path or a
// requirement ID among the prose noise of real lane prompts resolves to that
// row alone with no repeated governance, an anchorless prompt is silent, and a
// subagent SessionStart that carries agent_id gets no workflow argv or
// governance, each inside a pinned byte bound.
func TestV12762CorvintShapedPromptEnvelopesResolveOrAbstain(t *testing.T) {
	t.Parallel()
	root := corvintShapedRepository(t)
	for _, test := range []struct{ prompt, path, relation string }{
		{"Fix internal/hooks/envelope.go after origin/main moved to go1.27.3; see https://example.com/x and the 7.7% audit.", "internal/hooks/envelope.go", "explicit-path"},
		{"Repair HKR-V0-001 per the 2026-10-26 value audit, e.g. pass/fail on linux/amd64.", "docs/specs/hooks-v0.md", "requirement-definition"},
	} {
		text := claudeHookContext(t, root, "user-prompt", map[string]any{"prompt": test.prompt})
		envelope := hookEnvelopeJSON(t, text)
		rows, _ := envelope["task_evidence"].([]any)
		var row map[string]any
		if len(rows) == 1 {
			row, _ = rows[0].(map[string]any)
		}
		if row["path"] != test.path || row["relation"] != test.relation {
			t.Fatalf("%q: task_evidence %v, want one %s row for %s", test.prompt, rows, test.relation, test.path)
		}
		// An unresolved packet projects its resolution (hookUnresolved); a resolved one does not.
		for _, key := range []string{"resolution", "governance"} {
			if _, ok := envelope[key]; ok {
				t.Fatalf("%q: envelope carries %s: %s", test.prompt, key, text)
			}
		}
		if len(text) > hookEnvelopeBytes {
			t.Fatalf("%q: envelope is %d bytes, bound %d", test.prompt, len(text), hookEnvelopeBytes)
		}
	}
	for _, prompt := range []string{
		"can you make it faster now, it is NOT_RUN and pointless",
		"rebase on origin/main and rerun the 28/28 pass/fail check for go1.27.3",
	} {
		if text := claudeHookContext(t, root, "user-prompt", map[string]any{"prompt": prompt}); text != "" {
			t.Fatalf("%q: anchorless prompt was not silent: %s", prompt, text)
		}
	}
	main := claudeHookContext(t, root, "session-start", map[string]any{"source": "startup"})
	if !strings.Contains(main, "Corvint local workflow argv") {
		t.Fatalf("main-thread SessionStart lost the workflow argv: %s", main)
	}
	sub := claudeHookContext(t, root, "session-start", map[string]any{"source": "startup", "agent_id": "a1", "agent_type": "general-purpose"})
	if strings.Contains(sub, "Corvint local workflow argv") || strings.Contains(sub, "governance") || len(sub) > hookEnvelopeBytes || len(sub) >= len(main) {
		t.Fatalf("subagent SessionStart (%d bytes, main %d) repeats content: %s", len(sub), len(main), sub)
	}
}

// hookEnvelopeJSON returns the JSON object inside the untrusted-data markers.
func hookEnvelopeJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	begin := strings.Index(text, "\n{")
	end := strings.LastIndex(text, "}\n")
	if !strings.HasPrefix(text, untrustedDataPrefix) || begin < 0 || end < begin {
		t.Fatalf("not a framed envelope: %q", text)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(text[begin+1:end+1]), &envelope); err != nil {
		t.Fatalf("envelope %q: %v", text, err)
	}
	return envelope
}
