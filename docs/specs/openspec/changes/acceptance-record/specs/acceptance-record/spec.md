# Spec delta: `acceptance-record` (change `acceptance-record`)

## ADDED Requirements

### Requirement: ACR-V1-001 One decision record accepts the plan as amended

One decision record MUST accept `docs/specs/PRODUCT-PLAN-1.0.md` as amended, naming the plan commit
SHA, and MUST be signed by the owner before it counts as accepted. (answer 18; owner decision R1
(2026-11-09))

#### Scenario: Plan changed after signing

- **GIVEN** a signed record naming plan SHA A
- **WHEN** the plan on `origin/main` is at SHA B with different content
- **THEN** the audit reports the record as stale

### Requirement: ACR-V1-002 Drafted wording is named

The record MUST list every requirement whose wording was written rather than quoted from an owner
answer. (owner decision R1 (2026-11-09))

#### Scenario: Reading the record

- **GIVEN** the decision record
- **WHEN** a reader looks up a PROD requirement written without an owner answer
- **THEN** the requirement is in the drafted wording list

### Requirement: ACR-V1-003 In-flight work stays experimental

The record MUST state that work in flight when it is signed stays experimental until its own change
is accepted. (owner decision R1 (2026-11-09))

#### Scenario: A lane in flight

- **GIVEN** a change folder with intent "proposed" when the record is signed
- **WHEN** its delivery is reported
- **THEN** it is reported as experimental, not delivered

### Requirement: ACR-V1-004 Supersession table with quoted text

The record MUST hold a table quoting each superseded text at a named commit and naming its
replacement, covering at least decision 0433 item 1, answer 3, answer 11, the G9 MCP row, R26,
decision 0594 item 4 and the dogfood sections of `AGENTS.md`. (owner decision R1 (2026-11-09); G9;
owner decisions R8, R9, R17, R39 and R40 (2026-11-09))

The table MUST also hold this row (final round, M6): "The use-case receipt contract is
superseded by the per-product end-to-end qualification of REL-V1-012; `conformance/use-cases-v0`
is removed in B8 together with its two lints, and no 1.0 claim cites a use-case receipt." The
name of the spec that governs the receipt contract is UNVERIFIED; the row names it once the lane
has found it.

The table MUST name change `github-action` as the home of GHA-V1-040 to GHA-V1-043, with their
IDs unchanged, and MUST say that `release-process` cites them. (decided 2026-11-09 under R42, D6)

The table MUST quote the old and new wording of product invariants 3, 4 and 5 in `AGENTS.md`
(decided 2026-11-09 under R42, E3 and D17). Invariant 3 drops "and learned task traces".
Invariant 4 drops its `--learn-slot-weights` clause and gains the agent hook session state
sentence of AGT-V1-022. Invariant 5 becomes: "Corvint 1.0 learns nothing from use. Any later
learning must be explicit, local, bounded, secret-screened and evaluation-gated under its own
decision."

#### Scenario: A 1.0 claim cites a use-case receipt

- **GIVEN** a release record row whose evidence is a use-case receipt
- **WHEN** the audit runs
- **THEN** it names the row, because the receipt contract is superseded

#### Scenario: A stale quote

- **GIVEN** a table row whose quote does not match its source at the cited commit
- **WHEN** the audit runs
- **THEN** it names the row

### Requirement: ACR-V1-005 Python file-level selection is a release exception

The record MUST list Python file-level selection as a release exception with a link to the 1.0.x
ticket for the Python line collector. (owner decision R9 (2026-11-09))

#### Scenario: Release review

- **GIVEN** the RC release record
- **WHEN** a reader looks for Python selection precision
- **THEN** the exception and the ticket are listed

### Requirement: ACR-V1-006 Acceptance log with one row per requirement

`docs/specs/openspec/acceptance-log.tsv` MUST hold one row for every PROD requirement, every checklist
O-row and every accepted change, with mode `owner`, `auto` or `pending-owner`, date, owner reference
and rationale. Only the owner MUST write `owner` rows. (G14)

#### Scenario: A missing row

- **GIVEN** a PROD requirement with no log row
- **WHEN** `make spec-acceptance-audit` runs
- **THEN** it names the requirement

### Requirement: ACR-V1-007 Log modes agree with spec markers

The audit MUST check that each log mode agrees with the requirement's spec marker: `owner` with
`(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`, `auto` with the
`standing` marker, `pending-owner` with no acceptance marker. (G14; SRG-V0-009; SRG-V0-010; file
`docs/SPEC-DRIVEN-DEVELOPMENT.md` lines 113 to 114 (integ 9ec3a29d0))

#### Scenario: Mode mismatch

- **GIVEN** a requirement marked `standing` in its spec and `owner` in the log
- **WHEN** the audit runs
- **THEN** it names the requirement and both modes

### Requirement: ACR-V1-008 Meaning changes go back to the owner

A change that alters an accepted requirement's meaning MUST set its log row to `pending-owner`, and a
lane MUST NOT write an `owner` row. (G14)

#### Scenario: A lane edits accepted text

