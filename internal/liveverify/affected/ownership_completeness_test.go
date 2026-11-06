// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// AZU-V0-002, AZU-V0-8900: declarations add verification owners without
// suppressing the existing reader rules or the unowned-path frontier cascade.
func TestRepositoryOwnershipComplete_AZU_V0_8900(t *testing.T) {
	root := repositoryRoot(t)
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00")
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	if got := graph.OwnershipAudit(tracked); len(got) != 0 {
		t.Fatalf("unclaimed tracked paths (AZU-V0-002):\n%s", strings.Join(got, "\n"))
	}
	// Simulate git add of a new path outside every declaration. The audit
	// classifies the tracked listing, so a missing live body cannot hide it.
	probe := "azu-unclaimed-new-file.dat"
	if got := graph.OwnershipAudit(append(slices.Clone(tracked), probe)); !slices.Equal(got, []string{probe}) {
		t.Fatalf("new unclaimed tracked file escaped audit: %q", got)
	}
}
