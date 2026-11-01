// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0442RefreshOrdinalsNeedBinding pins two review
// findings on V1-3240. An ordinal --refresh row resolves only through the
// citation-plan binding recorded for that plan: once a later prepare has
// renumbered the map, the map on disk would carry the row onto another hunk,
// so without a matching binding the row is dropped with a note. A valid
// deferred<TAB>PATH<TAB>REASON row is carried as written, never dropped as a
// malformed hunk row (DCW-V0-370).
func TestProposeCitationsDCWV0442RefreshOrdinalsNeedBinding(t *testing.T) {
	root, _ := preflightRepository(t, nil)
	gitDir := strings.TrimSpace(testGit(t, root, "rev-parse", "--absolute-git-dir"))
	const (
		first  = "hunk:sha256:1111111111111111111111111111111111111111111111111111111111111111"
		second = "hunk:sha256:2222222222222222222222222222222222222222222222222222222222222222"
		added  = "hunk:sha256:3333333333333333333333333333333333333333333333333333333333333333"
	)
	plan := "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\ndeferred\tdocs/specs/x.md\tout of scope for this lane\n"
	planPath := writeInput(t, plan)
	// The current map numbers a newly added, earlier-sorting hunk first.
	owed := []proposedHunk{{ordinal: 1, id: added, path: "a.go"}, {ordinal: 2, id: first, path: "pre.go"}, {ordinal: 3, id: second, path: "pre.go"}}
	run := func() *proposal {
		var stderr bytes.Buffer
		p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &stderr}, options: ProposeOptions{Root: root, Refresh: planPath}}
		p.readRefreshPlan()
		p.carry(owed, map[string]bool{})
		return p
	}

	unbound := run()
	if len(unbound.carried) != 0 {
		t.Errorf("ordinal rows carried without a binding: %q", unbound.carried)
	}
	if notes := strings.Join(unbound.carryNotes, "\n"); strings.Count(notes, "cannot be resolved, because no citation-plan binding") != 2 || !strings.Contains(notes, "full hunk ID") {
		t.Errorf("unbound ordinal rows were not dropped with a recovery:\n%s", notes)
	}
	if want := []string{"deferred\tdocs/specs/x.md\tout of scope for this lane"}; strings.Join(unbound.refreshDeferred, "\n") != want[0] {
		t.Errorf("deferred row = %q, want it carried as written", unbound.refreshDeferred)
	}

	// The binding the bind loop recorded for this plan (its deferred rows
	// split off) names the hunks its ordinals meant.
	stripped, _, _ := splitDeferredRows([]byte(plan))
	if err := os.MkdirAll(filepath.Join(gitDir, "corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "corvint", "citation-plan-binding"), []byte(sha256Hex(stripped)+"\n"+first+"\n"+second+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bound := run()
	if got := bound.carried[first]; len(got) != 1 || got[0] != first+"\tpre.go\t3:3\timplementation" {
		t.Errorf("ordinal 1 carried to %q, want hunk %s", bound.carried, first)
	}
	if got := bound.carried[second]; len(got) != 1 || got[0] != second+"\tpre.go\t20:20\timplementation" {
		t.Errorf("ordinal 2 carried to %q, want hunk %s", bound.carried, second)
	}
	if _, ok := bound.carried[added]; ok {
		t.Errorf("the added hunk took a carried row: %q", bound.carried)
	}
}

// TestProposeCitationsDCWV0885RefreshCarriesOrdinalsThroughSealedCEM
// (V1-3293): after a seal moved the map to .corvint/changes/ and a later
// commit renumbered the hunks, an ordinal plan with no binding is carried
// through the sealed CEM whose hunks hold its rows; a sealed CEM that does not
// hold every row, or two that both do, leave the rows dropped.
func TestProposeCitationsDCWV0885RefreshCarriesOrdinalsThroughSealedCEM(t *testing.T) {
	const (
		first  = "hunk:sha256:1111111111111111111111111111111111111111111111111111111111111111"
		second = "hunk:sha256:2222222222222222222222222222222222222222222222222222222222222222"
		added  = "hunk:sha256:3333333333333333333333333333333333333333333333333333333333333333"
	)
	sealed := func(firstSpan string) string {
		return `{"evidence": [{"id": "e1", "path": "pre.go", "span": {"start": ` + firstSpan + `, "end": ` + firstSpan + `}}, {"id": "e2", "path": "pre.go", "span": {"start": 20, "end": 20}}], "hunks": [` +
			`{"id": "` + first + `", "disposition": "supported", "basis": [{"evidenceId": "e1", "relation": "implementation"}]}, ` +
			`{"id": "` + second + `", "disposition": "supported", "basis": [{"evidenceId": "e2", "relation": "implementation"}]}, ` +
			`{"id": "hunk:sha256:4444444444444444444444444444444444444444444444444444444444444444", "disposition": "unknown", "reason": "insufficient-evidence"}]}` + "\n"
	}
	plan := "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n3\tunknown\tinsufficient-evidence\tno caller yet\n"
	planPath := writeInput(t, plan)
	owed := []proposedHunk{{ordinal: 1, id: added, path: "a.go"}, {ordinal: 2, id: first, path: "pre.go"}, {ordinal: 3, id: second, path: "pre.go"}}
	for name, tc := range map[string]struct {
		seals map[string]string
		want  bool
	}{
		"one matching seal": {map[string]string{strings.Repeat("a", 40): sealed("3")}, true},
		"seal cites other":  {map[string]string{strings.Repeat("a", 40): sealed("4")}, false},
		"two matching":      {map[string]string{strings.Repeat("a", 40): sealed("3"), strings.Repeat("b", 40): sealed("3")}, false},
	} {
		t.Run(name, func(t *testing.T) {
			root, base := preflightRepository(t, nil)
			files := map[string]string{}
			for commit, data := range tc.seals {
				files[".corvint/changes/"+commit+".cem.json"] = data
			}
			writeTree(t, root, files)
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "seal change evidence")
			var stderr bytes.Buffer
			p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &stderr, base: base, target: testGit(t, root, "rev-parse", "HEAD")}, options: ProposeOptions{Root: root, Refresh: planPath}}
			p.readRefreshPlan()
			p.carry(owed, map[string]bool{})
			notes := strings.Join(p.carryNotes, "\n")
			if !tc.want {
				if len(p.carried) != 0 || strings.Count(notes, "cannot be resolved") != 3 {
					t.Errorf("rows carried without one matching seal: %q\n%s", p.carried, notes)
				}
				return
			}
			if got := p.carried[first]; len(got) != 1 || got[0] != first+"\tpre.go\t3:3\timplementation" {
				t.Errorf("ordinal 1 carried to %q, want hunk %s\n%s", p.carried, first, notes)
			}
			if got := p.carried[second]; len(got) != 1 || got[0] != second+"\tpre.go\t20:20\timplementation" {
				t.Errorf("ordinal 2 carried to %q, want hunk %s", p.carried, second)
			}
			if !strings.Contains(notes, "resolved through the sealed CEM .corvint/changes/"+strings.Repeat("a", 40)+".cem.json") ||
				!strings.Contains(notes, "refresh row 3 dropped: hunk hunk:sha256:4444") {
				t.Errorf("notes do not name the seal and the one unmapped row:\n%s", notes)
			}
		})
	}
}
