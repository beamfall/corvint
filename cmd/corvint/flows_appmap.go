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
  corvint [--root PATH] flows appmap entries --map FILE [--ledger FILE] [--budget N | --full] [--revision REV]
  corvint [--root PATH] flows appmap stats --map FILE [--budget N | --full] [--min-resolved PCT]
  corvint [--root PATH] flows appmap refs --map FILE --source DIR [--source DIR]... [--budget N | --full]
  corvint [--root PATH] flows appmap pageobjects --map FILE --source DIR [--source DIR]... [--budget N | --full]
  corvint [--root PATH] flows appmap steps --map FILE --source DIR [--source DIR]... [--budget N | --full]
  corvint [--root PATH] flows appmap actions --map FILE [--budget N | --full]
  corvint [--root PATH] flows appmap blocker --map FILE --note TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap branches --map FILE --cite PATH:START[-END] [--cite ...] [--budget N | --full]
  corvint [--root PATH] flows appmap find --map FILE --text TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap scaffold --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]

Navigation edge ledger (experimental, ANL-V0):
  corvint [--root PATH] flows appmap edges --map FILE [--receipt FILE]... [--revision REV]
      [--declarations FILE] [--require EDGE_ID]... [--observation FILE]... [--budget N | --full]
Single-map declared adjacency ledger; UNKNOWN triggers retain missing interaction evidence.
The JSON sidecar declares tests with test_key, proves and assumes edge lists. --observation
(observe output) binds each observed crossing's test key to the matching edge; a receipt at the
observed revision still has to prove it. An assumed edge whose only prover is the assuming
test is unsatisfied. Unproven requirements exit 1; invalid inputs exit 2. No browser runs and no repository files change.

Navigation impact (experimental, ANL-V0):
  corvint [--root PATH] flows appmap impact --map FILE --base COMMIT [--ledger FILE]
      [--revision REV] [--budget N | --full]
Maps paths changed since --base to screens (router anchors, lineage descendants, attributed test
files, flow anchors), touched edges with ledger status and re-proving test, and spec files. An
unplaceable UI path (templates are not bound by the map) lists as unknown and the verdict is
full-suite-required. Learned, read-only, offline.

Navigation observation (experimental, ANL-V0):
  corvint flows appmap observe --map FILE --trace FILE|DIR|ZIP [--test-key KEY] [--revision REV]
      [--budget N | --full]
Reads playwright-navigation-events/0 reporter JSON, or one Playwright trace zip in memory (with
--test-key; navigations from main-frame snapshot URLs, never extracted to disk), and prints
application-navigation-observation/0: ordered screen visits resolved through the map, observed
edges with their typed trigger (goto, click, form submit, key press, history back/forward, reload,
redirect, else UNKNOWN) and any annotated declared_trigger, UNKNOWN joins and unmapped-url.
Secret-shaped query values and typed input values are redacted. Learned overlay facts only.
internal/appmap/navigation-events.cjs writes that JSON per test, with any trace setting, from one
line in playwright.config: require('<corvint>/internal/appmap/navigation-events.cjs')
.install({outputDir: 'DIR'}). Each run clears DIR's earlier documents; pass --trace DIR (at most
64 tests) or DIR/<testId>.json.

build compiles application-map/0 from the manifest's router definitions, flow intents and
test suite read at REV (default HEAD) and writes it to stdout. Router dialects are
ui-router-states/0, rails-routes/0 and react-router/0 (literal, experimental subsets).
Optional manifest router_globs report matched files absent from routers as router-not-declared
in app_coverage; this measures only the declared glob scope. A manifest whose tests name
"repo": ALIAS reads them from the root --repo binds to ALIAS, at its HEAD commit, which the
map pins; --manifest-repo reads the manifest from such a root. An undeclared alias, a root
that is not a Git worktree, or uncommitted changes under what the map reads from it refuse.
Projections read only --root, so anchors from an aliased root read UNKNOWN. screen, flow, find and
scaffold are capped projections of a map file: each writes one JSON document within
--budget bytes (256..65536; defaults 4096, 6144, 2048 and 6144) or --full (1 MiB), with
per-section omitted counts. Anchors are checked against --revision (default HEAD) and
read FRESH, STALE or UNKNOWN. Unresolved routes, imports and steps read UNKNOWN. None of
these commands writes to the repository.

Static entries (experimental, V1-1136): entries classifies UI-router entry candidates,
parameters and guards; default cap 6144 bytes. It never visits a URL or verifies loading.
Changed source reads UNVERIFIED_AT_HEAD. --ledger (application-edge-ledger/0 built from this map
at the evaluated revision) adds each screen's shortest path of PROVEN ledger edges from a
deep-linkable entry, with the proving test keys; a screen with none reads NOT_OBSERVED.

Qualification report (experimental, V1-1138): stats reads only the map file and reports
screens resolved/total, the top 5 unknown reasons with example anchors, test-join and
page-object binding counts, edges by basis and the selector strength histogram (default cap
6144 bytes). --min-resolved PCT exits 1 below the threshold, naming the dominant reasons.

