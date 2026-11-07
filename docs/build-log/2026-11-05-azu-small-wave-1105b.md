# Azu-small Wave 1105b: true-base qualification and complete merge replay

Date: 2026-11-05
Scope: V1-4688 / AZU-V0-005, V1-4689 / AZU-V0-006, V1-4690 / AZU-V0-007.
Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Starting checked revision: `0b4c2e605f16a0a1c922cc246a62ae81fb202801`.

## Decisions and repair

The earlier 49-versus-49 comparison compared two lane revisions, not the supplied base.
This round ran the entire existing CLI package at the actual base in a detached scratch
checkout under the mandated TMPDIR. Exactly 46 of the previous 49 failure names reproduce
there; no additional base failure appeared. The three lane-caused failures are
`TestAffectedUntestedGoPackageIsUnknownScope`,
`TestAffectedUntestedPackageGetsCompileCheck_AFP_V0_468`, and
`TestUseCaseHostileChangeConsequence`. The previous claim that all 49 were shared with
the base is withdrawn. The per-test attribution records every previous failure name,
its base-log line, current test result and raw-log hashes outside the repository.

AZU-V0-006 had removed an existing behavioural coverage warning from compile-only advice.
`compileAffectedAdvice` now also projects `plan.uncovered` into the existing advice warning
list. This restores AFP-V0-468's `NO_SELECTABLE_TEST: UNIT_ID` warning without adding a wire
member or moving a proven coverage gap back into selection uncertainty. The shared AZU spec
and the parent AFP-V0-468 clause state that distinction. Rollback removes this one projection and the accompanying clarification;
it restores the missing advice warning, not broader selection changes.

Two frozen assertions require both `UNKNOWN` scope and a testless unit in `plan.unknown`
where the explicitly requested AZU-V0-006 contract requires `plan.uncovered` instead.
Changing selection back would violate V1-4689; changing these assertions is outside the
charter's prohibition on weakening or deleting an existing assertion to pass. No fixture,
assertion, golden, core-freeze output, error, exit code or stderr contract was changed.
V1-4689 qualification therefore remains PARTIAL on this precisely named owner-contract
conflict; the other two tickets' bounded acceptance criteria are met. No native ticket
completion is claimed. The owner must reconcile those assertions with AZU-V0-006 before
whole-package qualification can pass independently of the shared host failures.

## Eight-merge acceptance replay

Both binaries read identical immutable merge content for the before/after runs, with
`affected --full --no-cache --base FIRST_PARENT`. The before binary's Go inputs at
`83d703f98` equal BASE's Go inputs (`git diff BASE 83d703f98 -- '*.go' go.mod go.sum` is
empty). Its retained binary digest is in the original class-fix record. The final round's
after binary includes the advice repair. There are no lane merges after BASE.

Configured runs overlay only this lane's binary-consumer declaration, and use the original
merge paths with `--worktree --paths-from`; they are labelled mutable configuration
experiments, not frozen merge receipts. Every one of the 16 after comparisons retains every
before selection. The summary retains exact added units, removed/added unknown entries,
parent/tree identities, command argv, receipt hashes and observed wall-clock times.

| Merge | Before selected / unknown | After selected / unknown | Configured selected / unknown |
|---|---:|---:|---:|
| 8e2c13aa9 | 339 / 92 | 342 / 90 | 344 / 88 |
| 39c031980 | 66 / 84 | 68 / 84 | 68 / 84 |
| d436dff76 | 261 / 99 | 263 / 99 | 265 / 97 |
| 6c33df33b | 185 / 83 | 187 / 83 | 189 / 81 |
| ea770fa17 | 75 / 63 | 88 / 63 | 88 / 63 |
| 24d8c04f9 | 38 / 79 | 39 / 79 | 39 / 79 |
| f13f52b16 | 46 / 86 | 47 / 86 | 47 / 86 |
| 9fdc2f02e | 36 / 79 | 37 / 79 | 37 / 79 |

At 8e2c13aa9, removed unknown entries are `UNINDEXED_SOURCE_PATH` for
`.github/testconfine/cmd/main.go` and `.github/testconfine/cmd/main_test.go`.
Configured runs at 8e2c13aa9, d436dff76 and 6c33df33b additionally remove
`UNSELECTED_BINARY_CONSUMER` for `tools/web-flows/test/e2e.mjs` and
`tools/web-flows/test/navigation-e2e.mjs`. No other unknown entry was removed.
The two testless shell units retain undeclared inbound uncertainty; no coverage gap is
invented to shrink this count. Shared frontier and unowned entries stay visible.
Per-command observed wall-clock costs range from 2.643s to 5.699s on the shared WSL host;
these are not quiet-host benchmarks. Zero missing baseline selections establishes bounded
replay non-regression, not full-suite/mutation safety or AZU-V0-009 zero-unknown acceptance.

## Acceptance and validation

V1-4688: hidden testconfine ownership/selection and disk/immutable parity retain their existing
whole-package evidence and targeted regressions. The extended replay now covers all eight
named merges, preserving every selection and naming the removed unindexed entries.

V1-4689: the optional uncovered wire, documentation, reader/caller preservation and undeclared
inbound uncertainty retain their existing tests. Compile-only coverage advice is repaired.
The same eight-merge replay retains unresolved inbound entries. Whole CLI qualification
remains subject to the two exact frozen assertions above, rather than being attributed to
the environment or the base.

