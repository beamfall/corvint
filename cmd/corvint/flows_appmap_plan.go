package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// flowsAppmapPlanHelp documents `flows appmap plan`; help.go appends it to flowsHelp.
const flowsAppmapPlanHelp = `
Scenario planner usage (experimental, AMSP-V0):
  corvint [--root PATH] flows appmap plan --map FILE [--map FILE]... (--step TEXT... | --request TEXT)
      [--draft] [--budget N | --full] [--revision REV] [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...

plan resolves each plain-language step (repeat --step, or one --request split at ';' and line
breaks) to one mapped flow of the given application maps (1..8, one per app) and writes one
application-map-plan/0 document within --budget bytes (default 16384) or --full: one browser
session per app, navigation that stays on the screen a previous step left, route parameters
handed from the step that reaches them, and each flow's named outcome assertions. A step with
no single matching flow, an unplaced flow step, or anchors STALE at --revision reads UNMAPPED
or STALE with the exploration it needs; nothing is guessed. --receipt and --bind take the same
Playwright receipts and step bindings as screen and flow (RVN-V0); a step's verification then
reads VERIFIED@REV only when its receipt ran at the evaluated --revision, else
UNVERIFIED_AT_HEAD, CONTRADICTED or unverified. Selectors and reused methods stay unverified, so
a step with either reads candidate, never run-verified. Without --receipt every element is
unverified. --draft adds a Playwright skeleton with one test.step per step. The command writes
nothing.
`

type repeated []string

func (r *repeated) String() string     { return strings.Join(*r, ",") }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

func runFlowsAppmapPlan(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap plan", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var maps, steps, receipts, binds repeated
	f.Var(&maps, "map", "application-map/0 file (repeatable)")
	f.Var(&steps, "step", "one plain-language step (repeatable)")
	f.Var(&receipts, "receipt", "Playwright external provider receipt (repeatable)")
	f.Var(&binds, "bind", "STEP_ID=TEST_KEY binding (repeatable)")
	request := f.String("request", "", "plain-language request split at ';' and line breaks")
	revision := f.String("revision", "", "revision to check anchors against")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise the cap to the full ceiling")
	draft := f.Bool("draft", false, "add the Playwright skeleton")
	if f.Parse(args) != nil || len(maps) == 0 || (len(steps) == 0) == (*request == "") || f.NArg() != 0 {
		return errors.New("flows appmap plan requires --map FILE (repeatable) and either --step TEXT (repeatable) or --request TEXT, optional --draft, --budget N or --full, --revision REV, --receipt FILE and --bind STEP_ID=TEST_KEY")
	}
	if *request != "" {
		steps = appmap.SplitRequest(*request)
	}
	loaded := make([]*appmap.Map, 0, len(maps))
	for _, name := range maps {
		m, err := appmap.LoadMap(name)
		if err != nil {
			return err
		}
		loaded = append(loaded, m)
	}
	verification, err := appmap.LoadPlanVerification(loaded, receipts, binds)
	if err != nil {
		return err
	}
	o := appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full}
	if verification != nil {
		base := o
		for _, m := range loaded {
			o.Overlays = append(o.Overlays, verification.Overlay(m, base))
		}
	}
	data, err := appmap.Plan(ctx, loaded, steps, appmap.PlanOptions{Options: o, Draft: *draft})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
