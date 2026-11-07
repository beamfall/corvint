<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 5: shell canonicalization withdrawn

Date: 2026-11-03. Lane selprec2, on `494ba68d2`. Ticket: V1-4173.
Requirement: AFP-V0-2246 withdrawn and superseded by the earlier AFP-V0-2204 rule.
Orchestrator-made, owner-delegated decision: soundness over precision.

## Decision and independent findings

The independent review of `494ba68d2` returned CHANGES. Its supplied counterexamples show
that the AFP-V0-2246 allowlist does not prove what `cd` and `pwd` execute:

1. Compound prefixes hide refused commands: `if eval`, conditional source, `! enable`,
   `while`, `for` and `time` with `eval`, conditional `set -T` and DEBUG traps, and dash aliases.
2. A command word's slash can disappear through backticks or nested parameter expansion.
3. A script can fire a permitted INT trap itself with `kill -INT $$`.
4. A backslash-newline can split a function name or the `function` keyword.
5. The tracing refusal lacked a standalone `set -T` regression.

The narrowing is withdrawn entirely, rather than extending the allowlist. The original
AFP-V0-2204 literal temp-root rule survives: the literal suffix must be written directly
after the proven root expansion. Computed canonicalizations remain assignments and their
path tokens stay selected. This includes plain canonicalization, `pwd -P`, variable aliases,
and directly substituted paths. No canonicalization override, trap, function or command-word
allowlist remains. `shell.go` is byte-identical to `27d87691b` (before AFP-V0-2246).

AFP-V0-2245's TypeScript withdrawal and its counterexample tests are unchanged. AFP-V0-2247
and all earlier lane work are unchanged. The new review is owned by the orchestrator.

## Acceptance evidence

- `TestTempRootPathsAreNoTokens_AFPV02204`: all 30 canonical cases retain their tokens,
  including the four previously admitted cases. Existing literal-root controls still pass.
- `TestCanonicalizationCounterexamplesStaySelected_AFPV02246`: 26 table cases retain
  `internal/x/y.json`, covering all five review classes and every supplied sub-case, standalone
  `set -T`, six interpreter/header variants and six direct or assigned derivation controls.
- Fault injection: run that table with a Go overlay supplying `shell.go` from `494ba68d2`.
  It compiles and fails 17 subtests: all 13 bypass variants, plain bash/sh/dash, and a derived
  alias. The new implementation passes all 26 cases. Standalone `set -T` and the remaining
  conservative controls already passed the old implementation; they remain explicit regressions.
- Full `internal/liveverify/affected/shell` and `internal/liveverify/affected/typescript`
  package tests pass, one package per command through `script/heavy.sh`, with
  `HEAVY_WHOLE_REASON="selprec2 lane"`, `GOTOOLCHAIN=local`, `-count=1` and `-timeout 30m`.
  The TypeScript run includes all 17 cases of
  `TestTypeScriptAnchoredCounterexamplesStaySelected_AFPV02245`.

The pre-test `corvint affected --base cb491adb15dd88da032be365eee7f1f0ae06a958` receipt is
retained privately as `corvint/round5-affected.json` in the worktree Git directory: 194
selected, 81 unknown. This is an orientation plan over the full lane, not a before/after
precision measurement. Round 5 requires no new measurement campaign; previous measurements
remain dated evidence and do not claim the withdrawn shell narrowing.

## Verification and limits

Spec generation, formatting, documentation gates, Windows vet and the final CEM binding/check
are recorded with their commands and final results in `~/lanes/selprec2.handback.md` and the
private dogfood evidence. The round-4 handback is preserved as `selprec2.handback.prev.md`.
This entry records the decision and regression evidence; a CEM structural pass does not
substitute for the orchestrator's independent review.

`make gate` and broad Go package tests are NOT_RUN under the scoped round-5 instructions.
Landlock qualification (V1-4283), CI observation, and unconfined CEM guarding-test evidence
remain NOT_PRODUCED/NOT_OBSERVED. No ticket writes, publication or seal are authorized.
Rollback is already applied: retain the pre-canonicalization AFP-V0-2204 rule. Reintroducing
canonicalization narrowing needs a new accepted contract and all retained regressions.
