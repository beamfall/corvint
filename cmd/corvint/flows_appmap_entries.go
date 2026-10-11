package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

func runFlowsAppmapEntries(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap entries", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	revision := f.String("revision", "", "committed revision to check (default HEAD)")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap entries requires --map FILE, optional --revision REV and --budget N or --full; static only")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	data, err := appmap.ProjectEntries(ctx, m, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
