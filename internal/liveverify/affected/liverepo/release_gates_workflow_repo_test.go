// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// AZU-V0-214: the repository's release-gates workflow reuses its setup steps
// through anchors; it parses, so an edit to it no longer raises
// driver:unparsable-file (V1-4999, V1-5184). The test reads the live
// workflow, so it lives here and not in the hub package, whose declared read
// scope is empty (AFP-V0-107).
func TestReleaseGatesWorkflowParses_AZU_V0_214(t *testing.T) {
	testsupport.RequireLiveTier(t)
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{".github/workflows/release-gates.yml"})
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.FrontierDriverUnparsableFile {
			t.Errorf("release-gates.yml is unparsable: %s", unknown.Detail)
		}
	}
}
