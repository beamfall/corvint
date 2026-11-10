# Spec delta: `ticket-taxonomy` (change `ticket-taxonomy`)

## ADDED Requirements

### Requirement: TTX-V1-001 Closed facet vocabulary

A store policy MUST be able to declare a `facets` block that lists the allowed values of the label
namespaces `component`, `product`, `type`, `runner` and `agent`. With the block present, a ticket
create or update MUST be refused when it carries a label outside these namespaces (other than
`suspected`) or a value not in the list. (owner request 2026-11-09; PROD-TASK-005)

The component list of the Corvint store MUST include `rig`, for friction about the build rig
(lanes, hosts, CI plumbing). A `rig` ticket MUST have milestone `post-1-0`; a create or update
that gives it another milestone MUST be refused. A `rig` ticket MAY use `UNPLANNED` (TTX-V1-004)
and MUST be left out of the `v1-0` release object and of the default roadmap view. A defect in a
shipped CI surface (the action, the required checks, the release workflow) is not rig and MUST be
filed under component `action`, `release` or `gates`. Rig friction MUST NOT be kept in a list
outside the store. When the lane tooling moves to the private operator repository (G27, B8b),
open `rig` tickets move with it and the component is retired. (decided 2026-11-09 under R42, C21)

#### Scenario: A free-text label

- **GIVEN** a policy with the `facets` block
- **WHEN** a ticket is created with label `ci-friction`
- **THEN** the create is refused with `FACET_VALUE` naming the label, and nothing is written

#### Scenario: A rig ticket in the 1.0 milestone

- **GIVEN** a payload with `component:rig` and milestone `v1-0`
- **WHEN** the ticket is created
- **THEN** the create is refused with `FACET_VALUE` naming `milestone`

#### Scenario: A failing required check

- **GIVEN** a defect in a required check of the shipped CI
- **WHEN** it is filed
- **THEN** its component is `gates`, not `rig`

### Requirement: TTX-V1-002 Component required, product computed

Every ticket MUST carry exactly one `component` label. Its `product` label MUST be the product the
vocabulary assigns to that component. A create that states a different product MUST be refused.
(owner request 2026-11-09; PROD-INT-005)

Component `rig` (TTX-V1-001) follows the same rule. C21 names the component and its fixed
milestone but not its product; the product the vocabulary assigns to `rig` is NOT_DECIDED and
MUST be recorded in the vocabulary file before the component is used. (decided 2026-11-09 under
R42, C21)

#### Scenario: Product disagrees with component

- **GIVEN** component `mcp`, which the vocabulary assigns to product `docs`
- **WHEN** a ticket is created with `component:mcp` and `product:testing`
- **THEN** the create is refused with `FACET_VALUE` naming `product`

### Requirement: TTX-V1-003 Type required and kind agrees

Every ticket MUST carry exactly one `type` label from: bug, delivery, test-gap, optimization,
decision, chore. Its kind MUST be the kind the vocabulary assigns to that type. A ticket whose
purpose is to list other tickets MUST NOT be created. (owner request 2026-11-09)

#### Scenario: Kind disagrees with type

- **GIVEN** a payload with `type:bug` and kind FEATURE
- **WHEN** the ticket is created
- **THEN** the create is refused with `FACET_VALUE` naming `kind`

### Requirement: TTX-V1-004 Anchor required and existing

Every ticket MUST name at least one entry in `requirementRefs`. A `delivery` ticket and every ticket
with milestone `v1-0` MUST name at least one `PROD-*` requirement, or the create is refused. Each
named ID MUST exist in the configured requirements file. The value `UNPLANNED` MUST be accepted
only on a ticket that has milestone `post-1-0` and type bug, optimization, test-gap, chore or
decision, and only with a one-line reason. A ticket anchored to `UNPLANNED` MUST NOT enter the
`v1-0` release object. (PROD-TASK-006; PROD-INT-011; decided 2026-11-09 under R42, C20)

Earlier wording, superseded by C20: "The value `UNPLANNED` MUST be accepted only on a ticket that
has milestone `post-1-0` and is not a `delivery`."

#### Scenario: A delivery with no requirement

