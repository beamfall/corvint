# Source removal B23 self-review repair — V1-5694

## Decision and scope

RMV-V1-008/009 removes the test-validity MCP consumer dependency while retaining
provider/editor qualification. Both the core transition harness and installed VS Code
harness now read original retained provider bytes through one bounded local reader.
The reader checks observation/projection agreement; the core still checks stdout byte
identity, all test execution/freshness/strength axes and Go preview status. Installed
comparison still checks the document digest, provider kind, complete unique test/run
item sets and Go input identity. The retired transport-only assertions have no surviving
transport to qualify; no MCP response is synthesized.

The class is an obsolete transport dependency in retained qualification, with an explicit
fail-closed decoder boundary. Its closed forms are unprofiled JS unit/E2E receipts and
Go `corvint-go-live-session-event/0` passed/failed/stale events. JS outcomes are only
passed/failed/skipped, Go actions pass/fail/skip/none/empty; nonterminal and unknown
forms, unknown members, unknown editor providers, omissions and inconsistent axes
fail qualification. JS run-level axes must remain UNSUPPORTED when no run observation
exists. Reading a retained file never establishes current-input freshness. Stale Go
negatives remain stale. The only ignored directory entries are the exact private
provider temporary-file form used before atomic retention rename.

Legacy summary field names and capture basenames remain. Their MCP-labelled slots now
refer to original retained provider bytes and the local reader source identity, as the
fixture README explains; these are no longer MCP qualification artifacts. Product
JSON, provider output, errors/exits, committed goldens and core-freeze outputs are
unchanged. Full release-host qualification is still NOT_RUN on this Linux sandbox.

AFP-V0-822 coverage is restored verbatim from the lane base as
`cmd/corvint/test_cache_afpv0822_unix_test.go`. It still compares stdout and manifest
bytes uncached/cold/warm before and after source changes, and checks graph hits and scan
memo creation. The active spec witness no longer says removed. No assertion changed.

Rollback: revert this repair commit; that reopens both reported findings. No product
implementation, provider protocol or frozen output needs migration.

## Verification

- `corvint-final affected --base 9f4d5d89ab0648425db40e617f3d890598f9ff79`
  succeeded, selecting cmd/corvint and internal/specindex. Scope FULL, 109 retained
  unknowns; no full-selection gate was run. The installed `corvint` refused as stale.
- Through `script/heavy.sh`, the full existing cmd/corvint package ran, failed in
  162.262 s on seven top-level tests whose nonrepository scratch paths discovered
  `/home/russell/lanes/.git`. No diagnostics/assertions/goldens were relaxed.
- Through `script/heavy.sh`, `TestAFPV0822DryRunCachedPlanMatchesUncached` passed
  independently in 0.645 s. The whole-package attempt is not represented as a pass.
- `make -s interactive-alpha-test`: 10 tests, 8 passed, 2 browser cases skipped
  without CORVINT_TEST_PLAYWRIGHT. The new regression invokes the core selection
  helper and the installed harness comparison for pass/fail/fix/stale transitions;
  unknown profiles, members, states, editor providers and false projections refuse.
- `node --check` passed for both consumer entry points and the retained process client.
- `make -s spec-requirements-check line-citations-check` passed, including the
  heavy-wrapped whole internal/specindex package. Repin found no moved citations.
- Go toolchain check passed: go1.27.4. Restored test bytes match the base exactly.

Independent review, broad repository checks, Windows/Darwin qualification, installed
release/VS Code/browser qualification and extension npm tests are NOT_RUN here: the
owner delegates broad checks/review, the host is Linux, and extension dependencies and
pinned installed tools are absent. No pushes, PRs, task writes or seals are authorized.
CEM bind/check is unavailable under docs/DOGFOOD.md (decision 0594); retain final
attempt details and the scratch input files in the external handback. Overall ticket
acceptance remains PARTIAL for the previously recorded external integration/qualification
blockers and the absent finish workflow, despite both reported repairs being implemented.

## Evidence

Raw evidence stays under `~/lanes/sr-b23-evidence/`. SHA-256:

- `self-review-affected-final.json`: `3662bc8bdea08caf6f52ae6a81a6d2309a8aa941936d3b3940418398c9284683`.
- `self-review-corvint-test.log`: `0a7540f0786b5304b123c9c6a47b4c31c6f98ffcdb16a27c87afe2db0d291870`.
- `self-review-cache-parity.log`: `f009ac8cf1dc686b8790be29093639ecd94c99ea925e689c441d52027d0887ed`.
- `self-review-harness-test-final.log`: `d4e67a18f5611f6e96615c1ae87143ba39444de4e29942c878877fd26230145b`.
- `self-review-spec-check.log`: `82923957c1350f7b0dac64841ee646f3592852c2602fc604de3f3b5a2a9f1631`.
- `self-review-repin.log`: `dce4b7fe3eadc26103692787d6775a7701ed62b33f01ccc227f7182433b8ea7a`.