- **GIVEN** a lane change that narrows an `owner` requirement
- **WHEN** the change is reviewed
- **THEN** its log row reads `pending-owner` until the owner reads it back

### Requirement: ACR-V1-009 Signed release record before RC and stable

The RC publish and the stable promotion MUST each have an owner-signed release record listing each
requirement's evidence or demotion, NOT_RUN and NOT_OBSERVED items and deferred P2 and P3 tickets.
(G10; REL-V1-013)

#### Scenario: Unsigned record

- **GIVEN** an RC tag and an unsigned release record
- **WHEN** the release workflow starts the publish step
- **THEN** it refuses and names the missing signature

### Requirement: ACR-V1-010 Release record format readable by the workflow

The release record MUST use one documented file format under `docs/releases/` that the release
workflow reads without network access beyond the repository. (G10)

#### Scenario: Malformed record

- **GIVEN** a release record missing the NOT_RUN list
- **WHEN** the workflow reads it
- **THEN** it refuses and names the missing section

### Requirement: ACR-V1-011 Acts that need the owner's key

Four acts MUST carry a tag signed by the owner with a touch-required key, held in hardware or in
a Secure Enclave and listed in a committed allowed-signers file. (decided 2026-11-09 under R42,
D12 and D13)

1. Plan acceptance: a signed tag on the commit that holds the acceptance decision file.
2. RC acceptance: a signed tag on the commit that holds the `docs/releases/<version>` record.
3. Stable acceptance: a signed tag on the commit that holds the stable release record.
4. Any change to the allowed-signers file. A key rotation commit MUST be tagged with the old key.

The RC and stable release records (ACR-V1-010) MUST name the candidate tag's commit SHA and the
`SHA256SUMS` digest. The publish job MUST verify the signed tag against the allowed-signers file
as it stood at the previously accepted tag, or for the first release at the commit named in the
acceptance decision file. It MUST NOT use the allowed-signers file in the released tree, and it
MUST refuse on any mismatch.

Every other act is by convention under the owner's account and MUST be recorded with identity
`NOT_VERIFIED`: the candidate tag push, the environment approval click, merges, status waivers
and `auto` log rows. Whether a token can approve an environment deployment is NOT_VERIFIED. A
separate machine account for agents is a 1.0.x ticket.

The owner's acts, done in person: generate the key on the owner's own device, check its
fingerprint by eye, sign the three tags and give the two environment approvals. No agent creates,
holds or uses the key (decision 0433).

Fallback. If the owner declines a key, acceptance MUST be an owner-merged decision file plus the
protected-environment approval, and every record MUST state that the owner's identity is by
convention and NOT_VERIFIED.

#### Scenario: Tag signed by an unlisted key

- **GIVEN** an RC acceptance tag signed by a key that is not in the allowed-signers file as at
  the previously accepted tag
- **WHEN** the publish job verifies the tag
- **THEN** it refuses and names the key fingerprint

#### Scenario: Rotation without the old key

- **GIVEN** a commit that replaces the key in the allowed-signers file
- **AND** no tag on that commit signed with the old key
- **WHEN** the next acceptance tag is verified
- **THEN** verification uses the earlier file and the new key is refused

#### Scenario: Convention act recorded

- **GIVEN** a status waiver posted under the owner's account
- **WHEN** the release record lists it
- **THEN** the row gives the identity as `NOT_VERIFIED`

#### Scenario: Owner declines a key

- **GIVEN** the owner has recorded that no signing key will be used
- **WHEN** the RC is accepted through the owner-merged decision file and environment approval
- **THEN** the release record states that the owner's identity is by convention and NOT_VERIFIED

### Requirement: ACR-V1-012 Edits of AGENTS.md invariants 3, 4, 5, 7 and 8 belong to this change

Every edit of the `AGENTS.md` product invariants 3, 4, 5, 7 and 8 MUST be made by this change and
by no other change. Invariants 3, 4 and 5 MUST take the wording quoted in the acceptance record's
superseded-text table. Invariant 7 MUST take the accepted rewording, which keeps the citations of
active documents and drops the archived ones. Invariant 8 MUST NOT be reworded by this change,
because the proposed rewording dropped "human-owned intent" and the owner rejected it. Any later
rewording of invariant 8 MUST keep the words "human-owned intent" and MUST go to the owner. (owner
decisions H4 and H5 (2026-11-10))

#### Scenario: An edit of invariant 8 drops the words

- **GIVEN** a commit that rewrites invariant 8 without the words "human-owned intent"
- **WHEN** the audit runs
- **THEN** it names the commit and the check fails

#### Scenario: An invariant edit from another change

- **GIVEN** a commit of another change that edits invariant 3, 4, 5, 7 or 8
- **WHEN** the audit runs
- **THEN** it names the commit and refuses the edit as not owned by this change

#### Scenario: The accepted edits

- **GIVEN** the edit made by this change
- **WHEN** the `AGENTS.md` diff is read back
- **THEN** invariants 3, 4, 5 and 7 carry the accepted wording and invariant 8 is unchanged
