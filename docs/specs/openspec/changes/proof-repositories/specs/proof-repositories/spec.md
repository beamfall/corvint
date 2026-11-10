# Spec delta: proof-repositories

The lane copies each statement below into `proof-repositories-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: PRR-V1-001 The selection rule is measured, then committed and hashed

The rule MUST be committed with its SHA-256 before any run: licence MIT, Apache-2.0 or BSD; at least
500 tests; full suite at most 30 minutes; default-branch green rate at least 85 percent; at least
100 merged PRs in 12 months. The green rate MUST be computed as successful runs divided by completed
runs of the repository's named test workflow on default-branch pushes over the 90 days before the
seal, read from the GitHub Actions API; the manifest names the workflow. Each named repository's
values MUST be measured before the rule is hashed. The Playwright repository MUST additionally run
browser E2E against a running application. (G12, review release-tasks 15)

#### Scenario: Rule edited after a run

- **GIVEN** a committed rule hash and a recorded run
- **WHEN** the rule file changes
- **THEN** every selection check refuses with `rule-hash-mismatch`

#### Scenario: Green rate computed

- **GIVEN** 180 completed runs of the named test workflow on default-branch pushes in the 90 days
  before the seal, 160 of them successful
- **WHEN** the rule check runs
- **THEN** the recorded green rate is 88.9 percent with the API query as its source

### Requirement: PRR-V1-002 Seven named repositories

One named repository per blocking runner MUST be recorded with its measured rule values and used for
tuning and published numbers: the G12a primaries, or a swap made under PRR-V1-010. fastapi/fastapi,
solidusio/solidus, payloadcms/payload and helm/helm MUST NOT be used as named repositories or
alternates, because they are held out (owner decision R21 (2026-11-09); helm/helm decided
2026-11-09 under R42, A14). (G12, G12a, answer 17)

#### Scenario: Measured values recorded

- **GIVEN** the pinned rubocop/rubocop manifest
- **WHEN** the rule check runs
- **THEN** it records test count, suite time, green rate and merged PRs with their sources

#### Scenario: Held-out repository proposed as a swap

- **GIVEN** payloadcms/payload proposed as the Playwright swap
- **WHEN** the swap is checked
- **THEN** it refuses with `held-out-repository`

### Requirement: PRR-V1-003 Manifest pins everything

Each manifest MUST pin the repository URL, commit SHA, lockfile hashes, toolchain versions, browser
build where used and container image digest, and the manifest itself MUST be hashed. Source MUST be
fetched at the pinned SHA, never vendored. (G1, G12)

Flows on the pinned Playwright repository (decided 2026-11-09 under R42, A16). Agents write the
flow declarations on the freeCodeCamp pin. Corvint proposes links and the owner accepts them, per
flow, with `corvint-docs flows accept --flow ID --by NAME`. The target is 10 flows with at least 40
accepted links, accepted in one sitting by 2026-12-04. The flow file MUST then be frozen and its
hash recorded in the freeCodeCamp manifest before the Playwright fault set is sealed (FIH-V1-002).
The acceptances are the owner's act. Held-out repositories get no flows (PRR-V1-007).

Qualification mirrors (decided 2026-11-09 under R42, N4; recorded with low confidence). The seven
named repositories each need a public mirror in the `beamfall` organisation, a pushed copy of the
pinned commit and not a GitHub fork, named by runner so that a swap keeps the name:
`beamfall/corvint-qual-go`, `-vitest`, `-jest`, `-playwright`, `-pytest`, `-rspec` and `-minitest`.
They are needed by 2026-11-27 so that the 30 shadow PRs can start. Creating them is the owner's
act. They do not exist at the time of writing; each named manifest records its mirror URL only
after the mirror is created.

#### Scenario: Lockfile drift

- **GIVEN** a manifest whose lockfile hash differs from the fetched lockfile
- **WHEN** a run starts
- **THEN** it refuses with `pin-mismatch`

#### Scenario: Flow file not frozen

- **GIVEN** the freeCodeCamp manifest with no flow file hash
- **WHEN** the Playwright fault set is sealed
- **THEN** sealing refuses and names the missing flow file hash

#### Scenario: Mirror not yet created

- **GIVEN** a named manifest whose mirror the owner has not created
- **WHEN** the manifest is read
- **THEN** it holds no mirror URL, and no URL is assumed from the name

### Requirement: PRR-V1-004 Four held-out repositories, frozen before qualification

Owner decision R21 (2026-11-09). The held-out repositories are fastapi/fastapi (pytest),
solidusio/solidus (RSpec) and payloadcms/payload (Playwright). The held-out Go repository is
helm/helm, with the same treatment as those three (100 faults, the 50-PR replay), verified and
frozen before the M5a tag; if it fails a check, its alternate is go-task/task (decided 2026-11-09
under R42, A14). The helm/helm test count is not yet verified and MUST be verified before
freezing. Each MUST be checked against the
rule's licence and activity criteria and the untouched rule (PRR-V1-005) before the held-out list is
frozen; the frozen list, with each repository's runner, pinned SHA and check results, and its hash
MUST be committed before qualification starts. A repository that fails a check MUST NOT be frozen;
it goes back to the owner with a proposed replacement from its pool (PRR-V1-011). (G12)

Earlier wording, superseded by A14: "Three held-out repositories, frozen before qualification",
naming only fastapi, solidus and payload; G12 asked for four families and R21 named three.

#### Scenario: All four pass

- **GIVEN** fastapi, solidus, payload and helm passing licence, activity and untouched checks
- **WHEN** the held-out list is frozen
- **THEN** its hash is committed and names the four repositories with their runners

#### Scenario: helm fails a check

- **GIVEN** helm/helm failing one check before freezing
- **WHEN** the held-out list is frozen
- **THEN** helm/helm is not frozen, and go-task/task is checked as the Go alternate

#### Scenario: One fails

- **GIVEN** solidus measured below 100 merged PRs in 12 months
- **WHEN** the held-out list is frozen
- **THEN** freezing refuses, and the owner receives a proposed replacement from the Ruby pool

### Requirement: PRR-V1-005 Untouched means untouched

A held-out repository or pool entry MUST have had no Corvint run, configuration, flow or fix keyed to
it before the freeze. go-chi/chi MUST be excluded because run-001 used it. (G12)

#### Scenario: Previously used repository

- **GIVEN** go-chi/chi in a proposed Go pool
- **WHEN** the pool is sealed
- **THEN** sealing refuses and names chi as touched

### Requirement: PRR-V1-006 One hundred frozen faults per held-out repository

Each held-out repository MUST carry 100 injected faults frozen and hashed before execution, produced
by the FIH-V1 method (FIH-V1-012). (G12, G3, owner decision R19 (2026-11-09))

#### Scenario: Fault set frozen

- **GIVEN** a frozen held-out repository
- **WHEN** its 100 faults are generated
- **THEN** their hash is committed before any selected run

### Requirement: PRR-V1-007 No flows on held-out repositories

Held-out runs MUST use no flow declarations and MUST run E2E tests unnarrowed. (G12, G2)

#### Scenario: E2E on held-out

- **GIVEN** a held-out JS/TS repository with Playwright tests
- **WHEN** the plan is computed
- **THEN** every E2E test is selected with reason `held-out-unnarrowed`

### Requirement: PRR-V1-008 Held-out blocking conditions

A held-out result MUST block the release on more than 3 confirmed misses in N = 100 (reported with
the 95 percent Wilson interval), on excluded tests above 5 percent of the suite, on refusals above 25
percent of cases, or on a result that differs between two identical reruns. Over-selection MUST be
reported and MUST NOT block. Each confirmed held-out miss within the bound MUST be published with its
Wilson interval, filed as a P1 ticket and fixed by widening before the release candidate; the
seven named repositories' fault sets allow zero known misses. (G12, owner decision R19
(2026-11-09), settled release-tasks S6, R42 (panel pick) A9)

#### Scenario: Refusal ceiling

- **GIVEN** 26 of 100 cases refused on a held-out repository
- **WHEN** the verdict is computed
- **THEN** it blocks with `refusal-above-ceiling`

#### Scenario: Miss bound

- **GIVEN** 4 confirmed misses in 100 counted faults on a held-out repository
- **WHEN** the verdict is computed
- **THEN** it blocks with `miss-bound-exceeded`

### Requirement: PRR-V1-009 Where and when held-out runs happen

Held-out runs MUST run on the benchmark box, on Linux or macOS, only against the M5a
qualification-candidate tag or an RC tag. The run on the M5a tag gives the results the owner
uses for demotions before the freeze; the run on the RC tag confirms them. (G12, G1; decided
2026-11-09 under R42, A18)

#### Scenario: Early run attempt

- **GIVEN** neither a qualification-candidate tag nor an RC tag
- **WHEN** a held-out run is started
- **THEN** it refuses with `held-out-before-rc`

### Requirement: PRR-V1-010 Swaps are made by rule and reported

A named repository that fails the rule MUST be replaced only by a repository that passes the hashed
rule, chosen before the rule is hashed and before any Corvint run on that repository. Each swap MUST
be reported in the manifest with the failed criterion, its evidence and the replacement's measured
values. Only two cases MUST go back to the owner: a Minitest alternate that is a second Rails
component, and Playwright if freeCodeCamp fails with no passing candidate. (G12a, G10; R42 (panel
pick) A13)

#### Scenario: Flask fails activity

- **GIVEN** the measured 20 merged PRs for pallets/flask
- **WHEN** it is proposed as the pytest alternate
- **THEN** the rule check fails it, and only a replacement that passes the rule and has no Corvint
  run can be recorded, with the swap reported in the manifest

### Requirement: PRR-V1-011 Held-out replacement pools

Review release-tasks 16. For each held-out repository a pool file MUST list at least 5 replacement
candidates, each entry stating its runner and the search showing it is untouched (the query, its
date and the result). The pool files and their hashes MUST be committed with the frozen held-out
list. A replacement is proposed from the pool only under PRR-V1-004 and takes effect only by owner
decision.

#### Scenario: Short pool

- **GIVEN** a Python pool file with 4 candidates
- **WHEN** the pools are sealed
- **THEN** sealing refuses with `pool-too-small`

#### Scenario: Missing untouched search

- **GIVEN** a pool entry without a recorded untouched search
- **WHEN** the pools are sealed
- **THEN** sealing refuses and names the entry
