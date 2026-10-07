# 2026-10-07: compact default `affected` plan (V1-0943)

## Intent

Ticket V1-0943 records that the default `corvint affected` document repeats every exclusion
(one object per excluded unit, each restating the same reason, graph universe and invalidation)
and every selected test file. On this repository that made up most of the bytes an agent reads
to choose its tests. The owner asked for a lean default, but consumers that need the full lists
must keep them through an explicit option. The change adds proposed AFP-V0-035 to
`docs/specs/affected-plan-v0.md` and amends CCF-V1-002, CCF-V1-005 and CCF-V1-007 in
`docs/specs/core-compatibility-freeze-v1.md`. Both are pending owner acceptance.

## Decisions

- **A new profile, not a retyped one.** `plan.excluded` changes from an array to an object, and
  CCF-V1-005 freezes that member. Under CCF-V1-006 this is a breaking change, so the default is
  now `affected-plan/1`. `--full` writes the `affected-plan/0` document, byte-identical to the
  previous default; the renamed core-freeze goldens (`affected-full-*.json`) prove this. The
  CCF-V1-006 decision record is still owed. This lane does not number decisions.
- **Projection after compilation.** `compactAffectedReceipt` projects the finished `/0` receipt,
  so the selector, the advice, the provider and the snapshot form are unchanged. The digest is
  the SHA-256 of the exact `plan.excluded` bytes that `--full` writes, so a reader can check a
  full list against the compact one without a second encoding rule.
- **Groups instead of hoisted scalars.** Each exclusion has the same `reason`, `universe` (the
  graph digest) and `invalidation` today. These values are stated once per distinct triple, with
  a count, so a future second reason does not need another wire change.
- **Consumers.** The profile checks in the `internal/companionrelease` core smoke,
  `.github/cishards` and `tools/corvint-pr-tests` accept both identifiers. They read only unit
  IDs, scope, unknown and provider. `tools/retrieval-bench` reads `selected[].tests`, so it now
  passes `--full`. `tools/gate-affected-select`, `script/gate-affected.sh`, the benchmark
  harnesses and the CEM recipe need no change. The `review` guidance still embeds the full `/0`
  receipt; compacting it is a follow-up.
- **N-1 replay.** 0.8.1 has no `/1`, so the default affected modes are skipped. The `--full`
  modes replay with `--full` removed, because that is the N-1 default.

## Measurements

The commands were run from the lane worktree at base `0b5096ca`, with `corvint affected --base
0b5096ca0896351a584bca271812d9e1dd34f06c` and the change applied. Sizes are the bytes on stdout.

| Form | Total | `plan.selected` | `plan.excluded` |
|---|---:|---:|---:|
| `--full` (`affected-plan/0`, the previous default) | 149,768 | 55,601 | 77,697 |
| default (`affected-plan/1`) | 37,461 | 20,657 | 334 |

With 89 selected units, the default is 75% smaller. An earlier run of the same diff, with 48
selected units, measured 18,681 B for the default and 133,902 B for `--full` (86% smaller).
What remains is mostly witnesses, `advice` (the advisory Go command repeats the package list) and
`provider`. Those are follow-up candidates and are not changed here.

## Rollback

Delete `cmd/corvint/affected_compact.go` and the `--full` option. Restore `affected-plan/0` as
the default, restore the consumer profile checks and the `retrieval-bench` argument, and rename
the `affected-full-*` goldens back.
