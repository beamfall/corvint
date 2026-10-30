package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestImpactProviderCommandFlagParsing: `--provider-command` is the only way
// to select the command transport, takes one strict JSON argv, and shares the
// provider bound and the incompatibilities of `--provider` (EEP-TR-001,
// EEP-TR-002).
func TestImpactProviderCommandFlagParsing(t *testing.T) {
	t.Parallel()
	parsed, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--provider", "a.json", "--provider-command", `["/bin/cat","b.json"]`, `--provider-command=["/bin/cat"]`, "pkg/main.go"}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.impactProviders) != 3 || parsed.impactProviders[0] != "a.json" || !strings.Contains(parsed.impactProviders[1], "b.json") {
		t.Fatalf("providers = %q", parsed.impactProviders)
	}
	refusals := map[string][]string{
		"missing value":       {"pkg/main.go", "--provider-command"},
		"not json":            {"--provider-command", "/bin/cat b.json", "pkg/main.go"},
		"relative executable": {"--provider-command", `["cat","b.json"]`, "pkg/main.go"},
		"fifth provider":      {"--provider", "1", "--provider", "2", "--provider", "3", "--provider", "4", "--provider-command", `["/bin/cat"]`, "pkg/main.go"},
		"with --base":         {"--provider-command", `["/bin/cat"]`, "--base", strings.Repeat("a", 40)},
	}
	for name, arguments := range refusals {
		if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, arguments, "darwin"); err == nil {
			t.Errorf("%s: expected an argument error", name)
		}
	}
}

// TestImpactProviderCommandEndToEnd: a command that prints the record yields
// the same external section as the file, the core receipt stays identical,
// and the read stays read-only (EEP-TR-005, EEP-TR-007).
func TestImpactProviderCommandEndToEnd(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record := providerRecord(t, root)
	argv, _ := json.Marshal([]string{"/bin/cat", record})
	status := affectedGit(t, root, "status", "--porcelain")
	code, fromFile, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider: exit %d stderr %q", code, stderr)
	}
	code, fromCommand, stderr := runCLI(t, "--root", root, "impact", "--provider-command", string(argv), "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider-command: exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(fromCommand, `"mutates":false`) {
		t.Fatalf("receipt must report mutates false: %s", fromCommand)
	}
	var filePayload, commandPayload map[string]any
	if json.Unmarshal([]byte(fromFile), &filePayload) != nil || json.Unmarshal([]byte(fromCommand), &commandPayload) != nil {
		t.Fatal("receipts must be JSON")
	}
	fileExternal := filePayload["context"].(map[string]any)["external"].(map[string]any)
	commandExternal := commandPayload["context"].(map[string]any)["external"].(map[string]any)
	commandRow := commandExternal["providers"].([]any)[0].(map[string]any)
	if commandRow["source"] != "command:"+string(argv) || commandRow["state"] != "loaded" {
		t.Fatalf("command provider row = %v", commandRow)
	}
	commandRow["source"] = fileExternal["providers"].([]any)[0].(map[string]any)["source"]
	left, _ := contextindex.CanonicalJSON(filePayload)
	right, _ := contextindex.CanonicalJSON(commandPayload)
	if !bytes.Equal(left, right) {
		t.Fatalf("receipts must match apart from the provider source:\n%s\n%s", left, right)
	}
	if after := affectedGit(t, root, "status", "--porcelain"); after != status {
		t.Fatalf("worktree status changed:\n%s\n%s", status, after)
	}
}
