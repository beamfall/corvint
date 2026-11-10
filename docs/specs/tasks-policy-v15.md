<!-- SPDX-License-Identifier: Apache-2.0 -->
# Tasks Policy Version 15 V1

Owner: Russell Lewis
Date: 2026-11-10
Intent status: proposed
Delivery status: not-started
Authoritative inputs: owner decision R3 (2026-11-09), change folder
`docs/specs/openspec/changes/tasks-policy-v15/`, milestone M0 in `docs/specs/PRODUCT-PLAN-1.0.md`,
`.taskman/policy.json` line 1 (integ 9ec3a29d0), REL-V1-001 in `release-process-v1.md`.

## Agent digest
- Claim: Tasks policy version 15 drops the CEM and OCM requirements and the two gates that run removed tooling, after the M0 removal lands.
- Status: proposed / not-started
- Exists: policy version 14 in the store and in `.taskman/policy.json`; no version 15 yet.
- Blocked on: the M0 removal on `origin/main` (TPV-V1-004) and owner acceptance of the change.
- Read next: Requirements; Failure modes; Acceptance and rollback.

## User and boundary

Milestone M0 removes the CEM and OCM commands from `corvint`. Policy version 14 still requires a
CEM and an OCM for every attempt and declares the gates `cem-verdict` and `context-focused`, which
run removed commands or removed tests. Owner decision R3 (2026-11-09) drops those members in a new
policy version. This spec fixes the content of version 15 and the order of the update.

Non-goals:

- no change to the `taskman-policy/0` profile or to `corvint-tasks` code;
- no change to the remaining gates' commands (whether `context-vet` is retargeted is open);
- no migration of existing receipts that cite `cem-verdict`.

## Requirements

- `TPV-V1-001`: Policy version 15 MUST NOT require a CEM or an OCM for any attempt: `cemRequired`
  and `ocmRequired` are removed or false.
- `TPV-V1-002`: Version 15 MUST NOT declare the gates `cem-verdict` and `context-focused`, and MUST
  keep every other member at its version 14 value. A JSON diff of the two versions shows only
  `policyVersion`, `cemRequired`, `ocmRequired` and the two gate entries.
- `TPV-V1-003`: The store policy MUST be updated with `corvint-tasks policy update` before
  `.taskman/policy.json` changes, and the tracked file MUST change in one PR with no other paths,
  matching the store bytes.
- `TPV-V1-004`: The update MUST run only after the M0 removal is merged on `origin/main`
  (REL-V1-001).
- `TPV-V1-005`: Before the update, no release object and no OPEN or CLAIMED ticket MUST reference
  `cem-verdict` or `context-focused`.
- `TPV-V1-006`: An attempt admitted under version 14 and refused with `STALE_POLICY` after the update
  MUST be resubmitted under version 15, not forced through.
- `TPV-V1-007`: The version 14 bytes and their SHA-256 MUST be retained in the build-log entry
  before the update. Rollback MUST restore those bytes, or publish version 16 with that content if
  a lower version is refused.
- `TPV-V1-008`: At least one ticket MUST be submitted, gated and completed under version 15 without
  a CEM before the change is complete; `receipt audit` passes and the receipt cites version 15.

## Failure modes

| Failure | Behavior |
|---|---|
| A release or an OPEN or CLAIMED ticket still names a removed gate | The update stops at the reference check; the reference is fixed first (TPV-V1-005). |
| The M0 removal is still open | The update stays blocked (TPV-V1-004). |
| The store update succeeds but the carry PR is not merged | Store and tracked file differ; the carry PR is the only open action (TPV-V1-003). |
| The carry PR holds other paths | Refused in review until it holds only `.taskman/policy.json` (TPV-V1-003). |
| An attempt admitted under version 14 is refused with `STALE_POLICY` | It is resubmitted under version 15 (TPV-V1-006). |
| The store refuses a lower `policyVersion` on rollback (not verified) | Version 16 is published with the version 14 content (TPV-V1-007). |

## Acceptance and rollback

Acceptance evidence: the JSON diff of versions 14 and 15 showing only the listed members; the empty
reference check; the store reporting version 15; the merged carry PR whose bytes match the store;
one attempt completed under version 15 without a CEM, with a passing `receipt audit`.

Rollback: restore the retained version 14 bytes, checking their SHA-256, through `corvint-tasks
policy update`, and revert the carry PR; if the store refuses the lower version, publish version 16
with the version 14 content.

## Traceability

| Requirement | Proof |
|---|---|
| TPV-V1-001, TPV-V1-002 | JSON diff of version 14 and version 15 (task 2.4) |
| TPV-V1-003 | store policy read and carry PR diff (tasks 2.5, 2.6) |
| TPV-V1-004 | merge commit of the M0 removal (task 2.1) |
| TPV-V1-005 | empty reference check (task 2.2) |
| TPV-V1-006, TPV-V1-008 | receipt of one attempt under version 15 (task 2.7) |
| TPV-V1-007 | retained hash in the build-log entry (task 2.3) |
