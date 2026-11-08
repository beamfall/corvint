# Runner selection T2: language-neutral shadow plans

V1-5104 implements `RSP-V0-011..017` and the selector portion of `RSP-V0-040`
step 1 on lane base `230892cf27a054661b8ae72a49897afa8692788c`. The prescribed
dogfood start base was `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

`internal/runnerselection` consumes the existing evidence records, freshness
snapshot, runner pieces and head inventory. Its inputs are core-owned in-process
values; neither its qualification view nor its admission bits are a plugin wire.
It launches nothing, writes nothing, and exposes no enforcement switch. The
existing language-name ratchet includes this package with a zero baseline.

Selection is the union of rules (a) through (g), missing components, outside
contact, stale/missing records and project mandates. Qualification failures,
incomplete diffs, invalid inventories and unsupported inputs produce named FULL
plans. Only the traced model is supported here; other models remain FULL. The
base snapshot checks freshness before the head diff is evaluated. An insertion
checks both base neighbours; uncovered lines consult their enclosing body.
Deletion and rename-away evaluate every ancestor listing through the root.
Unknown path representations and repository identities cannot certify a
non-match. The tracked-head fixtures include disappearing ancestors at multiple
depths, renames, a surviving sibling and an unrelated known deletion.

The charter's shadow-output change is an additive
`advice.runner_selection_shadow` member in the ordinary full and compact
`affected` receipts. It reports current runner declarations as FULL, labels the
governing policy `static-unqualified`, and explicitly disclaims soundness before
enforcement. Existing static plan and provider values remain the execution
inputs. No command, flag, existing field, error, exit code, stderr text or
committed golden was changed. Mechanical line-citation moves retain their
content hashes.

Current declarations have no inventory, qualification or evidence pieces. The
CLI therefore does not fabricate test identities from static test-file lists or
discover a new index location: inventory counts are explicitly unknown, and
unretained loading/selection timings are null. The pure selector reports actual
selection time and accepts a separately measured loading time. The legacy
index replay fixture uses T1's `EvidenceRecords` adapter and keeps incomplete
legacy records selected. Real qualification, index admission/loading and
enforcement remain later rollout work; no runner is promoted by this slice.

Validation in this build-only lane: `go vet` passed for
`internal/runnerselection`, `internal/langplugin` and `cmd/corvint`; formatting
is clean. Spec generation reports no generated changes. The static spec index,
worktree and README checks and line-citation check passed. Fifteen new golden
plans cover seven rules and eight required components; each named requirement
has positive and negative tests. Tests are written and type-checked, not run.

NOT_RUN: every Go test (including the three touched packages as whole packages),
frozen-merge replay, reference-host p95 latency, live qualification, repository
gates and independent review. The owner assigned testing and review to the
integration phase. The aggregate `spec-requirements-check` target also remains
NOT_RUN because it invokes Go tests; its static constituents passed separately.
This is implementation evidence, not observed acceptance or qualification.

The required dogfood start failed with missing citation, intent and verification
inputs, and reported `coordination-time-impact NOT_PRODUCED
unsupported-impact-range` and absent prechange receipts. Its generated working
CEM was restored to the starting version. Final binding, dogfood-check and seal
are NOT_PRODUCED under the build-only override. Raw logs stay outside the
repository in the lane evidence directory. Rollback removes the shadow advice
and selector slice; the governing execution plan does not depend on either.
