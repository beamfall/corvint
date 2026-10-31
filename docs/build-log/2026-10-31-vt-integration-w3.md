# VanillaTwisted integration blockers, wave 3 — V1-3526, V1-3527, V1-3498, V1-3525

Lane `vt-integration-w3`, base `dbc1713779fc17ce0386fe95d3e0f6347f5f9aee`. Four owner-priority
tickets that block an honest Corvint integration of the VanillaTwisted repository (a pnpm, Vite and
Playwright app). Requirement IDs come from the lane block 780-799.

## V1-3526: flat e2e layout (AMAP-V0-780)

Problem: `application-map-manifest/0` required a non-empty `tests.page_objects` disjoint from
`tests.specs`, so a suite with helpers beside its specs had to invent a page-object directory,
which then produced spurious `page-object-unbound` unknowns.

Decisions:

- `tests.page_objects` may be empty or omitted. Nothing then reads as a page object; files outside
  specs, workflows and scenarios stay support, as before.
- Only the specs/page-objects pair may share a directory. Any other repeated directory, or a
  directory repeated within one group, still refuses. That keeps the other roles unambiguous.
- Inside a shared directory, the role is decided by file name, using Playwright's default
  `testMatch` (`NAME.spec.EXT` or `NAME.test.EXT`): those files are specs and every other file is
  a page object. Considered instead: deciding by whether a file declares tests. Rejected because
  the name is what Playwright itself uses, it is predictable, and it needs no parse.
- The adoption guide (`docs/E2E-ADOPTION.md`) shows the flat layout with `web_root`.

Evidence: `TestAMAPV0780FlatLayout`; `internal/appmap` and `internal/changee2e` whole packages pass.

## V1-3527: browser-context imports (AMAP-V0-781, AMAP-V0-782)

Problem: imports inside `page.evaluate(async () => import("/src/x.ts"))` run in the browser, where
the Vite dev server resolves them, yet the map read them as the spec's own unresolved imports.
This was 28 of 31 unknowns on VanillaTwisted.

Decisions:

- Lexical scope: any `import(`/`require(` token inside the argument list of a `.evaluate(`,
  `.evaluateHandle(` or `.addInitScript(` member call. The receiver is not checked, so
  `locator.evaluate` and `frame.evaluate` count too; all of them run in the page.
- A specifier is browser-only when no literal occurrence of it lies outside such a call. A spec
  that also imports it in Node context keeps the AMAP-V0-005 rules, so no Node edge is lost.
- New import status `browser-context`. It is never an `unresolved-import` or
  `import-outside-tests`, never makes the file's join UNKNOWN, and is not a test-graph edge.
  Browser-reference unknowns have their own kind, `browser-reference`, which `testJoin` and
  `testGraphIncomplete` do not read. `testStageUnknown` includes the kind, so `--from` rebuilds
  re-derive it with the tests stage.
