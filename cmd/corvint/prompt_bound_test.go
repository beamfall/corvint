package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// overBoundProse is prompt text with no explicit anchor: plain words, a
// version number, an abbreviation and an ASCII sentence end.
const overBoundProse = "please look at why the hook keeps degrading, e.g. version 1.25 fails. "

func TestAHI016OverBoundPromptDerivesVerbatimAnchorQuery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	prompt := "Fix `prompt-over-query-bound` in cmd/corvint/host_adapter.go: normalizeAdapterInput and MAX_TASK_BYTES per AHI-016. " +
		strings.Repeat(overBoundProse, 40) + "Also AGENTS.md, then normalizeAdapterInput again."
	normalized, reason := normalizeAdapterInput("claude-code", "user-prompt", map[string]any{"session_id": "s", "prompt": prompt}, root)
	want := "prompt-over-query-bound cmd/corvint/host_adapter.go normalizeAdapterInput MAX_TASK_BYTES AHI-016 AGENTS.md"
	if reason != "" || normalized["task"] != want {
		t.Fatalf("task=%q reason=%q, want %q", normalized["task"], reason, want)
	}
	disclosure := promptBoundDisclosure("user-prompt", map[string]any{"prompt": "  " + prompt + "\n"})
	for _, fragment := range []string{"trusted adapter disclosure", "6 distinct explicit anchors", "106 characters long", "this context makes no claim about it"} {
		if !strings.Contains(disclosure, fragment) {
			t.Fatalf("disclosure lacks %q: %s", fragment, disclosure)
		}
	}
	if strings.Contains(disclosure, "normalizeAdapterInput") || strings.Contains(disclosure, "degrading") {
		t.Fatalf("disclosure echoes prompt text: %s", disclosure)
	}

	within := map[string]any{"session_id": "s", "prompt": "inspect normalizeAdapterInput"}
	normalized, reason = normalizeAdapterInput("claude-code", "user-prompt", within, root)
	if reason != "" || normalized["task"] != "inspect normalizeAdapterInput" || promptBoundDisclosure("user-prompt", within) != "" {
		t.Fatalf("within-bound prompt changed: %v %q", normalized, reason)
	}
}

func TestAHI016OverBoundPromptKeepsRefusalWhenAnchorsCannotServe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tooMany := make([]string, 0, 300)
	for index := range 300 {
		tooMany = append(tooMany, fmt.Sprintf("anchor_%03d_item", index))
	}
	for name, prompt := range map[string]string{
		"no anchors":            strings.Repeat(overBoundProse, 40),
		"anchor set over bound": strings.Join(tooMany, " ") + " " + strings.Repeat(overBoundProse, 10),
	} {
		payload := map[string]any{"session_id": "s", "prompt": prompt}
		if _, reason := normalizeAdapterInput("codex", "user-prompt", payload, root); reason != "prompt-over-query-bound" {
			t.Fatalf("%s: reason=%q, want the refusal", name, reason)
		}
		if disclosure := promptBoundDisclosure("user-prompt", payload); disclosure != "" {
			t.Fatalf("%s: refused prompt carried a disclosure: %s", name, disclosure)
		}
	}
}

func TestAHI016ClaudeOverBoundPromptInjectsDisclosedContextWithoutStoring(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	before := repositoryBytesDigest(t, root)
	prompt := "Why does internal/parser/token.go reject ParseToken delimiters? " + strings.Repeat(overBoundProse, 40)
	// The real hook entrypoint with a UserPromptSubmit payload shaped as Claude Code sends it.
	payload, err := json.Marshal(map[string]any{
		"session_id": "native-session", "transcript_path": "/Users/dev/.claude/projects/p/native-session.jsonl",
		"cwd": root, "permission_mode": "default", "hook_event_name": "UserPromptSubmit", "prompt": prompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	ctx := adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
	if status := runHostAdapter(ctx, []string{"claude-code", "user-prompt"}, bytes.NewReader(payload), &stdout); status != 0 {
		t.Fatalf("status=%d output=%s", status, &stdout)
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil || output["systemMessage"] != nil || strings.Contains(stdout.String(), "prompt-over-query-bound") {
		t.Fatalf("over-bound prompt still degraded: %s err=%v", &stdout, err)
	}
	contextText := output["hookSpecificOutput"].(map[string]any)["additionalContext"].(string)
	disclosureEnd := strings.Index(contextText, "Corvint local workflow argv")
	envelope := strings.Index(contextText, untrustedDataPrefix)
	if !strings.HasPrefix(contextText, "Corvint prompt bound (trusted adapter disclosure)") || disclosureEnd < 0 || envelope < disclosureEnd {
		t.Fatalf("disclosure is not trusted text ahead of the envelope: %s", contextText)
	}
	if strings.Contains(contextText, "degrading") || strings.Contains(contextText, "Why does") {
		t.Fatalf("context echoes elided prompt text: %s", contextText)
	}
	if after := repositoryBytesDigest(t, root); after != before {
		t.Fatal("over-bound prompt derivation wrote repository or .corvint state")
	}
}

// promptBoundaryCase is a boundaryCases entry of
// conformance/harness-event-v0/common-logical-interaction.json; the Gemini CLI
// and OpenCode twins in integrations/host-adapters.test.mjs read the same cases.
type promptBoundaryCase struct {
	Case  string `json:"case"`
	Input struct {
		TaskCharacter      string `json:"taskCharacter"`
		TaskCharacterCount int    `json:"taskCharacterCount"`
		PromptSegments     []struct {
			Text   string `json:"text"`
			Repeat int    `json:"repeat"`
		} `json:"promptSegments"`
	} `json:"input"`
	Expected struct {
		CorvintInvoked bool     `json:"corvintInvoked"`
		Code           string   `json:"code"`
		Task           string   `json:"task"`
		Disclosure     string   `json:"disclosure"`
		Hosts          []string `json:"hosts"`
	} `json:"expected"`
}

func (boundary promptBoundaryCase) prompt() string {
	var prompt strings.Builder
	prompt.WriteString(strings.Repeat(boundary.Input.TaskCharacter, boundary.Input.TaskCharacterCount))
	for _, segment := range boundary.Input.PromptSegments {
		for index := range segment.Repeat {
			prompt.WriteString(strings.ReplaceAll(segment.Text, "{n}", strconv.Itoa(index)))
		}
	}
	return prompt.String()
}

func TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../conformance/harness-event-v0/common-logical-interaction.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		BoundaryCases []promptBoundaryCase `json:"boundaryCases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, boundary := range fixture.BoundaryCases {
		payload := map[string]any{"session_id": "s", "prompt": boundary.prompt()}
		for _, host := range []string{"claude-code", "codex"} {
			normalized, reason := normalizeAdapterInput(host, "user-prompt", payload, root)
			task, _ := normalized["task"].(string)
			if reason != boundary.Expected.Code || task != boundary.Expected.Task {
				t.Fatalf("%s %s: task=%q reason=%q, want task=%q code=%q", boundary.Case, host, task, reason, boundary.Expected.Task, boundary.Expected.Code)
			}
			if invoked := reason == ""; invoked != boundary.Expected.CorvintInvoked {
				t.Fatalf("%s %s: corvintInvoked=%t, want %t", boundary.Case, host, invoked, boundary.Expected.CorvintInvoked)
			}
		}
		if disclosure := promptBoundDisclosure("user-prompt", payload); disclosure != boundary.Expected.Disclosure {
			t.Fatalf("%s: disclosure=%q, want %q", boundary.Case, disclosure, boundary.Expected.Disclosure)
		}
	}
}
