package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func answerabilityRepository(t *testing.T, extra ...map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".corvint/\n",
		"go.mod":     "module example.test/answer\n\ngo 1.27.0\n",
		"AGENTS.md":  "# Project instructions\n\nThe demux key splits every cache read.\n",
		"cache/demux.go": "package cache\n\n// SplitDemuxKey splits the demux key of a cache read.\n" +
			"func SplitDemuxKey(key string) string { return key }\n",
		"cache/reader.go": "package cache\n\nimport \"example.test/answer/store\"\n\n" +
			"// ReadCached reads one cached value.\nfunc ReadCached(key string) string { return store.Fetch(SplitDemuxKey(key)) }\n",
		"store/store.go": "package store\n\n// Fetch returns the stored value.\nfunc Fetch(key string) string { return key }\n",
		"docs/notes.md":  "# Notes\n\nThe cache demux key is described here.\n",
	}
	for _, added := range extra {
		for name, content := range added {
			files[name] = content
		}
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "answerability fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func runAnswerabilityForTest(t *testing.T, root string, extra ...string) (int, map[string]any, []byte) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runAnswerability(t.Context(), append([]string{"--root", root, "answerability"}, extra...), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("answerability exit %d: %s", code, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	return code, report, stdout.Bytes()
}

func answerabilitySignal(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	signal, ok := report["signal"].(map[string]any)
	if !ok {
		t.Fatalf("report has no signal block: %v", report)
	}
	return signal
}

// RDS-V0-003/RDS-V0-004/RDS-V0-006: a task naming a declared symbol whose file
// the packet also ranks yields overlap and a high-agreement answerable state.
func TestAnswerabilityHighAgreementProposesAnswerable(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "Fetch returns a stale value for the cached key")
	signal := answerabilitySignal(t, report)
	if signal["structural_vote"] != "present" {
		t.Fatalf("structural_vote = %v, want present", signal["structural_vote"])
	}
	if signal["agreement"] != "high" || signal["proposed_state"] != "answerable" {
		t.Fatalf("agreement %v / state %v, want high / answerable (signal %v)", signal["agreement"], signal["proposed_state"], signal)
	}
	if overlap, _ := signal["overlap_size"].(float64); overlap < 1 {
		t.Fatalf("overlap_size = %v, want at least 1", signal["overlap_size"])
	}
}

// RDS-V0-002: channel S admits only paths reached through index structure, and
// each row carries its structural reasons in sorted order. A prose file naming
// the same words is never a member.
func TestAnswerabilityStructuralChannelIsStructuralOnly(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "Fetch returns a stale value for the cached key")
	rows, _ := report["channel_s"].([]any)
	if len(rows) == 0 {
		t.Fatalf("channel_s is empty: %v", report)
	}
	structural := map[string]string{}
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		path, _ := row["path"].(string)
		reason, _ := row["reason"].(string)
		if path == "" || reason == "" {
			t.Fatalf("channel_s row = %v", row)
		}
		parts := strings.Split(reason, "; ")
		if !sort.StringsAreSorted(parts) {
			t.Fatalf("reasons are unsorted for %s: %q", path, reason)
		}
		for _, part := range parts {
			if !strings.HasPrefix(part, "declares ") && !strings.HasPrefix(part, "imports ") && !strings.HasPrefix(part, "imported by ") {
				t.Fatalf("non-structural reason for %s: %q", path, part)
			}
		}
		structural[path] = reason
	}
	if _, present := structural["store/store.go"]; !present {
		t.Fatalf("the file declaring Fetch is not a channel S member: %v", structural)
	}
	for _, prose := range []string{"docs/notes.md", "AGENTS.md"} {
		if reason, present := structural[prose]; present {
			t.Fatalf("prose file %s entered channel S as %q", prose, reason)
		}
	}
}

// RDS-V0-004: partial overlap is reported as low agreement and never upgraded
// to answerable.
func TestAnswerabilityLowAgreementProposesUncertain(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "SplitDemuxKey mishandles the demux key of a cache read")
	signal := answerabilitySignal(t, report)
	if signal["structural_vote"] != "present" {
		t.Fatalf("structural_vote = %v, want present", signal["structural_vote"])
	}
	if signal["agreement"] != "low" {
		t.Fatalf("agreement = %v, want low on partial overlap: %v", signal["agreement"], signal)
	}
	if signal["proposed_state"] != "uncertain" {
		t.Fatalf("proposed_state = %v, want uncertain on low agreement", signal["proposed_state"])
	}
}

