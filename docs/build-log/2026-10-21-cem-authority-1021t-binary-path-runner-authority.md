# 2026-10-21 — cem-authority-1021t: binary-path refusal, codec testdata, current Tasks authority

Lane `cem-authority-1021t`, base `ef2a01db5`. Every requirement below is accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). No release or promotion decision is made
here.

## Problem

- **V1-0301**, narrowed by decision 0548: a binary diff was refused with `BinaryPatch`, but the
  message did not say which path caused it.
- **V1-2036**: the protected local-authority command refused a Corvint-shaped target. The cause
  was not that the whole tree must be codec-only, because `onlyCodecPackage` reads only
  `internal/wp3codec`. The cause was that the check refused the package's `testdata` directory.
- **V1-2018**: the `CEM-V1-286` chain left two cases unshown:
  - that a `GRANT` recorded for a superseded tree has no authority to complete;
  - that separate runner repairs become admitted evidence only through one receipt over the
    composed source.

## Decisions

- **`CEM-V1-037`** (`cem-stable-v1.md`): both binary refusal sites in `internal/cem/patch` name the
  path.
  - The path comes from the `diff --git` header, or from the rename target.
  - With no header, the `Binary files a/X and b/Y differ` marker supplies the new side, or the old
    side when the new side is `/dev/null`. Either way the path is validated with
    `wire.ValidatePath`.
  - The refusal code stays `BinaryPatch`. Mode-only changes stay under `CEM-V1-300`.
  - CEM-PILOT-001 moves to a post-1.0 profile, as decision 0548 records.
- **`PLE-V0-013`** and decision 0553: the codec package check admits one `testdata` tree entry of
  mode `40000` and never reads its subtree. Every other entry is still refused.
- **`CEM-V1-038`** and **`CEM-V1-039`**: test-only coverage of existing behavior. No product code
  changed.
  - The chain ticket now requires `cem-verdict`.
  - After a committed tamper is submitted:
    - completion at the superseded seal commit is refused `STALE_TREE`;
    - completion at the tamper commit, on the historical `GRANT`, is refused `GATE_STALE`;
    - after a `FAILED` verdict, completion is refused `GATE_FAILED`.
  - A `git revert` restores the seal tree byte for byte. After resubmission, completion is still
    refused `GATE_STALE` until a fresh `GRANT` runs at that candidate, and then the ticket is
    `COMPLETED`.
  - A second test repairs one draft in two isolated commits, each with a real passing runner
    receipt. The plain producer with admission refuses both receipts with `runner-receipt-stale`
    and writes no bundle. It admits the composed receipt that selects both tests.

## Evidence

- **New tests fail without the fix and pass with it:**
  - `TestBinaryRefusalNamesPath` (`internal/cem/patch`)
  - `TestPrepareBinaryRefusalNamesPath` (`internal/cem/workflow`)
  - `TestPLEV0013CodecPackageAdmitsTestdata` (`tools/local-authority`)
- **Mutation proofs** for the V1-2018 tests, which cover existing behavior:
  - With the gate tree comparison in `internal/tasks` `gateVerdict` disabled, the chain test fails
    at "completion on the historical GRANT", because the ticket completes.
  - With the producer's target-digest staleness check disabled, `TestCEMV1039...` fails because an
    isolated repair receipt is admitted.
  - Both mutations were reverted, and the diff of `internal/tasks` and `internal/cemcandidate` is
    empty.
- The whole `./internal/cemchain/` package passes. Affected tiers 0 and 1 and the doc gates are
  recorded in the PR.

## Not delivered

- **V1-2107** was not started. It needs changes to the sealed `protocol/cem-1.0` RESULT schemas and
  a `CEM-V1-091` rebind. That collides with the peer-owned V1-2327 stable lift.
- **V1-1451** was not started. It needs 1,200 s measurement runs over three pairs, and decision 0534
  deferred it. It does not fit the lane budget.

## NOT_RUN

- **V1-2036 criterion 2**, a live protected enrollment over a real map. It needs the root-owned
  authority store and accounts. This repository's tree also exceeds the capsule closure bounds of
  1,024 objects and 8,192 entries.
- **Affected tier 2**, which is left to batch CI.

## Rollback

Revert the lane's commits. No wire member, refusal code, store or published evidence depends on
them.
