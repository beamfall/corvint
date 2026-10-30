package main

import (
	"context"
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
	f.Func("source", "[ALIAS:]DIR application source directory (repeatable)", func(v string) error {
		sources = append(sources, v)
		return nil
	})
	bound := bindSourceRepos(f)
	if _, err := parseSourceVerb(f, args, 0, mapFile, "flows appmap refs requires --map FILE, --source [ALIAS:]DIR (repeatable), optional --repo ALIAS=ROOT (repeatable) or --workspace FILE, --no-workspace, --budget N or --full"); err != nil {
		return err
	}
	// An explicit --budget 0 is out of range, not the default, and still conflicts with --full
	// (AMAP-V0-090/096); zero sources refuse in CheckRefs with appmap-invalid-query.
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	repos, err := bound.resolve(root)
	if err != nil {
		return err
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	report, err := appmap.CheckRefs(ctx, m, sources, appmap.Options{Root: root, Budget: *budget, Full: *full, Repos: repos})
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
