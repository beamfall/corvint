// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
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
// hold every row, or two that both do, leave the rows dropped. The sealed
// CEMs are real cem prepare, cite and mark output, whose evidence spans are
// bytes of the pinned blob while the plan's spans are BASE lines.
func TestProposeCitationsDCWV0885RefreshCarriesOrdinalsThroughSealedCEM(t *testing.T) {
	const added = "hunk:sha256:3333333333333333333333333333333333333333333333333333333333333333"
	plan := "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n3\tunknown\tinsufficient-evidence\tno caller yet\n"
	planPath := writeInput(t, plan)
	for name, tc := range map[string]struct {
		firstLines string
		seals      int
		want       bool
	}{
		"one matching seal": {"3:3", 1, true},
		"seal cites other":  {"4:4", 1, false},
		"two matching":      {"3:3", 2, false},
	} {
		t.Run(name, func(t *testing.T) {
			root, base := preflightRepository(t, map[string]string{"z.txt": "added\n"})
			cem := func(args ...string) {
				t.Helper()
				var stdout, stderr bytes.Buffer
				if code := cemcli.Run(context.Background(), root, args, &stdout, &stderr); code != 0 {
					t.Fatalf("cem %q = %d\n%s%s", args, code, &stdout, &stderr)
				}
			}
			const mapArg = ".corvint/change.cem.json"
			cem("prepare", "--base", base, "--target", testGit(t, root, "rev-parse", "HEAD"))
			cem("cite", "--map", mapArg, "--hunk", "1", "--evidence-path", "pre.go", "--lines", tc.firstLines, "--relation", "implementation")
			cem("cite", "--map", mapArg, "--hunk", "2", "--evidence-path", "pre.go", "--lines", "20:20", "--relation", "implementation")
			cem("mark", "--map", mapArg, "--hunk", "3", "--disposition", "unknown", "--reason", "insufficient-evidence")
			data, err := os.ReadFile(filepath.Join(root, mapArg))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, mapArg)); err != nil {
				t.Fatal(err)
			}
			var sealedMap citationMap
			if err := json.Unmarshal(data, &sealedMap); err != nil || len(sealedMap.Hunks) != 3 {
				t.Fatalf("sealed map = %v, %v\n%s", sealedMap.Hunks, err, data)
			}
			first, second, gone := sealedMap.Hunks[0].ID, sealedMap.Hunks[1].ID, sealedMap.Hunks[2].ID
			files := map[string]string{}
			for _, commit := range []string{strings.Repeat("a", 40), strings.Repeat("b", 40)}[:tc.seals] {
				files[".corvint/changes/"+commit+".cem.json"] = string(data)
			}
			writeTree(t, root, files)
			testGit(t, root, "add", ".corvint/changes")
			testGit(t, root, "commit", "-q", "-m", "seal change evidence")
			owed := []proposedHunk{{ordinal: 1, id: added, path: "a.go"}, {ordinal: 2, id: first, path: "pre.go"}, {ordinal: 3, id: second, path: "pre.go"}}
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
				!strings.Contains(notes, "refresh row 3 dropped: hunk "+gone) {
				t.Errorf("notes do not name the seal and the one unmapped row:\n%s", notes)
			}
		})
	}
}
