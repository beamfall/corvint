package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapActions prints the route-action check (AMAP-V0-100..107). A ROUTE-ACTION-MISSING
// finding exits 1 after the report is written; refusals exit 2 (AMAP-V0-106).
func runFlowsAppmapActions(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap actions", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap actions requires --map FILE and optional --budget N or --full")
	}
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	report, err := appmap.CheckActions(ctx, m, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	if _, err = out.Write(report.Document); err != nil {
		return err
	}
	if report.Findings > 0 {
		return &gateFailure{fmt.Sprintf("flows appmap actions: %d %s findings", report.Findings, appmap.RuleRouteActionMissing)}
	}
	return nil
}
