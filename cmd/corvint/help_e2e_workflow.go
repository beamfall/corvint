package main

import "github.com/Beamfall/corvint/internal/capcatalog"

// e2eAdoptionGuide is the adoption guide every step's help names (CCE-V0-120).
const e2eAdoptionGuide = "docs/E2E-ADOPTION.md"

// flowsAppmapBuildHelpTopic is the help topic of `corvint flows appmap build --help` (CCE-V0-118).
const flowsAppmapBuildHelpTopic = "flows-appmap-build"

// flowsAppmapBuildHelp is `corvint flows appmap build --help`: the build step of the chain.
func flowsAppmapBuildHelp() string {
	return capcatalog.StepHelp(capcatalog.BuildWorkflow(e2eAdoptionWorkflow), "appmap-build", "")
}

// e2eAdoptionWorkflow is the change-compiled E2E adoption chain, in the order an adopter runs it
// (CAP-V0-030, CCE-V0-118). `corvint help --json` derives its workflows member from it, and
// `corvint-test e2e SUBCOMMAND --help` prints the same step help from the committed catalog, so
// the chain has one source. Core declares the steps as help text only: it neither links nor runs
// the companion (CCE-V0-015).
var e2eAdoptionWorkflow = capcatalog.WorkflowSource{
	Name:     "change-compiled-e2e",
	Summary:  "compile and prove change-scoped Playwright scenarios from a committed application-map manifest and admitted test blocks",
	Maturity: "experimental",
	Specs:    []string{"AMAP-V0", "CCE-V0"},
	Guide:    e2eAdoptionGuide,
	Steps: []capcatalog.WorkflowStepSource{
		{ID: "manifest", Help: `Write the application-map-manifest/0 manifest (AMAP-V0-001, docs/specs/application-map-v0.md).

Inputs:
  One Git repository holding the application's router files and its Playwright tests. The
  manifest is hand-written; no command generates it.

Commit:
  appmap.json at the repository root (the e2e default --manifest), with at least: schema
  "application-map-manifest/0"; app matching ^[a-z0-9][a-z0-9._-]{0,63}$; 1 to 64 routers, each
  a path and a dialect (ui-router-states/0, rails-routes/0 or react-router/0); and tests with
  root, specs and page_objects, every directory under tests.root. Add flows, a directory of
  application-flow-intent/1 files, to get outcome blocks. A local commit is enough: every later
  step reads Git objects at a revision, never the worktree.

Produces:
  Nothing yet; flows appmap build checks that the manifest compiles.

Example:
  {"schema": "application-map-manifest/0", "app": "web", "flows": "flows",
   "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
   "tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}}
`},
		{ID: "appmap-build", Command: "corvint flows appmap build", Requires: []string{"manifest"}, Help: `Compile the committed manifest into an application map at one revision (experimental, AMAP-V0).

Usage:
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace] [--from MAP [--verify]]

Inputs:
  --manifest FILE names the committed application-map-manifest/0 document (AMAP-V0-001,
  docs/specs/application-map-v0.md), repository-relative. Build reads the manifest, routers,
  flows and tests from Git at --revision, so they must be committed there; uncommitted edits are
  not read.

Commit:
  The manifest and every router, flow and test directory it names, at --revision.

Produces:
  application-map/0 on stdout. Read it to check the routes, screens, flows and page objects the
  manifest declares, and fix the manifest until no unknown is unexpected. The e2e commands
  rebuild the map themselves; do not commit this output.

Options:
  --manifest FILE                 tracked manifest path inside the root (required)
  --revision REV                  revision to compile (default HEAD)
  --repo ALIAS=ABSOLUTE_ROOT      a second checkout holding the tests (AMAP-V0-017, repeatable);
                                  the corvint-test e2e commands take the same three root flags
  --manifest-repo ALIAS           the aliased root holding the manifest
  --repo-revision ALIAS=REV       read that root at REV instead of its HEAD (AMAP-V0-200, repeatable)
  --workspace FILE                read the three root flags from a corvint-workspace/0 file (WSD-V0)
  --no-workspace                  do not discover ROOT/.corvint/workspace.json (WSD-V0-003)
  --from MAP                      carry a previous map of the manifest forward when no input
                                  changed since its revision (AMAP-V0-211); re-derive only the
                                  flows for an intent edit (AMAP-V0-240), or the routers, test
                                  joins and flows for an application source edit (AMAP-V0-245);
                                  else build in full and say why (index-admission-changed, ...)
  --verify                        with --from, also build in full and refuse unless they agree

Example:
  corvint flows appmap build --manifest appmap.json --revision HEAD > /tmp/appmap-map.json
`},
		{ID: "commit-manifest", Requires: []string{"appmap-build"}, Help: `Commit the appmap.json that built cleanly at every revision the e2e commands read.

Inputs:
  The manifest that flows appmap build compiled without an unexpected unknown.

Commit:
  appmap.json, its routers, flows and tests at --rev (catalog, harvest) and at --head
  (obligations, compile, prove). --base may predate the manifest: a base without it compares
  nothing; a base with a manifest that does not build refuses e2e-map-unavailable. Optional
  .corvint/e2e-blocks.json (e2e-block-declarations/0: inputs, fixtures and the environment that
  serves the application) and .corvint/e2e-acceptances.json (e2e-acceptances/0) are read at the
  same revision.

Produces:
  A revision the e2e commands can read.
`},
		{ID: "e2e-catalog", Command: "corvint-test e2e catalog", Requires: []string{"commit-manifest"}, Help: `List the admitted test blocks at one revision (experimental, CCE-V0).

Usage:
  corvint-test e2e catalog --rev REV [--root DIR] [--manifest FILE] [--out FILE]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace]

Inputs:
  --rev REV is required. --root is the repository (default .), --manifest the committed
  application-map-manifest/0 path (default appmap.json). It is the manifest, not a built
  application-map/0: the command builds the map at --rev itself, so do not pass or commit the
  output of flows appmap build.
  For a two-repository layout, each --repo names an aliased root, --manifest-repo the alias
  holding the manifest and --repo-revision a root's pinned commit (CCE-V0-130..136).
  --workspace FILE, or a discovered ROOT/.corvint/workspace.json, supplies the same three flags
  (WSD-V0); --no-workspace skips discovery.

Commit:
  At --rev: the manifest, its page-object files (page-object blocks), routers (path blocks) and
  flows outcomes with a matcher, locator and non-empty value (outcome blocks); optionally
  .corvint/e2e-blocks.json for input and fixture blocks and .corvint/e2e-acceptances.json for
  harvested blocks. Outcome blocks, and so any REACHED obligation later, need the manifest's
  flows directory of application-flow-intent/1 files; with none, the catalog has no
  outcome blocks (only page-object, path and declared blocks) and obligations read UNKNOWN or
  INCOMPLETE.

Produces:
  e2e-block-catalog/0 on stdout, or at --out (a file outside the repository). A manifest that is
  missing or does not build refuses e2e-map-unavailable; exit 1.

Example:
  corvint-test e2e catalog --root . --rev HEAD
`},
		{ID: "e2e-harvest", Command: "corvint-test e2e harvest", Optional: true, Requires: []string{"e2e-catalog"}, Help: `Propose blocks from the existing Playwright suite (optional, experimental, CCE-V0).

Usage:
  corvint-test e2e harvest --rev REV [--root DIR] [--manifest FILE] [--source [ALIAS:]DIR]... [--out FILE]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace]

Inputs:
  --rev REV is required. Every tracked *.spec.* and *.test.* file at --rev is read for literal
  getByTestId assertions and page.goto navigations; the map at --manifest only binds proposals to
  flows and screens, and without it every proposal is unbound. Each --source directory is read
  into the site-knowledge IR at the map revision, which binds a line the map places on no flow
  or screen to the screen its test is on there (CCE-V0-310).

Commit:
  Nothing new to run it. To admit a proposal, commit its ID with a reason in
  .corvint/e2e-acceptances.json (e2e-acceptances/0); the catalog admits it at that revision.

Produces:
  e2e-block-proposals/0 on stdout, or at --out (outside the repository). A proposal is never a
  block until accepted.

Example:
  corvint-test e2e harvest --root . --rev HEAD --out /tmp/e2e-proposals.json
`},
		{ID: "e2e-obligations", Command: "corvint-test e2e obligations", Requires: []string{"e2e-catalog"}, Help: `Derive the change's E2E obligations between two revisions (experimental, CCE-V0).

Usage:
  corvint-test e2e obligations --base REV --head REV [--root DIR] [--manifest FILE] [--out FILE]
      [--coverage FILE] [--ledger FILE] [--module-graph]
      [--endpoint-source DIR]... [--rails-routes FILE]... [--openapi FILE]...
      [--remedy-summary [--remedy-commits N]]
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace]

Inputs:
  --base and --head are the change's revisions. --coverage (e2e-runtime-coverage/0 from prove
  --record-coverage), --ledger (application-edge-ledger/0) and --module-graph change how reach
  is computed (CCE-V0-045, CCE-V0-103, CCE-V0-054). --endpoint-source with --rails-routes
  (rails-routes/0) or --openapi joins server endpoints to client calls, so a changed controller or endpoint
  declaration reaches the calling screens; the join's own unknowns stay unknown spans
  (CCE-V0-152, AMAP-V0-220). Each unknown span carries a remedy: the
  declaration key and inferred candidates that would bound it, or the stated uncertainty
  (CCE-V0-150). --remedy-summary instead evaluates the newest --remedy-commits (default 20,
  at most 50) first-parent commits of base..head against their parents and writes
  e2e-remedy-summary/0, ranking the remedies that alone bound the most merges (CCE-V0-151).

Commit:
  The manifest at --head; the catalog's blocks at --head.

Produces:
  e2e-obligations/0 on stdout, or at --out (outside the repository). Gate CI on the top-level
  .state (REACHED, NONE or UNKNOWN), never on the exit code: an emitted document exits 0 even
  when .state reads UNKNOWN (CCE-V0-023 is not built). Only the top-level .state reads UNKNOWN.

Example:
  corvint-test e2e obligations --root . --base origin/main --head HEAD
`},
		{ID: "e2e-compile", Command: "corvint-test e2e compile", Requires: []string{"e2e-obligations"}, Help: `Compile one Playwright scenario per discharged obligation (experimental, CCE-V0).

Usage:
  corvint-test e2e compile --base REV --head REV --out DIR [--root DIR] [--manifest FILE]
      [--coverage FILE] [--ledger FILE] [--module-graph]
      [--endpoint-source DIR]... [--rails-routes FILE]... [--openapi FILE]...
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace]

Inputs:
  The obligations inputs (--coverage e2e-runtime-coverage/0, --ledger application-edge-ledger/0,
  --module-graph, the --endpoint-source join over rails-routes/0 or OpenAPI), plus --out: an existing directory outside the repository.

Commit:
  Nothing beyond the obligations step.

Produces:
  Scenario spec files under --out and e2e-compile/0 on stdout. Read .state: INCOMPLETE
  names the missing blocks, UNKNOWN what could not be decided, and the document still exits 0
  (CCE-V0-023 is not built). Do not gate CI on the exit code.

Example:
  corvint-test e2e compile --root . --base origin/main --head HEAD --out /tmp/e2e-compiled
`},
		{ID: "e2e-prove", Command: "corvint-test e2e prove", Requires: []string{"e2e-compile"}, Help: `Run the compiled scenarios, or one hosted test, against base and head (experimental, CCE-V0).

Usage:
  corvint-test e2e prove --base REV --head REV --out DIR [--node-bin FILE] [--node-modules DIR]
      [--root DIR] [--manifest FILE] [--coverage FILE] [--ledger FILE] [--module-graph]
      [--repeats N] [--package-json FILE] [--lockfile FILE] [--record-coverage]
      [--oracle-acceptance FILE]
      [--endpoint-source DIR]... [--rails-routes FILE]... [--openapi FILE]...
      [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...
      [--workspace FILE | --no-workspace]
  corvint-test e2e prove --base REV --head REV --out DIR [--node-bin FILE] [--node-modules DIR]
      --hosted-test FILE [--hosted-title TITLE] [--root DIR] [--repeats N] [--package-json FILE]
      [--lockfile FILE] [--record-coverage] [--oracle-acceptance FILE] [--no-workspace]

Inputs:
  --out is an existing directory outside the repository. Both toolchain inputs are required, by
  flag or environment: --node-bin (CORVINT_E2E_NODE_BIN) is the absolute path of the node binary
  with npx beside it, and --node-modules (CORVINT_PLAYWRIGHT_MODULES) an absolute node_modules
  holding @playwright/test. --hosted-test proves one existing hosted Playwright test file as
  given, without compiling; it takes no --coverage or --ledger. --coverage
  (e2e-runtime-coverage/0), --ledger (application-edge-ledger/0), --module-graph and the
  --endpoint-source join (rails-routes/0 or OpenAPI) are as in obligations; --oracle-acceptance is an e2e-oracle-acceptance/0 document outside the repository.

Commit:
  At --head: the page-object and fixture files, their relative imports, the runner configuration,
  package.json and the lockfile; .corvint/e2e-blocks.json environment to serve the application
  (without it each side is served statically).

Produces:
  e2e-prove/0 at --out/e2e-prove.json and on stdout, the scenarios under --out, and with
  --record-coverage e2e-runtime-coverage/0 beside it. Read each verdict: an emitted proof exits
  0 whatever its verdicts, and a refusal exits 1.

Example:
  corvint-test e2e prove --root . --base origin/main --head HEAD --out /tmp/e2e-proof \
      --node-bin "$(command -v node)" --node-modules "$PWD/node_modules"
`},
		{ID: "e2e-shadow", Command: "corvint-test e2e shadow", Requires: []string{"e2e-prove"}, Help: `Score a frozen window of recorded proofs against the hosted suite (experimental, CCE-V0).

Usage:
  corvint-test e2e shadow --window FILE [--root DIR] [--manifest FILE] [--out FILE]

Inputs:
  --window is an e2e-shadow-window/0 document of recorded prove results and hosted runs whose
  registration pins the mutation operators, seed, catalog digest and compiler version before the
  first run. Shadow runs neither side.

Commit:
  Nothing; the window is recorded evidence, kept outside the repository.

Produces:
  e2e-shadow/0 on stdout, or at --out (outside the repository).

Example:
  corvint-test e2e shadow --window /tmp/e2e-shadow-window.json
`},
	},
}
