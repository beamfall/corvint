package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapRefs prints the reference check (AMAP-V0-090..099). An unlabelled miss exits 1
// after the report is written; refusals exit 2 (AMAP-V0-098).
func runFlowsAppmapRefs(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap refs", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var sources []string
	f.Func("source", "repository-relative application source directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	if f.Parse(args) != nil || *mapFile == "" || len(sources) == 0 || f.NArg() != 0 {
		return errors.New("flows appmap refs requires --map FILE, --source DIR (repeatable) and optional --budget N or --full")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	report, err := appmap.CheckRefs(ctx, m, sources, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	if _, err = out.Write(report.Document); err != nil {
		return err
	}
	if report.Unlabelled > 0 {
		return &gateFailure{fmt.Sprintf("flows appmap refs: %d unlabelled reference misses (name-level check)", report.Unlabelled)}
	}
	return nil
}
