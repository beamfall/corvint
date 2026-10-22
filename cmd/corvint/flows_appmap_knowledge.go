package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapKnowledge prints the site-knowledge page-object model (pageobjects,
// SKH-V0-010..014) or the test-body step IR (steps, SKH-V0-020..023). Refusals exit 2.
func runFlowsAppmapKnowledge(ctx context.Context, root, verb string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap "+verb, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var sources []string
	f.Func("source", "repository-relative test or page-object directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap " + verb + " requires --map FILE, --source DIR (repeatable) and optional --budget N or --full")
	}
	// An explicit --budget 0 is out of range, not the default (as for refs).
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	report := appmap.PageObjects
	if verb == "steps" {
		report = appmap.Steps
	}
	doc, err := report(ctx, m, sources, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
