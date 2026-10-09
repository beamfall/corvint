# 2026-10-08: Mocha affected selection reconciliation (V1-0598)

## Intent

Ticket V1-0598 asks that Mocha affected selection and Mocha execution agree. An exact selection
fixture and its execution receipt must reconcile selected, expected and observed tests. No-match
and ambiguous selectors must keep their uncertainty.

## Inventory audit

The TypeScript affected adapter registers fifteen concrete runner IDs. Each one, including
`mocha`, is also a dynamic execution profile ID. The adapter's `unknown` runner has no profile.
The new `TestRunnerRegistrationsHaveExecutionProfiles` pins this as proposed `TRE-V0-025`. The
old dynamic README sentence said Mocha affected routing was unimplemented. That sentence was stale
and has been replaced.

## Failing before

The existing opt-in qualification was extended to six real Git fixtures. It was run against real
Mocha 12.0.3 (installed with `npm install --ignore-scripts mocha@12.0.3` in
`/private/tmp/claude-501/mocha-td/npm`, `bin/mocha.js` SHA-256 `442804c2…f13655e`) on Node
22.23.3, macOS arm64. Before the fix, three of the six cases failed. Log:
`/private/tmp/claude-501/mocha-td/before.log`, SHA-256 `ef4d22d7…2842e`.

- `no-match-selector`: Mocha only warned about `tests/missing.test.cjs`, exited 0, and the
  observation was complete.
- `configured-spec`: `.mocharc.json` `spec` added `other/extra.test.cjs` to the positional
  selector. The extra test ran, and the observation was still complete.
- `zero-test-selector`: this run was already incomplete (`no-tests`), but it did not name the
  selector.

## Change

Proposed `TRE-V0-024` adds `reconcileMochaSelection` in `internal/testrunner/dynamic`. It runs
for runner `mocha` only. Selectors are joined lexically to `SourceRoot`, so `Parse` stays pure. A
run is incomplete when it has a native file outside the selected files (`unselected-test-file`),
a selector with no observed test (`selector-without-tests`), or an identity outside non-empty
`ExpectedTests` (`unexpected-observed-test`). The shared `Normalize` boundary then sets public
states to UNKNOWN. Fourteen citations of later `parse.go` lines shifted by one line. Their content
anchors are unchanged.

The qualification derives selectors from the plan only for a `BOUNDED` plan whose selected units
are all `typescript:mocha:`. The untested-change, configured-spec and Mocha-plus-Jest fixtures
are `UNKNOWN` and yield no selectors.

## Passing after

Evidence: `/private/tmp/claude-501/mocha-td/after2/qualification.json`, SHA-256
`bc666080…0fd995c8e`, from the post-review build. All six subtests pass.

- `exact`: the plan is BOUNDED, with one selected file and one exclusion. Selected file,
  expected identity and observed identity reconcile, and the observation is complete.
- `no-match-selector`: incomplete with `selector-without-tests`.
- `zero-test-selector`: incomplete with `no-tests` and `selector-without-tests`.
- `untested-change` and `ambiguous-runner`: the plan is UNKNOWN, no selectors are derived and
  nothing is executed. The ambiguous unit is `typescript:unknown:`.
- `configured-spec`: the plan is UNKNOWN. A forced narrowed run is incomplete with
  `unselected-test-file` and `unexpected-observed-test`.

`TestMochaBuildExecuteParse` also passes against the same Mocha. It now canonicalizes its temp
root, because Mocha reports resolved paths.

Focused tests pass: `./internal/testrunner/...`, `./internal/liveverify/affected/typescript`
and `./internal/specindex`. The listed doc gates also pass.

## Independent review

Codex (`gpt-6-astra`, read-only) found no P0 or P1 issues. Its review is at
`/private/tmp/claude-501/mocha-td/review.md`. It raised one P2: the expected-identity check also
ran when no selectors were given. That broke the stated no-selector compatibility. The fix gates
the whole reconciliation on non-empty selectors and adds a no-selector regression with an
expected subset.

## Non-goals and failure modes

There is no shipped plan-to-command derivation. The selector derivation is qualification code.
Configured Mocha discovery is not resolved. Directory selectors, symlinked roots and partial
expected lists fail closed as incomplete. They are never widened into a pass.

## NOT_RUN

- ESM, TypeScript loaders, parallel mode, root hooks and `--recursive`.
- Other Mocha, Node or OS tuples.
- `make gate` and `./...`, which this shared host does not allow.
- The dogfood CEM bind and seal, the native ticket completion write and the PR. The coordinator
  owns these.

## Rollback

Remove `mocha_selection.go`, its single call in `Parse`, the new tests, and the TRE-V0-024 and
TRE-V0-025 section. Mocha observations then return to their earlier completeness, and no wire
or receipt field changes.
