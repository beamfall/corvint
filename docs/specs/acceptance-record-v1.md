# Acceptance Record V1 — plan acceptance, acceptance log and release records

Owner: Russell Lewis
Date: 2026-11-10
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-5272)
Delivery status: experimental; partially delivered
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/PRODUCT-PLAN-1.0.md`, decision 0596, decision 0433, and the change folder
`docs/specs/openspec/changes/acceptance-record/` (the proposal, design and spec delta this file
was created from).

## Agent digest
- Claim: One decision record accepts the 1.0 plan, a log holds each acceptance, and an audit compares the log with the spec markers.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-5272) / experimental; partially delivered
- Exists: decision 0596; `docs/specs/openspec/acceptance-log.tsv`; `script/acceptance-log-audit.sh`; `script/acceptance-plan-pin-check.sh`; `script/agents-invariants-check.sh`.
- Blocked on: the release record format and workflow read (ACR-V1-009, ACR-V1-010); the stale-quote audit.
- Read next: Requirements; Fallback in force; Acceptance evidence.

## User and measurable job

The owner needs one place that says what was accepted for 1.0, by whom and in which mode, and a
check that names any requirement whose spec marker and log row disagree. A release reader needs
one record per release that lists evidence, demotions, `NOT_RUN` and `NOT_OBSERVED` items and
deferred tickets, and a publish step that refuses without it.

Measured by: the audit names every plan requirement with no log row and every mode mismatch; the
invariant check fails on a foreign edit; the release workflow refuses a missing or malformed
release record (not yet built).

## Scope and non-goals

In scope: the plan acceptance decision record, its supersession table and drafted wording list,
the acceptance log and its audit, the release record format and its read by the release
workflow, the owner key rule and its fallback, and the ownership of edits to `AGENTS.md` product
invariants 3, 4, 5, 7 and 8.

Non-goals:

- No agent creates, holds or uses a signing key (decision 0433).
- This spec publishes nothing, promotes nothing and creates no repository.
- The audit does not prove who wrote a log row. Identity is by convention and `NOT_VERIFIED`.
- A separate machine account for agents is a 1.0.x ticket, not part of this spec.

## Requirements

Requirement IDs are stable. Each scenario under "Scenarios" names the requirement it tests.

- `ACR-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) One
  decision record MUST accept `docs/specs/PRODUCT-PLAN-1.0.md` as amended, naming the plan commit
  SHA, and MUST be signed by the owner before it counts as accepted. Under the fallback of
  ACR-V1-011 the owner-merged decision file stands in for the signature. Source: owner answer 18;
  owner decision R1 (2026-11-09).
- `ACR-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  record MUST list every requirement whose wording was written rather than quoted from an owner
  answer. Source: owner decision R1 (2026-11-09).
- `ACR-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  record MUST state that work in flight when it is signed stays experimental until its own
  change is accepted. Source: owner decision R1 (2026-11-09).
- `ACR-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  record MUST hold a table quoting each superseded text at a named commit and naming its
  replacement. The table MUST cover at least decision 0433 item 1, answer 3, answer 11, the G9
  MCP row, R26, decision 0594 item 4 and the dogfood sections of `AGENTS.md`. It MUST hold the
  use-case receipt row (final round, M6): the use-case receipt contract is superseded by the
  per-product end-to-end qualification of REL-V1-012, `conformance/use-cases-v0` is removed in B8
  with its two lints, and no 1.0 claim cites a use-case receipt. It MUST name change
  `github-action` as the home of GHA-V1-040 to GHA-V1-043 with their IDs unchanged, and say that
  `release-process` cites them. It MUST quote the old and new wording of `AGENTS.md` product
  invariants 3, 4 and 5. Source: owner decision R1 (2026-11-09); G9; owner decisions R8, R9, R17,
  R39 and R40 (2026-11-09); decided 2026-11-09 under R42 (D6, D17, E3, M6).
- `ACR-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  record MUST list Python file-level selection as a release exception with a link to the 1.0.x
  ticket for the Python line collector. Source: owner decision R9 (2026-11-09).
