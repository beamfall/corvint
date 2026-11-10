# Spec delta: `tasks-hardening` (change `tasks-hardening`)

## ADDED Requirements

### Requirement: TKF-V1-001 Plan and blocker reads treat an expired lease as reapable

`plan preview`, `ticket blockers`, `ticket show` and every other read that reports claimability MUST
treat a reservation entry whose attempt holds a lease (no supervision) with `expiresAt` at or before
the read's `observedAt` as reapable, not live. Such an entry MUST NOT collide with another entry and
MUST NOT count toward `maxActiveAttempts`. The plan result MUST list each one under `expiredLeases`
with its attempt ID, generation, `expiresAt` and the reap command, and MUST state `observedAt`. For
the ticket that holds the expired lease the blocker code MUST be `LEASE_EXPIRED`, not
`ATTEMPT_LIVE`, and `nextAction` MUST be `claim`. The read writes nothing. (PROD-TASK-005;
PROD-TASK-002)

Failure mode: the read and a later claim disagree because a holder renewed in between; the claim's
answer stands and the read names its `observedAt`.

Rollback: revert; reads return to counting every entry as live.

#### Scenario: Expired lease no longer blocks the preview

- **GIVEN** a store whose only reservation is a WHOLE_REPOSITORY lease that expired one hour ago
- **WHEN** `corvint-tasks plan preview --summary` runs
- **THEN** no entry is deferred with `RESOURCE_COLLISION` or `LIMIT_EXCEEDED` because of that
  reservation, `expiredLeases` names the attempt, and the store's files are unchanged

#### Scenario: Blocker for the expired holder's ticket

- **GIVEN** a ticket whose attempt lease expired
- **WHEN** `corvint-tasks ticket blockers` runs on it
- **THEN** the blocker code is `LEASE_EXPIRED` and `nextAction` is `claim`

### Requirement: TKF-V1-002 Queue status separates live and expired attempts

`queue status` MUST report `attemptsLive` and `attemptsExpired` counts, MUST list only attempts whose
lease is live (or supervised) under `liveAttempts`, and MUST list leased attempts past `expiresAt`
under `expiredLeases` with the reap command. (PROD-TASK-005)

#### Scenario: Three expired attempts

- **GIVEN** three leased attempts, all past `expiresAt`
- **WHEN** `corvint-tasks queue status` runs
- **THEN** `attemptsLive` is 0, `attemptsExpired` is 3 and `liveAttempts` is empty

### Requirement: TKF-V1-003 Holder progress extends the lease

An accepted `submit` or `gate run` by the lease holder at the attempt's current generation MUST set
the lease `expiresAt` to the later of its current value and the command's `recordedAt` plus L
minutes, where L is the command's `--lease-minutes` when given and `DefaultLeaseMinutes` (60)
otherwise, never above `MaxLeaseMinutes`. The extension MUST never shorten a lease. A command that
arrives after `expiresAt` MUST stay fenced exactly as today (CAL-V0-010 unchanged). (PROD-TASK-003)

Failure mode: a gate command runs longer than the remaining lease; the gate run is recorded after
the command finishes, so it is fenced. `finish` (TKF-V1-005) renews before each gate to prevent
this.

Rollback: revert; only `renew` extends a lease.

#### Scenario: Submit near expiry

- **GIVEN** an attempt whose lease expires in 2 minutes
- **WHEN** the holder runs `submit` at the current generation
- **THEN** the submit is accepted and the lease expires 60 minutes after the submit's `recordedAt`

#### Scenario: Submit after expiry

- **GIVEN** an attempt whose lease expired 1 minute ago
- **WHEN** the holder runs `submit`
- **THEN** the submit is refused with `FENCED` and the lease is not extended

### Requirement: TKF-V1-004 Heartbeat keeps the lease

An accepted `attempt heartbeat` on a leased attempt MUST extend the lease by the rule of TKF-V1-003
and record `lastHeartbeatAt`. `renew` stays as the way to request a specific length. `attempt
heartbeat --help` MUST state that heartbeat keeps the lease. Supervised attempts, which carry no
lease, MUST behave as today. (PROD-TASK-002; PROD-TASK-003)

#### Scenario: Heartbeats only

- **GIVEN** a 60-minute lease and a holder that sends `attempt heartbeat` every 10 minutes and never
  runs `renew`
- **WHEN** 3 hours pass
- **THEN** the lease is still live and the next `submit` is accepted

### Requirement: TKF-V1-005 One command finishes a claimed ticket

corvint-tasks MUST provide `corvint-tasks finish <ticket> --commit REV --holder LABEL --request-id
ID`. It MUST find the ticket's live attempt whose lease holder is LABEL and refuse with `NOT_FOUND`
when there is none. It MUST then, each as its own transaction with its own receipt: submit the
commit's tree unless it is already the candidate tree; for each required gate without a passing
result bound to this attempt, renew the lease and run the gate; then complete with `--commit REV`.
Derived request IDs MUST be `ID/submit`, `ID/renew/<gate>`, `ID/gate/<gate>` and `ID/complete`, so
rerunning with the same ID resumes without repeating an accepted step. It MUST stop at the first
refusal and report the step, its codes and the single command to rerun. It MUST NOT record a
completion that the existing `complete` would refuse. (PROD-TASK-003)

Rollback: remove the verb; the four-step path is unchanged underneath.

#### Scenario: Gate failure stops finish

- **GIVEN** a claimed ticket with one required gate that fails on the commit
- **WHEN** the holder runs `finish` on the ticket
- **THEN** submit is recorded, the gate failure is recorded, no completion is written, the ticket
  stays OPEN and the result names `gate run` as the failed step

#### Scenario: Rerun after a fix

