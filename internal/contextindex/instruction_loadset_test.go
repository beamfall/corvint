package contextindex

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func instructionFixture(files map[string][]byte) *Index {
	index := &Index{Revision: "pinned-tree", CommitRevision: "pinned-commit", Tracked: map[string]struct{}{"app/source.go": {}}, Sources: map[string]Source{}}
	for name, data := range files {
		index.Tracked[name] = struct{}{}
		index.Sources[name] = Source{Path: name, BlobHash: "blob-" + name, Mode: "100644", Data: data}
	}
	return index
}

func doctorOptions() InstructionLoadOptions {
	return InstructionLoadOptions{Host: "codex", Version: "0.153.2", CWD: "app", Profile: "default"}
}

func doctorRows(t *testing.T, index *Index) (map[string]any, []instructionLoadRow) {
	t.Helper()
	receipt, err := InstructionLoadSet(index, "app/source.go", doctorOptions())
	if err != nil {
		t.Fatal(err)
	}
	return receipt, receipt["rows"].([]instructionLoadRow)
}

func TestInstructionLoadSetDefaultSelection(t *testing.T) {
	index := instructionFixture(map[string][]byte{"AGENTS.md": []byte("root"), "app/AGENTS.override.md": []byte("override"), "app/AGENTS.md": []byte("shadow"), "app/CLAUDE.md": []byte("independent authority")})
	receipt, rows := doctorRows(t, index)
	if receipt["actual_session_load_set"] != "UNKNOWN" || receipt["project_authority"] != "UNCHANGED" {
		t.Fatal(receipt)
	}
	if len(rows) != 4 || rows[0].Path != "AGENTS.md" || rows[0].State != "LOADED" || rows[1].State != "LOADED" || rows[2].State != "SHADOWED" || rows[3].State != "IGNORED" {
		t.Fatal(rows)
	}
	index.Sources["app/AGENTS.override.md"] = Source{Mode: "100644", Data: []byte(" \t\n")}
	receipt, rows = doctorRows(t, index)
	if rows[1].State != "EMPTY_SELECTED" || rows[2].State != "SHADOWED" || receipt["remaining_inner_raw_budget"] != instructionDefaultBudget-4 {
		t.Fatal(receipt)
	}
}

func TestInstructionLoadSetPrefixBudgetAndLossyBytes(t *testing.T) {
	index := instructionFixture(map[string][]byte{"AGENTS.md": []byte(strings.Repeat("a", instructionDefaultBudget-2)), "app/AGENTS.md": []byte("€rest")})
	receipt, rows := doctorRows(t, index)
	if rows[1].State != "TRUNCATED" || rows[1].RetainedRawBytes != 2 || rows[1].DecodedBytes != 3 || receipt["remaining_inner_raw_budget"] != 0 || receipt["remaining_outer_decoded_budget"] != 0 {
		t.Fatal(receipt)
	}
	index = instructionFixture(map[string][]byte{"AGENTS.md": {0xff, 0xff}, "app/AGENTS.md": []byte("ok")})
	receipt, rows = doctorRows(t, index)
	if rows[0].DecodedBytes != 6 || receipt["remaining_inner_raw_budget"] != instructionDefaultBudget-4 || receipt["remaining_outer_decoded_budget"] != instructionDefaultBudget-8 {
		t.Fatal(receipt)
	}
	for _, c := range []struct {
		raw  []byte
		want string
	}{
		{[]byte{0xe2, 0x82}, "�"}, {[]byte{0xe2, 0x82, 0xff}, "��"}, {[]byte{0xe0, 0x80, 0x80}, "���"}, {[]byte{0xf0, 0x90, 0x80}, "�"}, {[]byte{0xed, 0xa0, 0x80}, "���"}, {[]byte("a€�z"), "a€�z"},
	} {
		if got := instructionLossyUTF8(c.raw); got != c.want {
			t.Fatalf("%x decoded %q want %q", c.raw, got, c.want)
		}
	}
}