- Resolution needs an explicit manifest field, `tests.web_root`. The dev-server root is not
  inferred from `vite.config.*`; a guess would be invented certainty (invariant 2). Without the
  field the reason is `browser-root-undeclared`, which states its own remedy. `web_root` must be
  a directory at the revision in the tests repository. Suffix probing uses the specifier's own
  suffix, then `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, then `/index` with each.
- Out of scope: impact does not yet use a browser-context target as evidence that a spec exercises
  that module (`flows appmap impact`, `testImporters`). That is a possible follow-up, not filed by
  the lane.

Evidence: `TestAMAPV0781BrowserContextImports` covers a literal and a computed specifier inside
`page.evaluate`, a missing target, an undeclared root, a Node-context duplicate and a missing
`web_root` directory.

## V1-3498: package manager for Node commands (NGI-V0-780, TJAA-V0-780)

Governing specification located: `docs/specs/non-go-impact-v0.md`. Impact's `verification` member
is closed by the declared gate (NGI-V0-060, NGI-V0-062), and the per-path language command
precedes it (`toolchainCommand`). That command was hard-coded `npm test`. NGI-V0-780 now states
both rules in one place.

Decisions:

- One shared detector, `projectprofile.NodePackageManager`, used by both impact and the
  Playwright adapter. It reads the `packageManager` member of the root `package.json`, then the
  one manager signalled by a root lockfile or workspace file, then falls back to npm.
- When files name more than one manager and no member settles it, impact names no Node command
  (abstain, invariant 2). The Playwright adapter keeps `npx` in that case, because an argv is
  mandatory there.
- Commands: `pnpm test`, `yarn test`, `bun run test` (`bun test` would run Bun's own runner, not
  the script). Exec prefixes: `pnpm exec`, `yarn`, `bunx`, `npx`.
- Playwright argv are rewritten in one post-pass at the end of `SelectPlaywrightWithEnvironment`
  (`applyPlaywrightPackageManager`). This keeps the edit to the TS adapter, which another lane is
  also changing, small. A non-npm manager is bound into the source digest
  (`corvint:node-package-manager`). Npm repositories keep their exact bytes and digest.
- Not verified end to end: the ticket's second observation was that the declared `pnpm check` was
  not read. That happens because VanillaTwisted's `AGENTS.md` is a symlink, which is V1-1272 in the
  wave-2 batch and not on this base. With a regular `AGENTS.md` the declared gate closes the
  member, as `TestNGIV0780NodeTestCommandFollowsPackageManager` shows for the ticket's tuple.
- The analyzer-input pin in `analyzer_schema_test.go` was repinned by `script/analyzer-schema.sh
  bump`. The schema number stays `corvint-analyzer/150`, because the audited output is unchanged.

Evidence: `TestNGIV0780NodeTestCommandFollowsPackageManager` and
`TestPlaywrightArgvFollowsPackageManager_V13498`. The `internal/contextindex`,
`internal/projectprofile` and `internal/liveverify/affected/typescript` packages pass whole, and
`cmd/corvint -run Playwright` passes.

## V1-3525: tasks init with a linked intent worktree (CTW-V0-780)

Problem: with the primary on a feature branch, a refusal after `git worktree add` had succeeded
still told the operator to run `git worktree add`. The rejection named the missing
`.taskman/queue.json`, but the fix always ended with prune/repair/add.

Decisions:

- Routing is unchanged. The hint stays in the primary's `queue.json` (CTW-V0-003), and the
  projection stays in the intent worktree. Reading the intent branch from a linked worktree when
  the primary has none would change CTW-V0-009's byte-identity guarantee. It is recorded as a
  non-goal, not built.
- A registered worktree that passes every registration check and is refused only by its
  projection (symlink, missing, malformed or foreign `queue.json`) already exists. The fix then
  names that root's `.taskman` and the primary's files as the example. It names `git worktree
  prune` only when a stale entry is also reported, and never names add or repair.
- `docs/TASKS-EXTERNAL-AGENTS.md` documents the copy requirement: both checkouts need
  `queue.json`, the intent worktree needs `policy.json`, and records land only in the intent
  worktree.
- The existing `TestCTWV0004_StaleOrForeignIntentWorktreeIsNotAdmitted` asserted `worktree add`
  for the three projection cases, which is the behavior the ticket reports as wrong. Its
  expectations were updated.

Evidence: `TestCTWV0780_InitFromFeatureBranchWithLinkedIntentWorktree` runs real `git worktree
add` and init. `internal/tasks/intent` and `internal/tasks/store` pass whole, and
`internal/tasks/cli -run CTW` passes.

## Verification

- Focused and whole-package tests, listed per ticket above. They ran through `~/lanes/heavy.sh`
  with a whole-package reason, because `corvint test --dry-run` selected 120 units (NARROWED).
- `make doc-gates`, `make line-citations-check`, `gofmt -l` on the changed Go files, `go vet` on
  the changed packages and `make windows-vet` all pass.
- Exhaustive `go test ./...` and `make gate`: `NOT_RUN`. The owner's standing preference is
  focused tests for scoped issue work. The Codex review is run by the orchestrator.

## Friction

1. `corvint test --base dbc17137... --dry-run` selected 120 packages whole. The reasons included
   `UNBOUNDED_READER via docs/E2E-ADOPTION.md` and `BINARY_EXEC via internal/appmap/compile.go`,
   for a change confined to `internal/appmap`. One adoption-guide edit widens the plan to dozens of
   unrelated packages. It also printed `DEGRADED self-binary-stale`, because the installed binary
   predates the worktree's own edits. Repro: edit `docs/E2E-ADOPTION.md` plus
   `internal/appmap/compile.go`, then run `corvint test --base <base> --dry-run`.
2. heavy.sh refuses a whole-package run until the caller pastes a free-text `HEAVY_WHOLE_REASON`.
   When Corvint itself selected the package whole, the reason could be taken from its own plan.
   Repro: `~/lanes/heavy.sh env GOTOOLCHAIN=local go test -count=1 ./internal/appmap` refuses with
   `whole-reason-missing`.
3. Any edit to `internal/contextindex/receipt.go` fails `analyzer_schema_test.go` until
   `script/analyzer-schema.sh bump` repins it. Here the schema number did not change, but the pin
   line did, so parallel lanes touching analyzer inputs will conflict on that one line. Repro: edit
   a comment in `receipt.go`, then run the `internal/contextindex` tests.
4. `make spec-requirements` refuses a new requirement range named in a spec's Agent digest Status
   (frozen header text, SRG-V0-980). The lane charter does not mention it, so the first attempt
   wasted a cycle. Repro: add `AMAP-V0-780..782 are experimental` to the Status line of
   `docs/specs/application-map-v0.md`, then run `make spec-requirements`.
5. `test-read-scopes-check`, in `doc-gates`, flags a path-shaped literal inside an error-substring
   assertion (`.taskman/queue.json is missing`) as a repository read. That happens because an
   unrelated test in the same package holds `"../..\n"`. Repro: add that literal to any
   `internal/tasks/store` test.
6. `git stash` is shared by every worktree of the checkout, so one lane's `git stash list` shows
   another lane's entries. Lanes using stash can drop each other's work. Repro: `git stash list`
   from `corvint-lanes/vt-w3` shows the affected-w3 lane's stash.
