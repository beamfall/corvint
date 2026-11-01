# Change-compiled E2E adoption

This guide is for an application repository with an existing hosted Playwright suite. It covers
the smallest set of committed files that takes that repository through the experimental
change-compiled E2E chain (`docs/specs/change-compiled-e2e-v0.md`, CCE-V0), and what each step
needs at the revision it reads. Every step's own help prints the same detail:
`corvint flows appmap build --help` and `corvint-test e2e SUBCOMMAND --help`. The ordered chain,
with each step's flags, inputs and commits, is the `workflows` member of `corvint help --json`.

```text
manifest -> corvint flows appmap build -> committed appmap.json -> e2e catalog
  -> e2e obligations -> e2e compile -> e2e prove -> e2e shadow
```

`e2e harvest` is optional. It sits after the catalog and proposes blocks from the existing suite.

## Which command answers which question

- **Which existing tests do I rerun for this change?** `corvint affected --base REV` and
  `corvint flows appmap impact` select from the hosted suite. CCE does not emit a hosted-test
  selection.
- **Which new scenario does CCE generate, and does it hold?** `corvint-test e2e obligations`,
  `compile` and `prove`. They derive scenarios from admitted blocks; they do not choose hosted tests.

## Before you start

- **Experimental.** Every `corvint-test e2e` command is experimental. An emitted document exits 0,
  whatever its state or verdicts, and a refusal exits 1. The CCE-V0-023 exit table is not built
  yet, so read `.state`: a CI step must gate on the document's top-level `state`, never on the exit code. That
  member is `REACHED`, `NONE` or `UNKNOWN` for `obligations`, and `COMPILED`, `NONE` or
  `INCOMPLETE` for `compile`. Each scenario entry of `compile` also has its own `state`. Read the
  verdicts of `prove` the same way. For example:

  ```sh
  corvint-test e2e compile --base "$BASE" --head HEAD --out "$OUT" > compile.json
  jq -e '.state != "INCOMPLETE"' compile.json
  ```
- **The manifest is not a built map.** `--manifest` (default `appmap.json`) is the
  `application-map-manifest/0` manifest. CCE builds the `application-map/0` itself at each
  revision, so never commit or pass the output of `flows appmap build`.
