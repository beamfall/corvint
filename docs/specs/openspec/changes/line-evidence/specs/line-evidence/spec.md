# Spec delta: line-evidence

Capability spec: `docs/specs/line-evidence-v1.md` (new). This delta is a proposal only; the canonical
spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: LEV-V1-001 One writing verb for line evidence
Source: review T-B1 and NEW-testing-3; product invariant 4.
Status: proposed 2026-11-09; delivery not-started.
`corvint line-evidence refresh` MUST be the only command that collects and writes line evidence.
`corvint affected`, `corvint test`, `corvint watch` and MCP MUST only read it and MUST NOT run a
collector.

#### Scenario: affected without evidence
- **GIVEN** no line evidence exists
- **WHEN** `corvint affected` plans a change
- **THEN** it collects nothing, widens with `line-evidence-absent` and writes no file

### Requirement: LEV-V1-002 Private storage per runner instance
Source: review T-B1 and NEW-testing-3.
Status: proposed 2026-11-09; delivery not-started.
Line evidence MUST be stored per runner instance (TIV-V1-003) under `<git-dir>/corvint/line-evidence/`
in the `linecov-index/4` encoding (GLS-V0-001) with the instance's language, runner family, command
and config recorded. Tests MUST be named by canonical test ID. The store MUST NOT be committed.

#### Scenario: two Vitest roots
- **GIVEN** Vitest instances at `apps/a` and `apps/b`
- **WHEN** refresh completes
- **THEN** two evidence files exist, one per instance, each naming its root and config

### Requirement: LEV-V1-003 Evidence bound to the tree
Source: review T-B1; GLS-V0-001.
Status: proposed 2026-11-09; delivery not-started.
A reader MUST compare the evidence commit, runner identity, config and per-file blob OIDs with the
planned tree. A file whose blob is not in the evidence MUST widen to tier `file`; a commit that is
not an ancestor of the planned base, or a different runner identity or config, MUST widen the
instance to tier `full`. Both MUST print reason `line-evidence-stale`.

#### Scenario: config changed
- **GIVEN** `jest.config.js` changed since the evidence was collected
- **WHEN** `corvint affected` plans a change
- **THEN** the Jest instance runs in full with reason `line-evidence-stale`

### Requirement: LEV-V1-004 Missing or damaged evidence widens
Source: owner decision R8 (2026-11-09); every unknown runs with a named reason.
Status: proposed 2026-11-09; delivery not-started.
Missing evidence for an instance MUST widen reached files to tier `file` with reason
`line-evidence-absent`. A damaged section MUST use the `index-section-invalid` fallback (GLS-V0-002)
and widen. Stale or missing evidence MUST NOT narrow.

#### Scenario: evidence deleted
- **GIVEN** the store directory is removed
- **WHEN** a change touches one Go source file
- **THEN** each reached package runs whole at tier `file` (the Go file position is the package,
  CTI-V1-003) with `line-evidence-absent`

### Requirement: LEV-V1-005 Line-exact runners
Source: owner decision R8 (2026-11-09); PROD-TEST-008.
Status: proposed 2026-11-09; delivery not-started.
Go, Vitest, Jest, RSpec and Minitest MUST produce per-test line evidence through
`corvint line-evidence refresh`: Go through the existing Go collector, Vitest and Jest through the
JavaScript collector, RSpec and Minitest through the Ruby hooks.

#### Scenario: Minitest method covers a line
- **GIVEN** fresh Minitest evidence where only `CalcTest#test_add` executes line 12 of `lib/calc.rb`
- **WHEN** a change edits line 12 and the runner is FIH-qualified
- **THEN** the plan selects `minitest:test/calc_test.rb::CalcTest::test_add` at tier `line`

### Requirement: LEV-V1-006 pytest selects by file with its tier label
Source: owner decisions R8 and R9 (2026-11-09); review T-M16.
Status: proposed 2026-11-09; delivery not-started.
pytest MUST select whole test files and every pytest selection MUST carry tier `file`. The release
acceptance record MUST list pytest file-level selection as a release exception, and the Python line
collector MUST be tracked as a 1.0.x ticket.

#### Scenario: pytest change
- **GIVEN** a change to `app/models.py` reached by `tests/test_models.py`
- **WHEN** `corvint affected` plans the change
- **THEN** `pytest:tests/test_models.py` is selected with tier `file`

### Requirement: LEV-V1-007 Line evidence never removes a browser E2E test
Source: G2 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
A browser E2E test MUST be removed from a selection only by accepted flow links plus static
reachability at file and symbol level. Line evidence MAY add a browser E2E test to a selection or
order selected tests, and MUST NOT remove one. An ordering MUST be shown as `rank` and MUST NOT be
presented as narrowing.

#### Scenario: line evidence shows no coverage
- **GIVEN** a Playwright test with an accepted link whose flow is reached, and line evidence that
  shows the test executed none of the changed lines
- **WHEN** `corvint affected` plans the change
- **THEN** the test is selected, with the flow reason printed

### Requirement: LEV-V1-008 Per-test E2E line narrowing is opt-in and experimental
Source: G2 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
Per-test line narrowing of browser E2E tests MUST run only under an explicitly named opt-in profile
labelled experimental. Promotion of that profile MUST require a fault-injected zero-miss proof and a
measured net wall-clock win.

