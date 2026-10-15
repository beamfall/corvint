# 2026-10-14 Test-slice generation and replay combine split outcomes (V1-1371, AFP-V0-049)

Ticket V1-1371. Base 92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431. Scope: 1.0 scope per `PRS-V1-018`.

## Intent

`tools/ci-test-slices generate` and `replay` refused any run in which a package ended twice. Full CI
runs `internal/tasks/store` as two slices (AFP-V0-041), so no full run could regenerate the slice
file or be replayed; the AFP-V0-041 Limits bullet recorded that. AFP-V0-049 (new, accepted under the
owner's standing auto-accept (2026-10-11); V1-1371) makes both modes combine a split package's
per-slice outcomes, the rule `tools/ci-shard-costs refresh` already applies (AFP-V0-041 (8)), and keeps
every other refusal. AFP-V0-041 (5) gains one pointer sentence; its accepted text is unchanged.

## Decisions

- The split set is the tested tree's: `generate` reads the allow-list and slice file of the
  `--revision` checkout, `replay` those of `--root`. `replay --slices` chooses only the placement
  it predicts, so a regenerated candidate can be replayed against the run that measured it.
- Same shape as `ci-shard-costs combine`: K terminal outcomes for K slices, sum of times; an
  unusable allow-list or slice file splits nothing, so a sliced run then refuses.
- Attribution, beyond the cost tool: each outcome's top-level tests must all sit in the slice that
  admits the first of them in byte order, and no slice is attributed twice. An outcome with no
  top-level test (a named slice whose tests the run lacks) counts toward K by count only.
- `replay` now prints `units U placed exactly once: W whole packages, S test slices` and refuses a
  plan that drops, repeats or adds a unit, or splits one package two ways (`placedOnce`).
- Slice set kept: the run splits only `internal/tasks/store` (895.6 s against the 797.1 s target,
  2 slices); `cmd/corvint` ran whole at 586.0 s, under the target. No change to which packages split.

## Hostile tests and mutation check

`tools/ci-test-slices/combine_test.go`: `TestAFPV0049ReplayCombinesSlicedRuns` (2 slices over 2
shards; 3 slices over 3 shards with a named slice whose tests are missing; catch-all holding a test
added after generation and `TestÜ`), `TestAFPV0049GenerateRegeneratesFromASlicedRun`,
`TestAFPV0049RefusesEveryOtherRepeat` (13 cases, each in replay and, with a slice file, generate,
asserting the reason and an unwritten slice file), `TestAFPV0049ReplayRefusesAPlanThatMisplacesAUnit`.
Each mutant of `combine` was applied to the working tree and the package tests run:

| Mutant | Killed by |
| --- | --- |
| M0 drop the combination (split set empty) | `ReplayCombinesSlicedRuns`, `GenerateRegeneratesFromASlicedRun` |
| M1 drop "package P has two terminal outcomes" (unsplit repeat; also the not-in-slice-file case) | `RefusesEveryOtherRepeat` "package not in the slice file" (accepted), `TestAFPV0041GenerateRefusesUnboundInputs` |
| M2 drop the cross-log duplicate-test refusal | `RefusesEveryOtherRepeat` "same test in two slices" by reason only: the slice-ended-twice refusal still refuses |
| M4 drop "outside test slice I/K" | `RefusesEveryOtherRepeat` "named test after a catch-all test" (logs `TestB` and `TestA0, TestC`: accepted under M4, probe exit 0) |
| M5 drop "slice ended twice" | `RefusesEveryOtherRepeat` "slice ended twice" (accepted) |
| M6 drop the K-outcomes count | `RefusesEveryOtherRepeat` "extra slice" (accepted) |
| M7 drop "more than --shards" | `RefusesEveryOtherRepeat` "more slices than shards" (accepted) |
| M8 drop the "outside every test slice" index guard | survives: equivalent, the catch-all admits every name the named slices do not |

M2 is killed by reason only, and no input is accepted under it: the slices partition the names
and every name of an outcome must sit in one slice, so two outcomes sharing a test are attributed
to the same slice and refuse as "ended twice".

## Codex review (one read-only round, log retained outside the repository)

Verdict "changes requested", two findings, both confirmed and fixed. Medium: this entry first
claimed M4 was backstopped; an outcome whose first test is a catch-all name (`TestA0`) followed by a
named one (`TestC`) is accepted under M4. The case was added and the claim corrected. Low:
`placedOnce` overwrote a package's slice count with each entry, so a plan with `b#0/3` and `b#1/2`
passed; it now refuses two counts for one package, with a test case.

## Regeneration and replay

Source: hosted FULL run 38485555207, head 5163d1ed4b61084fc32cf79355848899c3e8a5bb, go1.27.2,
artifacts `ci-shard-costs-0..7` (the run #880 used). `generate --shards 8` under local go1.27.2:
`whole cmd/corvint: 586014ms within the 797134ms target`, `split internal/tasks/store: 895576ms into 2
slices`. The new `.github/cishards/test-slices.json` (sha256 d9544075e0de...) records that source and
names 148 tests (448204 ms) against a 447372 ms catch-all; it supersedes the go1.27.1 source
(revision 3b428343, run 38078093034, sha256 7c763fa7d364...) per decision 0522.

Replay of the same run, `--root` a checkout of 5163d1ed4 (the old file is the one the run ran with),
costs taken from the run itself (not `package-costs.json`), seconds:

| | shard0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| observed (hosted, old file) | 749.7 | 826.9 | 640.4 | 945.9 | 650.5 | 852.6 | 910.1 | 800.8 | 945.9 |
| predicted, old go1.27.1 file | 811.2 | 793.1 | 717.6 | 811.2 | 811.2 | 811.2 | 810.2 | 811.2 | 811.2 |
| predicted, new go1.27.2 file | 796.6 | 797.0 | 797.0 | 796.6 | 797.6 | 797.4 | 797.4 | 797.4 | 797.6 |

Both replays report `universe 334 packages 6377.1s` and `units 335 placed exactly once: 333 whole
packages, 2 test slices`; the store slices are 139/248 tests (old) and 148/239 (new) of the same 387.
Predicted slice times rest on the share model the tool states is an inference. The observed-to-
predicted gap comes from the hosted placement using `package-costs.json` (V1-1241, #880, untouched).

No test pins the old slice revision, run, digest or test counts: `git grep` for `3b428343`,
`38078093034`, `7c763fa7`, `496425` and `509941` outside build logs finds nothing, and every
`ProfileDigest` use computes it. Tests reading the committed file passed unchanged: the isolated
`.github/cishards` module, `internal/sessiontest`, `tools/corvint-pr-tests`, `cmd/corvint -run TestSTRV00`.

## Failure modes

A sliced run whose outcomes are not one per slice, a repeated unsplit package, a test in two
outcomes or outside the slice its log ran, a slice attributed twice, or a split wider than
`--shards` refuses with exit 2 before any write. A run made before a split refuses on count.

## Limits and NOT_PRODUCED

- An outcome without top-level tests is attributed by count, not by which named slice it ran.
- Hosted replay of the new file is `NOT_OBSERVED` until a full run uses it.
- Exhaustive `go test ./...` and `make gate`: `NOT_RUN` (charter); the affected plan selected 87
  packages, mostly through the spec documents; the packages that read the slice file ran.

## Rollback

Revert `combine` and `placedOnce` in `tools/ci-test-slices/main.go` and restore the previous
`.github/cishards/test-slices.json`; both slice files are valid partition input.
