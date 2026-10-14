# V1-1373: the shell test units are declared in test-commands.json

Date: 2026-10-14. Branch `claude/v1-1373-shell-test-declaration`, base `origin/main`
`d63c1c3a1b091c6bc13aecc81bd03e0d3fa8c0a3`. Ticket V1-1373 (P1 bug). Contract:
`docs/specs/session-test-runner-v0.md` STR-V0-003, STR-V0-004 and STR-V0-006, unchanged; no
requirement added. Consumer: V1-1257 (STR-V0 promotion evidence), which found every one of 112
shadow `corvint test` plans over 14 runs `INCOMPLETE`.

## Why

`.github/cishards/test-commands.json` declared Go, `typescript` and six `none` languages but not
`shell`. The affected plan selects `shell:script/*_test.sh` units on nearly every PR (a dirty path
read by a shell test by path literal is enough), so `corvint test` recorded
`shell: no qualified adapter and .github/cishards/test-commands.json declares no shell full command`
and the plan was `INCOMPLETE` (exit 3) even on a PR touching no shell file. Forward rows stayed
CLEAN only because `.github/forward-rows/row.jq` `limit_only` reports a no-adapter gap as a limit.
This was a declaration gap, not a selector defect.

## Decision

- `none` is not available for shell: `make gate` runs fifteen `script/*_test.sh` suites as
  GATE_STEPS recipes, so STR-V0-004 forbids a `none` assertion (and `tools/test-commands-check`
  has no shell runner list, so it would refuse one anyway).
- No single gate step runs the shell suites, so the Makefile gains one recipe-less aggregate,
  `shell-test`, whose prerequisites are exactly the GATE_STEPS steps whose recipe runs a
  `script/*_test.sh`: go-format-test, go-archive-gate-test, spec-requirements-test,
  traceability-tests-test, decision-numbers-test, instruction-conflicts-test, eol-policy-test,
  license-headers-test, line-citations-test, ci-least-privilege-test, release-checklist-test,
  gate-receipt-test, error-code-ownership-test, host-package-versions-test,
  no-python-runtime-dependency-test. The declaration names it: `"shell": ["make", "shell-test"]`.
  This reads STR-V0-004's "declares that step's command" for a language spread over several gate
  steps as the command that runs exactly those steps; the spec text is not changed.
- `TestSTRV0004ShellTestIsTheGateShellSteps` (`tools/test-commands-check`) keeps the aggregate in
  sync: the declaration's shell entry is `make shell-test`, `shell-test` has one rule and no recipe,
  and its prerequisites equal the GATE_STEPS steps whose recipe text names a `script/*_test.sh`. A
  mutation removing `go-format-test` from the prerequisites fails it. `make test-commands-check`
  rule (e) dry-runs the new target. `TestSTRV0006DeclarationMatchesMakefile` pins the summary
  line, so its count moves from 1 to 2 declared make targets (the only pinned-fixture change).
- Not changed: the selector, `row.jq` and its `limit_only` semantics, the forward and shadow
  drivers, `tools/corvint-pr-tests`, `test-slices.json`, `package-costs.json`.

## Acceptance evidence

AC 2, same diff before and after. Before: the #874 tested merge `5c4844bc8` (first parent = PR
base `92c298a6e`, so its diff is exactly #874's 10 files, none shell). After: base' `aebb56368`
= `92c298a6e` + this change's declaration and Makefile, head' `f995b0ac3` = `5c4844bc8` + the same
two files (local scratch commits, not pushed); `git diff | git patch-id --stable` is `33665afb5` for both pairs. corvint built from
this branch; `--confine` is a stub path (a dry run never starts it).

    corvint --root WT test --base 92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431 --profile ci \
      --confine STUB --placement universe --shard 0/1 --dry-run --manifest M   (WT at 5c4844bc8)
    exit 3; scope NARROWED, 182 units; incomplete ["shell: no qualified adapter and
    .github/cishards/test-commands.json declares no shell full command"]
    (--profile local: exit 3, same incomplete)

    corvint --root WT test --base aebb56368f2c367a9a97a3bfd7a44dabba71df64 --profile ci \
      --confine STUB --placement universe --shard 0/1 --dry-run --manifest M   (WT at f995b0ac3)
    exit 0; scope NARROWED, 182 units; incomplete []; checks gain "make shell-test"
    (still required, not run, as the ci profile reports every declared command)
    (--profile local: exit 0, 183 units, the extra unit is declared:shell)

AC 3: `.github/forward-rows/selection.jq` over those outputs (the CI argument shape) gives
`incomplete` with the shell gap before and `[]` after, so row.jq's `$gaps` is empty and
`limit_only` filters nothing; a CLEAN row no longer depends on it. The hosted proof is this PR's
CI shadow artifact (`corvint-test-shadow-*`), collected at closeout: `NOT_OBSERVED` here.

Checks run: `make test-commands-check` pass; `make shell-test` pass (51 s, all fifteen suites);
`go test -race` and `go vet` on `tools/test-commands-check` pass; `.github/cishards` as the CI
temporary module, `-race` and vet, pass; focused packages from `corvint affected` pass
(`internal/sessiontest` TestSTRV0015BudgetBoundsTheShard failed once on its 1.6 s budget under
load and passed alone; `conformance/release-artifact-v0` needs a clean worktree and passed after
commit); `cmd/corvint -run 'STRV0|Declar'` pass; `script/gate-affected-corvint_test.sh` and
`script/dogfood-cache_test.sh` pass; actionlint clean; the CI doc gates pass.

## Failure modes and limits

- A new GATE_STEPS step running a `script/*_test.sh` without joining `shell-test` fails the sync
  test in `go test ./...`, not `make test-commands-check`; the sync test counts a step only when a
  recipe line starts a `script/*_test.sh` (after `@`, `+` or `-`), so a step that reaches a shell test
  through another script or make target is not seen, and it reads only the root Makefile, so a
  `shell-test` rule added in an included file is not seen (the Makefile includes nothing today).
- Shell suites outside the gate (for example `script/dogfood-change_test.sh`,
  `script/proof-tour-test.sh`, `script/gate-affected*_test.sh`) are a recorded limit the declared
  command does not run, as STR-V0-004 records for suites outside the gate.
- In the local profile a plan selecting any shell unit now runs all fifteen gate shell suites
  (about a minute) instead of exiting `INCOMPLETE`.

## Independent review

One Codex read-only round (`codex exec -s read-only`), verdict CONCERNS, two findings. Fixed: the
sync test matched a script name anywhere in a recipe line, so `@echo script/x_test.sh` would have
passed; it now requires the line to start the script, and that echo mutation fails it. Recorded,
not fixed: the test reads only the root Makefile, so an included `shell-test` rule is unseen (see
limits above).

## Rollback

Revert the commit: the `shell` entry, the `shell-test` target and the sync test go together, and
plans return to `INCOMPLETE` on the shell gap.
