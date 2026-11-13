package main

import "github.com/Beamfall/corvint/internal/capcatalog"

// appmapAdoptionGuide is the adoption guide every step's help names.
const appmapAdoptionGuide = "docs/APPLICATION-MAP.md"

// flowsAppmapBuildHelpTopic is the help topic of `corvint flows appmap build --help`.
const flowsAppmapBuildHelpTopic = "flows-appmap-build"

// flowsAppmapBuildHelp is `corvint flows appmap build --help`: the build step of the chain.
func flowsAppmapBuildHelp() string {
	return capcatalog.StepHelp(capcatalog.BuildWorkflow(appmapAdoptionWorkflow), "appmap-build", "")
}

// appmapAdoptionWorkflow is the application-map adoption chain, in the order an adopter runs it
// (CAP-V0-030). `corvint help --json` derives its workflows member from it. The change-compiled
// E2E steps that once followed the build left with the E2E proof lab (decision 0593).
var appmapAdoptionWorkflow = capcatalog.WorkflowSource{
	Name:     "application-map",
	Summary:  "write an application-map manifest and compile it into an application map at one revision",
	Maturity: "experimental",
	Specs:    []string{"AMAP-V0"},
	Guide:    appmapAdoptionGuide,
	Steps: []capcatalog.WorkflowStepSource{
		{ID: "manifest", Help: `Write the application-map-manifest/0 manifest (AMAP-V0-001, docs/specs/application-map-v0.md).

Inputs:
  One Git repository holding the application's router files and its Playwright tests. The
  manifest is hand-written; no command generates it.

Commit:
  appmap.json at the repository root, with at least: schema
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
  corvint [--root PATH] flows appmap build --manifest FILE --worktree [--no-workspace]

Inputs:
  --manifest FILE names the committed application-map-manifest/0 document (AMAP-V0-001,
  docs/specs/application-map-v0.md), repository-relative. Build reads the manifest, routers,
  flows and tests from Git at --revision, so they must be committed there; uncommitted edits are
  not read. --worktree instead reads the working tree of --root, uncommitted and untracked files
  included, and marks the map unpinned (AMAP-V0-976).

Commit:
  The manifest and every router, flow and test directory it names, at --revision.

Produces:
  application-map/0 on stdout. Read it to check the routes, screens, flows and page objects the
  manifest declares, and fix the manifest until no unknown is unexpected. Do not commit this
  output. corvint flows appmap stats --map FILE (served by corvint-docs) prints the map's counts
  of screens, edges and unknowns, with the unknowns grouped by reason (AMAP-V0-978).

Options:
  --manifest FILE                 tracked manifest path inside the root (required)
  --revision REV                  revision to compile (default HEAD)
  --repo ALIAS=ABSOLUTE_ROOT      a second checkout holding the tests (AMAP-V0-017, repeatable)
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
  --worktree                      build unpinned from the working tree of --root; combines with
                                  --no-workspace only (AMAP-V0-976, AMAP-V0-977)

Example:
  corvint flows appmap build --manifest appmap.json --revision HEAD > /tmp/appmap-map.json
`},
	},
}