- **GIVEN** a payload with `type:delivery` and empty `requirementRefs`
- **WHEN** the ticket is created
- **THEN** the create is refused with `ANCHOR_REQUIRED`

#### Scenario: A requirement that does not exist

- **GIVEN** a payload naming `PROD-TEST-999`, which the requirements file does not contain
- **WHEN** the ticket is created
- **THEN** the create is refused with `ANCHOR_UNKNOWN` naming the ID

#### Scenario: UNPLANNED on a 1.0 ticket

- **GIVEN** a payload with `type:bug`, milestone `v1-0` and `requirementRefs` of `UNPLANNED`
- **WHEN** the ticket is created
- **THEN** the create is refused with `ANCHOR_REQUIRED`

#### Scenario: UNPLANNED with no reason

- **GIVEN** a payload with `type:chore`, milestone `post-1-0`, `UNPLANNED` and no reason line
- **WHEN** the ticket is created
- **THEN** the create is refused with `ANCHOR_REQUIRED`

### Requirement: TTX-V1-005 Runner and agent where they apply

A ticket whose product is `testing` MUST carry at least one `runner` label. A ticket whose component
is `agents` MUST carry at least one `agent` label. Both MAY repeat. (owner request 2026-11-09;
PROD-TEST-011; PROD-INT-023)

#### Scenario: A testing ticket with no runner

- **GIVEN** a payload with `component:affected` and no `runner` label
- **WHEN** the ticket is created
- **THEN** the create is refused with `FACET_REQUIRED` naming `runner`

### Requirement: TTX-V1-006 Identity key grammar

Every ticket MUST carry one identity key of at most 60 bytes in the form its type requires:
`<requirement-id>.<slice>[.<runner>]` for delivery, `<component>.<class>.<locus>` for bug,
`<component>.gap.<locus>` for test-gap, `<component>.slow.<locus>` or `<component>.large.<locus>`
for optimization, `<component>.chore.<action>.<object>` for chore, and `<component>.decide.<topic>`
for decision. The component, class, slice and action parts MUST come from the vocabulary and MUST
agree with the ticket's facets. (owner request 2026-11-09)

#### Scenario: Key names another component

- **GIVEN** a payload with `component:affected` and key `release.fail.v-corvint-update`
- **WHEN** the ticket is created
- **THEN** the create is refused with `KEY_MISMATCH`

### Requirement: TTX-V1-007 Key collision is refused inside the write

A create whose identity key is held by any ticket, in any status, MUST be refused with
`DUPLICATE_KEY`, unless TTX-V1-008 is met. The refusal MUST list every holder with its ID, status and
title. The check MUST run inside the create transaction, so that of two concurrent creates with one
key at most one succeeds. (owner request 2026-11-09: "duplicates can't exist")

The collision rule is confirmed as written: a key is a bucket, and hard uniqueness is not used.
(decided 2026-11-09 under R42, C19) The `identityKey` field, its index and this refusal MUST be
on corvint-tasks main by 2026-11-27, before schema v1 freezes on 2026-12-01. (decided 2026-11-09
under R42, C18)

#### Scenario: Second ticket for one problem

- **GIVEN** an open ticket with key `affected.over.c-unindexed-source-path`
- **WHEN** another ticket with that key is created
- **THEN** the create is refused with `DUPLICATE_KEY` and the refusal names the open ticket

#### Scenario: Two creates at once

- **GIVEN** two processes creating tickets with one new key at the same time
- **WHEN** both writes are attempted
- **THEN** exactly one ticket exists and the other process receives `DUPLICATE_KEY`

### Requirement: TTX-V1-008 A second ticket states how it differs

A create with a held key MUST be accepted only when its `distinctFrom` list names every OPEN holder
of that key and gives a non-empty reason for each. The list MUST be stored on the ticket. (sample
finding R5: V1-4973, V1-5013 and V1-5034 share a key and are different defects; confirmed as
written, decided 2026-11-09 under R42, C19)

#### Scenario: One holder left out

- **GIVEN** two open tickets with one key
- **WHEN** a third is created naming only one of them in `distinctFrom`
- **THEN** the create is refused with `DUPLICATE_KEY` naming the holder that was left out

