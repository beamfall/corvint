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
  corvint [--root PATH] flows appmap plan --map FILE --request GOAL --draft --source DIR [--source DIR]...
      [--condition TEXT]... [--ledger FILE] [--corpus ARTIFACT] [--budget N | --full]

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

With --source (experimental, SKH-V0 stage 7) the request is one goal: plan writes the
site-knowledge-how/0 answer of ` + "`flows appmap how`" + ` with a draft head field instead. The draft
copies the primary test's file at the map revision line for line, without the file's other tests,
and inserts a test.fixme guard after its declaration; each line names the source line and blob it
copies, and a step, condition or secret-shaped line it cannot copy is a "// TODO <gap>:" line, so
the draft reads INCOMPLETE. A test or setup mined from Cypress or Ruby abstains as
source-framework-unsupported. The draft is a proposal, never accepted intent; it writes nothing.
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
	var sources, conditions repeated
	f.Var(&sources, "source", "with --request GOAL --draft: test, page-object or template directory for the how draft (repeatable)")
	f.Var(&conditions, "condition", "with --source: precondition text (repeatable)")
	ledgerFile := f.String("ledger", "", "with --source: application-edge-ledger/0 file")
	corpus := f.String("corpus", "", "with --source: built documentation corpus artifact")
	parsed := f.Parse(args) == nil
	if parsed && len(sources) > 0 {
		if len(maps) != 1 || *request == "" || !*draft || len(steps) > 0 || len(receipts) > 0 || len(binds) > 0 || *revision != "" || f.NArg() != 0 {
			return errors.New("flows appmap plan --source requires one --map FILE, --request GOAL and --draft, optional --condition TEXT, --ledger FILE, --corpus ARTIFACT and --budget N or --full; --step, --receipt, --bind and --revision do not apply")
		}
		f.Visit(func(fl *flag.Flag) {
			if fl.Name == "budget" && *budget == 0 {
				*budget = -1
			}
		})
		return writeHow(ctx, root, maps[0], *corpus, *ledgerFile, sources, *request, conditions, appmap.Options{Root: root, Budget: *budget, Full: *full}, true, out)
	}
	if len(conditions) > 0 || *ledgerFile != "" || *corpus != "" {
		return errors.New("flows appmap plan --condition, --ledger and --corpus require --source")
	}
	if !parsed || len(maps) == 0 || (len(steps) == 0) == (*request == "") || f.NArg() != 0 {
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
