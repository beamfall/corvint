package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const refusalLedger = ".corvint/self-observations.jsonl"

// diagnosticRefusalRepository is the impact fixture with an index snapshot written, so the
// SOL-V0-007 ledger is admissible and a previously produced artifact exists.
func diagnosticRefusalRepository(t *testing.T) string {
	t.Helper()
	root := impactCLIRepository(t)
	runIndexForTest(t, root, false)
	return root
}

func runWorkingTreeImpact(t *testing.T, root string, paths ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	arguments := append([]string{"--root", root, "impact", "--working-tree-untracked"}, paths...)
	exit := run(arguments, &forbiddenImpactReader{}, &stdout, &stderr)
	return exit, stdout.String(), stderr.String()
}

func ledgerRows(t *testing.T, root string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(refusalLedger)))
	if err != nil {
		t.Fatalf("no self-observation ledger, so the fixture proves nothing: %v", err)
	}
	rows := []map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		row := map[string]any{}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

// TestRefusalCodeSpellingsSurvive checks DRC-V0-006 over the converted family: the code and error
// bytes of a converted envelope are unchanged ahead of the additive members, an unconverted
// envelope of the same verb stays byte-identical, and the SOL-V0-007 row carries the code alone.
func TestRefusalCodeSpellingsSurvive(t *testing.T) {
	t.Parallel()
	root := diagnosticRefusalRepository(t)
	for _, test := range []struct {
		path, prefix, members string
	}{
		{
			path:    "pkg/main.go",
			prefix:  `{"code": "unsupported-working-tree-impact-path", "error": "working-tree impact accepts only paths absent from the captured revision: pkg/main.go", "evidence": [{"name": "revision", "value": "`,
			members: `, "ok": false, "subject": {"kind": "value", "value": "pkg/main.go"}, "supported_fixes": ["worktree-impact.remove-path", "impact.use-tracked-path-profile"]}` + "\n",
		},
		{
			path:    "pkg/notes.txt",
			prefix:  "{\"code\": \"unsupported-working-tree-impact-path\", \"error\": \"untracked-path impact is implemented for `.go` files only; commit or `git add -N` the file to use tracked-path impact\", \"evidence\": [], \"ok\": false",
			members: `, "subject": {"kind": "value", "value": "pkg/notes.txt"}, "supported_fixes": ["worktree-impact.remove-path", "git.add-intent-to-add", "impact.use-tracked-path-profile"]}` + "\n",
		},
	} {
		exit, _, stderr := runWorkingTreeImpact(t, root, test.path)
		if exit != 2 || !strings.HasPrefix(stderr, test.prefix) || !strings.HasSuffix(stderr, test.members) {
			t.Fatalf("path %s exit=%d stderr=%s", test.path, exit, stderr)
		}
	}
	unconverted := "{\"code\": \"invalid-working-tree-impact-path\", \"error\": \"working-tree impact path must be normalized and repository-relative: \\\"pkg/./new.go\\\"\", \"ok\": false}\n"
	if exit, _, stderr := runWorkingTreeImpact(t, root, "pkg/./new.go"); exit != 2 || stderr != unconverted {
		t.Fatalf("unconverted envelope changed: exit=%d stderr=%q", exit, stderr)
	}
	rows := ledgerRows(t, root)
	for _, row := range rows {
		if row["code"] != "unsupported-working-tree-impact-path" {
			t.Fatalf("ledger row code = %v", row)
		}
		for _, member := range []string{"subject", "evidence", "supported_fixes", "terminal"} {
			if _, leaked := row[member]; leaked {
				t.Fatalf("diagnostic member %s reached the ledger: %v", member, row)
			}
		}
	}
	if len(rows) != 2 {
		t.Fatalf("ledger rows = %d, want one per unsupported refusal", len(rows))
	}
}

// TestDiagnosticEmissionDoesNotMutate checks DRC-V0-007: emitting a diagnostic leaves every byte
// under the repository, .git and .corvint identical apart from the SOL-V0-007 ledger append.
func TestDiagnosticEmissionDoesNotMutate(t *testing.T) {
	t.Parallel()
	root := diagnosticRefusalRepository(t)
	before := repositoryBytesDigest(t, root, refusalLedger)
	if exit, _, stderr := runWorkingTreeImpact(t, root, "pkg/main.go"); exit != 2 || !strings.Contains(stderr, `"supported_fixes": [`) {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if after := repositoryBytesDigest(t, root, refusalLedger); after != before {
		t.Fatal("diagnostic emission changed repository or trace state beyond the ledger")
	}
	if rows := ledgerRows(t, root); len(rows) != 1 {
		t.Fatalf("ledger rows = %d, want the one permitted append", len(rows))
	}
}

// TestRefusalIsNeverSuccess checks DRC-V0-010 at the CLI, the one layer carrying the converted
// family: a refusal exits non-zero with an ok:false envelope and no stdout, and the index snapshot
// produced before it is left untouched rather than presented as the refused candidate's result.
func TestRefusalIsNeverSuccess(t *testing.T) {
	t.Parallel()
	root := diagnosticRefusalRepository(t)
	before := repositoryBytesDigest(t, root, refusalLedger)
	exit, stdout, stderr := runWorkingTreeImpact(t, root, "main.go")
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatal(err)
	}
	if exit != 2 || stdout != "" || envelope["ok"] != false || envelope["code"] != "unsupported-working-tree-impact-path" {
		t.Fatalf("exit=%d stdout=%q envelope=%v", exit, stdout, envelope)
	}
	if after := repositoryBytesDigest(t, root, refusalLedger); after != before {
		t.Fatal("refusal changed the previously produced index snapshot")
	}
}