Reference check (experimental, V1-1581): refs reads the map's test files and up to 16 --source
directories at the map revision and lists the routes, selector names (test ids, CSS id, class
and data-* names) and mocked endpoint segments tests reference that the source lacks, with kind,
value, match (exact or prefix) and test locations (default cap 6144 bytes). It is name-level
only. Templated values are prefixes; catch-all routes, regular-expression patterns and
application-templated names are labelled; text-level selectors and dynamic values are counted
as unchecked. An unlabelled miss exits 1; refusals exit 2. No repository file changes.

Site knowledge (experimental, SKH-V0): pageobjects reads up to 16 --source directories at the
map revision and lists page objects (Playwright and Cypress TypeScript classes and page-function
modules, Capybara helper classes, SitePrism pages) with their locator bindings, methods, effect
class (navigation-only, state-effect or unknown) and screen join. steps compiles each test body
to the closed step IR (visit, click, fill, select, check, upload, wait-for, assert, opaque),
inlining page-object and helper calls with bound arguments, and deduplicates repeated sequences
into reusable records with usage counts (default caps 6144 bytes). Unsupported constructs are
unknowns or opaque steps with a reason; secret-shaped values are screened. Learned authority,
static, no code runs and no repository file changes.

Route-action check (experimental, V1-1890): actions compiles every rails-routes/0 route (and its
one-level draws) to a controller and action and reports ROUTE-ACTION-MISSING when the handler
class, its ancestors, its included modules and its implicit view templates define no public
action, with the declaration line, handler file and action. only:/except: removals are honoured;
unresolvable handlers, ancestors, modules and dynamic definitions abstain with a reason. Other
dialects are not applicable. A finding exits 1; refusals exit 2. No repository file changes.

Blocker 404 cross-check (experimental, V1-1967): blocker reads a blocked/fixture-gap
classification or run failure note and resolves each METHOD /path cited on a line with a 404
against the compiled rails-routes/0 table. No matching route is flagged BLOCKER-404-NO-ROUTE
with up to 3 nearest routes; a matched route whose handler lacks the action is flagged
ROUTE-ACTION-MISSING; a matched route with a defined action is accepted; a mounted, rack or
redirect path, an uncompiled route or a non-literal request path is uncertain with a reason.
A flagged request exits 1; refusals exit 2. No repository file changes.

Branch tests and setup (experimental, V1-1891): branches reads up to 16 cited Ruby line ranges
and lists, per if/unless/elsif/when branch, the tests that name it grouped by boundary (unit,
request, e2e; "none at <boundary>" when none), and for a boundary not reached the guard's model
inputs with the setup endpoints, factories and fixtures that write them. An input it cannot
resolve is unresolved, never "no setup available". Static naming evidence, not coverage.

Run verification (experimental, RVN-V0): screen and flow accept up to 16 Playwright
external provider receipts (--receipt) and up to 64 agent-declared bindings (--bind). Each
printed step then gets a learned fact (source run-verification) whose kind is VERIFIED at the
receipt's application revision, UNVERIFIED_AT_HEAD when its source changed since, CONTRADICTED
by a failing outcome, or unverified with a reason. Verification is learned evidence; it never changes a node, edge,
selector strength or freshness. A --bind test key absent from every receipt refuses.
`

func runFlowsAppmap(ctx context.Context, root string, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("flows appmap requires build, screen, flow, find, scaffold, entries, stats, refs, pageobjects, steps, actions, blocker, branches, edges, observe, impact or plan")
	}
	verb, args := args[0], args[1:]
	if verb == "edges" {
		return runFlowsAppmapEdges(ctx, root, args, out)
	}
	if verb == "impact" {
		return runFlowsAppmapImpact(ctx, root, args, out)
	}
	if verb == "observe" {
		return runFlowsAppmapObserve(args, out)
	}
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
	if verb == "entries" {
		return runFlowsAppmapEntries(ctx, root, args, out)
	}
	if verb == "stats" {
		return runFlowsAppmapStats(ctx, root, args, out)
	}
	if verb == "refs" {
		return runFlowsAppmapRefs(ctx, root, args, out)
	}
	if verb == "pageobjects" || verb == "steps" {
		return runFlowsAppmapKnowledge(ctx, root, verb, args, out)
	}
	if verb == "actions" {
		return runFlowsAppmapActions(ctx, root, args, out)
	}
	if verb == "blocker" {
		return runFlowsAppmapBlocker(ctx, root, args, out)
	}
	if verb == "branches" {
		return runFlowsAppmapBranches(ctx, root, args, out)
	}
	if verb == "plan" {
		return runFlowsAppmapPlan(ctx, root, args, out)
	}
	type projector func(context.Context, *appmap.Map, string, appmap.Options) ([]byte, error)
	project, queryFlag := map[string]projector{
		"screen": appmap.ProjectScreen, "flow": appmap.ProjectFlow, "find": appmap.ProjectFind, "scaffold": appmap.ProjectScaffold,
	}[verb], map[string]string{"screen": "screen", "flow": "flow", "find": "text", "scaffold": "flow"}[verb]
	if project == nil {
		return errors.New("flows appmap requires build, screen, flow, find, scaffold, entries, stats, refs, pageobjects, steps, actions, blocker, branches, edges, observe, impact or plan")
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
