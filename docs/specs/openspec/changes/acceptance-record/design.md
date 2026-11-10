# Design: `acceptance-record`

## Decisions

- Decision record. One file `docs/decisions/NNNN-accept-product-plan-1-0-YYYY-MM-DD.md`, with NNNN
  the next free number when it is written. It names the plan path and commit SHA, lists the drafted
  requirement wording, states that in-flight work stays experimental, holds the supersession table
  and lists release exceptions (R9). It is written as a draft by a lane and becomes accepted only
  when the owner signs it.
- Supersession table. Columns: superseded source, quoted text, replacement (owner decision or plan
  section), effect (replaced, amended, removed). Rows: decision 0433 item 1; answer 3; answer 11;
  the G9 MCP row; R26; decision 0594 item 4; the `AGENTS.md` sections "Dogfood Corvint" and the
  dogfood paragraphs it links. Each quote is copied from the source at a named commit.
- Acceptance log. `docs/specs/openspec/acceptance-log.tsv`, tab separated, one header line, columns
  `id`, `kind` (`prod`, `o-row`, `change`), `mode`, `date`, `ownerRef`, `rationale`. Modes:
  - `owner`: the owner accepted it; `ownerRef` names a decision number or an owner decision Rn with
    date. Maps to the existing `owner` marker `(accepted, decision NNNN; V1-NNNN)`.
  - `auto`: accepted under the owner's standing auto-accept. Maps to the `standing` marker.
  - `pending-owner`: written, not accepted. Maps to no acceptance marker (`unrecorded`).
  Only the owner writes `owner` rows. A lane writes `auto` or `pending-owner` rows. A lane change
  that alters a requirement's meaning sets its row to `pending-owner`.
- Audit. `script/spec-acceptance-audit.sh` gains a log check: every PROD requirement and O-row has a
  row; a row's mode agrees with its spec marker by the mapping above; no `owner` row was added in a
  commit not authored by the owner. Decided 2026-11-09 under R42, D12 and D13: the signature
  mechanism is a tag signed with the owner's touch-required key (ACR-V1-011). `owner` log rows
  are covered by the signed acceptance tag on the commit that holds them; commit authorship alone
  is by convention, identity `NOT_VERIFIED`.
  It stays a report under `make spec-acceptance-audit` and is wired as a gate step only by the change
  `openspec-check`.
- Invariant edits (ACR-V1-012; owner decisions H4 and H5 (2026-11-10)). Every edit of `AGENTS.md`
  product invariants 3, 4, 5, 7 and 8 is made by this change and no other. 3, 4 and 5 take the
  wording quoted in the record's superseded-text table (section 7). 7 keeps the citations of active
  documents and drops the archived ones. 8 is not reworded, because the proposed rewording dropped
  "human-owned intent". A later rewording of 8 keeps those words and goes to the owner. The audit
  gains two checks: invariant 8 still contains "human-owned intent", and a commit that changes
  invariants 3, 4, 5, 7 or 8 belongs to this change.
- Release records. One file per signed event (RC publish, stable promotion) under
  `docs/releases/`, listing each requirement with its evidence path or demotion, NOT_RUN and
  NOT_OBSERVED items and deferred P2 and P3 tickets. The release workflow refuses to publish without
  the signed record (REL-V1-013).

## Failure modes

- A PROD requirement or O-row without a log row: the audit names it.
- A spec marker says `owner` while the log says `auto`: the audit names the mismatch.
- A proposal header says accepted with no log row: `openspec-check` fails (OSC-V1-009).
- A supersession quote that no longer matches its source at the cited commit: the audit names it.
- The plan SHA in the record differs from the plan on `origin/main`: the record is stale and must be
  re-signed.
- The signature mechanism is decided (2026-11-09 under R42, D12 and D13; ACR-V1-011). If the
  owner has neither generated a key nor recorded the fallback by the RC, the RC cannot publish.
- An acceptance tag signed by a key missing from the earlier allowed-signers file: the publish
  job refuses.
- An edit of invariant 8 that drops "human-owned intent", or an invariant edit made by another
  change: the audit names the commit and the change is refused (ACR-V1-012).

## Acceptance evidence

- The signed decision record and the plan SHA it names.
- `docs/specs/openspec/acceptance-log.tsv` with one row per PROD requirement and O-row.
- `make spec-acceptance-audit` output with zero log mismatches.
- Audit tests for a missing row, a mode mismatch and a stale quote.
- The signed RC release record, read by the release workflow.
- The `AGENTS.md` diff: invariants 3, 4, 5 and 7 changed as ACR-V1-012 states, invariant 8
  unchanged, and the audit tests for a dropped phrase and a foreign edit.

## Rollback

- The log and audit extension are additive; removing the log check returns the audit to its SRG-V0-010
  behavior.
- A decision record is never deleted; a wrong record is superseded by a new decision record.
