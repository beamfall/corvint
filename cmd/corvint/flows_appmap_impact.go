package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapImpact implements `flows appmap impact` (ANL-V0-015, ANL-V0-016): read-only.
func runFlowsAppmapImpact(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap impact", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	base := f.String("base", "", "base commit the change is measured from")
	ledger := f.String("ledger", "", "application-edge-ledger/0 file")
	revision := f.String("revision", "", "evaluated revision (default HEAD)")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	if f.Parse(args) != nil || f.NArg() != 0 || *mapFile == "" || *base == "" {
		return errors.New("flows appmap impact requires --map FILE and --base COMMIT; optional --ledger, --revision, --budget or --full")
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
	if root == "" {
		root = "."
	}
	data, err := appmap.Impact(ctx, m, l, *base, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
