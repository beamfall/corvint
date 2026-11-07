package affected_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// ownershipAuditBaseline lists the tracked paths that were unowned when the
// AZU-V0-002 gate landed, one per line. It only shrinks: a path leaves it
// when a plugin claims it or the project declares it.
const ownershipAuditBaseline = "testdata/ownership-audit-baseline.txt"

// TestOwnershipAuditRatchet_AZU_V0_002 is the ownership completeness gate over
// this repository: it fails when a tracked path is unowned and not in the
// baseline, so no new path lands without a claimant or declaration, and when
// a baseline path is claimed or no longer tracked, so the baseline only
// shrinks. AZU-V0-002 asks for an empty audit at HEAD; the baseline is the
// remaining work toward it, and it never changes what a plan selects.
func TestOwnershipAuditRatchet_AZU_V0_002(t *testing.T) {
	root := repositoryRoot(t)
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	var tracked []string
	for _, name := range strings.Split(string(listing), "\x00") {
		if name != "" {
			tracked = append(tracked, name)
		}
	}
	body, err := os.ReadFile(filepath.FromSlash(ownershipAuditBaseline))
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]bool{}
	previous := ""
	for _, line := range strings.Fields(string(body)) {
		if line <= previous {
			t.Fatalf("%s: %q is not in strictly ascending byte order", ownershipAuditBaseline, line)
		}
		baseline[line] = true
		previous = line
	}
	unowned, err := affected.AuditOwnership(context.Background(), root, tracked, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	var added, stale []string
	for _, path := range unowned {
		if !baseline[path] {
			added = append(added, path)
		}
	}
	for path := range baseline {
		if !slices.Contains(unowned, path) {
			stale = append(stale, path)
		}
	}
	slices.Sort(stale)
	if len(added) != 0 {
		t.Errorf("%d tracked paths are unowned: claim each with a plugin, or declare it in %s, %s or %s (AZU-V0-002):\n%s",
			len(added), affected.DocumentationPathsPath, affected.RuntimeReadersPath, affected.NonSourcePathsPath, strings.Join(added, "\n"))
	}
	if len(stale) != 0 {
		t.Errorf("%d paths in %s are now claimed or no longer tracked; delete their lines:\n%s",
			len(stale), ownershipAuditBaseline, strings.Join(stale, "\n"))
	}
}