### Requirement: TTX-V1-009 The refusal says how to add evidence

A `DUPLICATE_KEY` refusal MUST state the command that adds a recurrence or evidence to a holder, and
for a finished holder the command that reopens it. (owner request 2026-11-09)

#### Scenario: The holder is finished

- **GIVEN** a COMPLETED ticket holding the key
- **WHEN** a create with that key is refused
- **THEN** the refusal names the reopen command for that ticket

### Requirement: TTX-V1-010 Sibling list and lexical check on every create

Every create MUST return the open tickets that share its component and locus under another class,
and those that share its component and an anchor. Every create MUST run the lexical duplicate check
against open tickets of the same component, whatever its labels. (sample finding: three duplicate
pairs had different keys)

#### Scenario: Same target, different class

- **GIVEN** an open ticket with key `specs.slow.m-spec-requirements`
- **WHEN** a ticket with key `specs.fail.m-spec-requirements` is created
- **THEN** the create result lists the open ticket as a sibling

### Requirement: TTX-V1-011 Qualified effects at create

Every create MUST carry effects with coverage QUALIFIED and at least one touch path. A touch path
that is a single top-level directory MUST be refused. (owner request 2026-11-09; PROD-TASK-005)

#### Scenario: No touch path

- **GIVEN** a payload with coverage INCOMPLETE and no touch path
- **WHEN** the ticket is created
- **THEN** the create is refused with `EFFECTS_REQUIRED`

#### Scenario: After the seed

- **GIVEN** a store whose tickets were all created under this requirement
- **WHEN** the open tickets are counted by next action
- **THEN** the count for `set-effects` is 0

### Requirement: TTX-V1-012 Required lines per type

