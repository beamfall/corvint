package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

func init() { flowSubcommands["appmap"] = runFlowsAppmap }

// flowsAppmapHelp documents `flows appmap`; help.go appends it to flowsHelp.
const flowsAppmapHelp = `
Application map usage (experimental, AMAP-V0):
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV]
  corvint [--root PATH] flows appmap screen --map FILE --screen ID|STATE|TEMPLATE|URL [--budget N | --full] [--revision REV]
  corvint [--root PATH] flows appmap flow --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]
  corvint [--root PATH] flows appmap find --map FILE --text TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap scaffold --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]

build compiles application-map/0 from the manifest's router definitions, flow intents and
test suite read at REV (default HEAD) and writes it to stdout. screen, flow, find and
scaffold are capped projections of a map file: each writes one JSON document within
--budget bytes (256..65536; defaults 4096, 6144, 2048 and 6144) or --full (1 MiB), with
per-section omitted counts. Anchors are checked against --revision (default HEAD) and
read FRESH, STALE or UNKNOWN. Unresolved routes, imports and steps read UNKNOWN. None of
these commands writes to the repository.
`

func runFlowsAppmap(ctx context.Context, root string, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("flows appmap requires build, screen, flow, find, scaffold or plan")
	}
	verb, args := args[0], args[1:]
	f := flag.NewFlagSet("flows appmap "+verb, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	revision := f.String("revision", "", "revision to compile or to check anchors against")
	if verb == "build" {
		manifest := f.String("manifest", "", "tracked manifest path inside the root")
		if f.Parse(args) != nil || *manifest == "" || f.NArg() != 0 {
			return errors.New("flows appmap build requires --manifest FILE and optional --revision REV")
		}
		if *revision == "" {
			*revision = "HEAD"
		}
		m, err := appmap.Build(ctx, root, *manifest, *revision)
		if err != nil {
			return err
		}
		data, err := appmap.Encode(m)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	}
	if verb == "plan" {
		return runFlowsAppmapPlan(ctx, root, args, out)
	}
	type projector func(context.Context, *appmap.Map, string, appmap.Options) ([]byte, error)
	project, queryFlag := map[string]projector{
		"screen": appmap.ProjectScreen, "flow": appmap.ProjectFlow, "find": appmap.ProjectFind, "scaffold": appmap.ProjectScaffold,
	}[verb], map[string]string{"screen": "screen", "flow": "flow", "find": "text", "scaffold": "flow"}[verb]
	if project == nil {
		return errors.New("flows appmap requires build, screen, flow, find, scaffold or plan")
	}
	mapFile := f.String("map", "", "application-map/0 file")
	query := f.String(queryFlag, "", "query")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise the cap to the full ceiling")
	usage := "flows appmap " + verb + " requires --map FILE --" + queryFlag + " VALUE, optional --budget N or --full, and --revision REV"
	if f.Parse(args) != nil || *mapFile == "" || *query == "" || f.NArg() != 0 {
		return errors.New(usage)
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	data, err := project(ctx, m, *query, appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
