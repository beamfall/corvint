package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appflows"
)

func init() { flowSubcommands["navigate"] = runFlowsNavigate }

// flowsNavigateHelp documents `flows navigate`; help.go appends it to flowsHelp.
const flowsNavigateHelp = `
Navigation usage:
  corvint [--root PATH] flows navigate --flows DIR [--evidence FILE]... [--traffic FILE]... [--goal FLOW_ID [--max-effect CLASS]]

navigate writes application-navigation-map/0 from the intents' navigation blocks at
HEAD: states, and per step its locator, readiness condition, input fixture ID,
expected outcomes, effect class and verified state. Observed application-flow-traffic/0
records raise a declared read to write-irreversible and never lower a class; an
undeclared class is write-irreversible. --goal writes application-navigation-packet/0:
the precondition flows then the goal in run order, each step above --max-effect
(read, write-reversible, write-irreversible or external-side-effect; default read)
marked requires-grant. It writes nothing to the repository.
`

func runFlowsNavigate(ctx context.Context, root string, args []string, out io.Writer) error {
	f, dir, evidence := flowQueryFlags("flows navigate")
	traffic := &[]string{}
	f.Func("traffic", "application-flow-traffic/0 JSONL file", func(s string) error {
		*traffic = append(*traffic, s)
		return nil
	})
	goal := f.String("goal", "", "flow to reach")
	maxEffect := f.String("max-effect", appflows.EffectRead, "highest effect class granted")
	parseErr := f.Parse(args)
	effectSet := false
	f.Visit(func(fl *flag.Flag) { effectSet = effectSet || fl.Name == "max-effect" })
	if parseErr != nil || *dir == "" || f.NArg() != 0 || (effectSet && *goal == "") {
		return errors.New("flows navigate requires --flows DIR, optional repeatable --evidence FILE and --traffic FILE, and --max-effect CLASS only with --goal FLOW_ID")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	records, err := appflows.ReadRunEvidence(*evidence)
	if err != nil {
		return err
	}
	observed, err := appflows.ReadTraffic(*traffic)
	if err != nil {
		return err
	}
	var data []byte
	if *goal == "" {
		data, err = appflows.FlowNavigationMap(ctx, root, set, records, observed)
	} else {
		data, err = appflows.FlowNavigationPacket(ctx, root, set, records, observed, *goal, *maxEffect)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
