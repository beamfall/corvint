package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/cemdiscriminate"
)

// init installs the `cem discriminate` mutation runner (TCQ-V0-055..058) into
// the CEM workflow without adding internal/liveverify/mutate to the CEM seams'
// closure.
func init() { workflow.OpenHunkJudge = openCEMHunkJudge }

func openCEMHunkJudge(ctx context.Context, root, target string) (workflow.HunkJudge, error) {
	judge, err := cemdiscriminate.Open(ctx, root, target)
	if err != nil {
		return nil, err
	}
	return judge, nil
}
