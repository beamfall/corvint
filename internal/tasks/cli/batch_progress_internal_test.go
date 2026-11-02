// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestTLFV01101_AncestryFailureAbstains: when the merge reaches a mark's
// head but the first-parent ancestry query fails, the mark delivers nothing
// and names why; a failed query is never read as "not in the parent", so
// it never becomes delivery proof (TLF-V0-1101).
func TestTLFV01101_AncestryFailureAbstains(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "base")
	base := run("rev-parse", "HEAD")
	lane := run("commit-tree", "HEAD^{tree}", "-p", base, "-m", "lane")
	merge := run("commit-tree", "HEAD^{tree}", "-p", base, "-p", lane, "-m", "batch")
	failParent := func(root, a, b string) (bool, error) {
		if b == base {
			return false, errors.New("git merge-base failed: injected")
		}
		return store.Ancestry(root, a, b)
	}
	in := &batchInput{number: 50, batch: &ghPR{MergeCommit: &struct {
		Oid string `json:"oid"`
	}{Oid: merge}}, root: root, ancestry: failParent}
	head := lane
	p := &ticket.Progress{ProgressMark: ticket.ProgressMark{Phase: ticket.PhaseMergedAwaitingCloseout, Head: &head}}
	if _, ok, why := in.progressDelivery(p); ok || !strings.Contains(why, "NOT_OBSERVED") || !strings.Contains(why, "injected") {
		t.Fatalf("a failed ancestry query delivered (ok=%v why=%q)", ok, why)
	}
	in.ancestry = nil
	if _, ok, why := in.progressDelivery(p); !ok || why != "" {
		t.Fatalf("a merged head with readable ancestry: ok=%v why=%q", ok, why)
	}
}