func TestInstructionLoadSetUnknownsAndSafeScope(t *testing.T) {
	index := instructionFixture(map[string][]byte{"AGENTS.md": []byte("root"), "app/AGENTS.md": []byte("local")})
	index.Sources["AGENTS.md"] = Source{Mode: "100644", BlobHash: "missing-body"}
	receipt, rows := doctorRows(t, index)
	if receipt["state"] != "UNKNOWN" || rows[0].State != "UNKNOWN" || rows[1].State != "UNKNOWN" {
		t.Fatal(receipt)
	}
	if _, exists := receipt["remaining_inner_raw_budget"]; exists {
		t.Fatal("invented remaining budget")
	}
	options := doctorOptions()
	options.Version = "unconfirmed"
	receipt, err := InstructionLoadSet(index, "app/source.go", options)
	if err != nil || receipt["state"] != "UNKNOWN" || receipt["rules"] != nil {
		t.Fatal(receipt, err)
	}
	for _, host := range []string{"claude-code", "gemini-cli", "opencode", "pi"} {
		options.Host = host
		if receipt, err := InstructionLoadSet(index, "app/source.go", options); err != nil || receipt["state"] != "UNKNOWN" {
			t.Fatal(host, receipt, err)
		}
	}
	for _, cwd := range []string{"../app", "/app", "app/../app", "other", strings.Repeat("app/", 65) + "deep"} {
		options = doctorOptions()
		options.CWD = cwd
		if _, err := InstructionLoadSet(index, "app/source.go", options); err == nil {
			t.Fatal("accepted cwd", cwd)
		}
	}
	options = doctorOptions()
	options.Profile = "actual"
	if _, err := InstructionLoadSet(index, "app/source.go", options); err == nil {
		t.Fatal("accepted unknown profile")
	}
}

func TestInstructionLoadSetWarningsAndBounds(t *testing.T) {
	if got := instructionControls([]byte("\u061c\u034f\u180e\ufeff")); len(got) != 4 {
		t.Fatalf("missing invisible/directional controls: %v", got)
	}
	index := instructionFixture(map[string][]byte{"AGENTS.md": []byte("normal\n\u202e\u200b\U000e0061"), "app/AGENTS.md": []byte(strings.Repeat("\u200b", 40))})
	index.DirtyPaths = []string{"AGENTS.md"}
	_, rows := doctorRows(t, index)
	if len(rows[0].Warnings) != 4 || rows[0].Warnings[1].Line != 2 || rows[0].Warnings[1].CodePoint != "U+202E" || rows[0].Warnings[1].ByteOffset != 7 {
		t.Fatal(rows)
	}
	if len(rows[1].Warnings) != 33 || rows[1].Warnings[32].Code != "SUSPICIOUS_CONTROL_WARNINGS_CAPPED" {
		t.Fatal(rows[1])
	}
	index.Sources["AGENTS.md"] = Source{Mode: "100644", Data: make([]byte, contextMaxBytes+1)}
	receipt, _ := doctorRows(t, index)
	if receipt["state"] != "UNKNOWN" {
		t.Fatal("over-bound source admitted")
	}
}

func TestInstructionLoadSetUnconfirmedSelectionAndDepth(t *testing.T) {
	index := instructionFixture(map[string][]byte{"AGENTS.override.md": []byte("target"), "AGENTS.md": []byte("fallback"), "app/AGENTS.md": []byte("local")})
	index.Sources["AGENTS.override.md"] = Source{Mode: "120000", Data: []byte("target")}
	receipt, rows := doctorRows(t, index)
	if receipt["state"] != "UNKNOWN" || rows[0].State != "UNKNOWN" || rows[1].State != "UNKNOWN" || rows[2].State != "UNKNOWN" {
		t.Fatal(receipt)
	}
	options := doctorOptions()
	options.CWD = strings.Repeat("a/", 64) + "b"
	subject := options.CWD + "/source.go"
	index.Tracked[subject] = struct{}{}
	if _, err := InstructionLoadSet(index, subject, options); err == nil {
		t.Fatal("accepted over-bound ancestor walk")
	}
	// A source-only index cannot establish marker discovery: every prediction
	// retains the assumed Git-root profile and actual marker resolution UNKNOWN.
	if receipt["actual_session_load_set"] != "UNKNOWN" {
		t.Fatal("claimed actual no-marker behavior")
	}
}

func TestInstructionProfileManifestBinding(t *testing.T) {
	raw, err := os.ReadFile("../../integrations/instruction-profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Profiles []struct{ Host, Version, Source, SHA256 string }
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Profiles) != 1 || manifest.Profiles[0].Host != "codex" || manifest.Profiles[0].Version != "0.153.2" || manifest.Profiles[0].Source != instructionRuleURL || manifest.Profiles[0].SHA256 != instructionRuleSHA256 {
		t.Fatal("profile source binding drift", manifest)
	}
}
