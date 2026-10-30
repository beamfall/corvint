package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapBlocker prints the blocker 404 cross-check (AMAP-V0-140..146). A flagged cited
// request exits 1 after the report is written; refusals exit 2 (AMAP-V0-145).
func runFlowsAppmapBlocker(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap blocker", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	note := f.String("note", "", "blocker classification or run failure note")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap blocker requires --map FILE --note TEXT and optional --budget N or --full")
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
	report, err := appmap.CheckBlocker(ctx, m, *note, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	if _, err = out.Write(report.Document); err != nil {
		return err
	}
	if report.Flagged > 0 {
		return &gateFailure{fmt.Sprintf("flows appmap blocker: %d cited 404 requests flagged", report.Flagged)}
	}
	return nil
}
