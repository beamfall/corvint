package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// statusAfterCiting prepares base..target, cites every open hunk to
// evidencePath lines 1:1, commits the candidate and returns canonical status.
// With handWritten the citation is written into the map directly, as any
// route other than cem cite could, since cem cite refuses a self-modified
// governing file (CEM-V1-114).
func statusAfterCiting(t *testing.T, root, base, target, evidencePath string, handWritten bool) map[string]any {
	t.Helper()
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for {
		session := openSession(t, root)
		_, document, err := session.readProducerMap(wire.ExcludedCEMPath)
		if err != nil {
			t.Fatal(err)
		}
		open := -1
		for index, hunk := range document.Hunks {
			if hunk.Disposition != "supported" {
				open = index
				break
			}
		}
		if open < 0 {
			break
		}
		if handWritten {
			blob := []byte(gitCmd(t, root, "show", base+":"+evidencePath))
			line := blob[:bytes.IndexByte(blob, '\n')+1]
			digest := sha256.Sum256(line)
			record := wire.Evidence{
				BlobOid: strings.TrimSpace(gitCmd(t, root, "rev-parse", base+":"+evidencePath)), Path: evidencePath,
				Span: wire.Span{Start: 0, End: int64(len(line))}, SpanSha256: hex.EncodeToString(digest[:]),
			}
			record.ID = wire.EvidenceIdentity(record.BlobOid, record.Path, record.Span, record.SpanSha256)
			if err := attachEvidence(document, open, record, "specification"); err != nil {
				t.Fatal(err)
			}
			if _, err := session.writeMap(document, wire.ExcludedCEMPath, ""); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := session.Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: document.Hunks[open].ID, EvidencePath: evidencePath, Lines: "1:1", Relation: "specification",
		}); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	head := strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
	return stableStatus(t, root, wire.ExcludedCEMPath, base, head, PolicyLimits{})
}

// selfModifiedRepo commits a base and a target that changes both AGENTS.md
// and src/app.txt and leaves the governing CLAUDE.md unchanged.
func selfModifiedRepo(t *testing.T) (root, base, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "AGENTS.md", "# Rules\nreview every change\nkeep tests\n")
	writeFile(t, root, "CLAUDE.md", "# Claude\nfollow AGENTS.md\n")
	writeFile(t, root, "src/app.txt", "alpha\nbeta\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base = gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "AGENTS.md", "# Rules\nreview every change\nskip tests\n")
	writeFile(t, root, "src/app.txt", "alpha\nBETA\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

// TestCiteRefusesSelfModifiedGoverningAuthority is CEM-V1-114: cem cite
// refuses, for every hunk, a stable span of a governing file the same patch
// changes, writes nothing, and still admits an unchanged span of another
// file the patch changes and a governing file the patch leaves unchanged.
func TestCiteRefusesSelfModifiedGoverningAuthority(t *testing.T) {
	t.Parallel()
	root, base, target := selfModifiedRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	before, document, err := session.readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, hunk := range document.Hunks {
		_, err := openSession(t, root).Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: hunk.ID, EvidencePath: "AGENTS.md", Lines: "1:1", Relation: "specification",
		})
		if cemcode.CodeOf(err) != cemcode.CiteSelfModifiedAuthority {
			t.Fatalf("hunk %s (%s): got %v, want cite-self-modified-authority", hunk.ID, hunk.Path, err)
		}
	}
	after, _, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a refused citation changed the map (err %v)", err)
	}
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: document.Hunks[0].ID, EvidencePath: "src/app.txt", Lines: "1:1", Relation: "implementation",
	}); err != nil {
		t.Fatalf("an unchanged span of a changed non-governing file was refused: %v", err)
	}
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: document.Hunks[1].ID, EvidencePath: "CLAUDE.md", Lines: "1:1", Relation: "specification",
	}); err != nil {
		t.Fatalf("a governing file the patch leaves unchanged was refused: %v", err)
	}
}

// TestStatusReportsSelfModifiedAuthority is CEM-V1-270: a change that edits a
// governing instruction file and cites that file is reported, naming the
// changing hunk and every citing basis, without changing the status state.
func TestStatusReportsSelfModifiedAuthority(t *testing.T) {
	t.Parallel()
	root, base, target := selfModifiedRepo(t)
	result := statusAfterCiting(t, root, base, target, "AGENTS.md", true)
	if result["state"] != stableStates["ACCEPT"] || result["ok"] != true {
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
	t.Parallel()
	root, base, target := makeRepo(t)
	result := statusAfterCiting(t, root, base, target, "docs/rule.txt", false)
	if result["state"] != stableStates["ACCEPT"] {
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
