package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapHow prints the stage 6 task answer (site-knowledge-how/0) for one goal: the goal
// screen, the setup and conditions, the step path over admitted edges and the outcome oracle, or
// ABSTAINED with named gaps (SKH-V0-060..064, SKH-V0-143..152). It writes nothing. Refusals exit 2.
func runFlowsAppmapHow(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap how", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	corpus := f.String("corpus", "", "built documentation corpus artifact whose test-knowledge joins rank and block sources")
	ledgerFile := f.String("ledger", "", "application-edge-ledger/0 file built from this map")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	var sources, conditions []string
	f.Func("source", "repository-relative test, page-object or template directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	f.Func("condition", "precondition text to resolve against setup records (repeatable)", func(v string) error {
		conditions = append(conditions, v)
		return nil
	})
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 1 {
		return errors.New("flows appmap how requires --map FILE, --source DIR (repeatable), optional --condition TEXT, --ledger FILE, --corpus ARTIFACT, --budget N or --full, and one GOAL")
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
	trust := []appmap.TestTrust{}
	if *corpus != "" {
		if trust, err = corpusTrust(ctx, root, *corpus, m.Revision); err != nil {
			return err
		}
	}
	var ledger *appmap.EdgeLedger
	if *ledgerFile != "" {
		if ledger, err = appmap.LoadEdgeLedger(*ledgerFile); err != nil {
			return err
		}
	}
	doc, err := appmap.How(ctx, m, sources, f.Arg(0), conditions, trust, ledger, appmap.Options{Root: root, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
