package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// gateFailure is a threshold miss, not a refusal: the report is already written and the exit is 1.
type gateFailure struct{ message string }

func (g *gateFailure) Error() string { return g.message }

func runFlowsAppmapStats(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap stats", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	minResolved := f.Float64("min-resolved", -1, "exit 1 when resolved screens fall below this percentage")
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap stats requires --map FILE, optional --budget N or --full, and --min-resolved PCT (0..100)")
	}
	gated := false
	f.Visit(func(fl *flag.Flag) { gated = gated || fl.Name == "min-resolved" })
	if gated && !(*minResolved >= 0 && *minResolved <= 100) { // also refuses NaN
		return errors.New("flows appmap stats: --min-resolved must be a number from 0 to 100")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	data, err := appmap.ProjectStats(ctx, m, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	if _, err = out.Write(data); err != nil {
		return err
	}
	if sum := appmap.Stats(m); gated && sum.ResolvedPercent() < *minResolved {
		return &gateFailure{fmt.Sprintf("flows appmap stats: resolved %.2f%% (%d of %d screens) is below --min-resolved %.2f; dominant unknown reasons: %s",
			sum.ResolvedPercent(), sum.Resolved, sum.Total, *minResolved, strings.Join(sum.Dominant, ", "))}
	}
	return nil
}
