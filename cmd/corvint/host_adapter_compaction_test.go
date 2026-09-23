package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// compactionFixture is a committed module with one tracked dirty path and one untracked path,
// the AHI-003 shape, plus the adapter context that resolves it as the project root.
func compactionFixture(t *testing.T) (string, context.Context) {
	t.Helper()
	root := cliGoModuleRepository(t)
	writeFixtureFile(t, root, "pkg/sample.go", "package sample\n\n// edited\n")
	writeFixtureFile(t, root, "extra.md", "# extra\n")
	return root, adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
}

func compactionHookStdout(t *testing.T, ctx context.Context, root, event string, fields map[string]any) string {
	t.Helper()
	payload := map[string]any{"session_id": "0b5c7c8e-3f0e-4c55-9a53-7d1f3f0c2a11", "cwd": root, "trigger": "auto"}
	for key, value := range fields {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if status := runHostAdapter(ctx, []string{"claude-code", event}, bytes.NewReader(raw), &stdout); status != 0 {
		t.Fatalf("status=%d output=%s", status, &stdout)
	}
	return stdout.String()
}

func headTree(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

// AHI-026: the shipped hooks.json registers the two compaction events the host hook API documents,
// matcherless so both triggers fire, and compatibility.json names the host version that
// registration was verified against as the same version the tested range ends at.
func TestAHI026ClaudeCompactionHooksRegisteredAgainstHostAPI(t *testing.T) {
	t.Parallel()
	plugin := filepath.Join("..", "..", "integrations", "claude-code", "plugins", "corvint")
	raw, err := os.ReadFile(filepath.Join(plugin, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string   `json:"type"`
				Command string   `json:"command"`
				Args    []string `json:"args"`
				Timeout float64  `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		t.Fatal(err)
	}
	for name, event := range map[string]string{"PreCompact": "pre-compact", "PostCompact": "post-compact"} {
		groups := hooks.Hooks[name]
		if len(groups) != 1 || groups[0].Matcher != "" || len(groups[0].Hooks) != 1 {
			t.Fatalf("%s: want one matcherless group with one hook, got %+v", name, groups)
		}
		hook := groups[0].Hooks[0]
		if hook.Type != "command" || hook.Command != "corvint" || !reflect.DeepEqual(hook.Args, []string{"adapter", "claude-code", event}) || hook.Timeout != 2 {
			t.Fatalf("%s: unexpected hook %+v", name, hook)
		}
	}
	raw, err = os.ReadFile(filepath.Join(plugin, "compatibility.json"))
	if err != nil {
		t.Fatal(err)
	}
	var compat struct {
		Host struct {
			MaximumTestedVersion string `json:"maximumTestedVersion"`
		} `json:"host"`
		CompactionHooks struct {
			Events              []string `json:"events"`
			VerifiedHostVersion string   `json:"verifiedHostVersion"`
		} `json:"compactionHooks"`
	}
	if err := json.Unmarshal(raw, &compat); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compat.CompactionHooks.Events, []string{"PreCompact", "PostCompact"}) {
		t.Fatalf("compatibility.json compactionHooks.events=%v", compat.CompactionHooks.Events)
	}
	if compat.CompactionHooks.VerifiedHostVersion != compat.Host.MaximumTestedVersion {
		t.Fatalf("verified host version %q is not the tested maximum %q", compat.CompactionHooks.VerifiedHostVersion, compat.Host.MaximumTestedVersion)
	}
	if !strings.Contains(compactionDisclosure, "Claude Code "+compat.CompactionHooks.VerifiedHostVersion) {
		t.Fatalf("the compact SessionStart disclosure does not name the verified host version %q", compat.CompactionHooks.VerifiedHostVersion)
	}
	// The adapter admits exactly the documented triggers; an unknown trigger degrades by name.
	root, ctx := compactionFixture(t)
	for _, trigger := range []any{"", "scheduled", 7} {
		output := runClaudeAdapterTest(ctx, t, root, "pre-compact", map[string]any{"session_id": "s", "trigger": trigger})
		if output["systemMessage"] != "Corvint FALLBACK degraded: invalid-compaction-trigger; coding continues" {
			t.Fatalf("trigger %v: %v", trigger, output)
		}
	}
}

// AHI-027: PreCompact emits, as plain stdout the host joins into the compactor's instructions, a
// pin naming the compact SessionStart revision and its exact tracked dirty paths.
func TestAHI027ClaudePreCompactEmitsPinFromCompactionBlock(t *testing.T) {
	t.Parallel()
	root, ctx := compactionFixture(t)
	stdout := compactionHookStdout(t, ctx, root, "pre-compact", map[string]any{"hook_event_name": "PreCompact", "custom_instructions": ""})
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 || lines[0]+"\n" != compactionPinInstruction || strings.HasPrefix(stdout, "{") {
		t.Fatalf("pre-compact stdout is not the instruction plus one pin line: %q", stdout)
	}
	pin, ok := parseCompactionPin(lines[1])
	if !ok {
		t.Fatalf("pin line does not parse: %q", lines[1])
	}
	want := compactionPin{Revision: headTree(t, root), Tracked: 1, Untracked: 1, Paths: []string{"pkg/sample.go"}}
	if !reflect.DeepEqual(pin, want) {
		t.Fatalf("pin=%+v want %+v", pin, want)
	}
	over := compactionBlock{Revision: pin.Revision}
	for index := 0; index < compactionPinPathLimit+3; index++ {
		over.Request.Paths = append(over.Request.Paths, strings.Repeat("p", index%9+1)+"/f.go")
	}
	over.Request.Paths = append(over.Request.Paths, "with space.go", "-flag.go", "../escape.go")
	bounded, ok := parseCompactionPin(compactionPinLine(over))
	if !ok || len(bounded.Paths) != compactionPinPathLimit || bounded.Elided != 6 {
		t.Fatalf("over-bound pin: %+v ok=%v", bounded, ok)
	}
}

// AHI-028: PostCompact verifies the preserved pin against the object store and reports, by name,
// every pinned path the pinned revision cannot rehydrate; a lost or unresolvable pin degrades visibly.
func TestAHI028ClaudePostCompactReportsNonRehydratablePaths(t *testing.T) {
	t.Parallel()
	root, ctx := compactionFixture(t)
	tree := headTree(t, root)
	pin := compactionPinProfile + " revision=" + tree + " tracked=2 untracked=1 paths=pkg/sample.go,pkg/gone.go elided=0"
	summary := "The session edited pkg/sample.go.\n" + pin + "\nThen it continued."
	stdout := compactionHookStdout(t, ctx, root, "post-compact", map[string]any{"hook_event_name": "PostCompact", "compact_summary": summary})
	want := compactionReportProfile + " pinned=" + tree + " current=matches rehydrated=1 non-rehydratable=pkg/gone.go elided=0 untracked=1 current-dirty=1\n"
	if stdout != want {
		t.Fatalf("report=%q want %q", stdout, want)
	}
	zero := strings.Repeat("0", 40)
	cases := map[string]string{
		"no pin in the summary": "compaction-pin-not-preserved",
		"corvint-compaction-pin/0 revision=" + tree + " tracked=1 untracked=0 paths=../escape.go elided=0":  "compaction-pin-not-preserved",
		"corvint-compaction-pin/0 revision=" + zero + " tracked=1 untracked=0 paths=pkg/sample.go elided=0": "compaction-pin-revision-unavailable",
	}
	for summary, reason := range cases {
		output := runClaudeAdapterTest(ctx, t, root, "post-compact", map[string]any{"session_id": "s", "trigger": "manual", "compact_summary": summary})
		if output["systemMessage"] != "Corvint FALLBACK degraded: "+reason+"; coding continues" {
			t.Fatalf("%q: %v", summary, output)
		}
	}
}

// AHI-029: neither compaction hook writes repository, index, or trace state.
func TestAHI029ClaudeCompactionHooksMutateNothing(t *testing.T) {
	t.Parallel()
	root, ctx := compactionFixture(t)
	snapshot := func() map[string]int64 {
		sizes := map[string]int64{}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				sizes[path] = info.Size()
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return sizes
	}
	before := snapshot()
	stdout := compactionHookStdout(t, ctx, root, "pre-compact", nil)
	pinLine := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")[1]
	compactionHookStdout(t, ctx, root, "post-compact", map[string]any{"compact_summary": pinLine})
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("compaction hooks changed the tree:\nbefore=%v\nafter=%v", before, after)
	}
}
