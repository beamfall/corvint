// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/watch"
)

// runLineEvidenceWatch uses the same default planner as affected, test and MCP.
// No argv from an evidence file is executed: execution replans through test.
func runLineEvidenceWatch(ctx context.Context, root, index string, ix *linecov.Index, source *watch.Source, once, dryRun bool, out, diagnostic io.Writer) int {
	observed := *ix
	observed.Language = "line-evidence"
	failed := false
	o := watch.Options{Root: root, Index: &observed, Saves: source.C, Stopped: source.Err, Out: out,
		Inventory: func(ctx context.Context) any { return readInventoryVerdict(ctx, root, true) },
		WorkingTree: func(ctx context.Context) (any, error) {
			invocation := affectedInvocation{Root: root, lineEvidenceIndex: index, lineEvidenceFamily: ix.RunnerFamily, lineEvidenceConfig: ix.Config}
			receipt, _, err := compileAffectedGraph(ctx, invocation)
			return receipt, err
		}}
	if !dryRun {
		o.Run = func(ctx context.Context, _ watch.Plan) error {
			git, err := exec.LookPath("git")
			if err != nil {
				return err
			}
			head, err := affectedHeadRevision(ctx, git, root)
			if err != nil {
				return err
			}
			code := runTestWithEvidence(ctx, root, []string{"--base", head}, io.Discard, diagnostic, affectedInvocation{lineEvidenceIndex: index, lineEvidenceFamily: ix.RunnerFamily, lineEvidenceConfig: ix.Config})
			if code != 0 {
				failed = true
				return fmt.Errorf("test exited %d", code)
			}
			return nil
		}
	}
	if err := watch.Run(ctx, o); err != nil {
		return watchRefuse(diagnostic, "watch-failed", err.Error())
	}
	if once && failed {
		return 1
	}
	return 0
}
