# Non-code map V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Authoritative inputs: change folder `docs/specs/openspec/changes/non-code-map/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/non-code-map/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-002, PROD-TEST-008, PROD-TEST-012, PROD-TEST-019) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner answers G2, G3, G15b and G16,
owner decisions R31 and R32).

## Agent digest
- Claim: A changed non-code file runs every test unless a proven, accepted map entry narrows it; some paths always run everything.
- Status: accepted / partial. Change `non-code-map` auto-accepted under G14 (decision 0596); the FIH-V1 proof link waits for V1-5405.
- Exists: `internal/noncodemap` (strict `corvint-data-map/0` reader, entry status, never-narrow list, per-path decision, proposal from recorded reads); the AFP-V0-012 selector prints the decision and falls back to a full run; `UNOWNED_DIRTY_PATH` is off the narrowing allowlist.
- Blocked on: V1-5405 (fault-injection harness) for the proof link; until it lands no entry can be `proven`, so no entry narrows.
- Read next: Delivered rules; Traceability.

## User and boundary

An agent or reviewer runs `corvint affected` and `corvint test` on a change that edits a fixture, a
configuration file, a lockfile or a Corvint declaration. Before this change the AFP-V0-012 selector
narrowed such a change to the Go packages its reader rules attribute, and an `UNOWNED_DIRTY_PATH`
entry stayed on the narrowing allowlist (AFP-V0-982), so a path no plugin claims could narrow with
no proof that its readers were all selected.

This spec is the canonical home of requirements NCM-V1-001 to NCM-V1-009, copied from the change's
`specs/non-code-map/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds the delivered rules, the orchestrator decision on the
design's first open question, and the delivery record (Surfaces, Traceability).

## Requirements

- `NCM-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Full by default

A changed non-code path without a proven, accepted entry MUST select every test of every runner in
the repository, with reason `non-code-unmapped` and the path printed. A path no runner owns, and a
runner that was removed or is unavailable, MUST likewise select all tests, with a named reason
(`UNOWNED_DIRTY_PATH` or `runner-removed`) and the path or runner printed; `UNOWNED_DIRTY_PATH`
no longer narrows through the allowlist in `internal/gateselect/allowlist.go` (integ 9ec3a29d0).
Paths declared as documentation keep the `DECLARED_DOCUMENTATION` class and add no test. (G15b,
G2; owner decision R31 (2026-11-09); owner decision R32 (2026-11-09))

**Scenario: Fixture change, no entry**
- GIVEN a change to `tests/fixtures/users.json` and no map entry for it
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected and the reason names the path

**Scenario: Unowned path**
- GIVEN a change to `tools/notes.xyz` that no runner owns and no map entry covers
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected with reason `UNOWNED_DIRTY_PATH` naming the path

**Scenario: Declared documentation**
- GIVEN a change to a path listed in `.corvint/documentation-paths.json` only
- WHEN `corvint affected --base FULL_SHA` runs
- THEN no test is added for that path

- `NCM-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Map entry record

An entry MUST hold `path` (or `glob`), `tests`, `evidence`, `accepted_by`, `accepted_at`,
`pinned_commit` and `proof` (the identifier of the fault case that proved it). (G15b)

**Scenario: Missing proof**
- GIVEN an entry without `proof`
- WHEN the map is read
- THEN the entry is reported `proposed` and narrows nothing

- `NCM-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Recordings only propose

Entries derived from recorded test reads MUST be written as proposals to stdout and MUST NOT narrow
until a person accepts them. (G15b)

**Scenario: Proposal output**
- GIVEN a recorded run that read `config/app.yml` from three tests
- WHEN a proposal is requested
- THEN stdout holds a proposed entry and the map file is unchanged

- `NCM-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Narrowing needs a caught injected fault

An entry MUST narrow only if its `proof` names a fault case in the FIH-V1 non-code set where a fault
in the mapped file was killed and the killing test is within the entry's `tests`. (G15b, G3)

**Scenario: Fault not caught**
- GIVEN an accepted entry whose proving fault killed no test
- WHEN the map is read
- THEN the entry does not narrow and is reported `unproven`

- `NCM-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Paths never narrowed

The following MUST always select everything, whatever the map says: `go.mod`, `go.sum` and every
other lockfile; the toolchain pin; `.github/**`; `Makefile`; runner configuration; release
configuration; generator inputs; `go:embed` and build-tag inputs; snapshot update paths; the map
file itself; `.corvint/test-read-scopes.json`; secret and environment configuration. (G15b)

