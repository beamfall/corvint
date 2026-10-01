package workflow

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// statusAfterCiting prepares base..target, cites every open hunk to
// evidencePath lines 1:1, commits the candidate and returns canonical status.
func statusAfterCiting(t *testing.T, root, base, target, evidencePath string) map[string]any {
	t.Helper()
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for {
		session := openSession(t, root)
		_, document, err := session.readMapInput(wire.ExcludedCEMPath)
		if err != nil {
			t.Fatal(err)
		}
		open := ""
		for _, hunk := range document.Hunks {
			if hunk.Disposition != "supported" {
				open = hunk.ID
				break
			}
		}
		if open == "" {
			break
		}
		if _, err := session.Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: open, EvidencePath: evidencePath, Lines: "1:1", Relation: "specification",
		}); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestStatusReportsSelfModifiedAuthority is CEM-CB-026: a change that edits a
// governing instruction file and cites that file is reported, naming the
// changing hunk and every citing basis, without changing the status state.
func TestStatusReportsSelfModifiedAuthority(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "AGENTS.md", "# Rules\nreview every change\nkeep tests\n")
	writeFile(t, root, "src/app.txt", "alpha\nbeta\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "AGENTS.md", "# Rules\nreview every change\nskip tests\n")
	writeFile(t, root, "src/app.txt", "alpha\nBETA\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")

	result := statusAfterCiting(t, root, base, target, "AGENTS.md")
	if result["state"] != "ready-for-ci" || result["ok"] != true {
		encoded, _ := json.Marshal(result)
		t.Fatalf("the report must not change status state: %s", encoded)
	}
	rows := result["selfModifiedAuthority"].([]any)
	if len(rows) != 1 {
		t.Fatalf("selfModifiedAuthority = %v, want one row", rows)
	}
	row := rows[0].(map[string]any)
	hunks, citedBy := row["hunks"].([]any), row["citedBy"].([]any)
	if row["path"] != "AGENTS.md" || len(hunks) != 1 || len(citedBy) != 2 {
		t.Fatalf("row = %v, want AGENTS.md changed by one hunk and cited by both", row)
	}
	for _, entry := range citedBy {
		cited := entry.(map[string]any)
		if cited["relation"] != "specification" || cited["evidenceId"] == "" || cited["hunk"] == "" {
			t.Fatalf("citation = %v", cited)
		}
	}
}

// TestStatusOmitsSelfModifiedAuthorityWithoutAGoverningChange: a change that
// edits no governing file keeps the status envelope byte-compatible.
func TestStatusOmitsSelfModifiedAuthorityWithoutAGoverningChange(t *testing.T) {
	root, base, target := makeRepo(t)
	result := statusAfterCiting(t, root, base, target, "docs/rule.txt")
	if result["state"] != "ready-for-ci" {
		t.Fatalf("state = %v", result["state"])
	}
	if _, present := result["selfModifiedAuthority"]; present {
		t.Fatalf("selfModifiedAuthority present without a governing change: %v", result["selfModifiedAuthority"])
	}
}

// TestSelfModifiedAuthorityListsAnUncitedGoverningChange: the governing file
// is reported when it changes even when nothing cites it, and an evidence row
// on an unchanged governing file is not reported.
func TestSelfModifiedAuthorityListsAnUncitedGoverningChange(t *testing.T) {
	t.Parallel()
	document := &wire.Map{
		Evidence: []wire.Evidence{{ID: "e1", Path: "CLAUDE.md"}, {ID: "e2", Path: "docs/rule.txt"}},
		Hunks: []wire.Hunk{
			{ID: "1", Path: "pkg/AGENTS.md", Basis: []wire.Basis{{EvidenceID: "e1", Relation: "decision"}}},
			{ID: "2", Path: ".github/instructions/go.instructions.md", Basis: []wire.Basis{{EvidenceID: "e2", Relation: "specification"}}},
			{ID: "3", Path: "src/app.go"},
		},
	}
	rows := selfModifiedAuthority(document)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	first, second := rows[0].(map[string]any), rows[1].(map[string]any)
	if first["path"] != ".github/instructions/go.instructions.md" || second["path"] != "pkg/AGENTS.md" {
		t.Fatalf("paths = %v, %v", first["path"], second["path"])
	}
	if len(first["citedBy"].([]any)) != 0 || len(second["citedBy"].([]any)) != 0 {
		t.Fatalf("an unchanged governing file was reported as cited: %v", rows)
	}
	if selfModifiedAuthority(&wire.Map{Hunks: []wire.Hunk{{ID: "1", Path: "docs/AGENTS.txt"}}}) != nil {
		t.Fatal("a non-instruction path was reported")
	}
}