- **GIVEN** a `finish` that stopped at a failed gate and a new commit that passes it
- **WHEN** the holder runs `finish` again with the new commit and a new request ID
- **THEN** the ticket is COMPLETED with completion kind VERIFIED

### Requirement: TKF-V1-006 Completion health reports the manual share

`receipt audit` `completionHealth` MUST report `manualShare` (manual divided by manual plus
verified in the window, as a decimal string) and MUST set state `MOSTLY_MANUAL` when that share is
above the policy threshold `completionHealth.maxManualShare` (default 0.5). State `NO_VERIFIED` and
`NO_COMPLETIONS` keep their meaning. It MUST list as `manualOnlyDays` each day in the window with
manual above zero and verified zero, and MUST emit a warning naming the share when the state is not
`OK`. (PROD-TASK-003)

#### Scenario: 89% manual

- **GIVEN** a seven-day window with 288 manual and 36 verified completions
- **WHEN** `corvint-tasks receipt audit` runs
- **THEN** `manualShare` is about 0.89, the state is `MOSTLY_MANUAL` and a warning names the share

### Requirement: TKF-V1-007 A read answers what to work on next

corvint-tasks MUST provide `corvint-tasks next [--limit N]` (default 10, maximum 100). It MUST
return the tickets the default plan would select, in plan order, using the same rules as `claim
--next` after TKF-V1-001. Each item MUST give the ticket ID, priority, title and the exact `claim`
command, with `--scope` filled from the ticket's declared effects or a statement that the claim
reserves WHOLE_REPOSITORY. The result MUST give one count per exclusion reason. It MUST write
nothing. (PROD-TASK-005)

#### Scenario: Next on a busy store

- **GIVEN** a store with open tickets, some blocked by dependencies and one held
- **WHEN** `corvint-tasks next --limit 3` runs
- **THEN** it returns at most 3 claimable tickets each with a `claim` command, counts the blocked
  and held tickets by reason, and the store's files are unchanged

### Requirement: TKF-V1-008 Roadmap shows open work by default

`roadmap` without a status filter MUST list only OPEN and HELD tickets, by priority. `roadmap --all`
MUST list every status, and `--status` MUST keep working. The change MUST be stated in the
supported-command list of `tasks-conformance`. (PROD-TASK-005)

#### Scenario: Default roadmap

- **GIVEN** a store with ARCHIVED, COMPLETED, OPEN and HELD tickets
- **WHEN** `corvint-tasks roadmap` runs with no flags
- **THEN** every listed ticket is OPEN or HELD, ordered by priority

### Requirement: TKF-V1-009 Queue status counters mean their names

`queue status` `blocked` MUST count only OPEN and HELD tickets whose eligibility is BLOCKED; closed
tickets MUST appear only under `byStatus`. The member `intentChecksPassed` MUST be replaced by
`eligibilityUnknown`, holding the count of OPEN and HELD tickets whose eligibility is UNKNOWN. Both
field changes MUST be recorded in schema v1. (PROD-TASK-005)

#### Scenario: Closed tickets are not blocked

- **GIVEN** a store with 1,029 OPEN, 7 HELD, 2,560 COMPLETED and 1,684 ARCHIVED tickets, of which
  35 OPEN or HELD tickets have eligibility BLOCKED
- **WHEN** `corvint-tasks queue status` runs
- **THEN** `blocked` is 35 and no member named `intentChecksPassed` appears

### Requirement: TKF-V1-010 Old progress notes show as stale

When a ticket has no live attempt and its lifecycle would come from a progress note recorded more
than N days before the read's `observedAt` (policy `progressStaleDays`, default 7), the lifecycle
MUST be `STALE_PROGRESS` with the note's phase and age, and lifecycle facets MUST count it under
`STALE_PROGRESS`. A note younger than N days MUST behave as today. (PROD-TASK-005)

#### Scenario: Ten-day-old note

- **GIVEN** an OPEN ticket with no attempt and an `IN_PROGRESS` progress note recorded 10 days ago
- **WHEN** `corvint-tasks ticket search --status OPEN --count` runs
- **THEN** the ticket is counted under `STALE_PROGRESS`, not `IN_PROGRESS`

### Requirement: TKF-V1-011 A missing ticket is NOT_FOUND

Every command that takes a ticket ID MUST return code `NOT_FOUND` with outcome `REFUSED` when the ID
is well formed and names no ticket in the queue. A malformed call MUST keep `MALFORMED` with outcome
`ERROR`. `NOT_FOUND` MUST be added to the schema v1 code list. (PROD-TASK-007)

#### Scenario: Unknown ID

- **GIVEN** a queue with no ticket V1-9999999
- **WHEN** `corvint-tasks ticket show V1-9999999` runs
- **THEN** the codes are `["NOT_FOUND"]` and the outcome is `REFUSED`

### Requirement: TKF-V1-012 Repeated advisories and long ID lists are emitted once

When more than one item in a read result carries an advisory with the same code and detail, the
detail MUST appear once in a top-level `advisories` list with the item count, and each item MUST
carry only the code. An inline list of ticket IDs longer than 20 MUST be replaced by its count and
the command that lists the IDs. (PROD-TASK-007)

#### Scenario: Search with a shared advisory

- **GIVEN** 100 OPEN tickets, 92 of them with the WHOLE_REPOSITORY coverage advisory
- **WHEN** `corvint-tasks ticket search --status OPEN --limit 100` runs
- **THEN** the advisory detail appears once with count 92 and each of the 92 items carries only the
  code

#### Scenario: Serial-fallback list

- **GIVEN** 231 tickets deferred only by the WHOLE_REPOSITORY fallback
- **WHEN** `corvint-tasks queue status` runs
- **THEN** it reports the count 231 and the command that lists them, not the 231 IDs
