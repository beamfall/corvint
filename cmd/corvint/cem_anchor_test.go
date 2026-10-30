package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
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
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "src/main.rs", "fn main() { println!(\"hi\"); }\n")
	// The anchored map is the public cem/1.0 receipt-authentication fixture
	// rebound to this change's canonical patch, so it describes the commit it
	// is anchored on (CEM-V1-067); a retired cem/0.x map is refused by anchor
	// (CEM-V1-210).
	stable, err := os.ReadFile("../../protocol/cem-1.0/stable/receipt-authentication/stable.json")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "change")
	patch, err := repo.CanonicalDiff(context.Background(), base, cemGit(t, root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(patch)
	rebound := strings.Replace(string(stable), `"baseRevision":"467cbb73c1102cd7df0299cd74a6bef96ee3a9a4"`, `"baseRevision":"`+base+`"`, 1)
	rebound = strings.Replace(rebound, `"patchSha256":"9466aca391d358b6fd969537fd6fcc5088526e85d6b3f9fad13ecf35431f7ff8"`,
		`"patchSha256":"`+hex.EncodeToString(digest[:])+`"`, 1)
	if strings.Count(rebound, base)+strings.Count(rebound, hex.EncodeToString(digest[:])) != 2 {
		t.Fatal("fixture map members moved")
	}
	cemWrite(t, root, ".corvint/change.cem.json", rebound)
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
	// CEM-V1-273, CEM-V1-277: --artifacts appends the native verdict to the
	// ledger; --verdicts lists it without moving any ref.
	code, out, stderr = runCLI(t, "--root", root, "cem", "anchor", "--map", ".corvint/change.cem.json", "--artifacts", root)
	var recorded struct {
		Verdict map[string]any `json:"verdict"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &recorded) != nil || recorded.Verdict["written"] != true ||
		recorded.Verdict["ref"] != "refs/notes/corvint-verdicts" || recorded.Verdict["state"] != "verified" {
		t.Fatalf("anchor --artifacts: %d %s %s", code, out, stderr)
	}
	refs = cemGit(t, root, "for-each-ref")
	code, out, stderr = runCLI(t, "--root", root, "cem", "provenance", "--commit", "HEAD", "--verdicts")
	var listed struct {
		Mutates  bool `json:"mutates"`
		Verdicts struct {
			State   string           `json:"state"`
			Records []map[string]any `json:"records"`
		} `json:"verdicts"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &listed) != nil || listed.Mutates || listed.Verdicts.State != "verified" ||
		len(listed.Verdicts.Records) != 1 || listed.Verdicts.Records[0]["verdictSha256"] != recorded.Verdict["record"].(map[string]any)["verdictSha256"] ||
		cemGit(t, root, "for-each-ref") != refs {
		t.Fatalf("provenance --verdicts: %d %s %s", code, out, stderr)
	}
	tip := cemGit(t, root, "rev-parse", "refs/notes/corvint")
	cemGit(t, root, "update-ref", "-d", "refs/notes/corvint")
	code, out, stderr = runCLI(t, "--root", root, "cem", "provenance", "--commit", "HEAD", "--notes-tip", tip)
	var rolled struct {
		NotesTip map[string]any   `json:"notes_tip"`
		Evidence []map[string]any `json:"evidence"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &rolled) != nil || rolled.NotesTip["state"] != "rollback" ||
		rolled.Evidence[0]["kind"] != "cem-anchor-note" || rolled.Evidence[0]["state"] != "rollback" {
		t.Fatalf("provenance --notes-tip after a deleted notes ref: %d %s %s", code, out, stderr)
	}
	code, out, _ = runCLI(t, "cem", "anchor", "--help")
	if code != 0 || !strings.Contains(out, "cem anchor --map MAP [--commit REV] [--artifacts DIR]") {
		t.Fatalf("anchor help: %d %s", code, out)
	}
	code, out, _ = runCLI(t, "cem", "provenance", "--help")
	if code != 0 || !strings.Contains(out, "cem provenance --commit REV [--notes-tip OID] [--verdicts [--verdicts-tip OID]]") {
		t.Fatalf("provenance help: %d %s", code, out)
	}
}

// TestCEMV1345VerdictsTipThroughTheCLI is CEM-V1-345 through the real entry: a
// pinned ledger tip with no live ledger ref reads rollback with no records,
// and --verdicts-tip without --verdicts is refused.
func TestCEMV1345VerdictsTipThroughTheCLI(t *testing.T) {
	t.Parallel()
	root, _, target := cemRepo(t)
	code, out, stderr := runCLI(t, "--root", root, "cem", "provenance", "--commit", "HEAD", "--verdicts", "--verdicts-tip", target)
	var read struct {
		Verdicts struct {
			State   string            `json:"state"`
			Records []json.RawMessage `json:"records"`
			Tip     map[string]any    `json:"tip"`
		} `json:"verdicts"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &read) != nil || read.Verdicts.State != "rollback" ||
		len(read.Verdicts.Records) != 0 || read.Verdicts.Tip["state"] != "rollback" || read.Verdicts.Tip["pinned"] != target {
		t.Fatalf("pinned tip: %d %s %s", code, out, stderr)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "provenance", "--commit", "HEAD", "--verdicts-tip", target)
	if code != 2 || !strings.Contains(stderr, "invalid-arguments") {
		t.Fatalf("--verdicts-tip without --verdicts: %d %s", code, stderr)
	}
}
