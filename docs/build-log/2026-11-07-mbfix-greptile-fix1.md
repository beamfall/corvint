# mbfix Greptile fix round 1: a reached test stays a guard candidate by its own name

Date: 2026-11-07. Branch `claude/mbfix` (PR #130). The round started at the sealed head
`983de7dcd0e90fe0ccdfafc8bd9e19131811f02a`, sealed after the merge-forward onto origin/main
`538e823f4a1d220a3fb5b12c0529248f18924823` (`docs/build-log/2026-11-07-mbfix-merge-forward.md`).
Greptile raised one P1 on `growImported` in `internal/cem/verify/guards.go`. Requirement block
`TCQ-V0-7420..7429` was checked free against origin/main and the 50 open refs before use; only
`TCQ-V0-7420` is defined.

## Finding and verdict

**P1, direct caller tests disappear: INVALID at the round's base, hardened anyway.** Greptile's
claim, which the orchestrator's static read shared, was this. When a test such as
`func TestDirect(t *testing.T) { changed.Double(1) }` calls the changed import directly,
`growImported` adds the test's own name to its seed. But `testsMentioning` checks only what a test
mentions, never whether the test's own name was reached, so the test would be dropped and the hunk
could end as `no-candidate-test`.

Evidence that the test was not dropped:

1. `TestTCQV07408ImporterNamespace` gained `TestDirect` (a direct call of the changed import) and
   `TestForeignDirect` (a direct call of the other import's same-spelled `Double`). Run at the base
   with no production change, it passed with candidates `[TestActual TestDirect]`.
2. The mechanism is in `parseDecls`. The parser's object resolution sets a top-level
   `FuncDecl.Name.Obj.Decl` to that declaration itself, so `declaredWithin` returns false for the
   name and each declaration's own name lands in its `refs`. A test seeded by name therefore passes
   `d.mentions(reached)`. A throwaway mutation removing each declaration's own names from `refs`
   made the same test fail with `[TestActual]`, which confirms this is the only thing keeping the
   test.

Decision: the rule rested on an unstated parser detail that two independent readers misread, so
`testsMentioning` now keeps a base test whose own name is reached explicitly (`TCQ-V0-7420`). This
mirrors how the initialization check already matches a declaration's own names. The change alters
no current result, because a reached test already mentioned itself, and it does not widen the
`TCQ-V0-7408` namespace separation: `TestForeign` and `TestForeignDirect` are never seeded, since
`growImported` matches the changed import's path-qualified name only. With the hardening in place,
the same mutation no longer drops `TestDirect`.

`TestTCQV07420ReachedTestNamesStayCandidates` pins the rule without the parser: hand-built
declarations with no self-reference, where a test reached only by name stays a candidate, a test
reaching a name by reference stays one, and an unreached test does not. It failed at the base
(`reached test name dropped: [TestCaller]`) and passes with the change.

## The other callers of `testsMentioning`

Both callers outside `growImported`, the own-package path in `judgeGuards` and
`goPackage.candidates`, take their reached set from `closure`. `closure` never seeds a test
declaration: the hunk's file is non-test Go source (`goSource`), and it skips `d.test`. `grow` adds
a test's name only when that test mentions the set it has reached so far, and `testsMentioning`
evaluates the same `mentions` against the final set, which can only be larger. So neither caller
ever drops a reached test name, with or without the parser detail. A test not declared at the base
is excluded by design (`loadGoPackage`; a cite needs base evidence). No case was found and no fix
was needed. `TestTCQV07420ReachedTestNamesStayCandidates` adds a parsed own-package direct-caller
case through `candidates` as a regression.

## Evidence and limits

- `corvint affected --base 983de7dcd…` (lane build at HEAD) selected `go:internal/cem/verify`
  whole (`DIRECT_SOURCE_CHANGE`). It ran whole through `script/heavy.sh`: ok. `make windows-vet`:
  rc 0.
- Every other affected unit (358 selected, 69 `LANGUAGE_FRONTIER` unknowns) is NOT_RUN, per the
  round's scope. `make gate` is NOT_RUN under the owner's standing preference.
- Rollback: revert the fix and spec commits. The explicit check changes no current candidate set.
