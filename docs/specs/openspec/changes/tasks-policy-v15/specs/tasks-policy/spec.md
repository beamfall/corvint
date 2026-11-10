# Spec delta: `tasks-policy` (change `tasks-policy-v15`)

## ADDED Requirements

### Requirement: TPV-V1-001 Version 15 requires no CEM and no OCM

Policy version 15 MUST NOT require a CEM or an OCM for any attempt: `cemRequired` and `ocmRequired`
are removed or false. (owner decision R3 (2026-11-09); file `.taskman/policy.json` line 1 (integ
9ec3a29d0))

#### Scenario: Submit without a CEM

- **GIVEN** an attempt under version 15 with no CEM file
- **WHEN** it is submitted
- **THEN** the store does not refuse it for a missing CEM or OCM

### Requirement: TPV-V1-002 Gates that run removed tooling are dropped

Version 15 MUST NOT declare the gates `cem-verdict` and `context-focused`, and MUST keep every other
member at its version 14 value. (owner decision R3 (2026-11-09); file `.taskman/policy.json` line 1
(integ 9ec3a29d0))

#### Scenario: Diff of versions

- **GIVEN** the version 14 and version 15 policy documents
- **WHEN** they are compared as JSON
- **THEN** the only differences are `policyVersion`, `cemRequired`, `ocmRequired` and the two gate
  entries

### Requirement: TPV-V1-003 Store first, then one carry PR

The store policy MUST be updated with `corvint-tasks policy update` before `.taskman/policy.json`
changes, and the tracked file MUST change in one PR with no other paths, matching the store bytes.
(owner decision R3 (2026-11-09))

#### Scenario: Carry PR with extra paths

- **GIVEN** a carry PR that also edits a spec
- **WHEN** it is reviewed
- **THEN** it is refused until it holds only `.taskman/policy.json`

### Requirement: TPV-V1-004 Update only after the M0 removal

The update MUST run only after the M0 removal is merged on `origin/main`. (owner decision R3
(2026-11-09); REL-V1-001)

#### Scenario: Removal not merged

- **GIVEN** the M0 removal still open
- **WHEN** the policy update is attempted
- **THEN** the task stays blocked on task 2.1

### Requirement: TPV-V1-005 No reference to a removed gate

Before the update, no release object and no OPEN or CLAIMED ticket MUST reference `cem-verdict` or
`context-focused`. (owner decision R3 (2026-11-09))

#### Scenario: A release names a removed gate

- **GIVEN** a release object whose required gates include `cem-verdict`
- **WHEN** the reference check runs
- **THEN** it reports the release and the update does not start

### Requirement: TPV-V1-006 In-flight attempts resubmit

An attempt admitted under version 14 and refused with `STALE_POLICY` after the update MUST be
resubmitted under version 15, not forced through. (owner decision R3 (2026-11-09))

#### Scenario: Stale attempt

- **GIVEN** an attempt admitted under version 14
- **WHEN** its submit is refused with `STALE_POLICY`
- **THEN** the attempt is resubmitted and judged under version 15

### Requirement: TPV-V1-007 Version 14 bytes retained for rollback

The version 14 bytes and their SHA-256 MUST be retained in the build-log entry before the update.
Rollback MUST restore those bytes, or publish version 16 with that content if a lower version is
refused. (owner decision R3 (2026-11-09))

#### Scenario: Rollback

- **GIVEN** version 15 in the store and a decision to roll back
- **WHEN** the retained bytes are restored
- **THEN** their SHA-256 matches the retained value and the store reports version 14 or 16

### Requirement: TPV-V1-008 One attempt completes under version 15

At least one ticket MUST be submitted, gated and completed under version 15 without a CEM before the
change is complete. (owner decision R3 (2026-11-09))

#### Scenario: First completion

- **GIVEN** version 15 in the store
- **WHEN** this change's own ticket is completed
- **THEN** `receipt audit` passes and the receipt cites version 15
