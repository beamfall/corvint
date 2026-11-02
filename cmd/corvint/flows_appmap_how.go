package main

import (
	"context"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// howHelp documents `corvint how`, the top-level alias of `flows appmap how` (SKH-V0-810).
const howHelp = `Answer "how do I GOAL" from an application map (SKH-V0, experimental).

Usage:
  corvint [--root PATH] how --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--condition TEXT]...
      [--ledger FILE] [--corpus ARTIFACT] [--budget N | --full] GOAL

how is an alias of corvint flows appmap how: the same flags, the same
site-knowledge-how/0 bytes on stdout and the same refusals (exit 2). It writes
nothing. See corvint flows --help for the application map verbs.
`

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
	f.Func("source", "[ALIAS:]DIR test, page-object or template directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	bound := bindSourceRepos(f)
	f.Func("condition", "precondition text to resolve against setup records (repeatable)", func(v string) error {
		conditions = append(conditions, v)
		return nil
	})
	goal, err := parseSourceVerb(f, args, 1, mapFile, "flows appmap how requires --map FILE, --source [ALIAS:]DIR (repeatable), optional --repo ALIAS=ROOT (repeatable) or --workspace FILE, --no-workspace, --condition TEXT, --ledger FILE, --corpus ARTIFACT, --budget N or --full, and one GOAL")
	if err != nil {
		return err
	}
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	repos, err := bound.resolve(root)
	if err != nil {
		return err
	}
	return writeHow(ctx, root, *mapFile, *corpus, *ledgerFile, sources, goal[0], conditions, appmap.Options{Root: root, Budget: *budget, Full: *full, Repos: repos}, false, out)
}

// writeHow loads the map, corpus trust and ledger and writes the how answer, with the stage 7
// guarded draft when draft is set (`flows appmap plan --request GOAL --draft --source DIR`).
func writeHow(ctx context.Context, root, mapFile, corpus, ledgerFile string, sources []string, goal string, conditions []string, o appmap.Options, draft bool, out io.Writer) error {
	m, err := appmap.LoadMap(mapFile)
	if err != nil {
		return err
	}
	trust := []appmap.TestTrust{}
	if corpus != "" {
		if trust, err = corpusTrust(ctx, root, corpus, m.Revision); err != nil {
			return err
		}
	}
	var ledger *appmap.EdgeLedger
	if ledgerFile != "" {
		if ledger, err = appmap.LoadEdgeLedger(ledgerFile); err != nil {
			return err
		}
	}
	how := appmap.How
	if draft {
		how = appmap.HowDraft
	}
	doc, err := how(ctx, m, sources, goal, conditions, trust, ledger, o)
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
