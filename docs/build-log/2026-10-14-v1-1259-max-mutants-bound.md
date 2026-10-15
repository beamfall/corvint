# 2026-10-14 Discrimination witness mutant bound (V1-1259)

Branch `claude/v1-1259-max-mutants-bound`, from `origin/main`
`92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431`. Requirement `CEM-V1-090` (new) in
`docs/specs/cem-stable-v1.md`, accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). 1.0 scope per the V1-0020 dependency.

## Intent

Decision 0523 found that a `discriminates` witness claiming 1,000,000 mutants under
`bounds.maxMutants: 1` was accepted: the count rule checked only `killed + survived <= mutants`.
The ticket was filed against `cem/0.3` at `fb11b2274`. `cem/0.1..0.3` have since been removed
(V1-1278), so the orchestrator read the target as the `cem/1.0` native reader
(`internal/cem/wire/discriminate.go`), the portable consumer (`interop/cem01-go/cem.go`,
`stableDiscrimination`) and the shared stable tamper matrix.

## State at BASE

- Survivors inconsistent with `survived`: already refused `invalid-field` by both consumers at
  BASE (native `validateDiscrimination`, "survived must equal the number of survivors"; portable
  `uint64(len(d.Survivors)) != d.Survived`). A survivor line outside `newRange` was also already
  refused by both. That half of the acceptance was met; this lane adds the shared matrix case.
- `mutants > bounds.maxMutants`: admitted by both. This half was missing.
- No committed `.corvint/changes` map carries `discriminates` (`git grep -c '"discriminates"' --
  .corvint/changes` finds nothing at BASE), so the new refusal changes no committed verdict.
- The producer (`internal/cemdiscriminate` via `internal/liveverify/mutate.generateMutants`) stops
  at `maxMutants`, so it never emits a witness the rule refuses.

## Decisions

- **Bound.** `mutants <= bounds.maxMutants`, refused `invalid-field` at the wire stage by both
  consumers; equality is admitted. The inherited `ALGORITHMS.md` text says not to require it
  "without an existing native obligation"; `CEM-V1-090` is that obligation.
- **No new survivor rules.** Survivors need not be distinct: two mutants with the same operator on
  one line are legitimate, and decision 0523 asks only for consistency with the counts. The
  `newRange` rule already exists in both consumers and is unchanged.
- **Matrix cases.** Three cases join `protocol/cem-1.0/stable/tamper-matrix/`, each the clean map
  with a witness on the `greet.go` hunk: `discrimination-mutants-over-bound` (1,000,000 killed
  under `maxMutants` 1, wire `invalid-field`), `discrimination-survivors-inconsistent` (`survived`
  2, one survivor, wire `invalid-field`) and `discrimination-at-bound` (1 of 1, ACCEPT with the
  clean citation block). The witness is not part of the hunk identity, so hunk IDs are unchanged.
  The case files and `EXPECTED.json`/`manifest.json` entries were written by a scratch script (not
  committed), in the existing canonical encodings. `tools/cem-stable-regen` regenerates additive
  result members only and does not apply here; both consumers' manifest checks bind the new
  entries.
- **Required tamper classes unchanged.** The coverage lists in the two matrix tests are not
  extended; both tests already run every `EXPECTED.json` case. Extending the native list would
  touch `internal/cem/verify`, outside this lane's scope.

## Failure modes

A map whose witness over-reports mutants now fails the wire stage instead of verifying. A
consumer that skips the bound fails `discrimination-mutants-over-bound` by name: with the two
code lines reverted, `TestCEMV1226TamperMatrixNative` and `TestCEMV1226TamperMatrixPortable` both
fail exactly that subtest.

## Acceptance evidence

Host load average 30 to 65 during the runs (shared with six other lanes).

- `TestInheritedDiscriminationWitness` (`internal/cem/wire`): pass, including the new
  `mutants-exceed-max-mutants` refusal and the at-bound admission; `-race` pass.
- `TestCEMV1226TamperMatrixNative` and `TestCEMV1226TamperMatrixPortable`: pass with all 23 cases,
  `-race` pass on both. With the two consumer code changes reverted, both fail exactly
  `discrimination-mutants-over-bound`, and `discrimination-survivors-inconsistent` passes (the BASE
  behaviour).
- `go test -count=1 ./internal/cem/...` plus the bounded `corvint affected` readers
  (`internal/cemdiscriminate`, `tcq`, `witness`, `witnesscollapse`, `diagnostic`, `obscorpus`,
  `observations`, `semescalate`, `betarung`, `specindex`, `doccompiler`, `compactionkernel`,
  `tasks/obligation`, `conformance/frontier-v0`, `conformance/use-cases-v0`,
  `tools/cem-stable-regen`, `tools/fixture-maintenance-check`, `tools/gate-ledger`,
  `script/spec-coverage-audit`): pass, except four process-descendant tests in
  `internal/cem/gitrun`, `TestTrustedNavCancellation` (`internal/doccompiler`) and
  `TestDiscriminateRecordsWitnessAndReportDowngrades` (`internal/cem/workflow`, its weak run went
  `not-run` on the wall-time budget after 1,667s). Each passed when rerun alone: environment noise
  under load.
- `interop/cem01-go`: `go test -count=1 ./...` pass (654s), `go vet ./...` pass.
- `go vet ./internal/cem/wire/`, `gofmt -l` (empty) and the twelve CI doc gates: pass.

## Retained limits

- The 1.0-normative `protocol/cem-1.0/stable/ALGORITHMS.md` Witnesses paragraph does not yet
  name the bound. It is outside this lane's packet scope and is edited by open PR #879; the
  requirement text in the spec carries the rule.
- `tamper-matrix/README.md` still states the case count from before this lane (PR #879 also edits
  that file); `EXPECTED.json` is the authoritative list.
- NOT_RUN: `cmd/corvint` and the other dependency-path selections of `corvint affected` (178
  units in all, because `internal/cem/wire` is a foundation package); the change is confined to
  maps that carry a `discriminates` witness, which no committed map does. `make gate` was not
  run, per the owner's standing preference for scoped issue work.

## Independent review

One Codex read-only round: CONCERNS. (1) The CEM-V1-227 traceability row and the matrix README
still state the old case counts: the new CEM-V1-090 row now records 23 cases; the older row and
the README stay unchanged because this lane appends only and PR #879 edits the README. (2) The
1.0-normative `ALGORITHMS.md` Witnesses paragraph lacks the bound: recorded above as a retained
limit for the packet owner. Both consumers' code, the three cases and the manifest entries were
reported correct.

## Rollback

Revert the change commit. Both consumers admit any `mutants` count again, and the three matrix
cases and their manifest entries go away. No committed map, seal or golden depends on them.
