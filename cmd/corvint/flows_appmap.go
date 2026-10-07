package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/rootalias"
)

func init() { flowSubcommands["appmap"] = runFlowsAppmap }

// flowsAppmapHelp documents `flows appmap`; help.go appends it to flowsHelp.
const flowsAppmapHelp = `
Application map usage (experimental, AMAP-V0):
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS]
  corvint [--root PATH] flows appmap screen --map FILE --screen ID|STATE|TEMPLATE|URL [--budget N | --full] [--revision REV]
      [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...
  corvint [--root PATH] flows appmap flow --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]
      [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...
  corvint [--root PATH] flows appmap find --map FILE --text TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap scaffold --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]

build compiles application-map/0 from the manifest's router definitions, flow intents and
test suite read at REV (default HEAD) and writes it to stdout. A manifest whose tests name
"repo": ALIAS reads them from the root --repo binds to ALIAS, at its HEAD commit, which the
map pins; --manifest-repo reads the manifest from such a root. An undeclared alias, a root
that is not a Git worktree, or uncommitted changes under what the map reads from it refuse.
Projections read only --root, so anchors from an aliased root read UNKNOWN. screen, flow, find and
scaffold are capped projections of a map file: each writes one JSON document within
--budget bytes (256..65536; defaults 4096, 6144, 2048 and 6144) or --full (1 MiB), with
per-section omitted counts. Anchors are checked against --revision (default HEAD) and
read FRESH, STALE or UNKNOWN. Unresolved routes, imports and steps read UNKNOWN. None of
these commands writes to the repository.

Run verification (experimental, RVN-V0): screen and flow accept up to 16 Playwright
external provider receipts (--receipt) and up to 64 agent-declared bindings (--bind). Each
printed step then gets a learned fact (source run-verification) whose kind is VERIFIED at the
receipt's application revision, UNVERIFIED_AT_HEAD when its source changed since, CONTRADICTED
by a failing outcome, or unverified with a reason. Verification is learned evidence; it never changes a node, edge,
selector strength or freshness. A --bind test key absent from every receipt refuses.
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
		manifestRepo := f.String("manifest-repo", "", "alias of the root holding the manifest")
		roots := appmap.Roots{Repos: map[string]string{}}
		// --repo uses the multi-root MCP spelling and alias rules (MMR-V0-001, MMR-V0-002).
		f.Func("repo", "ALIAS=ABSOLUTE_ROOT (repeatable)", func(v string) error {
			alias, path, aliased := rootalias.Split(v)
			if !aliased || path == "" || roots.Repos[alias] != "" || len(roots.Repos) == rootalias.MaxRoots {
				return errors.New("invalid --repo")
			}
			roots.Repos[alias] = path
			return nil
		})
		if f.Parse(args) != nil || *manifest == "" || f.NArg() != 0 || (*manifestRepo != "" && !rootalias.Valid(*manifestRepo)) {
			return errors.New("flows appmap build requires --manifest FILE, optional --revision REV, --repo ALIAS=ABSOLUTE_ROOT (repeatable) and --manifest-repo ALIAS")
		}
		if *revision == "" {
			*revision = "HEAD"
		}
		roots.ManifestRepo = *manifestRepo
		m, err := appmap.BuildRoots(ctx, root, *manifest, *revision, roots)
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
	var receipts, binds []string
	if verb == "screen" || verb == "flow" {
		f.Func("receipt", "Playwright external provider receipt (repeatable)", func(v string) error { receipts = append(receipts, v); return nil })
		f.Func("bind", "STEP_ID=TEST_KEY binding (repeatable)", func(v string) error { binds = append(binds, v); return nil })
	}
	usage := "flows appmap " + verb + " requires --map FILE --" + queryFlag + " VALUE, optional --budget N or --full, and --revision REV"
	if f.Parse(args) != nil || *mapFile == "" || *query == "" || f.NArg() != 0 {
		return errors.New(usage)
	}
	m, err := appmap.LoadMap(*mapFile)
	if err != nil {
		return err
	}
	verification, err := appmap.LoadVerification(m, receipts, binds)
	if err != nil {
		return err
	}
	o := appmap.Options{Root: root, Revision: *revision, Budget: *budget, Full: *full}
	if verification != nil {
		o.Overlays = []appmap.Overlay{verification.Overlay(m, o)}
	}
	data, err := project(ctx, m, *query, o)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
