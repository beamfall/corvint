package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func observeProof(t *testing.T, root string, stdin string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "prove-observe"}, strings.NewReader(stdin), &stdout, &stderr)
	if stdout.Len() != 0 {
		t.Fatalf("prove-observe wrote stdout: %s", &stdout)
	}
	return stderr.String(), code
}

// TestProveObserveRecordsOnlyTheVerdictCounts pipes a real proof into the
// ledger and reads the rate back through prove and observations. prove
// itself still writes nothing: the tree digest is taken after the ledger row
// exists and is unchanged by the second prove.
func TestProveObserveRecordsOnlyTheVerdictCounts(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	receipt, document, stderr, code := runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, present := receipt["proof"].(map[string]any)["ledger"]; present {
		t.Fatalf("ledger block before any proof was recorded: %v", receipt["proof"])
	}
	if stderr, code := observeProof(t, root, string(document)); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	var row map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &row); err != nil {
		t.Fatal(err)
	}
	counts, _ := json.Marshal(row["counts"])
	if len(row) != 2 || row["kind"] != "proof" || string(counts) != `{"history-consistent":{"PASS":5}}` {
		t.Fatalf("row carries unexpected fields: %s", lines[len(lines)-1])
	}
	before := treeDigest(t, root)
	receipt, _, stderr, code = runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	ledger, _ := receipt["proof"].(map[string]any)["ledger"].(map[string]any)
	if integerAt(ledger, "proofs") != 1 || integerAt(ledger, "judged") != 5 || integerAt(ledger, "failed") != 0 || ledger["rate"] != "0/5" {
		t.Fatalf("ledger block: %v", ledger)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove wrote the tree")
	}
	stdout, stderr, code := runProveArguments(t, "--root", root, "observations")
	if code != 0 || !strings.Contains(string(stdout), "\nFALSIFICATION proofs=1 judged=5 failed=0 rate=0/5\nFALSIFIER key=history-consistent judged=5 failed=0 not-run=0 rate=0/5\n") {
		t.Fatalf("observations exit %d: %s%s", code, stdout, stderr)
	}
}

// TestProveObserveRejectsWhatIsNotAProof: nothing but a prove document's own
// counts shape reaches the ledger, and a rejected input leaves no ledger.
func TestProveObserveRejectsWhatIsNotAProof(t *testing.T) {
	t.Parallel()
	valid := `{"tool":"prove","profile":"` + proveProfile + `","proof":{"counts":{"history-consistent":{"PASS":1}},"rows":[{"falsifier":"history-consistent","falsified":"PASS"}]}}`
	for _, test := range []struct{ name, stdin, message string }{
		{"not json", "{", "not a JSON object"},
		{"oversized", `{"tool":"prove","pad":"` + strings.Repeat("x", maxProofDocumentBytes) + `"}`, "exceeds 8 MiB"},
		{"another tool", strings.Replace(valid, `"tool":"prove"`, `"tool":"query"`, 1), "not a " + proveProfile},
		{"no rows", `{"tool":"prove","profile":"` + proveProfile + `","proof":{"counts":{}}}`, "proof.rows or proof.counts is missing"},
		{"unknown falsifier", strings.ReplaceAll(valid, "history-consistent", "vibes"), "unknown falsifier or verdict"},
		{"unknown verdict", strings.ReplaceAll(valid, "PASS", "MAYBE"), "unknown falsifier or verdict"},
		{"inflated counts", strings.Replace(valid, `"PASS":1`, `"PASS":40`, 1), "does not match proof.rows"},
		{"counts without rows", strings.Replace(valid, `"rows":[{"falsifier":"history-consistent","falsified":"PASS"}]`, `"rows":[]`, 1), "does not match proof.rows"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			stderr, code := observeProof(t, root, test.stdin)
			if code != 2 || !strings.Contains(stderr, "invalid-proof-document") || !strings.Contains(stderr, test.message) {
				t.Fatalf("exit %d stderr %q", code, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, ".corvint", "self-observations.jsonl")); !os.IsNotExist(err) {
				t.Fatal("a rejected document reached the ledger")
			}
		})
	}
	root := cliRepository(t)
	if stderr, code := observeProof(t, root, valid); code != 2 || !strings.Contains(stderr, "observation-failed") {
		t.Fatalf("unignored ledger: exit %d stderr %q", code, stderr)
	}
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "prove-observe", "--proof", "x"}, strings.NewReader(valid), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "invalid-arguments") {
		t.Fatalf("flag: exit %d stderr %q", code, &stderr)
	}
}
