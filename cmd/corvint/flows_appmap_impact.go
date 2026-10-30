package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapImpact implements `flows appmap impact` (ANL-V0-015, ANL-V0-016; observed bindings
// from --records, OUE-V0-070..074; the endpoint join, AMAP-V0-220): read-only.
func runFlowsAppmapImpact(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap impact", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	base := f.String("base", "", "base commit the change is measured from")
	ledger := f.String("ledger", "", "application-edge-ledger/0 file")
	records := f.String("records", "", "observed-ui-records/0 file whose bindings add screens and tests")
	revision := f.String("revision", "", "evaluated revision (default HEAD)")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	eo := endpointFlags(f, "endpoint-source")
	bound := bindSourceRepos(f)
	if f.Parse(args) != nil || f.NArg() != 0 || *mapFile == "" || *base == "" {
		return errors.New("flows appmap impact requires --map FILE and --base COMMIT; optional --ledger, --records, --revision, --budget or --full, and --endpoint-source [ALIAS:]DIR with --rails-routes [ALIAS:]FILE and/or --openapi [ALIAS:]FILE and --repo ALIAS=ABSOLUTE_ROOT")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	var l *appmap.EdgeLedger
	if *ledger != "" {
		if l, err = appmap.LoadEdgeLedger(*ledger); err != nil {
			return err
		}
	}
	var rec *appmap.UIRecords
	if *records != "" {
		if rec, err = appmap.LoadUIRecords(*records); err != nil {
			return err
		}
	}
	if root == "" {
		root = "."
	}
	repos, err := bound.resolve(root)
	if err != nil {
		return err
	}
	data, err := appmap.ImpactWithRecords(ctx, m, l, rec, *base, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full, Endpoints: endpointsGiven(eo), Repos: repos})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
