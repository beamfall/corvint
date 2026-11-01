package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapCoverage prints the per-module reached/total screen report (AMAP-V0-810..812).
func runFlowsAppmapCoverage(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap coverage", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap coverage requires --map FILE and optional --budget N or --full")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	data, err := appmap.ProjectModuleCoverage(ctx, m, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