The body of a ticket MUST contain the labelled lines its type requires: bug `Version`, `Expected`,
`Actual` and `Repro`; delivery `Requirement`, `Slice` and `Missing today`; test-gap `Behaviour` and
`Gap`; optimization `Measured` and `Budget`; decision `Question`, `Options`, `Blocks` and `Decider`;
chore `Object`, `Action` and `Why now`. A bug without `Repro` MUST carry label `suspected` and MUST
NOT have priority P0 or P1. The body MUST NOT exceed 1,200 bytes. (owner request 2026-11-09: "the
exact information needed")

#### Scenario: A bug with no version

- **GIVEN** a `type:bug` payload whose body has no `Version:` line
- **WHEN** the ticket is created
- **THEN** the create is refused with `TEMPLATE_FIELD` naming `Version`

#### Scenario: A long body

- **GIVEN** a payload whose body is 1,500 bytes
- **WHEN** the ticket is created
- **THEN** the create is refused with `BODY_TOO_LONG`

### Requirement: TTX-V1-013 Title and acceptance

A title MUST be at most 100 bytes and MUST NOT start with a prefix ending in a colon or contain a
ticket ID. Every ticket MUST have at least one acceptance criterion. (owner request 2026-11-09)

#### Scenario: A prefixed title

- **GIVEN** a payload titled `affected: wrong count`
- **WHEN** the ticket is created
- **THEN** the create is refused with `TEMPLATE_FIELD` naming `title`

### Requirement: TTX-V1-014 Search by key, requirement, path and several labels

`ticket search` MUST accept `--key KEY`, `--requirement ID`, `--path PATH` (matching a ticket whose
touch path equals PATH or contains it) and `--label` more than once (all must match), together with
the existing filters. These filters MUST be on corvint-tasks main by 2026-11-27. (owner request
2026-11-09: "quickly searched"; PROD-TASK-005; decided 2026-11-09 under R42, C18)

#### Scenario: Open work for one requirement

- **GIVEN** three open tickets naming `PROD-TEST-016` and one that does not
- **WHEN** `ticket search --status OPEN --requirement PROD-TEST-016` runs
- **THEN** exactly the three tickets are returned

#### Scenario: Two labels

- **GIVEN** tickets labelled `type:test-gap`, some of them also `runner:pytest`
- **WHEN** `ticket search --label type:test-gap --label runner:pytest` runs
- **THEN** only tickets carrying both labels are returned

### Requirement: TTX-V1-015 Facet searches are timed

Each search of TTX-V1-014 and each single-facet search MUST be measured on a store of at least 5,000
tickets, and the times MUST be recorded with the build and host. These searches MUST NOT decode
every ticket record to answer. (PROD-TASK-005)

Queue health MUST report one number: the count of identity keys with more than one open holder.
It is the check on `distinctFrom` reasons that say nothing. (decided 2026-11-09 under R42, C19)

#### Scenario: Key lookup on a large store

- **GIVEN** a store of 5,000 tickets
- **WHEN** `ticket search --key K` runs
- **THEN** the answer comes from the key index and its time is in the timing record

#### Scenario: Three open tickets under one key

- **GIVEN** a store where one key has three open holders and every other key has one
- **WHEN** queue health is read
- **THEN** the count of keys with more than one open holder is 1

### Requirement: TTX-V1-016 Interim gate

Until a Tasks build enforces TTX-V1-001 to TTX-V1-013, tickets in this repository MUST be created
only through a gate script that applies the same rules and stores the key as label `key:<k>`. The
script MUST refuse when the vocabulary file and the store policy disagree. (owner request
2026-11-09)

The store cutover seeds with the interim `key:` label and this gate script. When the
`identityKey` field is on corvint-tasks main, a script MUST backfill the field from the label.
Build items B1 to B3 (the `identityKey` field and index, the `DUPLICATE_KEY` refusal inside the
create transaction and `distinctFrom`; the `facets` policy block; search by key, requirement,
path and several labels) MUST land by 2026-11-27. Build items B4 to B6 are best effort: any of
them not on corvint-tasks main by 2026-11-27 stays in the gate script for 1.0 and arrives in
1.0.x as a policy option that needs no format change. (decided 2026-11-09 under R42, C18)

#### Scenario: A create outside the vocabulary before the store enforces it

- **GIVEN** the installed Tasks build has no `facets` policy
- **WHEN** a payload with an unknown component is passed to the gate script
- **THEN** the script exits non-zero with `FACET_VALUE` and no create command runs

#### Scenario: Backfill after the field lands

- **GIVEN** a seeded ticket with label `key:affected.over.c-unindexed-source-path` and no
  `identityKey`
- **WHEN** the backfill script runs on a build that has the field
- **THEN** the ticket's `identityKey` equals the label value

### Requirement: TTX-V1-017 Seed gate

A ticket MUST NOT be seeded into a fresh store unless it passes every rule of TTX-V1-001 to
TTX-V1-013. A ticket that fails MUST be written to a held list with its reason code and MUST NOT be
seeded with a blank or placeholder value. Two seed tickets with one key MUST each name the other in
`distinctFrom`, or the newer one MUST be folded into the older as a recurrence. (owner approval
2026-11-09 for a fresh store; TSC-V1-009)

#### Scenario: A survivor with no touch path

- **GIVEN** a survivor row whose effects have no touch path
- **WHEN** the seed gate runs
- **THEN** the row is in the held list with `EFFECTS_REQUIRED` and is not in the seed list

#### Scenario: Two survivors with one key

- **GIVEN** survivor rows V1-5180 and V1-5203 with key `affected.over.c-unbounded-reader` and no
  `distinctFrom`
- **WHEN** the seed gate runs
- **THEN** only the older row is in the seed list, and the newer is recorded as its recurrence

### Requirement: TTX-V1-018 Part of schema v1 and the command list

The `identityKey` and `distinctFrom` members, the `facets` policy block and the search filters MUST
be in schema v1 and in the supported-command list before they are frozen. They MUST be on
corvint-tasks main by 2026-11-27, and schema v1 freezes on 2026-12-01. After the freeze a format
change restarts the 21-day trial (TCF-V1-009). (PROD-TASK-007; TCF-V1-001; TCF-V1-002; decided
2026-11-09 under R42, C18)

#### Scenario: Reading the command list

- **GIVEN** the corvint-tasks 1.0 supported-command list
- **WHEN** the entry for `ticket search` is read
- **THEN** it lists `--key`, `--requirement` and `--path`