- **Committed objects only.** Every input is read from Git objects at a revision, never from the
  worktree. The application and its tests can share one Git tree, or the tests can live in a
  second repository: see [Two repositories](#two-repositories-application-and-tests-apart).
- **Local commits.** A local commit is enough. Nothing needs to be pushed.

## 1. The minimum manifest

The manifest is hand-written: no command generates it. Write `appmap.json` at the repository
root, which is the e2e default `--manifest`. Its schema is `application-map-manifest/0`
(AMAP-V0-001, `docs/specs/application-map-v0.md`). Every e2e subcommand's `--manifest` names this
manifest, not the `application-map/0` map that `corvint flows appmap build` prints. The e2e
commands build the map from the manifest at each revision they read.

```json
{
  "schema": "application-map-manifest/0",
  "app": "web",
  "flows": "flows",
  "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
  "tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}
}
```

- **Required members:**
  - `schema`.
  - `app`, which matches `^[a-z0-9][a-z0-9._-]{0,63}$`.
  - 1 to 64 `routers`, each with a dialect: `ui-router-states/0`, `rails-routes/0` or
    `react-router/0`.
  - `tests`, with `root`, `specs` and `page_objects`. Every directory must be under `tests.root`.
    Point these at the hosted suite's own spec and page-object directories. A suite with no
    page-object directory may leave `page_objects` empty or omit it. A flat layout, with helpers
    such as `helpers.ts` beside the specs, lists the same directory in both: there,
    `*.spec.ts` and `*.test.ts` files are specs and every other file is a page object
    (AMAP-V0-780):

    ```json
    "tests": {"root": "apps/web/e2e", "specs": ["apps/web/e2e"], "page_objects": ["apps/web/e2e"], "web_root": "apps/web"}
    ```

    `web_root` (optional) names the dev-server root, such as Vite's `root`. A spec's
    `await import("/src/x.ts")` inside `page.evaluate(...)` runs in the browser, so the map
    resolves it against `web_root` as a browser-context reference rather than as a Node import
    of the spec (AMAP-V0-781, AMAP-V0-782).
- **Optional `flows`:** a directory of `application-flow-intent/1` files. The catalog admits
  outcome blocks only from flows, so a manifest without flows yields path and page-object blocks
  only. Flows are therefore the input the chain needs for a real result. Without them, no flow step
  expects an outcome, so every reached screen reads `UNKNOWN` with `screen-without-outcome`
  (CCE-V0-019). `obligations` then reads `UNKNOWN`, `compile` reads `INCOMPLETE`, and no
  scenario is compiled. The `flows` directory is the input to add first.
- **Other optional members:** the application-map spec lists them, for example `workflows`,
  `scenarios`, `page_object_screens`, `hash_prefix` and `base_path`.

## 2. Check it: `corvint flows appmap build`

```sh
git add appmap.json && git commit -m "Add application map manifest"
corvint flows appmap build --manifest appmap.json --revision HEAD > /tmp/appmap-map.json
```

The build reads the manifest, routers, flows and tests at `--revision`, so commit them first.

Read the `application-map/0` output. Check its routes, screens, flows and page objects, and fix
the manifest until no unknown is unexpected.

Do not commit this output. The e2e commands rebuild the map from the manifest themselves.

## 3. What each step needs committed

| Step | Committed at | Needs |
|---|---|---|
| `e2e catalog --rev REV` | `--rev` | `appmap.json`, its routers, flows and page-object files. Optional: `.corvint/e2e-blocks.json` (`e2e-block-declarations/0`: input and fixture blocks and the `environment` that serves the app) and `.corvint/e2e-acceptances.json` (`e2e-acceptances/0`) |
| `e2e harvest --rev REV` (optional) | `--rev` | Nothing new. It reads every tracked `*.spec.*` and `*.test.*` file |
| `e2e obligations --base B --head H` | `--head` | The manifest and blocks at head. The base may predate the manifest, but a base manifest that does not build refuses |
| `e2e compile ... --out DIR` | `--head` | Nothing beyond obligations. `--out` is an existing directory outside the repository |
| `e2e prove ... --out DIR` | `--head` | The page-object and fixture files with their relative imports, the runner configuration, `package.json` and the lockfile. The `environment` block, if you want the app served (otherwise each side is served statically) |
| `e2e shadow --window FILE` | nothing | A recorded `e2e-shadow-window/0` kept outside the repository |

`e2e prove` also needs two toolchain inputs, given by flag or environment variable:

- `--node-bin` (`CORVINT_E2E_NODE_BIN`): the absolute path of `node`, with `npx` beside it.
- `--node-modules` (`CORVINT_PLAYWRIGHT_MODULES`): an absolute `node_modules` directory holding
  `@playwright/test`.

## 4. Use the existing hosted suite

There are two routes that take nothing away from the hosted suite:

- **Prove one hosted test.** `corvint-test e2e prove --hosted-test FILE [--hosted-title TITLE]`
  proves an existing hosted test file as given, against base and head, with its negative controls.
  It compiles nothing. It takes no `--coverage` and no `--ledger`.
- **Harvest blocks.** Run `corvint-test e2e harvest --rev HEAD --out /tmp/e2e-proposals.json`.
  It proposes blocks from the suite's literal `getByTestId` assertions and `page.goto`
  navigations. A proposal becomes a block only when its ID is committed with a reason in
  `.corvint/e2e-acceptances.json`:

  ```json
  {"schema": "e2e-acceptances/0", "acceptances": [{"proposal_id": "<id from harvest>", "reason": "reviewed"}]}
  ```

Hosted tests retire only after a registered shadow window, as `e2e shadow` and the spec's Shadow
evaluation section describe. The chain never deletes or replaces a hosted test.

## 5. What the chain answers

### Rerun existing tests, or generate a scenario

The e2e chain does not select hosted tests. It names the outcomes a change puts at risk and
compiles at most one new scenario for each reached outcome (CCE-V0-004, CCE-V0-005). Automatic
narrowing of hosted selection is a V0 non-goal. Other commands answer "which existing tests do I
rerun":

| Question | Command | Output |
|---|---|---|
| Which existing Playwright files do I rerun? | `corvint affected --base FULL_SHA --playwright-config PATH [--playwright-discovery FILE]` | `playwright-affected/0` (TJAA-V0-010): selected file rows, or exactly one `fallbackArgv` that runs the full config when discovery is missing or unresolved (TJAA-V0-015) |
| Which screens and spec files does the change reach on the map? | `corvint flows appmap impact --map FILE --base COMMIT [--ledger FILE] [--records FILE]` | `application-map-impact/0` (ANL-V0-015): `screens`, `spec_files`, `test_keys`, `unknown` and a `verdict`; `--records` adds trace-observed page-object screens and tests (OUE-V0-070..074) |
| Navigation advice on the selection | `corvint affected --base FULL_SHA --appmap-impact REPORT` | `advice.app_navigation` (ANL-V0-031). It is advisory and never adds to or removes from the plan |
| Which scenario does CCE generate for this change? | `corvint-test e2e obligations`, then `compile` | `e2e-obligations/0` and `e2e-compile/0`, plus the scenario files under `--out` |

`flows appmap impact` takes the built map (`--map`), so save the build output from step 2 at
the head revision to a file outside the repository first.

### An unmapped changed path

A changed path that the map's impact relation places on no screen does not get a full-suite
fallback inside the e2e chain, and it is never narrowed away:

- `obligations` reads `UNKNOWN` overall and lists the path as an unknown span with reason
  `unplaced-path`. Any impact omission instead makes every changed path `impact-truncated`
  (CCE-V0-019, CCE-V0-020). It never reads `NONE` for that diff.
- `compile` over that report reads `INCOMPLETE` and lists the same spans, even when every reached
  obligation compiled (CCE-V0-004). It compiles scenarios only for `REACHED` obligations, and it
  emits no selector, URL or value that is not the exact text of an admitted block (CCE-V0-006).
- A path that should not count, such as documentation, is exempt only by a `reach_exempt` glob
  in `.corvint/e2e-blocks.json` read at base. The report cites the glob under `exempt`
  (CCE-V0-020).

This fail-closed reading is a contract of the documents: CCE-V0-004, CCE-V0-006 and CCE-V0-019
are MUST requirements, and the spec's fail-closed choices state that none of these states narrows
hosted selection. It is not yet a contract of the exit code. Until CCE-V0-023 is built, an
`UNKNOWN` or `INCOMPLETE` document exits 0, so gate on `.state`. Full-suite verdicts belong to
the selection commands, each from its own unknowns: `flows appmap impact` reads
`full-suite-required` for an unmapped UI path (ANL-V0-016), and `corvint affected
--playwright-config` falls back to the full relevant suite when its discovery or source
reachability is unresolved (TJAA-V0-015). An appmap verdict is advice only, and it never changes
the Playwright selection (ANL-V0-031).

### Machine-readable lists

Every document is deterministic JSON on stdout, or at `--out` where the step writes one.

- **CCE scenarios:** `e2e-compile/0` `.scenarios[]` has one entry per reached obligation, unless
  the byte cap drops entries from the end and records how many in `truncated` (CCE-V0-014). Each
  entry has `obligation` and `state`. A `COMPILED` entry adds `placement` (its file under
  `--out`) and `file_digest`, and an `INCOMPLETE` entry names its `missing` blocks. `.unknown[]`
  lists the unbounded spans. `e2e-obligations/0` `.obligations[]` lists each reached
  outcome with its `spans` and `chain`.
- **Existing tests to rerun:** the `playwright-affected/0` file rows or its `fallbackArgv`, and
  the `spec_files` and `test_keys` of `application-map-impact/0`. Compare an observed must-rerun
  set with these, not with the compiled scenarios, which are new files rather than a hosted
  selection.

### A synthetic change

The e2e commands take the change only as revisions. `--base` and `--head` must resolve to
commits, and the application and test sources are read from Git objects at those commits. The
`--coverage`, `--ledger` and `--window` documents are separate files you supply. No flag takes a
patch or diff file, and an uncommitted worktree edit is not read. To evaluate a mutant, commit it on a scratch branch: a
local commit is enough. Then pass its parent as `--base` and the mutant commit as `--head`.
`corvint affected` without `--base` reads the dirty worktree instead, so for selection an
uncommitted mutant works there.

## Two repositories: application and tests apart

The application can stay in `--root` while its Playwright suite lives in a second repository.
`corvint flows appmap build` and every map-reading e2e step (`catalog`, `harvest`, `obligations`,
`compile` and `prove`) take the same three aliased-root flags (AMAP-V0-017, AMAP-V0-200,
CCE-V0-130..136):

- `--repo ALIAS=ABSOLUTE_ROOT` (repeatable) binds an alias to the test checkout. The root must
  be absolute, the top of a Git worktree with a `HEAD` commit, and not the `--root` repository.
  The alias matches `^[a-z][a-z0-9-]{0,31}$`.
- `--manifest-repo ALIAS` reads the manifest from that aliased root. Omit it when `appmap.json` is
  committed in the application repository.
- `--repo-revision ALIAS=REV` (repeatable) pins that root to commit REV. Without it the root is
  read at its `HEAD` commit. `--repo-revision` for an alias that has no `--repo` is a usage error.

The manifest names the test repository's alias in `tests.repo`. Routers and flows always come from
`--root`. With `tests.root` of `"."`, the test directories are relative to the test repository's
top:

```json
{
  "schema": "application-map-manifest/0",
  "app": "web",
  "flows": "flows",
  "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
  "tests": {"repo": "e2e", "root": ".", "specs": ["specs"], "page_objects": ["pages"]}
}
```

### Revisions on each side

- **The diff is always `--root`'s.** `obligations`, `compile` and `prove` take the change as
  `--base..--head` in the application repository. Commits in the test repository are never part
  of the change.
- **The test root is pinned once.** Page objects, fixtures, `.corvint/e2e-blocks.json` (with its
  `environment` block) and `.corvint/e2e-acceptances.json` are read from the test repository at
  its pin. The same pin serves the base map and the head map. With `--repo-revision` the pin is
  REV, read from committed objects only. It records `"stated": true`, and later commits or
  uncommitted edits in that checkout do not change any document. Without it the pin is the
  checkout's `HEAD`. A staged, unstaged or untracked change under the manifest or `tests.root`
  then refuses (`appmap-root-dirty`). With `tests.root` of `"."`, only the declared specs,
  page-object, workflow and scenario directories count. A change elsewhere in that checkout does
  not refuse. Pass
  `--repo-revision` in CI so that every step reads the same test commit.
- **The documents record the pins.** The map and the catalog, harvest, obligations and compile
  documents carry `roots`, which lists `{repo, revision, inputs}` for each aliased root. Each span
  read from an aliased root carries `repo`. The proof records `test_root` and `test_revision`
  instead. A single-root map carries none of these members, so its bytes are unchanged.
- **Acceptances have no base side.** An acceptance ledger read from the test repository is outside
  `--root`'s diff, so `acceptances_in_diff` flags every proposal it accepts (CCE-V0-044).

### Invocation per step

Here `/src/app` is the application checkout and `/src/app-e2e` is the test checkout. `TESTS_REV` is
a full test-repository commit, and every `--out` is outside both checkouts.

```sh
ROOTS="--repo e2e=/src/app-e2e --repo-revision e2e=$TESTS_REV"

corvint --root /src/app flows appmap build --manifest appmap.json --revision HEAD $ROOTS > /tmp/appmap-map.json
corvint-test e2e catalog --root /src/app --rev HEAD $ROOTS
corvint-test e2e harvest --root /src/app --rev HEAD $ROOTS --out /tmp/e2e-proposals.json
corvint-test e2e obligations --root /src/app --base origin/main --head HEAD $ROOTS
corvint-test e2e compile --root /src/app --base origin/main --head HEAD $ROOTS --out /tmp/e2e-compiled
corvint-test e2e prove --root /src/app --base origin/main --head HEAD $ROOTS --out /tmp/e2e-proof \
    --node-bin "$(command -v node)" --node-modules /src/app-e2e/node_modules
```

When the manifest is committed in the test repository, add `--manifest-repo e2e`. `--manifest` is
then a path in that repository.

- **`catalog` and `harvest`** read the test files from the pinned test root. An unknown read from
  that root is never matched to an application path by path alone. A proposal binds to a map file
  anchor only when both its path and its `repo` match. An outcome proposal can also bind through a
  flow outcome with the same locator.
- **`obligations`** still reads `UNKNOWN` (`unplaced-path`) for a changed application path that no
  block or screen places, never `NONE`.
- **`compile`** compiles scenarios from the blocks at the test root's pin. `e2e-compile/0`
  records `roots` and keeps every unknown span, so an unplaced path reads `INCOMPLETE`. An `--out`
  inside `--root` or inside any `--repo` checkout refuses `e2e-out-not-allowed`.
- **`prove`** reads each scenario's test tree from the test root at the compile report's pin: the
  scenario, its relative imports, the generated runner configuration, `package.json` and the
  lockfiles. It stages that tree over `--root`'s head and base application trees. Both sides record
  the pin as `test_revision`, and the report records `test_root` (`{repo, revision}`). A test-tree
  file that the staging would collide with reads `INCONCLUSIVE` `test-tree-overlaps-application`
  before any run. That covers a file that differs from an application file at the same path, one
  that an application file imports, and an import that resolves to an application file. An
  `--oracle-acceptance` file inside the test checkout refuses `e2e-oracle-acceptance-invalid`.
- **`shadow`** reads only a recorded window. It takes no root flags.

### What remains unsupported

- **Hosted-test proof with aliased roots (CCE-V0-133).** `prove --hosted-test` reads the hosted
  test from `--root` at head, so with any `--repo` it refuses `e2e-roots-unsupported` before
  reading anything.
- **The test checkout's worktree.** Every step reads committed objects. Commit the test-repository
  change and pin it with `--repo-revision` (or `HEAD`) first.
- **A test revision other than the catalog's pin.** `prove` runs on the pin that compile recorded.
  It never runs on a newer test commit.
- **Live two-tree Playwright qualification is NOT_RUN.** CCE-V0-136 is proved offline only.
- **A Git submodule as the test root.** A revision's tree holds only the submodule's commit
  pointer, not its files. Check out the test repository on its own and bind it with `--repo`.

For a hosted-test proof, use one tree. You can make one with `git subtree add`, either by bringing
the test repository into a local branch of the application repository or the reverse. Then run
`prove --hosted-test` there without `--repo`. Use the two-root flags for everything else.

## Step flags

Every flag that a step's help declares, as `corvint help --json` lists it. A test fails when this
guide stops naming one (CCE-V0-120).

| Step | Flags |
|---|---|
| `corvint flows appmap build` | `--manifest`, `--revision`, `--repo`, `--manifest-repo`, `--repo-revision`, `--workspace`, `--no-workspace`, `--from`, `--verify` |
| `e2e catalog` | `--rev`, `--root`, `--manifest`, `--out`, `--repo`, `--manifest-repo`, `--repo-revision`, `--workspace`, `--no-workspace` |
| `e2e harvest` | `--rev`, `--root`, `--manifest`, `--source`, `--out`, `--repo`, `--manifest-repo`, `--repo-revision`, `--workspace`, `--no-workspace` |
| `e2e obligations` | `--base`, `--head`, `--root`, `--manifest`, `--out`, `--coverage`, `--ledger`, `--module-graph`, `--repo`, `--manifest-repo`, `--repo-revision`, `--workspace`, `--no-workspace`, `--endpoint-source`, `--rails-routes`, `--openapi`, `--remedy-summary`, `--remedy-commits`. The endpoint flags join server endpoints to client calls so a changed controller reaches its calling screens (CCE-V0-152). The last two write `e2e-remedy-summary/0` (CCE-V0-151) and take no `--coverage` or `--ledger` |
| `e2e compile` | the obligations flags except `--remedy-summary` and `--remedy-commits`. `--out` is a directory |
| `e2e prove` | the compile flags, plus `--node-bin`, `--node-modules`, `--repeats`, `--package-json`, `--lockfile`, `--record-coverage`, `--oracle-acceptance`, `--hosted-test` and `--hosted-title`. The last two take no root flags, `--coverage` or `--ledger` |
| `e2e shadow` | `--window`, `--root`, `--manifest`, `--out` |

## When it refuses

`e2e-map-unavailable` means the manifest at that revision is missing or does not build. The
refusal names the revision side and the build error. It then points back to step 2:
`corvint flows appmap build --manifest FILE --revision REV`, against the manifest schema in
`docs/specs/application-map-v0.md`. In a two-repository layout it also covers the aliased roots.
The build error then reads `appmap-root-unavailable` or `appmap-root-dirty`:

- `appmap-root-unavailable`: the manifest names an alias that no `--repo` declares, or that root is
  missing, not absolute, not the top of a worktree, has no `HEAD`, or is the `--root` repository.
  A `--repo-revision` that is not a commit of that root, or an alias the manifest reads nothing
  from, refuses the same way.
- `appmap-root-dirty`: the unpinned test checkout has changes under the manifest or `tests.root`.
  With `tests.root` of `"."`, only changes under the declared directories count. Commit them, or
  pin a commit with `--repo-revision`.

`harvest` with aliased roots refuses `e2e-map-unavailable` too, rather than harvesting from
`--root` as if the suite lived there.

`e2e-roots-unsupported` means `prove --hosted-test` was given `--repo`. A hosted test is read from
`--root` only, so prove it in one tree (above), or compile and prove scenarios with the root flags.
