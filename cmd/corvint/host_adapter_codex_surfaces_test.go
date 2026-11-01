package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

func readCodexPackageFile(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "integrations", "codex"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// AHI-206: the Codex review profile closes both doors the CLI controls: no escalation out of the
// read-only sandbox, and CORVINT_TASKS_READ_ONLY in every shell so corvint-tasks refuses mutations.
// A trusted project config layers above a profile, so the README's invocation repeats exactly the
// same keys as -c overrides, which outrank it.
func TestAHI206CodexReviewProfileClosesBothWriteDoors(t *testing.T) {
	section, values := "", map[string]string{}
	for _, line := range strings.Split(readCodexPackageFile(t, "profiles", "corvint-review.config.toml"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "["):
			section = strings.Trim(line, "[]") + "."
		default:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				t.Fatalf("unparsed profile line %q", line)
			}
			values[section+strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	want := map[string]string{
		"sandbox_mode":    `"read-only"`,
		"approval_policy": `"never"`,
		"shell_environment_policy.set.CORVINT_TASKS_READ_ONLY": `"1"`,
	}
	if len(values) != len(want) {
		t.Fatalf("profile keys %v, want exactly %v", values, want)
	}
	for key, value := range want {
		if values[key] != value {
			t.Fatalf("profile %s = %q, want %s", key, values[key], value)
		}
	}
	readme := readCodexPackageFile(t, "README.md")
	start := strings.Index(readme, "codex exec -p corvint-review -c ")
	if start < 0 {
		t.Fatal("Codex README lacks the overriding review invocation")
	}
	invocation, _, _ := strings.Cut(readme[start:], "```")
	overrides := map[string]string{}
	for _, match := range regexp.MustCompile(`-c '([^=']+)=([^']+)'`).FindAllStringSubmatch(invocation, -1) {
		overrides[match[1]] = match[2]
	}
	if !maps.Equal(overrides, want) {
		t.Fatalf("README review overrides %v, want exactly the profile keys %v", overrides, want)
	}
}

// AHI-207, AHI-373: the plugin declares one read-only `corvint mcp` server that binds the launch directory's
// Git worktree with the legacy protocol, and refuses before exec outside a worktree.
func TestAHI207CodexPluginMCPServerBindsLaunchWorktree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Codex plugin package is qualified on macOS only")
	}
	var manifest struct {
		MCPServers string `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(readCodexPackageFile(t, "plugins", "corvint", ".codex-plugin", "plugin.json")), &manifest); err != nil || manifest.MCPServers != "./.mcp.json" {
		t.Fatalf("plugin.json mcpServers = %q (%v)", manifest.MCPServers, err)
	}
	var declared struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(readCodexPackageFile(t, "plugins", "corvint", ".mcp.json")), &declared); err != nil {
		t.Fatal(err)
	}
	server, ok := declared.MCPServers["corvint"]
	if len(declared.MCPServers) != 1 || !ok || server.Command != "/bin/sh" || len(server.Args) != 2 || server.Args[0] != "-c" {
		t.Fatalf("unexpected MCP declaration %+v", declared.MCPServers)
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CORVINT_TEST_ARGV\"\n"
	if err := execfile.WriteFile(filepath.Join(bin, "corvint"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	launch := func(dir, ceiling string) (string, string, error) {
		argvFile := filepath.Join(t.TempDir(), "argv")
		cmd := exec.Command(server.Command, server.Args...)
		cmd.Dir = dir
		cmd.Env = testEnvironment("PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "CORVINT_TEST_ARGV="+argvFile, "GIT_CEILING_DIRECTORIES="+ceiling)
		output, err := cmd.CombinedOutput()
		argv, _ := os.ReadFile(argvFile)
		return string(argv), string(output), err
	}

	repository := cliGoModuleRepository(t)
	nested := filepath.Join(repository, "pkg")
	argv, output, err := launch(nested, filepath.Dir(repository))
	toplevel, _ := filepath.EvalSymlinks(repository)
	if got := strings.Split(strings.TrimSpace(argv), "\n"); err != nil || !slices.Equal(got, []string{"mcp", "--protocol-version", "2025-11-25", "--root", toplevel}) {
		t.Fatalf("worktree launch argv=%q output=%q err=%v (want root %s)", got, output, err, toplevel)
	}

	outside := t.TempDir()
	argv, output, err = launch(outside, filepath.Dir(outside))
	if exitErr, isExit := err.(*exec.ExitError); !isExit || exitErr.ExitCode() != 2 || argv != "" || !strings.Contains(output, "not inside a Git worktree") {
		t.Fatalf("outside-worktree launch argv=%q output=%q err=%v, want exit 2 before exec", argv, output, err)
	}
	// The root is bound once at startup, so both documents must tell the reader to rebind by
	// starting a new session after a worktree switch.
	for name, phrase := range map[string]string{
		"README.md": "after switching to another worktree, start a new Codex session there to rebind",
		"SKILL.md":  "after moving to another worktree start a new Codex session there",
	} {
		parts := []string{"README.md"}
		if name == "SKILL.md" {
			parts = []string{"plugins", "corvint", "skills", "corvint", "SKILL.md"}
		}
		if !strings.Contains(strings.Join(strings.Fields(readCodexPackageFile(t, parts...)), " "), phrase) {
			t.Fatalf("Codex %s lacks the startup-binding guidance %q", name, phrase)
		}
	}
}

// AHI-208: on Codex 0.162 the plugin registers exactly the four mapped events, each with at least
// one bounded `corvint adapter codex` handler and its applicable matcher; the other native events
// stay unregistered.
func TestAHI208CodexHooksRegisterOnlyMappedEvents(t *testing.T) {
	raw := []byte(readCodexPackageFile(t, "plugins", "corvint", "hooks", "hooks.json"))
	if problem := codexHookRegistrationProblem(raw); problem != "" {
		t.Fatalf("shipped hooks.json: %s", problem)
	}
	handler := map[string]any{"type": "command", "command": "corvint adapter codex", "timeout": 2}
	for name, mutate := range map[string]func(map[string]any){
		"empty Stop":             func(hooks map[string]any) { hooks["Stop"] = []any{} },
		"handlerless Stop":       func(hooks map[string]any) { hooks["Stop"] = []any{map[string]any{"hooks": []any{}}} },
		"missing SessionEnd":     func(hooks map[string]any) { delete(hooks, "SessionEnd") },
		"extra PreToolUse":       func(hooks map[string]any) { hooks["PreToolUse"] = []any{map[string]any{"hooks": []any{handler}}} },
		"narrowed start matcher": func(hooks map[string]any) { hooks["SessionStart"].([]any)[0].(map[string]any)["matcher"] = "startup" },
		"matcher on prompt hook": func(hooks map[string]any) { hooks["UserPromptSubmit"].([]any)[0].(map[string]any)["matcher"] = "x" },
		"unbounded Stop handler": func(hooks map[string]any) {
			hooks["Stop"] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "corvint adapter codex", "timeout": 30}}}}
		},
		"foreign Stop command": func(hooks map[string]any) {
			hooks["Stop"] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "sh -c true", "timeout": 2}}}}
		},
	} {
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		mutate(document["hooks"].(map[string]any))
		mutated, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if codexHookRegistrationProblem(mutated) == "" {
			t.Fatalf("%s: mutated hooks.json was accepted", name)
		}
	}
}

// codexHookRegistrationProblem returns why raw is not the AHI-208 registration, or "".
func codexHookRegistrationProblem(raw []byte) string {
	matchers := map[string]string{"SessionStart": "startup|resume|clear|compact|fork", "UserPromptSubmit": "", "Stop": "", "SessionEnd": "other"}
	var parsed struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err.Error()
	}
	if len(parsed.Hooks) != len(matchers) {
		return fmt.Sprintf("registered events %v, want exactly %v", slices.Sorted(maps.Keys(parsed.Hooks)), slices.Sorted(maps.Keys(matchers)))
	}
	for event, matcher := range matchers {
		groups, ok := parsed.Hooks[event]
		if !ok || len(groups) == 0 {
			return event + " has no handler group"
		}
		for _, group := range groups {
			if group.Matcher != matcher {
				return fmt.Sprintf("%s matcher %q, want %q", event, group.Matcher, matcher)
			}
			if len(group.Hooks) == 0 {
				return event + " group has no handler"
			}
			for _, hook := range group.Hooks {
				if hook.Type != "command" || hook.Command != "corvint adapter codex" || hook.Timeout < 1 || hook.Timeout > 2 {
					return fmt.Sprintf("%s hook %+v is not the bounded Codex adapter", event, hook)
				}
			}
		}
	}
	return ""
}

// AHI-209: the Codex package tells agents and operators that only a root-thread user message
// authorizes an escalation, that the exact action must not drift, and where decisions are recorded.
func TestAHI209CodexApprovalGuidanceNamesRootGrantAndRecord(t *testing.T) {
	skill := strings.Join(strings.Fields(readCodexPackageFile(t, "plugins", "corvint", "skills", "corvint", "SKILL.md")), " ")
	for _, phrase := range []string{
		"A human approval reaches automatic approval review only as a user message in the root thread.",
		"never reword or retarget an approved action",
		"Approval review is not a Corvint gate",
	} {
		if !strings.Contains(skill, phrase) {
			t.Fatalf("Codex skill lacks %q", phrase)
		}
	}
	readme := strings.Join(strings.Fields(readCodexPackageFile(t, "README.md")), " ")
	if !strings.Contains(readme, "A changed target is denied.") {
		t.Fatal("Codex README lacks the retarget denial")
	}
	for name, document := range map[string]string{"skill": skill, "README": readme} {
		for _, phrase := range []string{"`guardian_review` session under `$CODEX_HOME/sessions/`", "`codex exec --json` does not emit these decisions"} {
			if !strings.Contains(document, phrase) {
				t.Fatalf("Codex %s lacks %q", name, phrase)
			}
		}
	}
}
