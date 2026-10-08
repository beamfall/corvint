# 2026-10-08: V1-1026 milestone policy and unmilestoned count

## Intent

Ticket V1-1026. On 2026-10-08, with Core 1.0.0-rc.2 build 360, 438 of 546 OPEN tickets had no
milestone, so v1.0 scope could not be read from the queue. `ticket create` accepted a null
milestone without warning, and no read reported the count. The owner asked that every ticket carry
a milestone; a one-off backfill does not keep that true. The acceptance criteria ask for:

- an optional, closed policy field that refuses a missing milestone on create, and on a refine that
  clears it, with a named code, leaving default behavior unchanged;
- the OPEN-without-milestone count in `queue status` and `roadmap`;
- the spec recording both, with tests for refusal, default acceptance and the count.

## Change

- CAL-V0-195 (proposed): the optional top-level policy key `milestones {required}`, a closed object
  with a boolean, decoded like `knowHow` and `holderLiveness`. Omission keeps the policy bytes.
  `intent.Policy.MilestoneRequired()` gates two refusals in `mutation.Apply`: CREATE with a null
  milestone, and REFINE whose payload sets `milestone` to null. Both return `VALIDATION_FAILED`
  with the new §11 code `MILESTONE_REQUIRED`, classified not retryable. Every CREATE and REFINE
  path goes through Apply, so the rule covers all roles, batch refine and `ADOPT_FILE`. The create
  template marks `milestone` non-nullable and lists it in `fill` under the opt-in, and the verbose
  create and refine help state the rule.
- CAL-V0-196 (proposed): `queue status` (and `--summary`) reports `openWithoutMilestone`, the
  whole-inventory count of OPEN tickets with a null milestone. `roadmap` warns
  `N OPEN ticket(s) have no milestone` when N is non-zero, independent of paging, because its
  items are rows.
- Spec: a V1-1026 subsection, TCP-00 amendment A25 (77 codes), a slices row, traceability rows,
  the retryability table and the status line (mirrored in `docs/specs/README.md` and
  `INDEX.json`). `docs/TASKS-EXTERNAL-AGENTS.md` documents the key and the count.

## Decisions

- Numbered from CAL-V0-195 because unmerged batches F/G append CAL-V0-192..194.
- No retroactive enforcement: a REFINE that does not name `milestone` is admitted on a legacy
  unmilestoned ticket, so a backfill can proceed gradually, and receipts written under an earlier
  policy still audit because each receipt replays under its own policy.
- A new code rather than a reused `MALFORMED`, so agents can branch on the policy refusal.
- The count is reported on every policy, not only under the opt-in, because drift is the problem
  the ticket names.
- Known limits: adding the key is a policy change and fences live evidence handoffs `STALE_POLICY`;
  older binaries refuse a policy carrying it (A20/A23 downgrade rule), and, as with A24, their
  journal audit keeps refusing after the key is removed, because the historical policy post keeps it. The legacy closed queue
  status key-set readers in `internal/taskman/capture.go` and
  `internal/companionrelease/core_evidence.go` were not changed.

## Evidence

- `TestCALV0195_PolicyMilestonesOptIn` (`internal/tasks/intent`): default bytes, true/false decode,
  MALFORMED for unknown member, non-boolean, `{}`, null and non-object.
- `TestCALV0195_MilestoneRequiredPolicy` (`internal/tasks/cli`): default and `required:false`
  acceptance, both refusals writing nothing, admitted paths, template, receipt audit CONSISTENT,
  key removal restoring the default, help text, batch refine refusing the clearing entry alone. Disabling the create refusal makes it fail.
- `TestCALV0196_OpenWithoutMilestoneCount` (`internal/tasks/cli`): count 0 then 2 (DRAFT and
  milestoned excluded) in both status forms, roadmap warning only when non-zero on `--limit 1` at offsets 0 and 1.
- Independent review (Codex): one P2, that the first rollback text implied removing the key restored
  older binaries. Fixed in the spec rollback, A25 and failure modes. An old-binary regression test
  was not added: no older binary is built in tests, and A24 records the same limit. Of the review's
  test gaps, batch refine and a non-zero roadmap offset are now tested; `ADOPT_FILE` and HELD
  exclusion are covered by construction (the shared `mutation.Apply` path, and the count's
  `status == OPEN` test) but have no dedicated test.
- Not run: `make gate`, live qualification against the real store, native ticket completion.
