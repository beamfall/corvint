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
