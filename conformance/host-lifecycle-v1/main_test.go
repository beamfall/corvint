package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopedReceipt(t *testing.T) {
	good := `{"hookSpecificOutput":{"additionalContext":"guidance\n` + envelopeBegin + `\nuntrusted\n{\"ok\":true,\"mutates\":false,\"profile\":\"corvint-dogfood-event/0\",\"context\":{\"task_evidence\":[{\"path\":\"add.go\",\"blob_hash\":\"b1\"}]}}\n` + envelopeEnd + `\n"}}`
	receipt, _, err := envelopedReceipt(good)
	if err != nil {
		t.Fatal(err)
	}
	if !citesPath(receipt, "add.go", "b1") || citesPath(receipt, "add.go", "b2") {
		t.Fatalf("citesPath disagrees with the receipt evidence")
	}
	for name, output := range map[string]string{
		"not json":    "{",
		"no envelope": `{"hookSpecificOutput":{"additionalContext":"{\"ok\":true}"}}`,
		"failed":      `{"hookSpecificOutput":{"additionalContext":"` + envelopeBegin + `\n{\"ok\":false,\"mutates\":false,\"profile\":\"corvint-dogfood-event/0\"}\n` + envelopeEnd + `"}}`,
		"mutating":    `{"hookSpecificOutput":{"additionalContext":"` + envelopeBegin + `\n{\"ok\":true,\"mutates\":true,\"profile\":\"corvint-dogfood-event/0\"}\n` + envelopeEnd + `"}}`,
	} {
		if _, _, err := envelopedReceipt(output); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMissingCoreVerbs(t *testing.T) {
	help := "Commands:\n  frontier  ... Experimental.\nCommand maturity:\n  Core (decision 0332):\n    init, adopt, index, query, context, impact, affected, prove, cem, ocm,\n    frontier, dogfood\n  Experimental, no stability promise:\n    mcp\n"
	if missing := missingCoreVerbs(help); len(missing) != 0 {
		t.Fatalf("missing %v", missing)
	}
	partial := strings.Replace(help, "ocm,", "", 1)
	if missing := missingCoreVerbs(partial); strings.Join(missing, ",") != "ocm" {
		t.Fatalf("missing %v, want ocm", missing)
	}
	if missing := missingCoreVerbs("Command maturity:\n  Experimental, none\n"); len(missing) != len(coreVerbs) {
		t.Fatalf("absent Core section reported %v", missing)
	}
}

func TestListed(t *testing.T) {
	if !listed("Installed plugins:\n\n  ❯ corvint@corvint\n    Version: 0.2.3\n", "corvint@corvint") {
		t.Error("Claude Code row not recognized")
	}
	if !listed("corvint@corvint-source  installed, enabled  0.2.2  /x\n", "corvint@corvint-source") {
		t.Error("Codex row not recognized")
	}
	if listed("No plugins installed\n", "corvint@corvint") || listed("corvint@corvint-source  not installed\n", "corvint@corvint-source") {
		t.Error("absent plugin reported as listed")
	}
}

func TestRowIsScopedToSelector(t *testing.T) {
	claude := "Installed plugins:\n\n  ❯ other@m\n    Version: 0.2.3\n    Status: ✔ enabled\n  ❯ corvint@corvint\n    Version: 0.2.2\n    Status: ✘ disabled\n"
	text := row(claude, "corvint@corvint")
	if !strings.Contains(text, "disabled") || strings.Contains(text, "0.2.3") || strings.Contains(text, "✔ enabled") {
		t.Errorf("Claude Code row %q", text)
	}
	codex := "corvint@corvint-source  installed, disabled  0.2.2  /x\nother@m  installed, enabled  0.2.3  /y\n"
	text = row(codex, "corvint@corvint-source")
	if strings.Contains(text, "0.2.3") || strings.Contains(text, " enabled") {
		t.Errorf("Codex row %q", text)
	}
	if row(codex, "absent@m") != "" {
		t.Error("absent selector has a row")
	}
}

func TestReadHooks(t *testing.T) {
	directory := t.TempDir()
	argvShape := filepath.Join(directory, "argv.json")
	shellShape := filepath.Join(directory, "shell.json")
	if err := os.WriteFile(argvShape, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint","args":["adapter","claude-code","stop"]}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shellShape, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint adapter codex","timeout":2}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	argv, err := readHooks(argvShape, false)
	if err != nil || strings.Join(argv["Stop"], " ") != "corvint adapter claude-code stop" {
		t.Fatalf("argv shape: %v %v", argv, err)
	}
	shell, err := readHooks(shellShape, true)
	if err != nil || strings.Join(shell["Stop"], " ") != "corvint adapter codex" {
		t.Fatalf("shell shape: %v %v", shell, err)
	}
	several := filepath.Join(directory, "several.json")
	if err := os.WriteFile(several, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint adapter codex"}]},{"hooks":[{"type":"command","command":"other"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHooks(several, true); err == nil {
		t.Fatal("an event with two commands was accepted")
	}
	later := filepath.Join(directory, "later.json")
	if err := os.WriteFile(later, []byte(`{"hooks":{"Stop":[{"matcher":"x","hooks":[]},{"hooks":[{"type":"command","command":"corvint adapter codex"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hooks, err := readHooks(later, true); err != nil || strings.Join(hooks["Stop"], " ") != "corvint adapter codex" {
		t.Fatalf("command in a later group: %v %v", hooks, err)
	}
}

func TestSessionKeyPattern(t *testing.T) {
	key := strings.Repeat("ab", 32)
	for _, text := range []string{
		`["corvint","--root","/f","dogfood","status","--session-key","` + key + `"]`,
		`[\"corvint\",\"--session-key\",\"` + key + `\"]`,
	} {
		match := sessionKeyPattern.FindStringSubmatch(text)
		if match == nil || match[1] != key {
			t.Errorf("no key in %s", text)
		}
	}
}

func TestSummaryExitRequiresAllNinePass(t *testing.T) {
	var results []result
	for _, name := range caseOrder {
		results = append(results, result{name, "PASS", ""})
	}
	if summaryExit(results) != 0 {
		t.Fatal("nine PASS cases did not exit 0")
	}
	results[7].status = "NOT_RUN"
	if summaryExit(results) != 1 {
		t.Fatal("a NOT_RUN case exited 0")
	}
	if summaryExit(results[:8]) != 1 {
		t.Fatal("a missing case exited 0")
	}
}
