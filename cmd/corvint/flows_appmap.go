package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func init() { flowSubcommands["appmap"] = runFlowsAppmap }

// flowsAppmapHelp documents `flows appmap`; help.go appends it to flowsHelp.
const flowsAppmapHelp = `
Application map usage (experimental, AMAP-V0):
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace] [--from MAP [--verify]]
  corvint [--root PATH] flows appmap screen --map FILE --screen ID|STATE|TEMPLATE|URL [--budget N | --full] [--revision REV]
      [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...
  corvint [--root PATH] flows appmap flow --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]
      [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...
  corvint [--root PATH] flows appmap entries --map FILE [--ledger FILE] [--budget N | --full] [--revision REV]
  corvint [--root PATH] flows appmap stats --map FILE [--budget N | --full] [--min-resolved PCT] [--remedies]
  corvint [--root PATH] flows appmap coverage --map FILE [--budget N | --full]
  corvint [--root PATH] flows appmap refs --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--budget N | --full]
  corvint [--root PATH] flows appmap endpoints --map FILE --source [ALIAS:]DIR... [--rails-routes [ALIAS:]FILE]...
      [--openapi [ALIAS:]FILE]... [--repo ALIAS=ABSOLUTE_ROOT]... [--budget N | --full]
  corvint [--root PATH] flows appmap pageobjects --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--budget N | --full]
  corvint [--root PATH] flows appmap steps --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--budget N | --full]
  corvint [--root PATH] flows appmap setup --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--condition TEXT]...
      [--budget N | --full]
  corvint [--root PATH] flows appmap links --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--budget N | --full]
  corvint [--root PATH] flows appmap infer --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--corpus ARTIFACT]
      [--emit report|request] [--budget N | --full]
  corvint [--root PATH] flows appmap how --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]... [--condition TEXT]...
      [--ledger FILE] [--corpus ARTIFACT] [--budget N | --full] GOAL
  corvint [--root PATH] flows appmap actions --map FILE [--budget N | --full]
  corvint [--root PATH] flows appmap blocker --map FILE --note TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap branches --map FILE --cite PATH:START[-END] [--cite ...] [--budget N | --full]
  corvint [--root PATH] flows appmap find --map FILE --text TEXT [--budget N | --full]
  corvint [--root PATH] flows appmap scaffold --map FILE --flow FLOW_ID [--budget N | --full] [--revision REV]

--source ALIAS:DIR reads DIR from the root the map pinned under ALIAS (build --repo), at the
revision the map recorded for it; --repo ALIAS=ABSOLUTE_ROOT names that root's checkout. The
source verbs also take --workspace FILE or read --root/.corvint/workspace.json (unless
--no-workspace) for those roots, never merged with --repo. A map whose manifest is in an aliased
root reads it from that root. Flags parse in any order (how's after GOAL too); a refusal names the
flag that failed.

corvint flows appmap build --help prints the manifest's minimum members, what each step commits
and the change-compiled E2E chain to corvint-test e2e (guide docs/E2E-ADOPTION.md).

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
      [--records FILE] [--revision REV] [--budget N | --full]
      [--endpoint-source [ALIAS:]DIR ... (--rails-routes [ALIAS:]FILE | --openapi [ALIAS:]FILE) ...]
      [--repo ALIAS=ABSOLUTE_ROOT]...
Maps paths changed since --base to screens (router anchors, lineage descendants, attributed test
files, flow anchors), touched edges with ledger status and re-proving test, and spec files. An
unplaceable UI path (templates are not bound by the map) lists as unknown and the verdict is
full-suite-required. --records (observed-ui-records/0 from observe --records with --map) adds the
screens and tests that recorded traces bound a changed page object, workflow or spec to, under
basis observed-binding; it never removes an unknown. With the endpoint join inputs (AMAP-V0-220,
--endpoint-source is endpoints' --source) a changed Rails controller or endpoint declaration file
reaches the screens calling its endpoints under basis endpoint:ID; it stays unknown as
endpoint-callers-incomplete while the join has an unknown, endpoint-not-called for an endpoint no
read call reaches, and endpoint-declaration-changed for a declaration file, whose removed or
changed endpoints are not compared with base. Only endpoints declared in --root are matched to
changed paths; an endpoint join input from an aliased root is read at the map's pin. Learned,
read-only, offline.

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

Observed UI records (experimental, OUE-V0):
  corvint flows appmap observe --records --revision FULL_SHA --trace TEST_KEY=FILE.zip ...
      [--map FILE] [--routes FILE]
  corvint flows appmap structure-diff --base FILE --head FILE
  corvint flows appmap staleness --records FILE [--at REV] [--retire-after N]
observe --records reads up to 64 Playwright trace zips in memory and prints observed-ui-records/0
(LOCALLY_OBSERVED, bound to each test key and the revision): screens (map screen, else an
observed-ui-routes/0 template, else default ID normalization), transitions, endpoints with route
templates, and page-object/spec bindings from each action's file:line; a file with no observed
action stays UNKNOWN. Headers, bodies, typed text and query values are dropped and counted.
structure-diff compares scrubbed ARIA snapshot digests per screen: a candidate structural-change
signal, never a verdict. staleness states every record current, stale-unconfirmed, retired or
UNKNOWN at --at with the denominator; missing Git data never upgrades a record.

build compiles application-map/0 from the manifest's router definitions, flow intents and
test suite read at REV (default HEAD) and writes it to stdout. Router dialects are
ui-router-states/0, rails-routes/0 and react-router/0 (literal, experimental subsets).
Optional manifest router_globs report matched files absent from routers as router-not-declared
in app_coverage; this measures only the declared glob scope. A manifest whose tests name
"repo": ALIAS reads them from the root --repo binds to ALIAS, at its HEAD commit, which the
map pins; --manifest-repo reads the manifest from such a root. --repo-revision ALIAS=REV reads
that root at REV from committed objects instead of HEAD; the pin records "stated": true and its
checkout state is not read. An undeclared alias, a root that is not a Git worktree, a REV that is
not a commit there, or uncommitted changes under what a HEAD-read root supplies refuse.
--workspace FILE binds those three from a corvint-workspace/0 declaration (repos: alias -> root,
relative to the file's directory or absolute, and an optional default revision; manifest_repo);
without it and without any of the three flags, build reads --root/.corvint/workspace.json when it
exists, unless --no-workspace. A workspace never merges with the flags; an invalid one refuses
as workspace-invalid.
--from MAP builds from a previous map of the same manifest (AMAP-V0-211; there is no --previous
flag): when every path changed since its revision, in --root and in each aliased root it pins
(AMAP-V0-216), is a content edit no build input reads, the map is MAP carried to --revision with
its root pins moved, the map a full build gives. Otherwise it builds in full and writes the reason
to stderr (changed-input, builder-version-differs when MAP has no or another "builder",
aliased-roots, aliased-root-dirty, manifest-path-differs, from-revision-unavailable;
AMAP-V0-215). When the only read changes are content edits of flow intents (.json files directly
in the manifest's flows directory) that no other map entry names, only the flows are re-derived
and the rest of MAP is kept (AMAP-V0-240). When the read changes are content edits in --root of
sources other than the test sources, .json files, the manifest, Makefile, go.mod, Git metadata
and .corvint (a router or an application module, say), the routers, screens, test joins and
flows are re-derived and each test file's own entry is kept (AMAP-V0-245), as they are for a
path added in --root outside every manifest input, router glob and the test root, with no
.ts/.tsx/.js/.jsx/.mjs/.cjs/.json suffix and a base name no test import names (AMAP-V0-368); an edit that would
change the context index's source set or bounds (index-admission-changed) or that parses as JSON
(config-shaped-source) builds in full. stderr names what was re-derived. Any other changed input
re-derives the whole map. Either way the output is the full build's map. --verify also builds in
full and refuses appmap-incremental-mismatch unless the digests agree.
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
--remedies adds every keyed declaration remedy of the map's unknowns, grouped by file, key and
candidates with the unknowns each names, and the count of unknowns no declaration names
(AMAP-V0-343).

Reference check (experimental, V1-1581): refs reads the map's test files and up to 16 --source
directories at the map revision and lists the routes, selector names (test ids, CSS id, class
and data-* names) and mocked endpoint segments tests reference that the source lacks, with kind,
value, match (exact or prefix) and test locations (default cap 6144 bytes). It is name-level
only. Templated values are prefixes; catch-all routes, regular-expression patterns and
application-templated names are labelled; text-level selectors and dynamic values are counted
as unchecked. An unlabelled miss exits 1; refusals exit 2. No repository file changes.

Endpoint join (experimental, AMAP-V0-212..222, 351..358): endpoints reads server endpoints from
rails-routes/0 files (--rails-routes, with handler and the controller file under the Rails
application root, the nearest directory holding config/routes.rb or config/application.rb;
nested draws up to 4 levels keep the drawing scope, an optional scope segment yields both
paths, and a non-route statement is a per-statement unknown) and
OpenAPI 2/3 documents (--openapi; JSON, or block-style YAML with one-line scalars, anything else
openapi-yaml-unsupported; a $ref path item refuses its document as openapi-path-ref-not-read),
and client calls from $http, axios, axios.create instances, $resource factories and fetch
(JSX read in .js, .jsx and .tsx) in up to 16 --source
directories, from --root at the map revision, or as ALIAS:PATH from the root the map pinned under
ALIAS (with --repo; an endpoint, call and unknown read there carry repo, and a calling file
there is aliased-root-caller-not-placed, since screens are anchored in --root). A call's URL is read from literals, templates
and + joins; a substitution may spell one or more whole path segments. It prints
application-map-endpoints/0:
each endpoint with every call that may reach it and the screens those calls' files reach (a router
anchor, else the reference hop impact uses), each call with its endpoints and screens, and an
unknown for every call, route, document or file it cannot read or place; complete is true only
with no unknown (default cap 6144 bytes). Request interceptors (request-interceptor-not-modelled),
axios.defaults, an axios instance with a non-literal baseURL or created elsewhere
(client-instance-not-modelled, -shadowed, -escapes) and generated clients (a generator header in
the first 2 KiB of a file naming a request API: generated-client-not-modelled) are not read and
are unknown. Refusals exit 2. No repository file changes.

Site knowledge (experimental, SKH-V0): pageobjects reads up to 16 --source directories at the
map revision and lists page objects (Playwright and Cypress TypeScript classes and page-function
modules, Capybara helper classes, SitePrism pages) with their locator bindings, methods, effect
class (navigation-only, state-effect or unknown) and screen join. steps compiles each test body
to the closed step IR (visit, click, fill, select, check, upload, press, focus, hover, wait-for,
assert, opaque),
inlining page-object and helper calls with bound arguments, and deduplicates repeated sequences
into reusable records with usage counts. setup lists test.extend fixtures and their chains,
hooks, storageState paths (never contents), route mocks (labelled mocked), API seeding calls,
cy.task and Playwright setup projects, plus the Rails factories, traits, fixture files, seeds and
test-setup endpoints of the map's rails-routes/0 roots with what they produce, and links each test
to the records that arrange it. --condition TEXT (up to 16) lists the records that can produce
the condition, or unresolved; every flow precondition is answered the same way. links lists the
screen graph's edges by provenance: static edges read from templates and components (a href,
ui-sref, routerLink, router <Link to>, form action, link_to, button_to, form_with url:,
redirect_to, $state.go, navigate and router or history push) beside the map's test-observed and
flow-declared edges, never merged; a Rails view or controller action binds the source screen,
and dynamic or relative targets and unsupported template languages are unknowns. infer clusters
test step sequences into candidate flows by start and end screen and into variations by action
sequence, with support, conflicts, the outcomes every source asserts and single-source labels;
--corpus ARTIFACT (built at the map revision) joins DCP-V1 test knowledge: a defect blocks its
flow, an unrelated test is no source, and run+reviewed candidates rank first. --emit request
prints the proposed candidates as a corvint-behavior-adapter-request/1 for flows import
--format behavior-adapter-request, which writes them as proposed intents (default caps 6144
bytes). Unsupported constructs are
unknowns or opaque steps with a reason; secret-shaped values are screened. Learned authority,
static, no code runs and no repository file changes. how answers one goal (site-knowledge-how/0):
the best candidate flow by lexical coverage, its goal screen, the setup that arranges its primary
test and each --condition, the steps with each hop's admitted static, test-observed and --ledger
PROVEN edges (provenance kept), and the outcome oracle its sources share; a missing piece is a
named gap (no-route, no-setup-for-precondition, no-oracle, no-page-object, ambiguous-goal,
stale-evidence) and no candidate or a tie ABSTAINS. The answer is a proposal, never accepted intent.

Route-action check (experimental, V1-1890): actions compiles every rails-routes/0 route (and its
nested draws) to a controller and action and reports ROUTE-ACTION-MISSING when the handler
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

func runFlowsAppmap(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	if len(args) == 0 {
		return errors.New("flows appmap requires build, screen, flow, find, scaffold, entries, stats, coverage, refs, endpoints, pageobjects, steps, setup, links, infer, how, actions, blocker, branches, edges, observe, structure-diff, staleness, impact or plan")
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
	if verb == "structure-diff" {
		return runFlowsAppmapStructureDiff(args, out)
	}
	if verb == "staleness" {
		return runFlowsAppmapStaleness(ctx, root, args, out)
	}
	f := flag.NewFlagSet("flows appmap "+verb, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	revision := f.String("revision", "", "revision to compile or to check anchors against")
	if verb == "build" {
		manifest := f.String("manifest", "", "tracked manifest path inside the root")
		from := f.String("from", "", "a previous application-map/0 of the same manifest")
		verify := f.Bool("verify", false, "also build in full and refuse unless the digests agree")
		roots := appmap.BindRootFlags(f)
		usage := "flows appmap build requires --manifest FILE, optional --revision REV, --repo ALIAS=ABSOLUTE_ROOT (repeatable), --manifest-repo ALIAS and --repo-revision ALIAS=REV (repeatable) or --workspace FILE, --no-workspace, and --from MAP [--verify]"
		if f.Parse(args) != nil || *manifest == "" || f.NArg() != 0 || roots.Check() != nil || (*verify && *from == "") {
			return errors.New(usage)
		}
		if err := roots.Resolve(root); err != nil {
			return err
		}
		if *revision == "" {
			*revision = "HEAD"
		}
		m, err := buildAppmap(ctx, root, *manifest, *revision, *roots, *from, *verify, diag)
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
	if verb == "coverage" {
		return runFlowsAppmapCoverage(ctx, root, args, out)
	}
	if verb == "refs" {
		return runFlowsAppmapRefs(ctx, root, args, out)
	}
	if verb == "endpoints" {
		return runFlowsAppmapEndpoints(ctx, root, args, out)
	}
	if verb == "pageobjects" || verb == "steps" || verb == "setup" || verb == "links" {
		return runFlowsAppmapKnowledge(ctx, root, verb, args, out)
	}
	if verb == "infer" {
		return runFlowsAppmapInfer(ctx, root, args, out)
	}
	if verb == "how" {
		return runFlowsAppmapHow(ctx, root, args, out)
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
		return errors.New("flows appmap requires build, screen, flow, find, scaffold, entries, stats, coverage, refs, endpoints, pageobjects, steps, setup, links, infer, how, actions, blocker, branches, edges, observe, structure-diff, staleness, impact or plan")
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

// buildAppmap builds the map in full, or from a previous map when from names one (AMAP-V0-211).
// --verify also builds in full and refuses appmap-incremental-mismatch unless the digests agree.
// A --from build that rebuilt in full says why on diag (AMAP-V0-215), and one that re-derived
// only some sections names them (AMAP-V0-240, AMAP-V0-245).
func buildAppmap(ctx context.Context, root, manifest, revision string, roots appmap.Roots, from string, verify bool, diag io.Writer) (*appmap.Map, error) {
	if from == "" {
		return appmap.BuildRoots(ctx, root, manifest, revision, roots)
	}
	prev, err := appmap.LoadMap(from)
	if err != nil {
		return nil, err
	}
	m, inc, err := appmap.BuildFrom(ctx, root, manifest, revision, roots, prev)
	if inc.Reason != "" {
		fmt.Fprintf(diag, "flows appmap build: --from %s rebuilt in full: %s\n", from, inc.Reason)
	}
	if len(inc.Rederived) > 0 {
		fmt.Fprintf(diag, "flows appmap build: --from %s re-derived only: %s\n", from, strings.Join(inc.Rederived, ", "))
	}
	if err != nil {
		return nil, err
	}
	if verify && (inc.Reused || len(inc.Rederived) > 0) {
		full, err := appmap.BuildRoots(ctx, root, manifest, m.Revision, roots)
		if err != nil {
			return nil, err
		}
		if full.Digest != m.Digest {
			return nil, &gokernel.Error{Code: "appmap-incremental-mismatch", Message: fmt.Sprintf("--from build digest %s differs from the full build digest %s", m.Digest, full.Digest)}
		}
	}
	return m, nil
}