#### Scenario: default plan
- **GIVEN** no selection profile is named
- **WHEN** a Playwright plan is made
- **THEN** no browser E2E test is removed by line evidence

### Requirement: LEV-V1-009 One tier vocabulary
Source: owner decision R16 (2026-11-09); settled S4.
Status: proposed 2026-11-09; delivery not-started.
Every selected unit in a plan MUST carry `tier` with one of `line`, `file`, `flow`, `full`, and a
separate boolean `exact`, true only when the runner ran exactly the selected tests. Tier `flow` MUST
NOT count as narrowed in the no-missed-test claim.

#### Scenario: Playwright selection by flow
- **GIVEN** a Playwright test selected through an accepted flow link
- **WHEN** the narrowing report counts narrowed selections
- **THEN** that selection has tier `flow` and is not counted as narrowed for the no-missed-test claim

### Requirement: LEV-V1-010 One-line reason per selection
Source: G2 (owner answer).
Status: proposed 2026-11-09; delivery not-started.
Every selection MUST print one line naming the runner instance, the tier and the reason.

#### Scenario: stale evidence
- **GIVEN** stale Vitest evidence
- **WHEN** a plan is printed
- **THEN** the Vitest line names the instance, tier `full` and `line-evidence-stale`

### Requirement: LEV-V1-011 Tier line only after fault-injection proof
Source: review T-B1; owner decision R19 (2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
A runner's evidence MUST produce tier `line` in plans only after that runner is line-level proven by
the fault-injection harness (FIH-V1-015); the harness computes its own line plans for that proof.
Before that, fresh evidence MUST be used at tier `file` with reason
`line-evidence-unqualified`. Go keeps its qualification check (GLS-V0-034).

#### Scenario: Jest before qualification
- **GIVEN** fresh Jest evidence and no Jest FIH record
- **WHEN** a change reaches one Jest test
- **THEN** its whole file is selected at tier `file` with `line-evidence-unqualified`

### Requirement: LEV-V1-012 One reader for every consumer
Source: review T-B1.
Status: proposed 2026-11-09; delivery not-started.
`corvint affected`, `corvint test`, `corvint watch` and the MCP affected tool MUST use one reader for
line evidence. `corvint watch` MUST accept evidence for every supported language and MUST NOT refuse
a non-Go index.

#### Scenario: watch on a Vitest project
- **GIVEN** fresh Vitest evidence
- **WHEN** `corvint watch` handles a save
- **THEN** it plans from that evidence instead of refusing with `watch-index-language`

### Requirement: LEV-V1-013 The Action builds and caches evidence
Source: review NEW-testing-3 and T-B1.
Status: proposed 2026-11-09; delivery not-started.
The GitHub Action MUST run `corvint line-evidence refresh` on the default branch and cache the store
keyed by commit and runner instance. A PR job MUST restore evidence only from an ancestor commit; a
cache miss MUST widen with `line-evidence-absent`.

#### Scenario: first PR after setup
- **GIVEN** no cached evidence
- **WHEN** the Action plans a PR
- **THEN** reached files run at tier `file` with `line-evidence-absent`

### Requirement: LEV-V1-014 Argument limits
Source: review T-M9.
Status: proposed 2026-11-09; delivery not-started.
When the selected names for one invocation exceed the runner's argument limit, Corvint MUST pass them
in an argument file where the runner accepts one, and otherwise MUST widen the invocation to file
level with reason `argv-limit`.

#### Scenario: 5000 selected Go tests
- **GIVEN** a selection whose `-run` pattern would exceed the argument limit
- **WHEN** `corvint test` runs it
- **THEN** it widens to the selected packages with reason `argv-limit`

### Requirement: LEV-V1-015 One-shot commands plan the working tree
Source: selection verdicts finding 10; R42 (panel pick) A7.
Status: proposed 2026-11-09; delivery not-started.
One-shot `corvint affected`, `corvint test` and the MCP plan tool MUST use the `corvint watch` rule
of decision 3: the planned change is the working tree and index against HEAD, with HEAD checked to
descend from the evidence commit. A changed file with no matching blob MUST widen to `file` with
reason `line-evidence-stale`; HEAD not descending from the evidence commit MUST give FULL.

#### Scenario: staged edit
- **GIVEN** fresh evidence at HEAD and a staged edit to `src/math.ts`
- **WHEN** `corvint affected` runs
- **THEN** it plans the edited lines from that evidence instead of the `worktree-dirty` fallback

#### Scenario: untracked file
- **GIVEN** fresh evidence at HEAD and a new untracked source file `src/extra.ts`
- **WHEN** `corvint test` runs
- **THEN** `src/extra.ts` widens to tier `file` with reason `line-evidence-stale`

## MODIFIED Requirements

None. GLS-V0-010 (explicit `--line-index FILE`) is unchanged. The Go-only `watch-index-language`
refusal in `cmd/corvint/watch.go` (integ 9ec3a29d0) has no requirement ID; LEV-V1-012 replaces it.

## REMOVED Requirements

None.
