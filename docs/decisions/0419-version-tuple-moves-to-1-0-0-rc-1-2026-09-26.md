# Decision 0419: the version tuple moves to 1.0.0-rc.1 before v0-6 is promoted

Date: 2026-09-26. Status: accepted (owner answer 2026-09-26: "Bump VERSION to 1.0.0-rc.1").
Tickets: V1-0011, V1-0018. Amends decision 0382 item 3 for v0-6.

## Context

v0-6 was captured at `9c918a8` and its `full-gate`, `focused-docs`, `interop-gate` and
`companion-release` runs pass there. `script/release-checklist --pre-promotion` fails at that
commit: `VERSION` is `0.8.1`, tag `v0.8.1` names an earlier notes commit, and a present tag that is
not a single-parent child of `HEAD` fails the tag row (`ARTIFACT-RDY-V0-003`, decision 0380). Decision
0382 item 3 avoided this for v0-5, v0-7 and v0-8 by running the checklist before `v0.8.1` existed.
Every `HEAD` after a published tag fails the same way until the tuple moves.

## Decision

1. The `PUB-V0-001` version tuple moves from `0.8.1` to `1.0.0-rc.1`, the next planned release
   (decision 0384). Its tag `v1.0.0-rc.1` does not exist, so the tag row is `NOT_RUN` and the
   pre-promotion checklist judges only the candidate rows.
2. v0-6 is captured, gated and attested again at the merge commit of this change, with all five
   required gates run once in a clean clone at that commit.
3. Nothing is tagged or published. The rc.1 freeze (V1-0018) still decides the candidate commit;
   `1.0.0-rc.1` in `VERSION` is the development identity until then.
4. The N-1 compatibility baseline stays the published `0.8.1` (`CCF-V1-007`), and qualification
   evidence measured against `0.8.1` keeps that identity.

## Rollback

Revert this change. No tag, archive or store promotion depends on it before v0-6 is promoted; after
that, a store promotion is corrected by a later release, never by rewriting the journal.