- `ACR-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272)
  `docs/specs/openspec/acceptance-log.tsv` MUST hold one row for every PROD requirement, every
  checklist O-row and every accepted change, with mode `owner`, `auto` or `pending-owner`, date,
  owner reference and rationale. Only the owner MUST write `owner` rows. Source: G14.
- `ACR-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  audit MUST check that each log mode agrees with the requirement's spec marker: `owner` with
  `(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`, `auto` with the
  `standing` marker, `pending-owner` with no acceptance marker. Source: G14; SRG-V0-009;
  SRG-V0-010.
- `ACR-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) A change
  that alters an accepted requirement's meaning MUST set its log row to `pending-owner`, and a
  lane MUST NOT write an `owner` row. Source: G14.
- `ACR-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The RC
  publish and the stable promotion MUST each have an owner-signed release record listing each
  requirement's evidence or demotion, `NOT_RUN` and `NOT_OBSERVED` items and deferred P2 and P3
  tickets. Under the fallback of ACR-V1-011 the owner-merged record and the protected-environment
  approval stand in for the signature. Source: G10; REL-V1-013.
- `ACR-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) The
  release record MUST use one documented file format under `docs/releases/` that the release
  workflow reads without network access beyond the repository. Source: G10.
- `ACR-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) Four
  acts MUST carry a tag signed by the owner with a touch-required key, held in hardware or in a
  Secure Enclave and listed in a committed allowed-signers file: plan acceptance, RC acceptance,
  stable acceptance, and any change to the allowed-signers file (a key rotation commit MUST be
  tagged with the old key). The RC and stable release records MUST name the candidate tag's
  commit SHA and the `SHA256SUMS` digest. The publish job MUST verify the signed tag against the
  allowed-signers file as it stood at the previously accepted tag, or for the first release at
  the commit named in the acceptance decision file; it MUST NOT use the file in the released
  tree, and it MUST refuse on any mismatch. Every other act is by convention under the owner's
  account and MUST be recorded with identity `NOT_VERIFIED`: the candidate tag push, the
  environment approval click, merges, status waivers and `auto` log rows. No agent creates, holds
  or uses the key. Fallback: if the owner declines a key, acceptance MUST be an owner-merged
  decision file plus the protected-environment approval, and every record MUST state that the
  owner's identity is by convention and `NOT_VERIFIED`. Source: decided 2026-11-09 under R42 (D12
  and D13); decision 0433.