// RDS-V0-006: a task naming no declared symbol leaves channel S empty; the
// lexical channel alone never proposes answerable.
func TestAnswerabilityAbsentStructuralVoteProposesUncertain(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "the upstream vendor changed their billing invoices last quarter")
	signal := answerabilitySignal(t, report)
	if signal["structural_vote"] != "absent" {
		t.Fatalf("structural_vote = %v, want absent", signal["structural_vote"])
	}
	if signal["proposed_state"] != "uncertain" {
		t.Fatalf("proposed_state = %v, want uncertain", signal["proposed_state"])
	}
	rows, _ := report["channel_s"].([]any)
	if len(rows) != 0 {
		t.Fatalf("channel_s = %v, want empty", rows)
	}
}

// RDS-V0-005: the report is byte-identical across runs over one revision.
func TestAnswerabilityIsDeterministic(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	task := "SplitDemuxKey and Fetch disagree about the demux key"
	_, _, first := runAnswerabilityForTest(t, root, "--task", task)
	for attempt := 0; attempt < 3; attempt++ {
		_, _, again := runAnswerabilityForTest(t, root, "--task", task)
		if !bytes.Equal(first, again) {
			t.Fatalf("attempt %d differs:\n%s\n%s", attempt, first, again)
		}
	}
}

// RDS-V0-007: the command is read-only and carries the fixed disclosure.
func TestAnswerabilityWritesNothingAndDiscloses(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	before := corvintDigest(t, root)
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "SplitDemuxKey mishandles the demux key")
	if after := corvintDigest(t, root); after != before {
		t.Fatalf(".corvint changed: %s -> %s", before, after)
	}
	if report["mutates"] != false {
		t.Fatalf("mutates = %v, want false", report["mutates"])
	}
	if report["disclosure"] == nil || report["disclosure"] == "" {
		t.Fatalf("report carries no disclosure: %v", report)
	}
}

// RDS-V0-008: the coverage block carries the denominators the signal was
// computed over, so a channel built on a partly-unparsed index is never read as
// a complete one.
func TestAnswerabilityCoverageNamesUnparsedSources(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t, map[string]string{"broken/broken.go": "package broken\n\nfunc (\n"})
	_, report, _ := runAnswerabilityForTest(t, root, "--task", "SplitDemuxKey mishandles the demux key")
	coverage, ok := report["coverage"].(map[string]any)
	if !ok {
		t.Fatalf("report carries no coverage block: %v", report)
	}
	sources, _ := coverage["sources"].(float64)
	if sources < 1 {
		t.Fatalf("coverage sources = %v, want the indexed source count", coverage["sources"])
	}
	if _, present := coverage["symbols"]; !present {
		t.Fatalf("coverage carries no symbol denominator: %v", coverage)
	}
	if _, present := coverage["packet"]; !present {
		t.Fatalf("coverage omits the packet's own denominators: %v", coverage)
	}
	unparsed, _ := coverage["unparsed_sources"].([]any)
	if float64(len(unparsed)) > sources {
		t.Fatalf("unparsed sources %d exceed the source denominator %v", len(unparsed), sources)
	}
	named := false
	for _, row := range unparsed {
		entry, _ := row.(map[string]any)
		if entry["path"] != "broken/broken.go" {
			continue
		}
		named = true
		if reason, _ := entry["reason"].(string); reason == "" {
			t.Fatalf("unparsed row carries no reason: %v", entry)
		}
	}
	if !named {
		t.Fatalf("coverage does not name the unparsed source: %v", coverage["unparsed_sources"])
	}
}

// RDS-V0: ctx is main's signal context, so a cancellation already in flight
// when the verb starts aborts the index load instead of completing it.
func TestRunAnswerabilityHonorsCancellation(t *testing.T) {
	t.Parallel()
	root := answerabilityRepository(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	code := runContext(ctx, []string{"--root", root, "answerability", "--task", "does Split keep empty keys", "--subject", "cache/demux.go"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cancel") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
