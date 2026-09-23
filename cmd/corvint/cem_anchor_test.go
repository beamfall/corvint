package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCEMAnchorAndProvenanceInteropThroughTheCLI is the FPK-V0-040 fixture
// through the real `corvint cem` entry: anchor a committed map (a mutation
// receipt), read the pointer back verified, and read a hand-written Git AI
// refs/notes/ai note and a commit carrying both trailers as untrusted
// repository-history rows, without moving any ref.
func TestCEMAnchorAndProvenanceInteropThroughTheCLI(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "src/main.rs", "fn main() {}\n")
	cemWrite(t, root, ".corvint/change.cem.json", `{"spec":"cem/0.2",`+
		`"baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",`+
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",`+
		`"excludedPath":".corvint/change.cem.json","evidence":[],"hunks":[]}`)
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "assisted change\n\nAssisted-by: Claude <noreply@anthropic.com>\n"+
		"Agent-Logs-Url: https://example.invalid/logs/42")
	note := filepath.Join(t.TempDir(), "ai-note")
	if err := os.WriteFile(note, []byte("src/main.rs\n  s_c9883b05a2487d 1\n---\n"+
		`{"schema_version":"authorship/3.0.0","base_commit_sha":"7734793b756b3921c88db5375a8c156e9532447b",`+
		`"prompts":{},"sessions":{"s_c9883b05a2487d":{"agent_id":{"tool":"cursor","id":"x","model":"m"}}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "notes", "--ref=refs/notes/ai", "add", "-F", note, "HEAD")

	code, out, stderr := runCLI(t, "--root", root, "cem", "anchor", "--map", ".corvint/change.cem.json")
	var receipt map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &receipt) != nil || receipt["mutates"] != true ||
		receipt["written"] != true || receipt["verification"] != "verified" {
		t.Fatalf("anchor: %d %s %s", code, out, stderr)
	}
	refs := cemGit(t, root, "for-each-ref")
	code, out, stderr = runCLI(t, "--root", root, "cem", "provenance", "--commit", "HEAD")
	var read struct {
		Mutates  bool             `json:"mutates"`
		Evidence []map[string]any `json:"evidence"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &read) != nil || read.Mutates || cemGit(t, root, "for-each-ref") != refs {
		t.Fatalf("provenance: %d %s %s", code, out, stderr)
	}
	kinds := []string{}
	for _, row := range read.Evidence {
		kinds = append(kinds, row["kind"].(string))
		if row["trust"] != "repository-history" || row["authority"] != "git-history" {
			t.Fatalf("row trust: %v", row)
		}
	}
	if strings.Join(kinds, ",") != "cem-anchor-note,git-ai-authorship-note,assisted-by-trailer,agent-logs-url-trailer" ||
		read.Evidence[0]["state"] != "verified" || read.Evidence[1]["state"] != "parsed" {
		t.Fatalf("provenance rows: %s", out)
	}
	code, out, _ = runCLI(t, "cem", "anchor", "--help")
	if code != 0 || !strings.Contains(out, "cem anchor --map MAP") || !strings.Contains(out, "cem provenance --commit REV") {
		t.Fatalf("help: %d %s", code, out)
	}
}
