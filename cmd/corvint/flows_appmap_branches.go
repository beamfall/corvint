package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapBranches prints the branch tests and setup query (AMAP-V0-110..116). It always
// exits 0 after the report is written; refusals exit 2.
func runFlowsAppmapBranches(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap branches", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var cites []string
	f.Func("cite", "PATH:START[-END]", func(s string) error { cites = append(cites, s); return nil })
	if f.Parse(args) != nil || *mapFile == "" || len(cites) == 0 || f.NArg() != 0 {
		return errors.New("flows appmap branches requires --map FILE, one or more --cite PATH:START[-END] and optional --budget N or --full")
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
	report, err := appmap.QueryBranches(ctx, m, cites, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(report.Document)
	return err
}
