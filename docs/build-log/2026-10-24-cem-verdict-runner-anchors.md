# 2026-10-24 — cem-verdict runner-bound anchors (V1-2577)

Lane p1-cem2, stacked on #1703. Requirements `CEM-V1-606` to `CEM-V1-608`
(`docs/specs/cem-stable-v1.md` § Runner-bound verdict anchors), amending `TCB-V0-011` by
cross-reference. Status: proposed, experimental.

## Decision

#1703's sealed verdict gate refused its full touched scope with `tier-below-anchored`. 13 of 20
paths were new files or committed runner artifacts, which CEM-V1-081 can never anchor from a BASE
span. Two options were rejected:

- Raising the tier in `verify.Citation` was rejected. It is the portable cross-consumer block that
  `interop/cem01-go` mirrors, and TCB-V0-017 compares the two.
- A "named by an anchored spec span in the same change" rule was rejected. The gate has no parsed
  patch, and that rule would cover only three of the thirteen paths.

The gate instead admits two runner-bound satisfaction bases. Both read the map's own
`runnerReceipts`, `criterionLinks` and `artifacts` back from the candidate commit's objects.
`runner-input` applies when a passed receipt's plan pins the hunk file's candidate bytes.
`runner-artifact` applies when the hunk path is a plan or receipt the map digests and binds. Tiers
are reported unchanged. An unsatisfied hunk names a closed reason, and the gate still refuses.

## Encoding note

The gate document `taskman-cem-verdict-gate/0` gains two optional per-hunk members, `anchor` and
`reason`, both omitted when empty.

- A GRANT whose hunks are all satisfied by tier encodes byte for byte as before. Its pinned
  evidence keeps reverifying under TCB-V0-013, as `TestCEMV1608AnchoredGrantEncodesWithoutRunnerMembers`
  shows.
- A pinned refusal now gains `reason` members, so it no longer reverifies byte-equal. A refusal was
  never admitted, so nothing that consumes one changes.
- A path row's `satisfied` now means that every governed hunk at the path is satisfied. Before this
  change it meant the minimum-tier hunk was satisfied, which was equivalent, so documents without
  runner anchors are unchanged.

## Evidence

The full-scope gate over #1703 was rerun with the new binary, built from this branch with
go1.27.3 on darwin/arm64:

- inputs: `--base 0949876eba7b706a77988d94944a46c0979690d2 --candidate
  adda562c8b8f534c3f7c04ddffaec3811ce3da69 --predicate touched-anchored`, the 20 touched paths of
  base..SEAL excluding `.corvint/changes/`, and `--portable-consumer` set to `cem01-go` built from
  the same tree;
- before: REFUSE `CEM_VERDICT_PREDICATE_FAILED` / `tier-below-anchored`, output SHA-256
  `98d4cba04b84d083d790ce7b25619a68618764aa1aae80d2c709d0730d90aa6a`, which matches #1703;
- after: **GRANT**, verdict `5058b4ccfe2d6d7b1371e708ba8ed853b69e1c0f4b3dab20356a31ab8da9f605`,
  output SHA-256 `e57c6ba4bc4ca9005044feb02afff020e47830538d3e6b97ccb51cc7ef600b8f`;
- governed rows: 12 `anchored`, 18 `cited` on `runner-input` and 6 `cited` on `runner-artifact`.
  All 20 path rows are satisfied.

Tests (`internal/taskman`, linux/amd64 on WSL2 through heavy.sh):

- `TestCEMV1606*`, `TestCEMV1607*` and `TestCEMV1608*` pass;
- the existing `TestTCBV00*` tests pass unchanged, apart from the `predicate` signature in one
  unit test.

## Rollback

Revert the commit. The anchors and the two members go away, and those paths refuse again.
