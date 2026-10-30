package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
)

// runFlowsAppmapEndpoints prints the endpoint join (AMAP-V0-212..214); refusals exit 2.
func runFlowsAppmapEndpoints(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows appmap endpoints", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mapFile := f.String("map", "", "application-map/0 file")
	budget := f.Int("budget", 0, "byte budget")
	full := f.Bool("full", false, "raise cap to 1 MiB")
	eo := endpointFlags(f, "source")
	bound := bindSourceRepos(f)
	if f.Parse(args) != nil || *mapFile == "" || f.NArg() != 0 {
		return errors.New("flows appmap endpoints requires --map FILE, --source [ALIAS:]DIR (repeatable), --rails-routes [ALIAS:]FILE and/or --openapi [ALIAS:]FILE (repeatable), --repo ALIAS=ABSOLUTE_ROOT (repeatable), --budget N or --full")
	}
	// An explicit --budget 0 is out of range, not the default (AMAP-V0-090).
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "budget" && *budget == 0 {
			*budget = -1
		}
	})
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	repos, err := bound.resolve(root)
	if err != nil {
		return err
	}
	doc, err := appmap.Endpoints(ctx, m, *eo, appmap.Options{Root: root, Budget: *budget, Full: *full, Repos: repos})
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}

// endpointFlags registers the endpoint join inputs on f; the client source flag is named source
// (endpoints) or endpoint-source (impact) (AMAP-V0-212, AMAP-V0-220). Each takes ALIAS:PATH for a
// root the map pinned (AMAP-V0-222).
func endpointFlags(f *flag.FlagSet, sourceFlag string) *appmap.EndpointsOptions {
	eo := &appmap.EndpointsOptions{}
	f.Func(sourceFlag, "[ALIAS:]DIR client source directory (repeatable)", func(v string) error {
		eo.Sources = append(eo.Sources, v)
		return nil
	})
	f.Func("rails-routes", "[ALIAS:]FILE rails-routes/0 file (repeatable)", func(v string) error {
		eo.RailsRoutes = append(eo.RailsRoutes, v)
		return nil
	})
	f.Func("openapi", "[ALIAS:]FILE OpenAPI 2/3 document, JSON or block YAML (repeatable)", func(v string) error {
		eo.OpenAPI = append(eo.OpenAPI, v)
		return nil
	})
	return eo
}

// endpointsGiven returns eo when any endpoint join input was given, else nil.
func endpointsGiven(eo *appmap.EndpointsOptions) *appmap.EndpointsOptions {
	if len(eo.Sources)+len(eo.RailsRoutes)+len(eo.OpenAPI) == 0 {
		return nil
	}
	return eo
}
