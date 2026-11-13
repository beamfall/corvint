# Non-code map V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/non-code-map/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/non-code-map/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-002, PROD-TEST-008, PROD-TEST-012, PROD-TEST-019) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner answers G2, G3, G15b and G16,
owner decisions R31 and R32).

## Agent digest
- Claim: A changed non-code file runs every test unless a proven, accepted map entry narrows it; some paths always run everything.
- Status: accepted / partial. Change `non-code-map` auto-accepted under G14 (decision 0596); the FIH-V1 proof link waits for V1-5405.
- Exists: `internal/noncodemap` (strict `corvint-data-map/0` reader, entry status, never-narrow list, per-path decision, proposal from recorded reads); the AFP-V0-012 selector prints the decision and falls back to a full run for a path no runner attributes (`unowned-path`); a Go-attributed path stays owned and narrows (orchestrator ruling 2026-11-11).
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

A changed non-code path that no runner attributes and no proven, accepted entry covers MUST select
every test of every runner in the repository, with reason `unowned-path` and the path printed. A
non-code path a Go rule already attributes is owned and MUST NOT force a full run: it selects its
attribution (the enclosing Go package directory owns the file, so its tests and dependents are
selected; go.mod/go.sum readers; a package's testdata; a declared Go reader). A runner that was
removed or is unavailable MUST select all tests, with reason `runner-removed` and the runner or path
printed. Paths declared as documentation keep the `DECLARED_DOCUMENTATION` class and add no test.
(G15b, G2; owner decision R31 (2026-11-09); owner decision R32 (2026-11-09); orchestrator ruling
2026-11-11 under the owner's 2026-11-02 delegation, recorded in
`docs/build-log/2026-11-11-ncm-owned-paths.md`)

Earlier wording, superseded by the 2026-11-11 ruling: "A changed non-code path without a proven,
accepted entry MUST select every test of every runner in the repository, with reason
`non-code-unmapped` and the path printed. A path no runner owns [...] MUST likewise select all
tests, with a named reason (`UNOWNED_DIRTY_PATH` or `runner-removed`) [...]; `UNOWNED_DIRTY_PATH`
no longer narrows through the allowlist in `internal/gateselect/allowlist.go`."

**Scenario: Fixture change, no entry**
- GIVEN a change to `tests/fixtures/users.json` that no Go package encloses or names, no runner
  owns, and no map entry covers
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected and the reason names the path

**Scenario: Unowned path**
- GIVEN a change to `tools/notes.xyz` that no runner attributes and no map entry covers
- WHEN `corvint affected --base FULL_SHA` runs
- THEN every test is selected with reason `unowned-path` naming the path

**Scenario: Data beside a package**
- GIVEN a change to `internal/app/data.json`, which the Go package `internal/app` encloses
- WHEN `corvint affected --base FULL_SHA` runs
- THEN `internal/app` and its dependents are selected and no full run is added for that path

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
configuration; generator inputs; build-tag inputs; snapshot update paths; the map file itself;
`.corvint/test-read-scopes.json`; secret and environment configuration. A `go:embed` input is owned
by its embedding package (NCM-V1-001) and selects that package and its dependents. (G15b;
orchestrator ruling 2026-11-11) No map entry narrows `.corvint/test-read-scopes.json`; NCM-V1-006
alone decides it, and keeps the selector's own entry-by-entry attribution only when that
attribution is provable (owner-delegated amendment 2026-11-13, V1-5465).

Earlier wording, superseded by the 2026-11-11 ruling, listed "`go:embed` and build-tag inputs"
together.

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
test; and a sealed CEM directly under `.corvint/changes/` named by a 40- or 64-digit lowercase
hexadecimal commit, which is change evidence, not configuration, and MUST keep the selector's
ADN-V0-011 evidence attribution (its planned readers, else rule (c)). Any other name under
`changes/` stays configuration. The read-scope declaration `.corvint/test-read-scopes.json` and its
literal exemptions `.corvint/test-read-scope-literals.json` are configuration too, but a change the
selector can attribute MUST keep the selector's attribution, printed `read-scope-declaration` with
the path: for the declaration, when the current declaration is valid and the plan's base
declaration is known, only the packages whose entries were added, changed or removed, those whose
declared scope holds the declaration and the declaration's own readers (AFP-V0-050, AFP-V0-9712);
for the literal exemptions, which only the declaration tools read, when the current file is absent
or a valid record, its readers (AFP-V0-9713). A change it cannot attribute MUST select everything
with reason `corvint-config`. (G16, narrowed by owner decision R31 (2026-11-09); sealed CEMs
added 2026-11-12, `docs/build-log/2026-11-12-ncm-sealed-cem-evidence.md`; read-scope declarations
added 2026-11-13 as an owner-delegated amendment under the owner's 2026-11-12 decision that filed
V1-5465, `docs/build-log/2026-11-13-selwalk1113.md`)

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

**Scenario: One read-scope entry changed**
- GIVEN a change to one package's entries in `.corvint/test-read-scopes.json` and a known plan base
- WHEN the plan is computed for that change
- THEN that package and the declaration's own readers are selected, with the line
  `noncode read-scope-declaration .corvint/test-read-scopes.json`, and with no base every test is
  selected with reason `corvint-config`

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
   its `UNOWNED_DIRTY_PATH` entries and scope `UNKNOWN`, which name the path. A root module
   definition (`go.mod`, `go.sum`, `go.work`, `go.work.sum`) keeps the selector's earlier
   `FALLBACK root module definition is dirty: <path>`, which runs everything first.
2. Order of the per-path rules, first match wins:
   1. the two private ledgers add nothing (`private-ledger`);
   2. the flow file and link records under `.corvint/` (the declared intent directory, when it lies
      under `.corvint/`) add no full run (`flow-linked`): the flow join of `corvint affected`
      (APK-V0-004) selects the linked tests. When the plan reports the flow declaration invalid
      (`FLOWS_DECLARATION_INVALID` in `plan.e2e.unknown`, or `E2E_FLOWS_INVALID`), or names no flow
      join for a changed flow file (`.corvint/flows.json` or the deprecated `.corvint/e2e-flows.json`),
      the changed flows cannot be computed and the path selects everything (`flows-unreadable`);
   3. a declared documentation path under `.corvint/` keeps `DECLARED_DOCUMENTATION`; a read-scope
      declaration edit the selector can attribute keeps its attribution (`read-scope-declaration`,
      no full run); every other path under `.corvint/` selects everything (`corvint-config`), the
      map file and an unattributable read-scope declaration edit included;
   4. a path on the never-narrow list (rule 4) selects everything (`never-narrowed-path`), or
      `runner-removed` when the path is runner configuration the change deleted;
   5. a declared documentation path adds nothing (`DECLARED_DOCUMENTATION`; the existing
      `documentation <path>` audit line);
   6. a path the plan reports as `UNOWNED_DIRTY_PATH` narrows to a `proven` entry's tests when one
      covers it (`proven-entry`); otherwise it is owned, gets no line and keeps the AFP-V0-012
      attribution when a Go rule attributes it; otherwise it selects everything (`unowned-path`).
      Go attribution (`repositoryIndex.attributed`) is: a Go package directory encloses the path
      (rule (b): the enclosing packages, the nearest one's dependents and every embedding
      ancestor's dependents, which covers a package's testdata, data beside it and its go:embed
      inputs); a package's files name it (rule (c)); a declared read scope bears on it
      (AFP-V0-023); or the plan selects a Go unit for it with a declared-reader witness
      (`DECLARED_EXISTENCE_READER`, AFP-V0-980, or `DECLARED_READ_SCOPE`). A map entry that covers
      an attributed path but is not `proven` is still reported on its `data-map` line. Because the
      selector, which holds the Go index, decides this, `UNOWNED_DIRTY_PATH` stays on the AFP-V0-982
      allowlist that `corvint test`, the CI selector and the line-evidence plan share: wherever the
      selector decides the run, an admitted entry is an attributed one. Ownership is the full
      plan's: the qualified selector's Go-only re-observation may report another plugin's source
      (a Playwright spec) as `UNOWNED_DIRTY_PATH`, which the allowlist admits, so it adds nothing;
      nothing is set aside, so no observation's uncertainty is dropped;
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
   - `go:embed` inputs are not on the list: rule 2.6 attributes a non-Go file below a package to
     the enclosing packages and the dependents of every embedding ancestor. The index records that
     a package embeds, not which files, so every non-Go file below an embedding package is
     attributed to the whole package and its dependents: a sound over-approximation of the exact
     embed patterns, which would need a gateselect scan-memo profile bump to record. Build-tag
     inputs are Go sources, which are code and keep the Go rules;
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
   falls back to an empty map. An invalid map therefore forces that fallback, so a full run, on
   every plan of the repository, whatever its dirty paths, until the map is repaired: fail-closed
   by design (review P3-1).
7. Proposal (NCM-V1-003). `corvint affected --propose-data-map RECORDED` reads a recorded-reads
   file (JSON lines `{"test": "...", "path": "..."}`, at most 8 MiB) and writes to stdout a
   `corvint-data-map/0` document holding one entry per read path, with the reading tests sorted,
   `evidence` naming the recording, and no `accepted_by`, `accepted_at`, `pinned_commit` or `proof`,
   so every proposed entry reads `proposed`. It writes no file; a person copies an entry into
   `.corvint/data-map.json` and adds the acceptance and proof.
8. A full run the decision names keeps its reason: `SelectQualified` returns the selector's
   `FALLBACK <reason> <path>` (`noncodemap.NamesFullRun`) before its generic input checks, which
   would otherwise replace it with another full run such as `the selection input has no supported
   scope`.

## Orchestrator decision on the design's open question 1

OWNER-delegated orchestrator call (2026-11-11, recorded by lane ncm under V1-5424): for 1.0,
`.corvint/changes/`, `.corvint/deferred-intents/`, the committed inventory file and the
`.corvint/unplanned-reads.enabled` marker stay configuration: a change to any of them selects
everything with reason `corvint-config`. The design's second open question, folding the
hand-written read declarations into the map, stays open for the owner and is out of scope here.

## Orchestrator ruling on owned paths

Orchestrator decision (2026-11-11, under the owner's 2026-11-02 delegation; the same ruling was given
to lane rnc; recorded in `docs/build-log/2026-11-11-ncm-owned-paths.md`): a non-code path that a Go
rule already attributes is owned and must not force a full run. Go attribution means the enclosing Go
package directory owns the file (its tests and dependents are selected, as `select.go` already
does), go.mod/go.sum readers, a package's testdata, and declared readers. A full run with reason
`unowned-path` follows only when no runner attributes the path at all. NCM-V1-001, NCM-V1-005 (its
go:embed item) and rule 2.6 carry it; the `non-code-unmapped` reason is retired.

## Supersession

The selector now runs everything where these earlier rules narrowed; each test that asserted the
narrowing asserts the named full run instead, and keeps covering the attribution where
`packageBounds` (tools/gate-ledger) still applies it:

- An edit of `.corvint/test-read-scopes.json` was narrowed against the plan's base by rule
  AFP-V0-050; it ran everything (`corvint-config`) from V1-5424 until V1-5465, which restores the
  narrowing when the base declaration is known (`read-scope-declaration`, AFP-V0-9712) and keeps
  the full run otherwise.
- A sealed CEM under `.corvint/changes/` and the retired sidecar `.corvint/change.cem.json` were
  narrowed to their resolving readers by rules AFP-V0-152, AFP-V0-2744, AFP-V0-2746, AFP-V0-3423
  and ADN-V0-011; both now run everything (`corvint-config`, the orchestrator's call above), and
  build-log entries keep their narrowing.
- An edit of `.corvint/gated-root-locators.json` was narrowed by the proof of rule TSP-V0-608; it
  now runs everything (`corvint-config`).
- A Go package under `.github/` was narrowed by rules AZU-V0-005, AZU-V0-9920 and AFP-V0-9012; it
  now runs everything (`never-narrowed-path`).
- An `UNOWNED_DIRTY_PATH` no Go rule attributes now runs everything (`unowned-path`): an unclaimed
  tracked addition outside every package (AZU-V0-9900) is the case a test asserts. Under the
  orchestrator ruling above, the attributed cases of rules CIN-V0-860 (V1-3480), AFP-V0-9013,
  AFP-V0-9310, AFP-V0-980 and AFP-V0-2377 (a hosted fixture under a package's testdata, a deleted
  manifest-named file a declared existence reader reads, a data file beside a package, an embedded
  data file) keep their narrowing, and so does a Go assembly file the plan leaves unowned
  (TSP-V0-006, case "an assembly file"), which its package directory encloses.

## Surfaces

- `internal/noncodemap`: `Parse`, `Read`, `Evaluate`, `NeverNarrowed`, `Decide`, `Propose`.
- `internal/gateselect`: `selectPackagesFrom` applies `Decide` to every plan, with
  `repositoryIndex.attributed` and the plan's declared-reader witnesses as Go attribution;
  `AllowedUnknown` keeps admitting `UNOWNED_DIRTY_PATH`; `SelectQualified` keeps a named NCM full
  run's reason.
- `corvint affected --propose-data-map RECORDED`.

## Delivery state

- Delivered under V1-5424: NCM-V1-001, 002, 003, 005, 006, 007, 008 and 009; the refusal side of
  NCM-V1-004 (no entry narrows without a confirmed fault case).
- Waiting: NCM-V1-004's proof link to FIH-V1 non-code fault cases, after V1-5405 lands (change task 6).
- Acceptance fixtures (change task 7): see the build-log entry
  `docs/build-log/2026-11-11-non-code-map.md` for the fixtures delivered and missing.

## Traceability

| Requirement | Evidence (package) |
| --- | --- |
| NCM-V1-001 | `TestNCMV1001UnmappedFixtureSelectsEverything`, `TestNCMV1001AttributedPathStaysOwned`, `TestNCMV1001UnownedPathSelectsEverything`, `TestNCMV1001DeclaredDocumentationAddsNothing` (also inside an embedding package), `TestNCMV1001RemovedRunnerSelectsEverything` (`internal/noncodemap`); `TestNCMV1SelectorUnownedFallsBack`, `TestNCMV1SelectorEmbeddedInputNarrowsToItsPackage`, `TestNCMV1SelectorUnavailableRunner`, `TestNCMV1001UnownedAttributedOnAllowlist`, `TestNCMV1QualifiedKeepsThePlansOwnership`, `TestNCMV1QualifiedKeepsTheNamedReason`, `TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860` (`internal/gateselect`); `TestDeletedManifestNameSelectsExistenceReaderCLI_AFPV0980`, `TestOwnershipNamedDryRun_AZU_V0_9900` (`cmd/corvint`) |
| NCM-V1-002 | `TestNCMV1002MissingProofIsProposed` (`internal/noncodemap`) |
| NCM-V1-003 | `TestNCMV1003ProposalWritesStdoutOnly` (`internal/noncodemap`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`) |
| NCM-V1-004 | `TestNCMV1004UnprovenEntryDoesNotNarrow` (`internal/noncodemap`) |
| NCM-V1-005 | `TestNCMV1005NeverNarrowedPaths` (go:embed input not listed), `TestNCMV1005LockfileIgnoresEntry` (`internal/noncodemap`); `TestNCMV1SelectorNeverNarrowed`, `TestNCMV1SelectorEmbeddedInputNarrowsToItsPackage` (`internal/gateselect`) |
| NCM-V1-006 | `TestNCMV1006CorvintConfigSelectsEverything`, `TestNCMV1006FlowFileAddsLinkedTestsOnly` (both flow files unjoined), `TestNCMV1006LedgerAppendAddsNothing`, `TestNCMV1006ReadScopeDeclarationNarrowsWhenAttributable_V15465` (`internal/noncodemap`); `TestNCMV1SelectorCorvintConfig`, `TestNCMV1SelectorLedgerAndFlow`, `TestSelectPackagesNarrowsDeclarationEditsAgainstTheBase_AFPV0050`, `TestReadScopeDeclarationNarrowingFaults_AFPV09712`, `TestReadScopeLiteralsNarrowToReaders_AFPV09713` (`internal/gateselect`); `TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050` (`tools/gate-affected-select`) |
| NCM-V1-007 | `TestNCMV1007RefusesUnknownVersion`, `TestNCMV1007RefusesUnknownMember`, `TestNCMV1007RefusesMalformedOrOversized` (`internal/noncodemap`); `TestNCMV1SelectorInvalidMap` (`internal/gateselect`); `TestNCMV1003ProposeDataMapCommand` (`cmd/corvint`, exit 2) |
| NCM-V1-008 | `TestNCMV1008StaleEntryDoesNotNarrow` (`internal/noncodemap`); `TestNCMV1SelectorInvalidMap`, `TestNCMV1SelectorConfirmsCitedTests` (`internal/gateselect`) |
| NCM-V1-009 | `TestNCMV1009RepeatedReadIsByteIdentical`, `TestNCMV1009DecisionIsDeterministic` (`internal/noncodemap`) |