**Scenario: Lockfile**
- GIVEN a change to `package-lock.json` and an entry claiming two tests
- WHEN the plan is computed
- THEN every test is selected with reason `never-narrowed-path`

- `NCM-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Corvint configuration runs everything

A change to a Corvint configuration file under `.corvint/`, including the map file, MUST select
everything, with reason `corvint-config` and the path printed. Configuration means every file under
`.corvint/` except: the flow file (`.corvint/flows.json`, or the deprecated `.corvint/e2e-flows.json`)
and its link records, whose change MUST select the tests linked to the changed flows; and the two
private ledgers `self-observations.jsonl` and `unplanned-reads.jsonl`, whose change MUST add no
test. (G16, narrowed by owner decision R31 (2026-11-09))

Earlier wording, superseded by R31: "A change to any `.corvint/` declaration or to the map file MUST
select everything."

**Scenario: Map edit**
- GIVEN a change that adds one map entry
- WHEN the plan is computed for that change
- THEN every test is selected with reason `corvint-config`

**Scenario: Flow file edit**
- GIVEN a change to `.corvint/flows.json` that edits one flow linked to two tests
- WHEN the plan is computed for that change
- THEN exactly those two tests are added for that path

**Scenario: Ledger append**
- GIVEN a change that only appends to `.corvint/self-observations.jsonl`
- WHEN the plan is computed for that change
- THEN no test is added for that path

- `NCM-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Versioned and strict

The map MUST carry the profile version `corvint-data-map/0` (review docs F17: the first version is
`/0`, as for every other declaration new in 1.0), and a reader MUST refuse an unknown version or
unknown member with `data-map-invalid`. (G15b; strictness mirrors `internal/gateselect/readers.go`)

**Scenario: Newer version**
- GIVEN `corvint-data-map/1`
- WHEN a 1.0 reader loads it
- THEN it exits 2 with `data-map-invalid`

- `NCM-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Stale entries stop narrowing

An entry whose cited test is gone, or whose `pinned_commit` is not an ancestor of the head, MUST NOT
narrow and MUST be reported `stale`. (G15b)

**Scenario: Deleted test**
- GIVEN an entry citing a test that was removed
- WHEN the plan is computed
- THEN the entry is `stale` and the path selects everything

- `NCM-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5424) Deterministic and read-only

Reading the map and planning with it MUST write nothing and MUST give byte-identical output for the
same inputs. (invariant 4; G15b)

**Scenario: Repeated run**
- GIVEN the same commit and map
- WHEN the plan is computed twice
- THEN both outputs are identical and the worktree is unchanged

## Delivered rules

These rules state how V1-5424 reads the requirements above. They add no requirement.

1. Where the decision is printed. The AFP-V0-012 selector (`internal/gateselect`, read by
   `corvint test` and by `tools/gate-affected-select`) decides whether a run narrows, so it is the
   planner that applies this spec. It prints one `noncode <reason> <path>` line per path a rule
   below decides, one `data-map <key> <status>: <detail>` line per map entry that covers a dirty
   path, and, when any rule needs a full run, the verdict `FALLBACK <reason> <path>` naming the
   first such path in byte order. `corvint test` turns that verdict into a FULL scope, which runs
   every declared language suite, so every test of every runner. The affected plan itself keeps
   its `UNOWNED_DIRTY_PATH` entries and scope `UNKNOWN`, which name the path.
