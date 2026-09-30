# Test Attack V0

Owner: Russell Lewis
Date: 2026-09-30
Requirement prefix: `TAT-V0`
Intent status: proposed technical contract; user-approved product intent (explicit request to deliver “Attack my tests” with the six wow additions)
Delivery status: experimental
Authoritative inputs: repository-owner request; `AGENTS.md` invariants 1, 2, 4, 7 and 8; `docs/specs/falsifiable-packet-v0.md` FPK-V0-014 and FPK-V0-017.

## Agent digest

- Claim: an explicit committed-range Go test attack continues past the first kill and exposes surviving mutants in the bounded selected set.
- Status: proposed technical contract / experimental implementation.
- Exists: `cmd/corvint/prove_attack.go`, `cmd/corvint/prove_attack_test.go`, and the existing `internal/liveverify/mutate` complete traversal.
- Blocked on: independent review and retained final integration evidence; no semantic adequacy or external usefulness qualification.
- Read next: Requirements, Failure modes, Acceptance evidence, Rollback.

## User and measurable job

An engineer whose tests pass asks whether a changed condition can be broken without those tests noticing. The existing `prove --base FULL_SHA --mutate` stops a row at its first killed mutant. An explicit `--attack-tests` continues through its selected set and identifies surviving mutants with immutable locations. A passing first-kill verdict can coexist with survivors. The attack is evidence to inspect and improve assertions, never evidence of complete test coverage or correct behavior.

## Verified current state

At base `6dc8ed1bceaa563c4e2cddb505b5891741bbce5a`, `mutate.Request.Complete` and `Report.Survivors` already exist. `cmd/corvint/prove.go` uses grouped first-kill judgments and does not expose those survivor records. Changed-line confinement, immutable export, baseline admission, offline execution, row and invocation budgets, and sandbox cleanup come from FPK-V0-014/017. This slice composes those facilities; it introduces no new mutation operator.

## Requirements

- `TAT-V0-001`: `prove --base FULL_SHA --mutate --attack-tests` MUST be an explicit opt-in. The flag MUST require both range and mutation modes, refuse repetition, and respect positional `--`. Only Go source/Go test mutation rows execute in attack mode; other mutation languages remain `NOT_RUN` with an unsupported-language reason.
- `TAT-V0-002`: The attack MUST reuse the immutable range export and changed-line spans, evaluate at most eight mutants per row, and retain the existing ten-minute row and thirty-minute invocation budgets. It MUST use the complete single-claim runner instead of the grouped first-kill runner.
- `TAT-V0-003`: Every judged mutation row MUST expose an additive `attack` object with `mutants`, `killed`, `survived`, `uncompilable`, `skipped`, `survivors`, `detail`, `status`, and `selected_mutant_set_complete`. `COMPLETED` means all mutants in that bounded selected set were observed; it MUST NOT mean exhaustive mutation, adequate tests, semantic coverage, or correctness. The legacy falsifier verdict retains its existing meaning, so `PASS` can coexist with surviving mutants.
- `TAT-V0-004`: Budget interruption MUST retain every already observed count and survivor even when the row verdict is `NOT_RUN`; an interrupted nonempty selected set is `PARTIAL`. No sandbox, baseline failure, no mutable sites, unsupported language, unavailable inputs, or invocation exhaustion before selection MUST remain explicit `NOT_RUN` observations. Uncompilable mutants count separately and never earn kill credit. Infrastructure errors retain the existing typed error instead of a fabricated result.
- `TAT-V0-005`: Each observed survivor MUST name its operator, changed path, committed blob, one-based source line and zero-based end-exclusive byte span; the attack MUST retain the checkout commit. The parent row retains the test identity. Counts and locations describe the selected row's tests only.
- `TAT-V0-006`: The attack MUST preserve caller repository bytes and the existing sandbox, offline dependency and exported-copy cleanup contract. Cancellation MUST retire ordinary descendants in the run's process group. The inherited macOS residual for a descendant that changes session remains explicit; this slice MUST NOT claim arbitrary hostile-descendant termination.
- `TAT-V0-007`: Invocation without `--attack-tests` MUST retain existing output and execution semantics. No Stop hook, automatic mutation execution, persistent ledger, network service or test-authority promotion is introduced.

## Failure modes

| Condition | Required observation |
|---|---|
| Flag without range/mutation or repeated | Argument error before execution |
| Dirty cited path, no changed-line sites, unsupported mutation language | Row `NOT_RUN`, reason retained |
| Sandbox unavailable, baseline red, dependencies unavailable offline | Row `NOT_RUN`, reason retained |
| Budget before selection | Attack `NOT_RUN`, no invented counts |
| Budget after some selected mutants | Attack `PARTIAL`, observed counts/survivors retained; row `NOT_RUN` |
| All selected mutants fail compilation | `COMPLETED` selected set, separate uncompilable count; row `NOT_RUN` |
| Kill and survivor in same selected set | Legacy `PASS`, surviving locations remain visible |
| Drift or export/runner infrastructure failure | Existing typed refusal; no fabricated completed receipt |

## Acceptance evidence

`TestProveAttackAdmission` covers explicit flag admission. `TestProveAttackFindsWeakConditionAndStrongTestKillsIt` uses a changed `n > 0 && n < 10` condition: an inside-only assertion kills negation but permits `&&` to `||`; outside-range assertions kill that survivor. It checks immutable survivor bindings and unchanged repository bytes. `TestProveAttackUnavailableAndDefault` covers baseline rejection, invocation exhaustion and default omission. `TestProveAttackPreservesPartialReport` checks projection of actual runner report fields, including mixed observations under interruption; it is not a live interruption witness. `TestProveAttackPreflight` covers dirty inputs and unsupported language before execution. `TestRunCancellationRetiresProcessGroupDescendants` observes a live child before cancellation and checks retirement and unchanged repository bytes. Existing mutation tests retain sandbox-unavailable, offline dependencies, unbuildable mutants and changed-line bounds.

Required delivery evidence: focused tests with skips retained, a real CLI weak/strong fixture, independent review, and root-coordinated final dogfood CEM/OCM integration. A passing focused suite is not the repository-wide gate. External outcome qualification remains `NOT_RUN`.

## Non-goals and limits

No new operators, language expansion, automatic hooks, exhaustive coverage, semantic correctness claim, user repository edits, or test generation. Selection may omit tests or paths; existing packet/affected uncertainty remains visible. Equivalent mutants may survive. Per-row complete traversal can cost more than grouped first-kill execution; the inherited deadlines bound it. A completion flag describes only the capped selection, not all possible mutants. The inherited sandbox permits reads and confines writes; on macOS a descendant that changes session can outlive group cleanup while remaining confined.

## Rollback

Remove the explicit flag, its additive report and its tests/spec admission. Default commands retain the existing behavior throughout. There is no persisted data migration, automatic hook or service to unwind.