- `ACR-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5272) Every
  edit of the `AGENTS.md` product invariants 3, 4, 5, 7 and 8 MUST be made by the change
  `acceptance-record` and by no other change. Invariants 3, 4 and 5 MUST take the wording quoted
  in the acceptance record's superseded-text table. Invariant 7 MUST take the accepted rewording,
  which keeps the citations of active documents and drops the archived ones. Invariant 8 MUST NOT
  be reworded by this change. Any later rewording of invariant 8 MUST keep the words "human-owned
  intent" and MUST go to the owner. Source: owner decisions H4 and H5 (2026-11-10).

## Fallback in force

Owner decision H14 (2026-11-10), the owner's words: "No key". From that decision:

- No allowed-signers file exists and no acceptance tag is signed.
- The plan acceptance is the owner-merged decision file, decision 0596.
- RC and stable acceptance are each an owner-merged release record plus the owner's approval of
  the protected environment. Neither release is accepted without its approval.
- Every record states that the owner's identity is by convention and `NOT_VERIFIED`.
- The tag verification of ACR-V1-011 and its two key scenarios are `NOT_RUN` while the fallback
  is in force. If the owner later creates a key, they apply again without a change to this spec.

## Scenarios

- Scenario for ACR-V1-001 (plan changed after acceptance): given a record naming plan SHA A, when
  the plan on `origin/main` has different content, then the audit reports the record as stale.
- Scenario for ACR-V1-002: given the decision record, when a reader looks up a PROD requirement
  written without an owner answer, then the requirement is in the drafted wording list.
- Scenario for ACR-V1-003: given a change folder with intent "proposed" when the record is accepted,
  when its delivery is reported, then it is reported as experimental, not delivered.
- Scenario for ACR-V1-004 (receipt claim): given a release record row whose evidence is a use-case
  receipt, when the audit runs, then it names the row.
- Scenario for ACR-V1-004 (stale quote): given a table row whose quote does not match its source at
  the cited commit, when the audit runs, then it names the row.
- Scenario for ACR-V1-005: given the RC release record, when a reader looks for Python selection
  precision, then the exception and the ticket are listed.
- Scenario for ACR-V1-006: given a PROD requirement with no log row, when `make
  spec-acceptance-audit` runs, then it names the requirement.
- Scenario for ACR-V1-007: given a requirement marked `standing` in its spec and `owner` in the log,
  when the audit runs, then it names the requirement and both modes.
- Scenario for ACR-V1-008: given a lane change that narrows an `owner` requirement, when the change
  is reviewed, then its log row reads `pending-owner` until the owner reads it back.
- Scenario for ACR-V1-009: given an RC tag and no accepted release record, when the release workflow
  starts the publish step, then it refuses and names what is missing.
- Scenario for ACR-V1-010: given a release record missing the `NOT_RUN` list, when the workflow
  reads it, then it refuses and names the missing section.
- Scenario for ACR-V1-011 (unlisted key): given an RC acceptance tag signed by a key that is not in
  the allowed-signers file as at the previously accepted tag, when the publish job verifies the tag,
  then it refuses and names the key fingerprint.
- Scenario for ACR-V1-011 (rotation without the old key): given a commit that replaces the key and
  no tag on it signed with the old key, when the next acceptance tag is verified, then verification
  uses the earlier file and the new key is refused.
- Scenario for ACR-V1-011 (convention act): given a status waiver posted under the owner's account,
  when the release record lists it, then the row gives the identity as `NOT_VERIFIED`.
- Scenario for ACR-V1-011 (owner declines a key): given the owner has recorded that no key will be
  used, when the RC is accepted through the owner-merged record and environment approval, then the
  release record states that the owner's identity is by convention and `NOT_VERIFIED`.
- Scenario for ACR-V1-012 (dropped words): given a commit that rewrites invariant 8 without the
  words "human-owned intent", when the check runs, then it fails and names the commit.
- Scenario for ACR-V1-012 (foreign edit): given a commit of another change that edits invariant 3,
  4, 5, 7 or 8, when the check runs, then it fails and names the commit.
- Scenario for ACR-V1-012 (accepted edits): given the edit made by this change, when the `AGENTS.md`
  diff is read back, then invariants 3, 4, 5 and 7 carry the accepted wording and invariant 8 is
  unchanged.

## The acceptance log and its audit

`docs/specs/openspec/acceptance-log.tsv` has six tab-separated columns: `id`, `kind`, `mode`,
`date`, `ownerRef`, `rationale`. `kind` is `prod` (a PROD requirement of the plan), `o-row` (a
checklist row of the plan) or `change` (a change folder name). `mode` is `owner`, `auto` or
`pending-owner`.

`script/acceptance-log-audit.sh` reads the log, `docs/specs/REQUIREMENTS.tsv` and the plan, and
prints one line per finding:

- `MISSING_ROW`: a PROD requirement in the index, or a checklist O-row in the plan, with no row.
- `UNKNOWN_ROW`: a row whose id is not a PROD requirement, a plan O-row or a change folder.
- `DUPLICATE_ROW`: a second row for one id.
- `MODE_MISMATCH`: a `prod` row whose mode disagrees with the index: `owner` wants `owner`,
  `auto` wants `standing`, `pending-owner` wants `unrecorded`. The line names both modes.

It ends with a `total` line. It reports and exits 0; with `--strict` it exits 1 on any finding.
A malformed log or index, or a row with an empty date, owner reference or rationale, exits 2.
Only an immediate folder of `changes/` or `changes/archive/` counts as a change.
`make spec-acceptance-audit` runs it after the mode listing.

The plan marks each of its 77 PROD requirements `(accepted, decision 0596; V1-5272)`, so the
log and the index agree and `make spec-acceptance-audit` runs the audit with `--strict`.

## The plan pin check

`script/acceptance-plan-pin-check.sh` compares the plan with the blob the acceptance record
names (ACR-V1-001). It compares words, so a re-wrapped paragraph is no difference. It ignores
three things, and nothing else:

- the acceptance marker of a PROD requirement line, `(proposed YYYY-MM-DD)` or `(accepted,
  decision NNNN; V1-NNNN)`. Owner decision H17 (2026-11-10): "Ignore markers in pin";
- the line numbers of a `path:LINE@hash` citation. A repin moves them and the hash holds;
- the record's own path, which changed when the proposed record took its decision number.

The second and third are lane decisions, not owner answers; both were already true of the plan
on `main` when the check was written. Any other difference prints `STALE_PLAN` with the first
differing words and exits 1. A record that names no blob, or a blob the clone does not hold,
exits 2, and the pin is then `NOT_VERIFIED`. The pin no longer covers the acceptance markers:
the log audit covers those. The plan's "Intent: proposed" sub-lines are pinned text and stay.

## Failure modes

- A lane writes an `owner` row. The audit cannot see who wrote a row; review and the owner's
  merge are the control, and identity stays `NOT_VERIFIED`.
- The log and a spec marker drift apart. The audit names the requirement and both modes.
- An accepted change has no `change` row. Not detected: no machine-readable list of accepted
  changes exists. Review is the control.
- A release is published with no record. Not prevented until the workflow read of ACR-V1-009
  and ACR-V1-010 is built; until then the owner's environment approval is the only control.
- An invariant is edited outside this change. `script/agents-invariants-check.sh` fails until the
  pinned hash is updated in a commit of this change.

## Acceptance evidence

| Requirement | Evidence | State |
|---|---|---|
| ACR-V1-001 | Decision 0596 names plan commit `1d1f06706` and blob `e4accbefd`; merged on the owner's instruction (H15, H16); `script/acceptance-plan-pin-check_test.sh` | delivered |
| ACR-V1-002 | Decision 0596 section 4 | delivered |
| ACR-V1-003 | Decision 0596 section 5 | delivered |
| ACR-V1-004 | Decision 0596 section 7 | table delivered; stale-quote audit `NOT_RUN` |
| ACR-V1-005 | Decision 0596 section 6 | delivered |
| ACR-V1-006 | `docs/specs/openspec/acceptance-log.tsv`; `script/acceptance-log-audit_test.sh` (missing row) | delivered |
| ACR-V1-007 | `script/acceptance-log-audit_test.sh` (mode mismatch) | delivered; 77 known mismatches reported |
| ACR-V1-008 | Review rule; `pending-owner` is checked against the index by the audit | review control; authorship `NOT_VERIFIED` |
| ACR-V1-009 | none | `NOT_RUN` |
| ACR-V1-010 | none | `NOT_RUN` |
| ACR-V1-011 | Decision 0596 status line and section 11 (fallback, H14) | fallback recorded; tag verification `NOT_RUN` |
| ACR-V1-012 | `script/agents-invariants-check.sh`, `script/agents-invariants-check_test.sh`; build log `2026-11-10-agents-invariants-acr012.md` | delivered |

## Rollback

Remove `script/acceptance-log-audit.sh`, its test and its line in the `spec-acceptance-audit`
target, and delete this file and its index row. The decision record, the log and the invariant
check are separate deliveries and stay. No stored state or wire format changes.