2. Order of the per-path rules, first match wins:
   1. the two private ledgers add nothing (`private-ledger`);
   2. the flow file and link records under `.corvint/` (the declared intent directory, when it lies
      under `.corvint/`) add no full run (`flow-linked`): the flow join of `corvint affected`
      (APK-V0-004) selects the linked tests. When the plan reports the flow declaration invalid
      (`FLOWS_DECLARATION_INVALID` in `plan.e2e.unknown`, or `E2E_FLOWS_INVALID`), or names no flow
      join for a changed `.corvint/flows.json`, the changed flows cannot be computed and the path
      selects everything (`flows-unreadable`);
   3. a declared documentation path under `.corvint/` keeps `DECLARED_DOCUMENTATION`; every other
      path under `.corvint/` selects everything (`corvint-config`), the map file and
      `.corvint/test-read-scopes.json` included;
   4. a path on the never-narrow list (rule 4) selects everything (`never-narrowed-path`), or
      `runner-removed` when the path is runner configuration the change deleted;
   5. a declared documentation path adds nothing (`DECLARED_DOCUMENTATION`; the existing
      `documentation <path>` audit line);
   6. a path the plan reports as `UNOWNED_DIRTY_PATH` selects everything unless a `proven` entry
      covers it: `non-code-unmapped` when a map entry covers it or a Go package directory encloses
      it (a runner owns the tree, but no proven entry names its readers), else
      `UNOWNED_DIRTY_PATH` (no runner owns it);
   7. every other path is code, or non-code data whose every possible reader the plan already
      proves selected (`goReadData`, AFP-V0-600), or a path an existing reviewed declaration
      classifies (`DECLARED_RUNTIME_READERS`, `DECLARED_NON_SOURCE`, `DECLARED_TEST_OWNER`,
      `TASK_STORE_PROJECTION`); it keeps the existing AFP-V0-012 attribution. Whether those
      hand-written declarations fold into the map is the design's second open question, which the
      owner keeps (proposal, Impact).
3. A runner the plan reports unavailable (`provider.go.state` neither `RUNNABLE` nor
   `EMPTY_SELECTION`) selects everything with `runner-removed go: provider.go.state is <state>`.
4. The never-narrow list (`noncodemap.NeverNarrowed`), matched on the slash path; a base-name rule
   matches at any depth:
   - lockfiles and module files: `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `package-lock.json`,
     `npm-shrinkwrap.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `Gemfile.lock`,
     `poetry.lock`, `Pipfile.lock`, `uv.lock`, `pdm.lock`, `Cargo.lock`, `composer.lock`;
   - toolchain pins: `.go-version`, `.tool-versions`, `.nvmrc`, `.node-version`,
     `.python-version`, `.ruby-version`, `rust-toolchain`, `rust-toolchain.toml`, `mise.toml`,
     `.mise.toml`;
   - `.github/**`;
   - make files: `Makefile`, `makefile`, `GNUmakefile`, `*.mk`;
   - runner configuration: `package.json`, `tsconfig.json`, `tsconfig.*.json`, `vitest.config.*`,
     `vitest.workspace.*`, `vite.config.*`, `jest.config.*`, `playwright.config.*`, `pytest.ini`,
     `pyproject.toml`, `setup.cfg`, `tox.ini`, `conftest.py`, `Gemfile`, `.rspec`, `Rakefile`;
   - release configuration: `.goreleaser.yml`, `.goreleaser.yaml`,
     `tools/release-binary/binaries.json`;
   - generator inputs: `*.proto`, `buf.yaml`, `buf.gen.yaml`, `sqlc.yaml`, `sqlc.yml`,
     `gqlgen.yml`;
   - `go:embed` inputs: a file a Go package of the selector's index embeds; build-tag inputs are Go
     sources, which are code and keep the Go rules;
   - snapshot update paths: anything under a `__snapshots__` directory, `*.snap`, `*.golden`;
   - the map file `.corvint/data-map.json` and `.corvint/test-read-scopes.json` (decided first by
     rule 2.3 as `corvint-config`);
   - secret and environment configuration: `.env`, `.env.*`, `.envrc`, `config/credentials.yml.enc`,
     `config/master.key`, `config/secrets.yml`.
5. Map status, first match wins: `invalid` never reaches status (the reader refuses the map);
   `proposed` when `proof`, `accepted_by`, `accepted_at` or `pinned_commit` is missing; `ignored`
   when the entry's path or glob is on the never-narrow list; `stale` when a cited test is gone or
   `pinned_commit` is not an ancestor of the head (or cannot be read); `unproven` when no FIH-V1
   resolver confirms that `proof` names a non-code fault case in the mapped file that a test in
   `tests` killed; `proven` otherwise. Only `proven` narrows. V1-5424 wires no resolver, so every
   accepted entry with a proof is `unproven` with detail `fault case <proof> is unknown: FIH-V1 is
   not linked (V1-5405)`.