V1-4690: exact argv selection and real command execution retain the existing consumer,
runner and CLI tests; legacy entries without run remain unknown. Configured merge replays
add the consumers only when their builds are reached and retain every original selection.

All new Go testing uses script/heavy.sh, one selected package per command, with
TMPDIR=/home/russell/lanes/azu-small.tmp (mode 700), GOTOOLCHAIN=local go1.27.3, and
the configured semaphore directory unchanged. Installed and current-tree affected plans
were captured before testing; the latter selects cmd/corvint and retains three language
frontiers. Only cmd/corvint Go source changed in this repair. Previous whole-package evidence
for unchanged plugin, graph, session runner, gate selector, flow and hidden-package inputs
is reused with its recorded revision and limits; no earlier execution is relabelled as new.

The whole current CLI package ran all existing tests without a run filter: FAIL,
428.898s package time / 442.8s admitted command. Its 48 top-level failures are exactly
the 46 base failures plus the two frozen plan assertions. No unrelated new failure appears.
The AFP-V0-468 compile-advice test now PASSes. Both current AZU CLI tests, the existing flow
goldens and both frozen core profile/envelope groups PASS as part of that whole run.
An additional base run of all three lane-caused test names PASSes (0.597s), independently
confirming that their earlier failures were introduced by this lane, not the supplied base.

The documentation gates ran once: 1 failed gate, use-case-receipts-check, because the repaired
source invalidated its existing implementation receipt. All other selected gates passed,
including whole internal/specindex, tools/test-commands-check and internal/lrfrepo runs.
The normal receipt repinner updates only source/receipt hashes and repositoryRevision; it
does not change the receipt's shape, result or attestation. The failed gate and the concluding
conformance runner are rerun after that pin repair, rather than repeating successful gates.
The first whole conformance/use-cases-v0 run raced ahead of the queued repinner and failed the
same subject-digest-mismatch; it is retained, not counted as final qualification.

After repinning, all 18 receipts are current. The final whole conformance/use-cases-v0
package PASSes (0.197s), and the concluding conformance runner returns valid=true,
22 use cases, 18 evidence entries, 21 UNPROVEN claims and 1 VERIFIED claim. No old claim
was promoted by repinning. Definition, traceability and line-citation checks pass; their
final staged check follows the last documentation-only clause/traceability updates.

## Evidence, limits and friction

Raw evidence and commands: `/home/russell/lanes/azu-small-evidence/wave1105b/`.
Base whole CLI: FAIL, 340.249s package time, 362.3s admitted command, 46 failures;
`base-cli-whole.log` SHA-256
`1e405ad9f461ab147624ffc09e8dde67ec3469e1499d48f7815e2d8bb9c5fd16`.
`replay-summary.json` SHA-256
`cb768ebbaeacc97c40da70001849f3bc52468de63f8aa063a6ce91647043eb66`.
Current whole CLI JSONL (4,342,853 bytes) SHA-256
`4466907b7819c9ccb286137d09dd631c5f2e5b5c1595da97dd9e416a3833f4a7`.
Final per-test attribution JSON SHA-256
`46a428885ea2f9b8eabb539ec2ab9f23bfcfa40269870b3519ace2064ca7efd9`.
Additional base three-test run SHA-256
`8f77104c98afb4bf071f901c21abadfd1f738d9ac8315269b6d717f01d27eca5`.
Initial doc-gates log SHA-256
`5dd7ff867b5d72f3133214989d8013568456d3786e095b7ef5400390f590d0bf`.
Final whole conformance package log SHA-256
`bdaf96c29f7a93eb33b4912f5923c82b96889b5ef23dd84deccb98e1ea32ce33`.
Final conformance runner log SHA-256
`9cac2b6354a0955e824b84395e386ced59d445410e2b98c0c3c26fe5d6ee5caf`.

1. Reproduce the 46 shared failures with the whole CLI command at BASE in the scratch
   checkout. Parent `/home/russell/lanes` has mode 775; scratch/executable qualification
   rejects that ancestor, and outside-repository tests observe that parent as a Git root
   during execution. The actual base receipts, not assumptions about the environment,
   establish each reproduced failure. The migration-ratchet digest failure is also retained.
2. Reproduce the two lane-induced frozen-contract failures with the whole current CLI
   package: both require testless units in plan.unknown while AZU-V0-006 requires uncovered.
   No assertion or fixture was weakened to remove the failures.
3. Heavy commands share the configured slots with other lanes and report low /tmp space;
   all task scratch stays under the prescribed ext4 TMPDIR. No lock directory, threshold,
   source qualification, parent mode or Git authority check was overridden.
   Receipt repinning waited 222.3s for its slot; the three-test base diagnostic waited 146.2s.

Used self-development routes: existing pre-change context/query/impact capture (reused),
affected selection, frozen replay, native CEM/OCM rebind and report inspection. No new start
capture runs. Guarding tests remain NOT_PRODUCED because their native execution bypasses
the required wrapper; automatic prechange receipts retain agent-receipt-absent. Native OCM
test linkage, execution-at-commit, criterion adequacy, external interoperability and
independent review retain their recorded NOT_OBSERVED / NOT_RUN state. Review and integrated
host qualification belong to the orchestrator. Repository-wide make gate, full-suite/mutation
AZU-V0-009 safety, real-browser/background-process qualification, non-Linux execution,
Windows vet before push, publication, native Tasks writes and seal are NOT_RUN under the
focused policy, runtime limits and explicit charter prohibitions. Final local outcome is
failed; a structural dogfood pass must not turn it into a qualification pass.
