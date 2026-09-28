# AFU default projects and stability-qualified verification

Owner intent: issues [316](https://github.com/beamfall/corvint/issues/316) and
[317](https://github.com/beamfall/corvint/issues/317), under AFU-V1-015/016,
019..024 and 041/042. Base: `86fe981154b53f388bec6c4378eeef97649566e9`.

## Decision and evidence

Playwright's explicitly present empty project string is a valid default identity. Missing/null
inventory fields remain invalid; canonical discovery retains its required field and immutable
revision/config/source bindings. Exact project/test reconciliation still controls full-suite
fallback. Strict and coverage outputs retain their frozen goldens.

A current passing run is insufficient for `verified`. Map/gaps, navigation, documentation and the
opt-in MCP flows profile accept a committed registry and share its qualification. The registry is
read at the captured intent revision. Every matching aggregate and every actual current project
must qualify, including cleanup and applicable negative controls on each contributing record.
Permissive thresholds cannot override flakiness. Missing, invalid, incomplete, stale and
below-threshold stability evidence stays explicit. Run-evidence and registry wire shapes remain
unchanged. Rollback is the issue slice revert, retaining an unqualified label for per-run passes.

Both original failures were reproduced before the fixes. Focused regression evidence covers empty,
missing and null identities; exact reconciliation and fallback; absent registry; qualifying and
below-threshold aggregates; missing repetitions/records; dirty and moved-HEAD policies; wildcard
projects, an explicit empty project and conflicting scopes; cleanup, controls and retry flakiness.
Existing AFU acceptance, docs, navigation, MCP parity and E2E selection corpus checks cover the
shared consumers. Four reviewed goldens changed with explicit owner approval: two fixture revision
IDs, and map/gaps checkout becoming incomplete without stability evidence.

The real interactive-alpha fixture used locked Playwright 1.63.0 and its matching Chromium headless
shell. Its checked-in `playwright.config.js` was copied byte-for-byte and never renamed the project.
A committed `data-testid` addition to `app/index.html` was the change. The native complete JSON list
reported `projectId: ""` and `projectName: ""`; the source-bound discovery matched, `e2e-safe` returned
`narrow-selection-allowed` with `e2e.spec.js`, and Chromium passed the test. An incomplete discovery
returned `full-relevant-suite-required`. Server/browser cleanup completed. Fixture base
`f93f419326516b1c67a53e358d6a7c91d921a916`; changed source
`c420bb36198d5b643339a74f8fdb6bd17e726bb0`. Raw list, selection, fallback, browser result and summary:
`/tmp/corvint-316-317/live-*.json`. Initial execution failed because a pre-existing browser cache
contained no executable; the matching runtime was installed into a task-owned temporary cache.

Independent Astra/medium review found no HIGH/MED correctness defect. Its test-coverage requests
were added, and a duplicated argument-help phrase was corrected. The existing process cleanup
regression initially hit a host `ps` timeout in its blocked-hook case; SIGINT/SIGTERM cases passed.
The blocked-hook retry passed (`/tmp/corvint-316-317/cleanup-final.log`). Real-browser
SIGINT/SIGTERM witnesses also passed (`/tmp/corvint-316-317/browser-cleanup-canonical.log`);
their first attempt correctly refused the noncanonical `/tmp` cache alias before launch.

## Self-use and verification boundary

Used: pre-change query and tracked-path impact, affected advice, focused regressions, and the
required CEM/OCM workflow. Original receipts are under the worktree's private Git directory. Query
omitted four ranked results; impact omitted 37 and named omitted CLI/MCP callers, which were read.
The first impact request named nonexistent `internal/appflows/map.go` and refused; the corrected
request named the discovered `query.go`. Initial dogfood coordination ran before changes and
retained its missing-map/outcome reasons. Evidence and checkpoint are under
`/tmp/corvint-316-317/`; unavailable billed tokens and full session cost stay `NOT_OBSERVED`.

Not applicable: retrieval-ranking or learning evaluations, external-provider production promotion,
source-view experiments and native host qualification. The AFU selection frozen corpus is exercised
by the focused CLI tests. Repository-wide `make gate` is `NOT_RUN` under the owner's scoped-issue
policy; selected tests, vet, documentation checks, live qualification and independent review are
this change's checks. Structural dogfood completion is separate from behavioral adequacy and does
not qualify a native host or any external producer/consumer matrix.