6. A missing map is an empty map: every non-code path runs everything (the design's rollback).
   A map that is not a regular file, larger than 256 KiB, malformed, or that carries an unknown
   profile or member, is refused with `data-map-invalid`: the selector falls back with
   `FALLBACK data-map-invalid: <detail>`, and `corvint affected --propose-data-map` exits 2. It never
   falls back to an empty map.
7. Proposal (NCM-V1-003). `corvint affected --propose-data-map RECORDED` reads a recorded-reads
   file (JSON lines `{"test": "...", "path": "..."}`, at most 8 MiB) and writes to stdout a
   `corvint-data-map/0` document holding one entry per read path, with the reading tests sorted,
   `evidence` naming the recording, and no `accepted_by`, `accepted_at`, `pinned_commit` or `proof`,
   so every proposed entry reads `proposed`. It writes no file; a person copies an entry into
   `.corvint/data-map.json` and adds the acceptance and proof.

## Orchestrator decision on the design's open question 1

OWNER-delegated orchestrator call (2026-11-11, recorded by lane ncm under V1-5424): for 1.0,
`.corvint/changes/`, `.corvint/deferred-intents/`, the committed inventory file and the
`.corvint/unplanned-reads.enabled` marker stay configuration: a change to any of them selects
everything with reason `corvint-config`. The design's second open question, folding the
hand-written read declarations into the map, stays open for the owner and is out of scope here.

## Supersession

NCM-V1-005 lists `.corvint/test-read-scopes.json` as never narrowed, so the selector no longer
narrows an edit of that declaration against the plan's base (AFP-V0-050); the edit now runs
everything with `corvint-config`. Rule (c) narrowing of the other dirty paths is unchanged.

## Surfaces

- `internal/noncodemap`: `Parse`, `Read`, `Evaluate`, `NeverNarrowed`, `Decide`, `Propose`.
- `internal/gateselect`: `selectPackagesFrom` applies `Decide` to every plan; `AllowedUnknown` no
  longer admits `UNOWNED_DIRTY_PATH`.
- `corvint affected --propose-data-map RECORDED`.

## Delivery state

- Delivered under V1-5424: NCM-V1-001, 002, 003, 005, 006, 007, 008 and 009; the refusal side of
  NCM-V1-004 (no entry narrows without a confirmed fault case).
- Waiting: NCM-V1-004's proof link to FIH-V1 non-code fault cases, after V1-5405 lands (change task 6).
- Acceptance fixtures (change task 7): see the build-log entry
  `docs/build-log/2026-11-11-non-code-map.md` for the fixtures delivered and missing.

## Traceability

| Requirement | Tests (package) |
| --- | --- |
| NCM-V1-001 | `TestNCMV1001UnmappedFixtureSelectsEverything`, `TestNCMV1001UnownedPathSelectsEverything`, `TestNCMV1001DeclaredDocumentationAddsNothing`, `TestNCMV1001RemovedRunnerSelectsEverything` (`internal/noncodemap`); `TestNCMV1SelectorUnownedFallsBack`, `TestNCMV1SelectorUnavailableRunner`, `TestNCMV1001UnownedOffAllowlist` (`internal/gateselect`) |
| NCM-V1-002 | `TestNCMV1002MissingProofIsProposed` (`internal/noncodemap`) |
| NCM-V1-003 | `TestNCMV1003ProposalWritesStdoutOnly` (`internal/noncodemap`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`) |
| NCM-V1-004 | `TestNCMV1004UnprovenEntryDoesNotNarrow` (`internal/noncodemap`) |
| NCM-V1-005 | `TestNCMV1005NeverNarrowedPaths`, `TestNCMV1005LockfileIgnoresEntry` (`internal/noncodemap`); `TestNCMV1SelectorNeverNarrowed` (`internal/gateselect`) |
| NCM-V1-006 | `TestNCMV1006CorvintConfigSelectsEverything`, `TestNCMV1006FlowFileAddsLinkedTestsOnly`, `TestNCMV1006LedgerAppendAddsNothing` (`internal/noncodemap`); `TestNCMV1SelectorCorvintConfig`, `TestNCMV1SelectorLedgerAndFlow` (`internal/gateselect`) |
| NCM-V1-007 | `TestNCMV1007RefusesUnknownVersion`, `TestNCMV1007RefusesUnknownMember`, `TestNCMV1007RefusesMalformedOrOversized` (`internal/noncodemap`); `TestNCMV1SelectorInvalidMap` (`internal/gateselect`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`, exit 2) |
| NCM-V1-008 | `TestNCMV1008StaleEntryDoesNotNarrow` (`internal/noncodemap`) |
| NCM-V1-009 | `TestNCMV1009RepeatedReadIsByteIdentical`, `TestNCMV1009DecisionIsDeterministic` (`internal/noncodemap`) |
