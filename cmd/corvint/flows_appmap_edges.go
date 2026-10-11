package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

var errAppmapEdgesUnproven = errors.New("required application edges are not proven")

func runFlowsAppmapEdges(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap edges", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application map")
	revision := f.String("revision", "", "evaluated revision")
	declarations := f.String("declarations", "", "edge proves/assumes JSON sidecar")
	budget := f.Int("budget", 0, "byte cap")
	full := f.Bool("full", false, "full byte cap")
	var receipts, required, observations []string
	f.Func("receipt", "receipt file (repeatable)", func(v string) error { receipts = append(receipts, v); return nil })
	f.Func("require", "required edge ID (repeatable)", func(v string) error { required = append(required, v); return nil })
	f.Func("observation", "navigation observation file (repeatable)", func(v string) error { observations = append(observations, v); return nil })
	if f.Parse(args) != nil || f.NArg() != 0 || *mapFile == "" {
		return errors.New("flows appmap edges requires --map FILE; optional --receipt, --revision, --declarations, --require, --observation, --budget or --full")
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	v, err := appmap.LoadVerification(m, receipts, nil)
	if err != nil {
		return err
	}
	var d *appmap.EdgeDeclarations
	if *declarations != "" {
		d, err = appmap.LoadEdgeDeclarations(*declarations)
		if err != nil {
			return err
		}
	}
	observed, err := appmap.LoadObservations(m, observations)
	if err != nil {
		return err
	}
	data, ok, err := appmap.ProjectObservedEdges(ctx, m, v, d, observed, required, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	if _, err = out.Write(data); err != nil {
		return err
	}
	if !ok {
		return errAppmapEdgesUnproven
	}
	return nil
}
