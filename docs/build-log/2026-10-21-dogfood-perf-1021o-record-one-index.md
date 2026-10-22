# Dogfood record compiles one index (V1-2272, LTPM-V0-017)

Date: 2026-10-21. Lane: claude/dogfood-perf-1021o (base ef2a01db5).

## Problem

`corvint dogfood-record` compiled a full `contextindex.Build` once at the start. Every stability
check that had a changed-path authority then compiled a second full index. One record ran six
compiles. The checks read only header fields (commit, tree, object format, status and dirty
paths), plus the tracked source set. The tracked source set is a function of the tree.

## Decision

`LTPM-V0-017` was added to `docs/specs/local-trace-producer-migration-v0.md`. It is accepted under
the owner's standing lane auto-accept of 2026-10-11.

- The recorder compiles once, through the test seam `buildRecordIndex`.
- Each stability check observes the header with `contextindex.Observe`. A commit, tree or
  object-format change still refuses with `repository-identity-changed`. Source status keeps its
  existing check.
- With an authority, the check re-derives candidate and admitted digests against the tracked set
  of the compiled index. A same-tree check cannot see a different tracked set, so this is sound.
- Trace bytes and refusal codes are unchanged.

## Evidence

- `TestDogfoodRecordCompilesOneIndex` fails without the fix ("compiled 6 indexes; want 1") and passes
  with it. It also proves three things: an unchanged repository passes, admitted-digest drift
  refuses, and a same-tree moved `HEAD` refuses. `TestDogfoodStabilityBindsCandidateAndAdmittedDigests`
  is unchanged and passes.
- Fixture: a 72-hunk docs change. Record wall time before was 31.09 s and 13.43 s (load 19 to 58).
  After it was 3.32 s and 2.56 s (load 58 to 69). The before and after record JSON are
  byte-identical.

## Other tickets in this lane

- V1-2273 was already delivered by #1451 (merge 5080bd4b0, DCW-V0-115). The prepare pass runs only
  `cem prepare`, and the bind pass skips query and impact. Query and impact therefore run once per
  finish, in the rebind, over the tree that holds the committed map. No earlier pass evaluated that
  tree, so there is nothing to reuse.
- V1-2274 was skipped because it needs an owner decision. The 3 `dogfood-ocm status` runs in
  check-seal are one run each by the base verifier, the tree verifier and the `CORVINT_BIN`
  override verifier. `script/dogfood-check.sh` passes the override, and `verifyAll` runs each
  verifier once. Running fewer verifiers would weaken verifier independence.
- V1-0953 was skipped because it needs the binding design recorded in
  `2026-10-21-v1-0953-staged-begin-reverted.md`.
- V1-2294 was skipped because its anchor, DCW-V0-117, exists only in open PR #1465.
- V1-2283 and V1-1724 AC1 were not started (new plan-helper features; out of lane budget).

## NOT_RUN

- The exhaustive gate (lane policy).
- Tier-2 affected packages beyond the cap; see the PR.

## Rollback

Revert this change. The recorder then compiles an index again for every stability check. No stored
format changes.
