package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0074_CheckProgramConfigHost proves the config host must equal the
// policy host, that Codex is only the absent host, and that a Codex config
// keeps its bytes.
func TestCALV0074_CheckProgramConfigHost(t *testing.T) {
	claude := &intent.SupervisionPolicy{Host: intent.SupervisedHostClaudeCode}
	codex := &intent.SupervisionPolicy{}
	cases := []struct {
		name   string
		policy *intent.SupervisionPolicy
		host   string
		refuse string
	}{
		{"absent policy keeps codex", nil, "", ""},
		{"codex policy and config", codex, "", ""},
		{"claude-code policy and config", claude, "claude-code", ""},
		{"config claude-code under codex policy", codex, "claude-code", `differs from policy host ""`},
		{"config claude-code without policy", nil, "claude-code", "differs from policy host"},
		{"codex config under claude-code policy", claude, "", `differs from policy host "claude-code"`},
		{"explicit codex is not canonical", codex, "codex", `host "codex" is unsupported`},
		{"unknown host", claude, "opencode", `host "opencode" is unsupported`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := CheckProgramConfig(ProgramConfig{Effort: "low", WallSeconds: 60, Host: tc.host}, tc.policy)
			if tc.refuse == "" && e != nil {
				t.Fatalf("refused: %v", e)
			}
			if tc.refuse != "" && (e == nil || !strings.Contains(e.Error(), tc.refuse) || wire.CodeOf(e) != wire.CodeUnsupported) {
				t.Fatalf("want %s refusal %q, got %s %v", wire.CodeUnsupported, tc.refuse, wire.CodeOf(e), e)
			}
		})
	}
	raw, _ := json.Marshal(ProgramConfig{Profile: "p", Effort: "low"})
	if strings.Contains(string(raw), `"host"`) {
		t.Fatalf("codex config bytes carry host: %s", raw)
	}
}

// TestCALV0075_ClaudeStageArgv pins the Claude Code invocation per stage:
// stage effort, project settings, no MCP, no prompts, edits only in
// implement, exact resume, and sorted sibling worktrees last in every stage.
func TestCALV0075_ClaudeStageArgv(t *testing.T) {
	c := ProgramConfig{Model: "pinned-model", Effort: "low", StageEfforts: map[string]string{"implement": "high", "review": "medium"}}
	common := "-p --output-format json --model pinned-model --effort "
	tail := " --setting-sources project --strict-mcp-config --permission-prompts none"
	readOnly := " --permission-mode dontAsk --disallowedTools Edit,Write,NotebookEdit"
	for _, tc := range []struct {
		stage, session string
		extras         map[string]string
		want           string
	}{
		{"implement", "", nil, common + "high" + tail + " --permission-mode acceptEdits"},
		{"implement", "session-7", nil, common + "high" + tail + " --permission-mode acceptEdits --resume session-7"},
		{"review", "", nil, common + "medium" + tail + readOnly},
		{"integrate", "", nil, common + "low" + tail + readOnly},
		{"implement", "session-1", map[string]string{"site": "/w/i@site", "docs": "/w/i@docs"}, common + "high" + tail + " --permission-mode acceptEdits --resume session-1 --add-dir /w/i@docs /w/i@site"},
		{"review", "", map[string]string{"docs": "/w/r@docs"}, common + "medium" + tail + readOnly + " --add-dir /w/r@docs"},
	} {
		if got := strings.Join(claudeStageArgv(c, tc.stage, tc.session, tc.extras), " "); got != tc.want {
			t.Fatalf("%s %q argv\n got %s\nwant %s", tc.stage, tc.session, got, tc.want)
		}
	}
	// The Codex argv is unchanged by the Claude Code host.
	if got := strings.Join(codexStageArgv(c, "implement", ""), " "); got != `exec --json --sandbox workspace-write --model pinned-model -c model_reasoning_effort="high" -c mcp_servers={} -` {
		t.Fatalf("codex argv changed: %s", got)
	}
}
