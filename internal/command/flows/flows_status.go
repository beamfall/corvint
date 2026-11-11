// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"context"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/appflows"
)

func init() { flowSubcommands["status"] = runFlowsStatus }

// runFlowsStatus is `flows status`: one report of each flow, its variations, the tests attached to
// each and the last recorded result of each test, optionally limited to the flows a change reaches
// (AFU-V1-160 to AFU-V1-165). It reads intents at HEAD and run evidence files; it writes nothing.
func runFlowsStatus(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f, dir, evidence, _ := flowQueryFlags("flows status")
	base := f.String("base", "", "base commit that limits the report to the flows the change reaches")
	if f.Parse(args) != nil || *dir == "" || f.NArg() != 0 || f.Lookup("registry").Value.String() != "" {
		return errors.New("flows status requires --flows DIR, optional repeatable --evidence FILE and optional --base SHA")
	}
	if err := refuseUnreadableFlows(root); err != nil {
		return err
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	records, err := appflows.ReadRunEvidence(*evidence)
	if err != nil {
		return err
	}
	resolved := ""
	if *base != "" {
		if resolved, err = appflows.ResolveRevision(ctx, root, *base); err != nil {
			return &unresolvedBase{*base}
		}
	}
	data, err := appflows.FlowStatus(ctx, root, set, records, resolved)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
