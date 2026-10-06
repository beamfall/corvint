# 2026-10-06: V1-0888 additive pool change keeps clean handoffs

## Intent

Tickets V1-0888 and V1-0882, GitHub [beamfall/corvint#643](https://github.com/beamfall/corvint/issues/643).
An OWNER `policy update` that only added a member to a pool fenced every live clean HANDOFF and
REVIEW_RETURNED release with STALE_POLICY, so the next claim was charged a retry. The issue asks that
a purely additive pool change stay compatible, that every other relevant change still fence, and
that `policy update` name the live attempts it fences.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds CAL-V0-122..124 in a V1-0888 subsection,
  all proposed (acceptance is the owner's), with non-goals, failure modes, acceptance evidence and
  rollback. It also annotates issue 482 item 1, adds a slices row, traceability rows, the issue-643
  input and the delivery status (mirrored in `docs/specs/README.md` and `INDEX.json`).
- `intent.HandoffPolicyCompatible` removes, from the current policy only, every member that the
  same-id original pool lacks, with its own `reservedFor` and `memberConfig` entries, before the raw
  tree comparison. It also drops `holderLiveness` with `policyVersion`. A current policy that no
  longer holds the allocated member is now incompatible rather than malformed; the original policy
  must still hold it.
- `policy update` computes a read-only `handoffFences` preview before calling the writer and reports
  it on a COMPLETED result, with a warning when it is non-empty (CAL-V0-124). The help text for the
  handoff preconditions names the new allowances and the field.

## Decisions

- **Additive scope is every same-id pool, pooled and no-pool attempts alike.** An added member cannot
  affect an attempt that never held it. Added pools, removed or reordered members, pool settings,
  budgets and capacity stay bound; a change bundled with an addition still fences.
- **No raised pool size limit.** Policy profile v0 has no pool size field, so that part of the issue
  does not apply.
- **holderLiveness (V1-0882) is compatible.** It is absent from this base; it is already named in the
  ignored-keys list, which grants nothing where DecodePolicy rejects the key. On an export of the
  batch 3 branch (`origin/claude/rc3-batch-3`) with these two intent files copied in,
  `TestCALV0122_*` and `TestCALV0123_*` both pass (not skipped), so landing batch 3 needs no further
  line here.
- **The preview is advisory, not a gate.** It reads one pinned snapshot outside the writer lock and
  audits no history, so an attempt claimed under an earlier policy is `NOT_OBSERVED`. Refusing or
  confirming before commit would change the writer contract; a `--dry-run` is left as a follow-up.
- **Occupied own-member changes are already refused by the writer** (MALFORMED "occupied pool
  definition cannot change"), so the store counterexamples for them assert that refusal, and the
  interval test covers other-member replacement and pool settings.

## Evidence

Focused tests, `GOMAXPROCS=3 go test -p 1 -count=1`: `internal/tasks/intent` (CALV0044, CALV0122,
CALV0123), `internal/tasks/store` (CALV0044 interval, CALV0122), `internal/tasks/cli` (CALV0122,
policy and handoff tests) all pass. With the issue 482 projection restored, the CLI replay fails
(the additive update reported fences and the release was refused), confirming the test exercises the
change. Live fleet qualification is NOT_RUN; dogfood CEM binding is NOT_RUN in this lane.
