# Corvint 1.0 product plan

Owner: Russell Lewis (repository owner). This file proposes a plan. It accepts nothing.
Date: 2026-11-09 (revision 3: folds in the owner answers of 2026-11-09, answers 1 to 18, G1 to G27,
the review-round decisions R1 to R41 and the final-round delegation R42; owner decisions H1 to
H12 of 2026-11-10 are applied in place)
Intent status: proposed
Delivery status: experimental

## Agent digest
- Claim: Proposed 1.0 product contract of 77 requirements for testing, documentation, task manager, integration, release, a GitHub Action and agent packages.
- Status: proposed / experimental. The owner has answered the questions; the plan itself is not yet accepted. Acceptance is one decision record (answer 18, owner decision R1 (2026-11-09)).
- Exists: this file, change folders under `docs/specs/openspec/changes/` (index in `docs/specs/openspec/README.md`), the capability specs it cites.
- Blocked on: the owner acceptance decision, and the open items in the owner acceptance checklist.
- Read next: "The 1.0 bar", then "Milestones and order", then "Proposal dependency table", then the checklist at the end.

## Owner goals

These are the owner's words. They are the authority for this plan. Everything else in this file
is a reading of them and of the owner answers, and is open to correction.

The bar for 1.0:

> I want to have a successful 1.0 launch that a) has the testing functionality we have been
> developing working perfectly b) has the documentation functionality we have been developing
> working perfectly c) has the task manager functionality we have been developing working
> perfectly — everything should be perfectly integrated and structured and we should be setup for
> success

What each product covers:

> documentation (mcp, auto docs generation, flows, E2E mapping)

> testing (e2e to code, test running etc)

> task management

The testing target:

> I have a code base. That code base has a huge selection of tests. I want to be able to a) only
> run the tests that are required for a change b) those tests could be unit, integration and e2e
> c) e2e tests would be tied to flows, which are features of the code so I would know that if I am
> changing code for a feature, that will require some flows to be run to test it works d) flows
> are built up with variants. These are the multiple use cases within a flow which have tests
> attached to them. All this should be managed by corvint, easy to track and reportable by the
> MCP and cli. also, it should have functionality like wallaby.js so we can instantly know what
> tests need to be run at any time to check code changes

The plan itself:

> you should rework the specs so that corvint has a solid plan again

> we should make sure we are taking advantage of flows in OpenSpec to ensure we are doing this in
> an industry standard way

> we really want to make sure we use OpenSpec and SDD to make sure we keep on top of everything.
> everything should be documented before being built

Later instructions on 2026-11-09:

> we want to make sure we have a world class and full corvint integrated CI with github actions as
> well. make sure that is planned for

> we also want to make sure that we have support for claude, codex, opencode and pi as part of this
> plan. the support should be world class and industry leading

> also, I want you to look at everything else in source. and if we are not confirming its use here
> are making sure its something we want to keep. and if not, making a plan to remove it

One word differs between the owner's text and the code. The owner says "variants". Corvint's flow
files call the same thing a "variation" (`variation_id`, `internal/appflows/types.go`). This plan
uses "variation" so that it matches the files.

## Owner answers folded into this revision

The owner answered 18 plan questions and 27 expert-panel questions (G1 to G27) on 2026-11-09. The
record is the owner-answers file of that date kept with the planning material; it is not in this
repository. This plan cites each answer as "answer N" or "GN". Where a later answer supersedes an
earlier one, the later one governs and the requirement says so.

| Answer | Subject | Where it lands |
|---|---|---|
| 1 | Watch save-to-answer limit 1 second | PROD-TEST-007 |
| 2, G1 | Blocking runners: Go, Vitest, Jest, Playwright, pytest, RSpec, Minitest; Cypress conditional (Cypress dropped by R6) | The 1.0 bar, PROD-TEST-006, PROD-TEST-011 |
| 3, G2 | Line-level narrowing for unit and integration; browser E2E narrowed by flows plus reachability | PROD-TEST-004, PROD-TEST-008 |
| 4 | Pinned E2E inventory written by a separate step; `affected` only reads it | PROD-TEST-017 |
| 5, G3 | Zero-miss proof by fault injection; replay reports selection size only | PROD-TEST-002, PROD-TEST-014, PROD-TEST-015 |
| 6 | CI narrowing through GitHub Actions only, kill switch, periodic full runs | PROD-TEST-012, GitHub Actions section |
| 7 | Generated docs are evidence-cited drafts a person accepts | PROD-DOCS-002 |
| 8 | Docs scope: drift detection, run evidence, `corvint how` (spec rewritten), docs search | PROD-DOCS-003, 007, 008, 009 |
| 9 | Covered variation = linked test of any kind | PROD-TEST-005 |
| 10 | Corvint proposes flows, a person accepts; unaccepted flows never drive selection | PROD-DOCS-004, PROD-DOCS-012 |
| 11 | Tasks serves its own MCP; `corvint mcp` has no tasks tools (Tasks MCP superseded by R17) | PROD-TASK-007 |
| 12, G9 | Beamfall takeover stays a 1.0 condition; coupled release, 21-day trial | PROD-TASK-009 to 013 |
| 13 | Loose integration through files and IDs; no shared library across repositories | PROD-INT-004 |
| 14 | Zero layer violations at 1.0; flow reader split | PROD-INT-001, PROD-INT-002 |
| 15 | Flow change requires all linked tests; zero-E2E flow reported, not blocking | PROD-TEST-005 |
| 16 | Platforms Linux and macOS; Windows through WSL only | PROD-INT-015 |
| 17, G12 | Pinned public qualification repositories, one per runner, plus four held-out | PROD-INT-008 |
| 18 | One decision record accepts the plan and supersedes the pre-cut 1.0 contract | Owner acceptance checklist |
| G4 | Tiered narrowing bar | PROD-TEST-016 |
| G5 | Flow reports that join test evidence are native in `corvint` | PROD-DOCS-011, PROD-INT-016 |
| G6 | One flow file `.corvint/flows.json`; flow selects, variation shown | PROD-TEST-004, PROD-DOCS-004 |
| G7 | Watch gated for all seven runners | PROD-TEST-007 |
| G8 | CEM, OCM and dogfood ceremony replaced; `corvint cem` removed | PROD-INT-009, PROD-INT-017, "Removed from 1.0" |
| G10, G10a | Definition of done, freeze 2026-12-15, stable no earlier than 2027-01-05 | PROD-INT-012, PROD-INT-013 |
| G11 | Unlinked E2E tests always run; links proposed and accepted | PROD-TEST-018, PROD-DOCS-012 |
| G13 | Test kind enum and kind source | PROD-TEST-003 |
| G14 | Accept the plan once; auto-accept implementing proposals; OpenSpec change folders (its "no npm tool, no `openspec/` root" clause amended by H12) | PROD-INT-011 |
| G15a | Tag-started release with owner approval in a protected environment | PROD-INT-014 |
| G15b | Non-code files select every possible reader unless a proven map entry narrows | PROD-TEST-019 |
| G16 to G18 | GitHub Action in `beamfall/corvint-action`; Corvint's own CI cut over after shadow | PROD-INT-018 to 022 |
| G19 to G21 | Agent packages in `beamfall/corvint-agents`; Claude Code and Codex block | PROD-INT-023 to 027 |
| G22 to G24, G26, G27 | Source removals and process tooling | "Removed from 1.0" |
| G25 | Revived `impact`, `review`, `context`; `feature`, `features`, `overview` fold into flows | PROD-INT-028 to 031, PROD-DOCS-013 |

Review-round decisions. After an expert review of revision 2 the owner answered 38 review
questions and three extra points on 2026-11-09. This plan cites each as "owner decision Rn
(2026-11-09)", or "Rn" for short. They are recorded in the same owner-answers file.

| Decision | Subject | Where it lands |
|---|---|---|
| R1 | One acceptance record accepts the plan as amended; it names the drafted wording; in-flight work continues as experimental | Agent digest, "Work in flight", checklist |
| R2 | Qualify before the freeze: M5a candidate tag about 2026-12-08, rebuilt only for blockers; stable 2027-01-05 | The 1.0 bar, PROD-INT-012, PROD-INT-013, milestones |
| R3 | Milestone M0 re-scopes the v1-0 release object; Tasks policy v15 without cemRequired, ocmRequired, cem-verdict and context-focused | Milestones, PROD-INT-017 |
| R4 | Docs review advisory by default in blocking consumers, with an opt-in setting that makes it blocking | PROD-INT-029 |
| R5 | Blocking Stop decision with a reason, at most one nudge per turn; SessionStart(source=compact) instead of PostCompact | PROD-INT-024 |
| R6 | Cypress dropped from 1.0, listed post-1.0; seven blocking runners | The 1.0 bar, PROD-TEST-006, "Removed from 1.0" |
| R7 | Seven-runner set confirmed; PROD-TEST-020 append-only run result record; Jest gets its own provider proposal | PROD-TEST-006, PROD-TEST-020 |
| R8 | Line-exact for Go, RSpec, Minitest, Vitest, Jest; pytest selects by file with a tier label | The 1.0 bar, PROD-TEST-008 |
| R9 | Python file-level selection is a release exception in the acceptance record; Python line collector is a 1.0.x ticket | The 1.0 bar, PROD-TEST-008 |
| R10 | One committed inventory under `.corvint/` for all seven runners, written by `corvint inventory refresh`; missing or stale runs all tests | PROD-TEST-017 |
| R11 | Canonical test ID (RNC-V1-012) with fresh requirement IDs | PROD-TEST-006 |
| R12 | Flow links proposed with a confidence score, accepted by a person only | PROD-TEST-018, PROD-DOCS-012 |
| R13 | Unknown `CORVINT_SELECTION` value exits 2 in CLI and CI, runs all tests in hooks and MCP; grammar `full:<runner>` everywhere | The 1.0 bar, PROD-TEST-013, PROD-INT-019 |
| R14 | Provider binaries retire after their built-in runner contract lands (G23); not 1.0 artifacts | PROD-TEST-006, PROD-INT-006, "Removed from 1.0" |
| R15 | Several instances per runner keyed by root directory; sharding in the Action only | PROD-TEST-006, PROD-INT-021 |
| R16 | One tier vocabulary `line`, `file`, `flow`, `full` plus `exact`; `flow` is not counted as narrowed for the no-miss claim | The 1.0 bar, PROD-TEST-007, PROD-TEST-008 |
| R17 | PROD-TASK-001 to 009 confirmed; Tasks publishes schema v1 and a supported-command list; CLI only, no Tasks MCP in 1.0 | Tasks section, PROD-TASK-007, PROD-TASK-010 |
| R18 | Tasks trial and conformance subject is `beamfall/core` | PROD-TASK-009, PROD-TASK-010 |
| R19 | 100 planted faults per held-out repository, at most 3 misses with a Wilson interval, at most 5 percent of tests excluded, per-test durations in every receipt | PROD-TEST-016, PROD-INT-008 |
| R20 | PR agreement figure: share of soak-window PRs whose narrowed pass/fail equals full pass/fail on the same commit | PROD-INT-012 |
| R21 | Held-out repositories fastapi, solidus, payload, each verified against the licence and activity rules first. The fixed list is superseded by H3 | PROD-INT-008 |
| R22 | One plan artifact with a digest; shards filter by runner; a final job verifies every shard used it | PROD-INT-021, PROD-INT-022 |
| R23 | Narrow on pull_request and push; fork PRs run everything | PROD-INT-018, PROD-INT-020 |
| R24 | Actions pinned by SHA; contents read plus id-token write only for attestation; no PR checkout under pull_request_target | PROD-INT-018, PROD-INT-020 |
| R25 | `corvint-action` and `corvint-agents` are Apache-2.0; nothing changes in AGENTS.md | PROD-INT-018, PROD-INT-023 |
| R26 | Agent qualification keys and cap (superseded by R39 and R40) | PROD-INT-026 |
| R27 | One pinned model per agent host; model ID and agent version recorded | PROD-INT-026 |
| R28, R29 | `corvint context` ships non-blocking with no claim of measured benefit; measurement in 1.0.x | PROD-INT-030 |
| R30 | Docs tools on one `corvint-docs mcp` server; `corvint mcp` keeps testing tools; one tool table | PROD-DOCS-001, PROD-DOCS-007 |
| R31 | Only Corvint configuration files under `.corvint/` force a full run; a flows edit runs linked tests; doc-only edits and the private ledgers add nothing | PROD-TEST-019 |
| R32 | A change no runner owns, or a removed runner, runs all tests with a named reason; UNOWNED_DIRTY_PATH comes off the allowlist | PROD-TEST-011 |
| R33 | Public repository `beamfall/corvint-vscode`; orphaned code deleted and tracked in a table | "Removed from 1.0" |
| R34 | One build-log entry per removal batch | "Removed from 1.0", PROD-INT-001 |
| R35 | One proposal dependency table; every code citation anchored to a fixed commit | "Proposal dependency table" |
| R36 | `make windows-vet` once per removal batch, before that batch's push | PROD-INT-015, milestones |
| R37 | PRS-V1-018 (c) and (d) carry over, (a), (b), (e) drop; out-of-scope specs archived in place with health classes | PROD-INT-010, PROD-INT-011, conflicts |
| R38 | Testing build order: canonical test ID, inventory, run results, line evidence, flow links | "Proposal dependency table" |
| R39 | No API keys and no spend cap; agent qualification uses the owner's subscription logins on the owner's machines, never GitHub-hosted runners; the result is a release evidence file | PROD-INT-026, PROD-INT-027 |
| R40 | Codex qualification uses the logged-in Codex CLI | PROD-INT-026 |
| R41 | Every change uses the OpenSpec propose, apply, archive stages under `docs/specs/openspec/changes/`; old process documents cleaned up by `spec-set-cleanup`. H12 runs the stages with the OpenSpec command | PROD-INT-010, PROD-INT-011, "Where this file sits" |
| R42 | Final round: every still-open question takes the expert panel's recommendation as the decision (delegated picks A1 to A18, B1 to B10, C1 to C23, D1 to D17 with D10a, E1 to E10, F1, F2, N1 to N11, M1 to M12). Publishing, stable promotion, credentials, repository creation and repository visibility stay with the owner | Cited in place as "R42" with the pick's id; listed pick by pick in section 3 of `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` |
| H1 | Stable is not published before the latest of 2027-01-05, the end of the 21-day Tasks RC trial, the Corvint RC publish plus 21 days and the end of the 7-day soak | "The 1.0 bar", PROD-INT-012, PROD-INT-013, milestone row M9 |
| H2 | A regression found on the RC blocks, for each of the seven blocking runners; it never becomes a demotion | PROD-INT-012 |
| H3 | One held-out repository per family (Go, JS/TS, Python, Ruby), drawn from a sealed pool seeded by the first candidate tag's commit SHA and never redrawn; R21's names may be pool candidates | PROD-INT-008, PROD-TEST-015 |
| H4 | `AGENTS.md` invariant 7 rewording accepted; invariant 8 not reworded and keeps "human-owned intent" | "Removed from 1.0" rows G8 and G22; "Corrections to the first draft" item 3; checklist row O35 |
| H5 | Every edit of `AGENTS.md` invariants 3, 4, 5, 7 and 8 is made by the change `acceptance-record` (ACR-V1-012) | Checklist row O35 |
| H6 | If the flow links miss 2026-12-04, M5a slips until they are signed; Playwright is not demoted | Milestone row M5a |
| H7 | M4 lands before the M5a tag | Milestone rows M4 and M5a |
| H8 | `impact` without `--base` uses the merge base of HEAD with the default branch, reports it, and refuses when it cannot determine one (low confidence) | PROD-INT-028 |
| H9 | `external-spec-not-verified` is an accepted status of the openspec check, reported and not failing | PROD-INT-011 |
| H10 | The acceptance record is signed without waiting for the removal branch; the 0594 Keeps-line row takes effect when that branch lands | "Conflicts with accepted text" |
| H11 | Per-merge macOS check advisory until the flake tickets close and 20 consecutive main runs pass, then required; still advisory per merge for 1.0 if not met by 2026-12-15; macOS required at release | PROD-INT-015, "Proposal dependency table" |
| H12 | Install OpenSpec and follow its flows: `docs/specs/openspec/` is the OpenSpec root, the command is pinned and `openspec validate --all` passes for every change folder | PROD-INT-011, "Where this file sits" |
| H13 | Keep flat specs: the canonical specs stay `docs/specs/*.md`; no `docs/specs/openspec/specs/` tree; `openspec archive` runs with `--skip-specs` | PROD-INT-011, "Where this file sits" |

## Where this file sits

`docs/SPEC-DRIVEN-DEVELOPMENT.md` sets an authority order: owner-accepted product intent first,
then the accepted capability spec, then tests and measured evidence, then the implementation, then
inference. It names four linked artifacts: a product contract, capability specs, an implementation
plan and an evidence record.

- This file is a proposed product contract for 1.0. It says what each product must do and why.
- The capability specs under `docs/specs/` keep the numbered requirements that say how. Each
  product requirement below names the capability requirements that implement it.
- The implementation plan for a change is its change proposal under
  `docs/specs/openspec/changes/<name>/` (G14; workflow proposed in
  `docs/decisions/PROPOSED-openspec-workflow.md`) and its ticket in `.taskman/`.
- The evidence record stays one file per entry under `docs/build-log/`.

Relation to the existing product contract. The accepted 1.0 contract today is
`docs/specs/corvint-1.0-product-and-release-v1.md` (prefix `PRS-V1`, decisions 0373 and 0426). It
predates decision 0594 and the owner answers. Answer 18 says one decision record accepts this plan
and supersedes that contract, carrying over the items that are still valid. Until that record
exists, the accepted text governs wherever the two differ. "Conflicts with accepted text" below
lists each difference and the resolution the acceptance record needs.

Acceptance mode after acceptance (G14). The owner accepts this plan once. A change proposal that
only implements an accepted `PROD` requirement is then auto-accepted and logged with its rationale.
A proposal that changes behaviour, a file or wire format, or the release bar waits for the owner.
1.0 acceptance, RC publication and stable promotion are carved out of the delegated-choices rule
(G10): only the owner signs them.

OpenSpec stages (owner instruction of 2026-11-09, owner decision R41 (2026-11-09)). Every change
uses the OpenSpec propose, apply and archive stages in its own folder under
`docs/specs/openspec/changes/`. Owner decision H12 (2026-11-10), the owner's words: "install
openspec and ensure we are following its flows for this". `docs/specs/openspec/` is an OpenSpec
root with `config.yaml` (schema `spec-driven`), and the OpenSpec command (`@fission-ai/openspec`,
pinned 1.17.0) is run from `docs/specs`; the repository root holds no `openspec/` directory. The
stages use `openspec new change`, `openspec validate`, `openspec status`, `openspec list` and
`openspec archive <name> --skip-specs`. The canonical specs stay the flat `docs/specs/*.md` files,
so archive skips the spec merge (owner decision H13 (2026-11-10): "Keep flat specs"). The planned
`openspec-check` change enforces the stages: `openspec validate --all` passes for every change
folder, with strict-mode warnings reported, not failing (OSC-V1-012). Its Go check needs no
OpenSpec command and the product build gains no npm dependency. The planned `spec-set-cleanup`
change gives every old process document and spec a verdict (keep, rewrite, archive, delete or move)
and leaves one workflow document, so that old process text cannot be mistaken for the current
workflow.

## How to read a requirement

Each requirement has the same fields.

- Source: the owner's words or owner answer it comes from, or `PROPOSED` when it is inferred beyond
  them. A `PROPOSED` item needs the owner's explicit yes before anything is built for it.
- Changed: present only on a requirement from the first draft whose statement or bar changed in
  this revision. It names the owner answer (answer N or GN) that changed it and says what it was.
- Intent: always `proposed` in this file.
- Delivery: the delivery status axis of `docs/SPEC-DRIVEN-DEVELOPMENT.md` (`not-started`,
  `experimental`, `implemented`, `validated`, `failed`, `deferred`).
- State: `DELIVERED`, `PARTIAL` or `MISSING`. `DELIVERED` means that code and tests for the whole
  statement exist on `origin/main`. It does not mean validated on outside repositories.
- Scenario: one concrete case in WHEN and THEN form.
- Implemented by: existing capability requirement IDs, or the change proposal that will hold them.
- Open tickets: tickets in `.taskman/` that deliver the rest.
- Evidence today: files that exist.
- Acceptance evidence at 1.0: what must be recorded before 1.0 can claim the requirement.

How the state was read. The first draft read files and spec headers at base commit `32140bb3b`
(branch `claude/product-specs`). This revision additionally read the unmerged integration branch
`claude/layers-integ` at commit `9ec3a29d0`, which carries the `oneplan`, `flowrep`, `misskill` and
watch lanes. A fact read only on that branch is marked "(integ)". Work that exists only on an
unmerged branch is never `DELIVERED`. No test was run for this plan. Counts of requirement IDs named
in test files are textual and do not prove that a test exercises the requirement.

## The 1.0 bar

This section states the bar in one place. The numbered requirements below carry each part.

Three products and their integration. Corvint 1.0 is three products that each work alone and join
through files and IDs (answer 13): testing (`corvint`), documentation (`corvint-docs`,
`corvint-docs-mcp`) and the task manager (`corvint-tasks`, separate repository and semver line,
coupled release, G9). Two adoption surfaces ship with them: a GitHub Action (G16 to G18) and agent
packages (G19 to G21).

Dates (G10a, as amended by owner decision R2 (2026-11-09)).
- Source removals: finish by 2026-12-01, before the qualification candidate.
- Qualification candidate (M5a): tagged about 2026-12-08. Qualification (fault injection, held-out
  repositories, agent qualification) runs on this candidate before the freeze. The candidate is
  rebuilt only for a blocker.
- Freeze: 2026-12-15. What is not proven by then ships as "preview" or runs the full suite with a
  printed reason. The owner records each demotion verdict before the freeze, from the qualification
  results on the M5a candidate.
- Release candidate: tagged first, then published after the owner's approval (M8).
- Stable (owner decision H1 (2026-11-10)): not published before the latest of 2027-01-05; the end
  of the 21-day Tasks RC trial (2026-12-08 to 2026-12-29, unless it restarts on a store format
  change under TCF-V1-009); the Corvint RC publish date plus 21 days; and the end of the 7-day soak
  on the frozen RC. Because 2026-12-15 plus 21 days is 2027-01-05, the earliest stable date holds
  only if the RC is published on the freeze date and no code change restarts a clock. A code
  change in the Corvint RC restarts the soak and the RC-publish clock, not the Tasks trial; a late
  batch (B8b, B11) restarts a clock only if it changes a shipped binary (REL-V1-003).

Seven blocking runners (G1, superseding answer 2's set; confirmed by owner decision R7
(2026-11-09)). Go (`go test`), Vitest, Jest, Playwright, pytest, RSpec and Minitest must select and
run for 1.0. Cypress is dropped from 1.0 and listed post-1.0 (owner decision R6 (2026-11-09)); it has
no change folder. Cypress and every other runner run in full with a stated reason. Every fallback is
printed in the run and in the release manifest.

Narrowing basis (answer 3 as amended by G2, owner decisions R8, R9 and R16 (2026-11-09)).
- Unit and integration tests: line-exact narrowing for Go, RSpec, Minitest, Vitest and Jest (R8).
  These five must be line-level or the release blocks.
- pytest selects by file in 1.0 and labels each selection with the `file` tier (R8). Python
  file-level selection is recorded as a release exception in the acceptance record; the Python line
  collector is a 1.0.x ticket (R9).
- Browser E2E tests (Playwright): removed from a selection only by accepted flow links plus static
  reachability at file and symbol level. Line evidence may add or rank a test, never remove one.
  Per-test line narrowing for E2E is an opt-in profile labelled experimental.
- Tiers (R16): every selection carries one tier from `line`, `file`, `flow` and `full`, plus the
  `exact` flag. A `flow`-tier selection is not counted as narrowed for the no-missed-test claim.
- Every selection prints a one-line reason.

Zero-miss proof (G3). Per blocking runner, at least 100 frozen, hashed fault-carrying PRs (about 30
bug-fix reverts, the rest changed-line mutants at 3 to 5 per PR) plus at least 30 non-code faults.
An unselected confirmed kill blocks the release and is never waived as a flake. Release wording:
"0 misses in N pre-registered fault-injected cases per runner on pinned repos; 95 percent upper
bound on the per-change miss rate 3/N; not a guarantee." On each held-out repository (owner decision
R19 (2026-11-09)): 100 planted faults, at most 3 misses with a Wilson interval reported, at most 5
percent of tests excluded, and per-test durations recorded in every runner's receipt.

Narrowing bar (G4). Go, Vitest, Jest and pytest must each narrow at least 60 percent of replayed PRs
(plan not FULL or UNKNOWN) with median selected test time at most 50 percent of the full suite.
Playwright, RSpec and Minitest publish their numbers with no bar. The zero-miss check is evaluated
first; a runner failing it cannot pass on narrowing.

Watch (answer 1, G7). `corvint watch` answers on save for all seven blocking runners within p95 at
most 1.0 s and p50 at most 300 ms, save to answer, warm cache, on named macOS and Linux reference
machines and a named reference repository. macOS is proven by a real run.

Kill switches.
- `CORVINT_SELECTION=full`, global, or `full:<runner>` per runner, makes every selection a full
  run. The grammar is owned by NMS-V1-008 and is the same everywhere, the Action included (owner
  decision R13 (2026-11-09)). An unknown value exits 2 with a clear message in the CLI and CI, and
  runs all tests in agent hooks and MCP (R13). It is tested in the RC (G10). The GitHub Action
  exposes the same switch as a repository variable (G16).
- `CORVINT_HOOKS=off` turns the agent hooks off (G20).
- Rollback order after release: kill switch, then a 1.0.1 forward fix, then withdraw. Tags are never
  deleted (G10).

Release gates (G8). The affected plan, the exhaustive `go test` and `go vet` in both modules, and
one scripted end-to-end qualification per product. These replace the CEM, OCM and dogfood
ceremony.

Floor (G10). If fewer than 4 of the 7 runners are selective at the freeze, or a whole product fails
its end-to-end scenario after two fix rounds and cannot ship under a preview label, 1.0 slips.

Also part of the bar (owner decisions of 2026-11-09).
- Docs review is advisory by default in every blocking consumer (Tasks gate, Action, agent hooks);
  an opt-in setting makes it blocking (R4, PROD-INT-029).
- `corvint context` ships non-blocking with no claim of measured benefit; its measurement runs in
  1.0.x (R28, R29, PROD-INT-030).
- The VS Code extension moves to the public repository `beamfall/corvint-vscode` and is not a 1.0
  artifact (G22, R33).
- Agent qualification runs on the owner's machines with the owner's subscription logins, never on
  GitHub-hosted runners, and its result is a release evidence file (R39, R40, PROD-INT-026).
- One held-out repository per family (Go, JS/TS, Python, Ruby) is drawn from a sealed pool, seeded
  by the commit SHA of the first pushed candidate tag and never redrawn (owner decision H3
  (2026-11-10), PROD-INT-008).

## Requirements

### Testing product

- `PROD-TEST-001`: (accepted, decision 0596; V1-5272) Given a commit range or a set of changed
  paths, Corvint MUST name the tests that the change requires and give a one-line reason for each.
  - Source: owner, "only run the tests that are required for a change"; G2 ("every selection
    prints a one-line reason").
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: one source file changes.
    - **WHEN** the user runs `corvint affected --base FULL_SHA` after a commit that changes one
      source file
    - **THEN** the plan lists each required test unit with the reason it was selected
    - **AND** a second run on the same inputs prints a byte-identical plan
  - Implemented by: AFP-V0-001 to AFP-V0-010, AFP-V0-012 and AFP-V0-021 in
    `docs/specs/affected-plan-v0.md`.
  - Open tickets: none of its own. The gaps are under the requirements that follow.
  - Evidence today: `cmd/corvint/affected.go`, `cmd/corvint/affected_test.go`,
    `internal/liveverify/affected/`. The plan spec's delivery status is `experimental`.
  - Acceptance evidence at 1.0: recorded plans for real changes on Corvint, on Beamfall and on the
    pinned qualification repositories (PROD-INT-008), each selection carrying a reason, with a
    replay that shows byte-identical plans.

- `PROD-TEST-002`: (accepted, decision 0596; V1-5272) Corvint MUST NOT leave out a test that a
  change requires; where evidence is missing it selects the test, widens to a full run, or reports
  the scope as UNKNOWN with a reason.
  - Changed: (changed 2026-11-09, answer 5 then G3) The acceptance evidence was "a measured miss
    count of zero over sampled full runs", with the sample size an owner decision. It is now the
    fault-injection proof of PROD-TEST-014; merged-PR replay measures selection size only
    (PROD-TEST-015).
  - Source: owner, "working perfectly"; decision 0594, "proves none were missed"; G3.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a changed file has no known test owner.
    - **WHEN** a commit changes a file that no selection rule covers
    - **THEN** the plan reports scope UNKNOWN or selects a full run, and names the file
    - **AND** it does not report that no test is required
  - Implemented by: AFP-V0-004 and AFP-V0-984, AZU-V0-001 to AZU-V0-004
    (`affected-zero-unknown-v0.md`), AEO-V0-001 to AEO-V0-003 (`affected-edit-oracle-v0.md`),
    TSP-V0-001 to TSP-V0-003 (`test-selection-precision-v0.md`) and `additive-narrowing-v0.md`.
  - Open tickets: V1-5199, V1-5200.
  - Evidence today: `internal/liveverify/affected/liverepo/ownership_completeness_test.go`,
    `.corvint/test-selection.json`. (integ) commit `ee6d51d2f` widens for unreviewed flow links
    (APK-V0-004), and the `misskill` lane recorded confirmed misses on main and a replay of #104 and
    #109 (commit `1dfc1718e`).
  - Acceptance evidence at 1.0: PROD-TEST-014 passing for every blocking runner.

- `PROD-TEST-003`: (accepted, decision 0596; V1-5272) One plan MUST cover unit, integration and
  end-to-end tests. Each selected test MUST carry a kind from the closed set `unit | integration |
  e2e | UNKNOWN` and a `kindSource` from `runner | declaration | default | conflict`. In 1.0 the
  kind is a reporting label only and never changes selection; the narrowing basis is a separate
  field.
  - Changed: (changed 2026-11-09, G13) The statement said only "each selected test carrying its
    kind". G13 fixes the enum, the kind source, the definitions below, and that kind never drives
    selection.
  - Source: owner, "those tests could be unit, integration and e2e"; G13.
  - Definitions (G13): `e2e` drives the running application through a real browser driver (the
    Playwright runner; pytest with pytest-playwright fixtures; RSpec `type: :system` or `js: true`
    with a browser driver; Capybara `rack_test` is not e2e). `integration` is declared by the
    project (Go build tags or filename suffix, pytest markers, Jest or Vitest project names, or the
    Corvint declaration file). `unit` is every other test, documented as "undeclared, line-level".
    A conflicting or unresolvable signal is `UNKNOWN`. A browser test outside Playwright is kind
    `e2e` but selects its whole group with a reason.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a change needs tests of two kinds.
    - **WHEN** a commit changes code that has unit tests and is linked to a flow variation
    - **AND** the user runs `corvint affected --base FULL_SHA` with no other option
    - **THEN** one plan lists the unit tests and the end-to-end tests, each with its kind and kind
      source
  - Implemented by: APK-V0-001 and APK-V0-002
    (`docs/specs/openspec/changes/affected-plan-kinds/proposal.md`); APK-V0-002 carries the
    kind and `kindSource`. APK-V0-005 is retired, superseded by APK-V0-009.
  - Open tickets: V1-5251.
  - Evidence today: (integ) commit `8cd570acb` joins declared flows into one plan with a
    `testKind` member (in `internal/liveverify/affected/e2eplan.go` on integ `9ec3a29d0`). No `kindSource` member and no
    test-kind declaration file were found.
  - Acceptance evidence at 1.0: one receipt from a repository that has all three kinds of test,
    recorded on the release candidate, with each kind source shown.

- `PROD-TEST-004`: (accepted, decision 0596; V1-5272) An end-to-end test MUST be selected through
  the flow it tests, so that changing a feature's code selects that feature's flows. A browser E2E
  test is removed from a selection only by accepted flow links plus static reachability (file and
  symbol level). Each selected entry also reports its variation.
  - Changed: (changed 2026-11-09, G2 and G6) It said "selected through the flow and variation".
    G6 makes 1.0 select per flow (frozen semantics) with the variation reported as an optional
    member; per-variation narrowing is additive after 1.0. G2 makes flow links plus reachability the
    only basis for removing a browser E2E test; line evidence may add or rank, never remove. The
    flow file is `.corvint/flows.json` (G6).
  - Source: owner, "e2e tests would be tied to flows ..."; G2; G6.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a change reaches one flow.
    - **WHEN** a commit changes a file that one accepted flow links to
    - **THEN** the plan selects that flow's tests, names the flow, and reports each entry's
      variation
    - **AND** tests of flows the change cannot reach by accepted links and static reachability are
      left out with a recorded reason
  - Implemented by: AFU-V1-017 and AFU-V1-019 to AFU-V1-024
    (`docs/specs/application-flow-understanding-v1.md`); APK-V0 proposal. A reachability report
    (how much narrowing reachability achieves, G2) needs a proposal.
  - Open tickets: V1-5251, V1-5252, V1-5195, V1-5200.
  - Evidence today: `internal/appflows/impact.go`, `internal/appflows/selection.go`; (integ)
    `internal/appflows/flowplan.go`. The tracked file on the integration branch is still
    `.corvint/e2e-flows.json`; its consumers map file paths to flow IDs with no variation.
  - Acceptance evidence at 1.0: on the Playwright qualification repository with accepted flows, a
    recorded change that selects exactly one flow's tests with the flow and variations named, and
    the same on Corvint's own suites; the reachability narrowing report published.

- `PROD-TEST-005`: (accepted, decision 0596; V1-5272) A flow MUST consist of variations with tests
  attached. A variation counts as covered when any linked test (unit, integration or E2E) is
  attached. A flow change requires every test linked to an affected variation. Corvint reports each
  variation with no test, each E2E test with no variation (`unassigned`), and each flow with zero
  E2E tests as "no end-to-end proof", which does not block.
  - Changed: (changed 2026-11-09, answers 9 and 15, G11) The two mismatch reports were `PROPOSED`;
    they are now owner-sourced. "Covered" and "no end-to-end proof" are new.
  - Source: owner, "flows are built up with variants ..."; answers 9 and 15; G11.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: one test is attached to nothing.
    - **WHEN** the E2E inventory holds a test that no variation attaches
    - **THEN** `corvint flows gaps` lists that test as `unassigned`
    - **AND** a variation with no test is listed with the existing `no-test` code
  - Implemented by: AFU-V1-001, AFU-V1-002, AFU-V1-049 to AFU-V1-053; AFU-V1-155 to AFU-V1-159,
    AFU-V1-166 and AFU-V1-167
    (`docs/specs/openspec/changes/flows-unlinked-tests-report/proposal.md`). The proposal was
    amended on 2026-11-09 for answers 9 and 15: AFU-V1-166 counts a linked test of any kind as
    coverage and AFU-V1-167 adds the `no-e2e-proof` record. Earlier text here named AFU-V1-150 to
    AFU-V1-154 (retired by R11) and said the proposal's E2E-only links conflicted with answer 9.
  - Open tickets: V1-5255.
  - Evidence today: (integ) commit `3c4cda623` adds the unlinked-test report on the CLI and MCP;
    `internal/flowdocs/flowinventory.go:59@d114c7d4` has an `unassigned` member.
  - Acceptance evidence at 1.0: both reports on a repository with real flows, with counts that a
    person checked by hand once.

- `PROD-TEST-006`: (accepted, decision 0596; V1-5272) One command MUST run exactly the selected
  tests for each of the seven blocking runners through one compiled-in runner contract, and record
  what ran and the result. Every other runner, Cypress included, runs in full with a printed reason.
  A runner may have several instances in one repository, each keyed by its root directory; sharding
  happens in the GitHub Action only.
  - Changed: (changed 2026-11-09, G1 and G23; R6, R7, R11, R14 and R15) It said "every supported
    language and runner". G1 names the seven runners; G23 requires one compiled-in contract and
    drops the external runner protocol. Owner decision R6 (2026-11-09) drops Cypress from 1.0; R7
    confirms the seven runners and gives Jest its own provider proposal; R11 approves one canonical
    test ID (RNC-V1-012) with fresh requirement IDs; R14 retires each provider binary after its
    built-in contract lands; R15 adds several instances per runner keyed by root.
  - Source: owner, "testing (e2e to code, test running etc)"; G1; G23; R6; R7; R11; R14; R15.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a change selects Go and Playwright tests.
    - **WHEN** the user runs `corvint test --base FULL_SHA`
    - **THEN** the selected Go tests and the selected Playwright tests run
    - **AND** one receipt lists each test that ran and whether it passed
  - Implemented by: STR-V0-001 to STR-V0-011 (`docs/specs/session-test-runner-v0.md`), Go only.
    Proposed: RNC-V1-001 to RNC-V1-018 (`docs/specs/openspec/changes/runner-contract/`),
    PYR-V1-001 to PYR-V1-010 (`docs/specs/openspec/changes/pytest-provider/`) and
    RBR-V1-001 to RBR-V1-012 (`docs/specs/openspec/changes/ruby-providers/`).
    Also proposed: CTI-V1-001 to CTI-V1-012 (`docs/specs/openspec/changes/canonical-test-id/`)
    and JST-V1-001 to JST-V1-010 (`docs/specs/openspec/changes/jest-provider/`).
  - Open tickets: V1-5256. New tickets needed for the pytest, RSpec, Minitest and Jest providers
    and their line evidence (answer list of consequences), and for an end-to-end check of the Jest
    path through `corvint test` (G23 open point).
  - Evidence today: `cmd/corvint/test.go`, `internal/sessiontest/`. G23 records three contracts
    today: the `sessiontest` provider adapter for Vitest and Playwright, a separate RSpec document
    format, and compiled-in pytest and Ruby code that nothing calls (panel finding, NOT_VERIFIED by
    this revision).
  - Acceptance evidence at 1.0: a `corvint-test-run/0` receipt for each blocking runner on its
    pinned qualification repository in which only selected tests ran.

- `PROD-TEST-007`: (accepted, decision 0596; V1-5272) `corvint watch` MUST state, after each save,
  which tests the current edit requires, for all seven blocking runners, within p95 at most 1.0 s
  and p50 at most 300 ms from save to answer on a warm cache. The default output is the list of
  tests with a ready-to-run command; `--run` runs them. Each answer carries its tier (`line`,
  `file`, `flow` or `full`) and the `exact` flag, the single tier vocabulary of owner decision R16
  (2026-11-09).
  - Changed: (changed 2026-11-09, answer 1 and G7) The time limit was `PROPOSED` with no number,
    and the requirement said the loop reruns tests "when asked". Answer 1 sets 1 second; G7 gates
    all seven runners (against the panel's recommendation of Go, Vitest and Jest only), adds the
    p50 bar, the list-by-default output and the tier label, and requires macOS proven by a real run.
  - Source: owner, "functionality like wallaby.js ..."; answer 1; G7.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a file is saved.
    - **WHEN** the developer saves a source file while `corvint watch` is running
    - **THEN** it prints the tests that the edit requires, each with its tier, within the bar
    - **AND** it runs them only when `--run` is given
  - Implemented by: LPCV-V0-960, LPCV-V0-7607 and LPCV-V0-1085 to LPCV-V0-1087
    (`docs/specs/live-proof-carrying-verification-v0.md`); PRB-V0-1014
    (`docs/specs/performance-profiling-and-read-budget-v0.md`). Proposed: WCH-V1-001 to
    WCH-V1-013 (`docs/specs/openspec/changes/watch-seven-runners/`).
  - Open tickets: V1-4372, V1-5253.
  - Evidence today (corrects the first draft; see "Corrections"): (integ) `corvint watch` is a
    `corvint` command (`cmd/corvint/watch.go` on integ `9ec3a29d0`). It is Go-only: the command
    refuses a non-Go index. It runs tests unless `--dry-run` is given, the opposite of G7's
    default. The macOS kqueue source is compile-checked only, `NOT_OBSERVED` live
    (`docs/specs/live-proof-carrying-verification-v0.md:14@3bb12604`). (integ) The integ copy of
    `internal/perfbudget/budgets.json` holds scenario `watch-replan` with an advisory p95
    target of 1000 ms and enforced provisional ceilings of 2500 ms and 64 MiB, and scenario
    `watch-replan-self`, followed by baseline samples (first sample 696.74 ms). No p50 target
    exists, and both scenarios are Go fixtures.
  - Acceptance evidence at 1.0: measured p50 and p95 per blocking runner on the reference machines
    (the owner's Mac and WSL2 Linux on the Windows benchmark box, cores capped at 8) and the seven
    pinned proof repositories, one per runner, sizes printed, Go also timed on the Corvint repository
    (R42 (panel pick) A11), recorded against the bar, with the macOS
    figures from a real save-event run; the budget targets promoted from advisory to enforced on a
    quiet host (G27).

- `PROD-TEST-008`: (accepted, decision 0596; V1-5272) For unit and integration tests, where line
  evidence is fresh, selection MUST narrow to the tests that execute the changed lines, for Go,
  RSpec, Minitest, Vitest and Jest; otherwise it falls back to the file-level selection and says so.
  These five MUST be line-level at RC or the release blocks. pytest selects by file in 1.0, with the
  `file` tier on every selection.
  - Changed: (changed 2026-11-09, answer 3 and G2) It was `PROPOSED` and Go-centred. Answer 3 put
    line-level in 1.0 for all listed runners with Go, JS and Playwright mandatory. G2 removes
    Playwright (browser E2E) from line-level narrowing: E2E is narrowed by PROD-TEST-004 only, and
    per-test line narrowing for E2E is an opt-in experimental profile whose promotion needs
    fault-injected zero-miss proof and a measured net wall-clock win. Owner decision R8
    (2026-11-09) names the line-exact set (Go, RSpec, Minitest, Vitest, Jest) and puts pytest at
    file level with a tier label. Owner decision R9 (2026-11-09) records Python file-level selection
    as a release exception in the acceptance record and makes the Python line collector a 1.0.x
    ticket.
  - Source: answer 3; G2; decision 0590; R8; R9.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: one function changes.
    - **WHEN** a commit changes lines that three tests execute, and the line index is fresh
    - **THEN** the plan selects those three tests with tier `line`
    - **AND** with a stale line index it selects the file-level set with tier `file` and says why
  - Implemented by: LPCV-V0-961 to LPCV-V0-969 and GLS-V0-001 to GLS-V0-003
    (`docs/specs/go-line-selection-v0.md`). Planned: `line-evidence`.
  - Open tickets: V1-5182. New tickets for line evidence of Vitest, Jest, RSpec and Minitest, and a
    1.0.x ticket for the Python line collector (R9).
  - Evidence today: `internal/linecov/`, `.github/linecov/qualification.json`.
    `internal/nativelinecov/` is removed by G26.
  - Acceptance evidence at 1.0: for Go, RSpec, Minitest, Vitest and Jest, line-level narrowing
    with PROD-TEST-014 passing; for pytest, file-level selection with the `file` tier and the
    release exception recorded in the acceptance record.

- `PROD-TEST-009`: (accepted, decision 0596; V1-5272) An agent MUST be able to ask through MCP which
  tests a change requires and receive the same answer as the command line.
  - Source: owner, "reportable by the MCP and cli".
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: an agent asks after an edit.
    - **WHEN** an MCP client calls the affected tool with a base commit
    - **THEN** the tool returns the same selections as `corvint affected --base FULL_SHA`
  - Implemented by: MMR-V0-1118 (`docs/specs/mcp-multi-root-v0.md`, integration branch).
  - Open tickets: V1-5254.
  - Evidence today: (integ) the integ copies of `internal/capcatalog/mcp.go` and
    `internal/mcp/toolgroup/affected.go` define `corvint.affected`, served by
    `corvint mcp --tool-profile affected`.
  - Acceptance evidence at 1.0: a conformance test that compares the tool's answer with the
    command's answer on the same inputs.

- `PROD-TEST-010`: (accepted, decision 0596; V1-5272) Corvint MUST give one report, on the command
  line and through MCP, of each flow, its variations, their tests and each test's last recorded
  result, from `corvint` alone.
  - Changed: (changed 2026-11-09, G5) The report is native in `corvint` and needs no other
    binary. The "last recorded result" needs a writer; see PROD-TEST-020.
  - Source: owner, "All this should be managed by corvint, easy to track and reportable by the
    MCP and cli"; G5.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a flow with two variations.
    - **WHEN** the user runs `corvint flows status`
    - **THEN** one report shows each flow, each variation, the attached tests and their last result
    - **AND** a test that never ran is shown as `NOT_OBSERVED`
  - Implemented by: AFL-V1-001 to AFL-V1-006
    (`docs/specs/openspec/changes/flows-status-report/proposal.md`; the first revision of this
    file named AFU-V1-160 to AFU-V1-165, which is wrong).
  - Open tickets: V1-5257.
  - Evidence today: (integ) commit `3c4cda623` adds `flows status` on the CLI and MCP. It reads
    `test-run-evidence/0` records only from files passed with `--evidence`
    (the acceptance evidence and result source section of the `flows-status-report` proposal).
  - Acceptance evidence at 1.0: the report from the command and from the tool on the same
    repository, shown equal, with results that came from PROD-TEST-020's writer.

- `PROD-TEST-011`: (accepted, decision 0596; V1-5272) The supported runners MUST be exactly the
  seven blocking runners. For each, selection and running work; any other runner (Cypress included),
  any path no runner owns, and any path whose runner was removed or is unavailable, gets a full run
  of all tests with a named reason.
  - Changed: (changed 2026-11-09, answer 2, G1, G26) The list was an open owner decision. G1 fixes
    it. G26 removes the C-family, .NET, Java, Kotlin, Rust and Swift providers and all 16
    `corvint-analyzer-*` binaries; removal fails closed (an unowned path is UNKNOWN). Owner decision
    R32 (2026-11-09) makes an unowned change or a removed or unavailable runner run all tests with a
    named reason, and takes UNOWNED_DIRTY_PATH off the allowlist. R6 drops Cypress.
  - Source: answer 2; G1; G26; R6; R32.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: an unsupported runner.
    - **WHEN** a repository uses a test runner that is not on the supported list
    - **THEN** the plan selects a full run for that runner and names the runner as the reason
  - Implemented by: RSP-V0-001 to RSP-V0-003 (external-protocol parts superseded by G23),
    TJAA-V0-015 to TJAA-V0-021.
  - Open tickets: V1-5256. New: a ticket for the removed-runner full run and the allowlist change
    (R32).
  - Evidence today: `internal/langplugin/`, `internal/liveverify/affected/typescript/`,
    `cmd/corvint-js-test-provider`, `cmd/corvint-ruby-test-provider`,
    `cmd/corvint-go-test-provider`. `docs/CAPABILITIES.md` lists eleven providers and must be
    amended (G26).
  - Acceptance evidence at 1.0: for each blocking runner, one recorded selection and one recorded
    run on its pinned repository; one recorded full-run fallback for an unsupported runner.

- `PROD-TEST-012`: (accepted, decision 0596; V1-5272) Continuous integration MUST run the same plan
  as the local command, through the Corvint GitHub Action only, run everything when the plan cannot
  be trusted, honour the kill switch, and run the full suite periodically.
  - Changed: (changed 2026-11-09, answer 6, G16, G18) It was `PROPOSED` with an unstated period.
    Answer 6 makes CI narrowing a product feature for GitHub Actions only, with a kill switch and
    periodic full runs. G16 to G18 define the action; see the GitHub Actions section.
  - Source: answer 6; G16; G18.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: the plan input is stale.
    - **WHEN** CI cannot verify the plan for a pull request
    - **THEN** CI runs the full suite and records why in the job summary
  - Implemented by: `docs/specs/ci-narrowing-v0.md` (`CIN-V0`, Corvint's own CI only), AFP-V0-028
    to AFP-V0-030, STR-V0-007 to STR-V0-009; PROD-INT-018 to 022.
  - Open tickets: V1-5182, V1-5199, V1-5200.
  - Evidence today: `.github/workflows/ci.yml` uses bespoke selection scripts; no `action.yml` and
    no `workflow_call` workflow exist (CI survey of the integration checkout, spot-checked only).
  - Acceptance evidence at 1.0: the action's own qualification (PROD-INT-021), and the period of
    periodic full runs stated in the action proposal.

- `PROD-TEST-013`: (accepted, decision 0596; V1-5272) `CORVINT_SELECTION=full`, set globally or for
  one runner, MUST make every selection for that scope a full run with the switch named as the
  reason, in `affected`, `test`, `watch`, MCP and the GitHub Action. The per-runner form is
  `full:<runner>` everywhere. An unknown value MUST exit 2 with a clear message in the CLI and CI,
  and MUST run all tests in agent hooks and MCP.
  - Changed: (changed 2026-11-09, R13) Owner decision R13 (2026-11-09) fixes the unknown-value
    behaviour and unifies the grammar as `full:<runner>`, the Action included.
  - Source: G10; answer 6; G16; R13.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: one runner switched off.
    - **WHEN** `CORVINT_SELECTION` selects full for Jest only
    - **THEN** the plan runs every Jest test and narrows the other runners as usual
  - Implemented by: NMS-V1-008 (`docs/specs/openspec/changes/narrowing-measurement/`), which owns the
    `CORVINT_SELECTION` grammar, the refusal and the reason text; WCH-V1-009, RNC-V1-008 and
    GHA-V1-009 cite it. No MCP requirement exists yet (gap). The unknown-value behaviour is fixed by
    R13 and must be written into NMS-V1-008. AFP-V0-030 is a kill switch for Corvint's own CI only.
  - Open tickets: none found.
  - Evidence today: `git grep CORVINT_SELECTION` finds nothing on the integration branch.
  - Acceptance evidence at 1.0: the switch tested on the RC binary, globally and per runner.

- `PROD-TEST-014`: (accepted, decision 0596; V1-5272) Before the RC, each blocking runner MUST pass
  a frozen fault-injection miss proof: at least 100 fault-carrying PRs (about 30 bug-fix reverts,
  the rest changed-line mutants at 3 to 5 per PR) and at least 30 non-code faults (config, fixtures,
  snapshots, lockfiles, generated files), frozen and hashed before execution (sampling frame, SHAs,
  mutation operators, seeds, exclusion rules), with zero unselected confirmed kills.
  - Source: G3 (supersedes answer 5).
  - Rules (G3): count PRs, not mutants. The selected run must execute the killing test and see it
    fail. Baseline runs 3 times; a test failing in any baseline run is quarantined and recorded,
    capped at 2 percent of the suite or the runner is disqualified. A kill is confirmed only if it
    fails 3 of 3 on the mutant and passes on baseline. An unselected confirmed kill blocks the
    release and is never waived as a flake. Mutants that kill nothing do not count. The qualification
    harness is pinned (toolchain, lockfile, browser build, repository SHA), runs on Linux and macOS,
    not on Unraid; the replay harness is built once for Go first (G1 conditions). Measure one PR per
    runner first to check the compute estimate of about a week.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a mutant is missed.
    - **WHEN** a confirmed kill's killing test is not in the selection
    - **THEN** the runner fails the proof and the release is blocked
  - Implemented by: FIH-V1-001 to FIH-V1-013 (`docs/specs/openspec/changes/fault-injection-harness/`).
  - Open tickets: none found.
  - Evidence today: (integ) the `misskill` lane replay records exist, but they are plain replay,
    not a frozen fault set.
  - Acceptance evidence at 1.0: per runner, the frozen hashed case manifest and the result record,
    with the release wording of "The 1.0 bar".

- `PROD-TEST-015`: (accepted, decision 0596; V1-5272) Corvint MUST report selection size and
  over-selection on a plain replay of merged PRs: 200 on Corvint, 100 on Beamfall and 50 on each of
  the four drawn held-out repositories (owner decision H3 (2026-11-10)): the 50 most recent merged
  PRs before each pin, the list frozen with the manifest (R42 (panel pick) A12; review completeness
  F16). The replay does not count as miss proof.
  - Source: G3 (supersedes answer 5's N as a miss proof).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: replay report.
    - **WHEN** the replay runs over the frozen PR list
    - **THEN** it reports per PR the plan class (narrowed, FULL, UNKNOWN), selected test count and
      selected test time
  - Implemented by: NMS-V1-001 to NMS-V1-010 (`docs/specs/openspec/changes/narrowing-measurement/`).
  - Open tickets: none found.
  - Evidence today: none for this sample size.
  - Acceptance evidence at 1.0: the three replay reports bound to the RC.

- `PROD-TEST-016`: (accepted, decision 0596; V1-5272) Go, Vitest, Jest and pytest MUST each narrow
  at least 60 percent of replayed PRs (plan not FULL or UNKNOWN) with median selected test time at
  most 50 percent of the full suite, measured from recorded per-test durations excluding build and
  setup. Playwright, RSpec and Minitest publish their numbers with no bar.
  - Source: G4; R19.
  - Rules (G4): the numbers are confirmed on the 200-PR Go replay and frozen before the RC is
    measured. PROD-TEST-014 is evaluated first; a runner failing it cannot pass on narrowing. Owner
    decision R19 (2026-11-09): per-test durations are recorded in every runner's receipt (planned
    change `per-test-durations`), and at most 5 percent of tests may be excluded from a
    measurement. A `flow`-tier selection is not counted as narrowed for the no-miss claim (R16).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a runner over-selects.
    - **WHEN** pytest narrows 45 percent of replayed PRs
    - **THEN** pytest fails the bar and is demoted or fixed before the freeze
  - Implemented by: PRR-V1-001 to PRR-V1-011 (`docs/specs/openspec/changes/proof-repositories/`) plus the NMS measurement.
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: the frozen thresholds and the per-runner measurements.

- `PROD-TEST-017`: (accepted, decision 0596; V1-5272) An explicit `corvint inventory refresh` step
  MUST run each of the seven runners' test listers and write one committed inventory under
  `.corvint/`. `corvint affected` only reads that inventory. When it is missing or stale, the plan
  runs all tests with reason `inventory-absent` or `inventory-stale`.
  - Changed: (changed 2026-11-09, R10) It covered E2E tests only and widened to all E2E tests.
    Owner decision R10 (2026-11-09) makes one inventory for all seven runners, names the refresh
    verb, and makes a missing or stale inventory run all tests.
  - Source: answer 4 (product invariant 4 kept); R10.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: stale inventory.
    - **WHEN** the inventory was written at an older tree than the plan's head
    - **THEN** the plan selects all tests and names `inventory-stale` as the reason
  - Implemented by: AFU-V1-140 to AFU-V1-149
    (`docs/specs/openspec/changes/e2e-inventory-discovery/proposal.md`), to be generalised by the
    planned `test-inventory` change.
  - Open tickets: V1-5252; a new ticket for `corvint inventory refresh`.
  - Evidence today: (integ) commit `8cd570acb` derives an inventory inside the plan. A grep found
    no separate refresh command.
  - Acceptance evidence at 1.0: the refresh step and a recorded stale-widening plan.

- `PROD-TEST-018`: (accepted, decision 0596; V1-5272) An E2E test linked to no flow MUST always run
  and be counted as `unassigned` in every plan and in `flows gaps`. A link goes stale, returning its
  test to `unassigned`, when the cited test content hash or routes change; static reachability must
  still agree; dangling links are reported. An optional CI ratchet refusing new unassigned tests is
  off by default. Links are proposed with a confidence score and accepted by a person only (owner
  decision R12 (2026-11-09)).
  - Source: G11; R12.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: an edited test.
    - **WHEN** a linked test's content changes after its link was accepted
    - **THEN** the link is stale and the test runs as `unassigned` until a person accepts again
  - Implemented by: AFU-V1-150 to AFU-V1-154 (partial); the link record and staleness go in the
    planned `flow-link-proposals` change (PROD-DOCS-012).
  - Open tickets: V1-5255.
  - Evidence today: (integ) the `unassigned` report exists; link staleness was not found.
  - Acceptance evidence at 1.0: a recorded stale link and a recorded unassigned test in a plan.

- `PROD-TEST-019`: (accepted, decision 0596; V1-5272) A changed non-code file MUST select every test
  that might read it, unless an accepted map entry narrows it. Recordings may only propose entries;
  an entry narrows only after an injected data-file fault proves it is caught. Never narrowed:
  `go.mod`, `go.sum` and other lockfiles, the toolchain pin, `.github/**`, `Makefile`, runner
  config, release config, generator inputs, `go:embed` and build-tag inputs, snapshot update paths,
  the map file itself, `.corvint/test-read-scopes.json`, and secret or environment config. Under
  `.corvint/`, only Corvint configuration files force a full run; an edit to `.corvint/flows.json`
  runs the tests linked to the changed flows; doc-only edits and the two private ledgers add no
  tests.
  - Changed: (changed 2026-11-09, R31) Owner decision R31 (2026-11-09) narrows the `.corvint/`
    full-run rule to configuration files.
  - Source: G15b; R31.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a fixture changes.
    - **WHEN** a commit changes a JSON fixture with no accepted map entry
    - **THEN** every test that might read it is selected, with that reason
  - Implemented by: the test read-scope requirements behind `.corvint/test-read-scopes.json`
    (exact IDs NOT_VERIFIED by this revision). Proposed: NCM-V1-001 to NCM-V1-009
    (`docs/specs/openspec/changes/non-code-map/`).
  - Open tickets: none found.
  - Evidence today: `.corvint/test-read-scopes.json` exists (integ).
  - Acceptance evidence at 1.0: the never-narrowed list tested; one map entry proven by an injected
    fault.

- `PROD-TEST-020`: (accepted, decision 0596; V1-5272) Every test run that Corvint executes (`corvint
  test`, `corvint watch --run`, the GitHub Action) MUST record each test's result as run evidence in
  one documented private location that the flow reports read by default, so that "last recorded
  result" has a writer. The record is append-only.
  - Changed: (changed 2026-11-09, R7) It was `PROPOSED`. Owner decision R7 (2026-11-09) adds it as
    an append-only run result record, amending answer 3 and answer 8.
  - Source: owner decision R7 (2026-11-09); needed by PROD-TEST-010 and answer 8 ("run evidence
    from recorded E2E"). Found while checking the first draft; see "Corrections".
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: run then report.
    - **WHEN** the user runs `corvint test` and then `corvint flows status` with no `--evidence`
    - **THEN** the report shows the results of that run
  - Implemented by: the planned `run-evidence` change. Options for it: `corvint test` also writes
    `test-run-evidence/0` records, or the flow readers accept `corvint-test-run/0` receipts. Either
    is a writing command, not a read command, so invariant 4 is kept.
  - Open tickets: none found.
  - Evidence today: see "Corrections", item 4.
  - Acceptance evidence at 1.0: the scenario above recorded on the RC.

### Documentation product

- `PROD-DOCS-001`: (accepted, decision 0596; V1-5272) Corvint MUST serve its read answers to agents
  through a local MCP server, bounded in size and without changing the repository. The documentation
  tools are served by one `corvint-docs mcp` server; `corvint mcp` keeps the testing tools; one
  table fixes every tool name and schema.
  - Changed: (changed 2026-11-09, R30) It named `corvint mcp` as the server and was `DELIVERED`.
    Owner decision R30 (2026-11-09) puts the documentation tools on one `corvint-docs mcp` server
    with one tool table; the state is `PARTIAL` until that server and table ship.
  - Source: owner, "documentation (mcp, ...)"; R30.
  - Intent: proposed. Delivery: implemented. State: PARTIAL.
  - Scenario: an agent lists tools.
    - **WHEN** an MCP client connects to `corvint-docs mcp` over stdio and lists the tools
    - **THEN** it receives the documentation tools named in the tool table, and each call returns a
      bounded answer
    - **AND** `git status` shows no change after the session
  - Implemented by: MCPV0-001 to MCPV0-005 (`mcp-server-2026-07-28-v0.md`), MMR-V0-001 to
    MMR-V0-003 (decision 0441) and MMR-V0-012. Planned: `mcp-tool-table`.
  - Open tickets: a new ticket for the single docs server and the tool table.
  - Evidence today: `cmd/corvint-mcp`, `cmd/corvint-docs-mcp`, `internal/mcp/`,
    `conformance/mcp-2026-07-28/`, `docs/MCP-SERVER.md`. The `test-validity` MCP group and
    `cmd/corvint-test-validity-mcp` are removed by G24.
  - Acceptance evidence at 1.0: the conformance suite recorded against the RC binary, and one
    recorded agent session on a held-out repository.

- `PROD-DOCS-002`: (accepted, decision 0596; V1-5272) Corvint MUST generate draft documentation in
  which every statement cites its source, and a person accepts each draft; nothing is published
  unaccepted.
  - Source: owner, "auto docs generation"; answer 7 (confirms); PRS-V1-015.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a package gets a draft.
    - **WHEN** the user asks for a documentation draft of one package
    - **THEN** each statement in the draft cites a file at a pinned commit
    - **AND** nothing is written to the published documentation until a person accepts it
  - Implemented by: SDD-V0-001 to SDD-V0-006 (`source-documentation-draft-v0.md`, intent
    `proposed`) and HDCV0-001 to HDCV0-004 (`human-documentation-compiler-v0.md`).
  - Open tickets: none found.
  - Evidence today: `cmd/corvint-docs/`, `internal/doccompiler/`, `internal/docstools/docstools.go`.
    Three of the 66 HDCV0 IDs are named in a test file.
  - Acceptance evidence at 1.0: drafts for Corvint and one outside repository, every citation
    resolved by the citation check, with the acceptance step recorded.

- `PROD-DOCS-003`: (accepted, decision 0596; V1-5272) Corvint MUST detect documentation that no
  longer matches the source it cites, and fail the documentation check.
  - Changed: (changed 2026-11-09, answer 8) Source was `PROPOSED`; answer 8 puts drift detection in
    1.0.
  - Source: answer 8.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a cited line moves.
    - **WHEN** a commit changes a source line that a document cites
    - **THEN** the documentation check fails and names the document and the citation
  - Implemented by: DCG-V0-001 to DCG-V0-004, FDM-V0-001 to FDM-V0-007, DCI-V0-001 to DCI-V0-003.
  - Open tickets: none found.
  - Evidence today: `script/check-line-citations.sh`, `internal/doccitations`,
    `internal/docmaintain/docmaintain.go`.
  - Acceptance evidence at 1.0: the check failing on a seeded drift and passing after repair, on
    Corvint and on one outside repository.

- `PROD-DOCS-004`: (accepted, decision 0596; V1-5272) A project MUST declare its flows and
  variations in one file `.corvint/flows.json` with a versioned profile, and be able to import,
  export and read them, each with an identifier that is never reused. Corvint proposes flows; a
  person accepts them; an unaccepted flow never drives selection. A reader refuses an unknown schema
  version.
  - Changed: (changed 2026-11-09, answer 10, G5, G6) It said "declare, import, export and read"
    with commands taking `--flows DIR`. G6 fixes one file named `.corvint/flows.json`;
    `.corvint/e2e-flows.json` is read for one release with a deprecation notice; both present fails
    closed with `E2E_FLOWS_INVALID`; AFU-V1-1651 and AFU-V1-1655 are rewritten for the new name.
    Answer 10 adds propose-and-accept. G5 adds the schema version and refusal.
  - Source: owner, "documentation (..., flows, ...)"; answer 10; G5; G6.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: both files present.
    - **WHEN** a repository has both `.corvint/flows.json` and `.corvint/e2e-flows.json`
    - **THEN** every flow reader refuses with `E2E_FLOWS_INVALID`
  - Implemented by: AFU-V1-001 to AFU-V1-005, AFU-V1-018, AFU-V1-1651, AFU-V1-1655.
  - Open tickets: V1-5251, V1-5195.
  - Evidence today: (integ) `.corvint/e2e-flows.json` is tracked; no `.corvint/flows.json` exists.
  - Acceptance evidence at 1.0: flows declared and accepted on Corvint and on one outside
    repository, with an export and import round trip and the both-files refusal recorded.

- `PROD-DOCS-005`: (accepted, decision 0596; V1-5272) Corvint MUST generate and check flow
  documentation from accepted flows, and report when a flow document has drifted from its flow.
  - Source: owner, "auto docs generation" with "flows"; PRS-V1-015.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a variation is added.
    - **WHEN** an accepted flow gains a variation and the flow document is not regenerated
    - **THEN** the flow document check fails and names the flow
  - Implemented by: AFU-V1-043 to AFU-V1-048, AFU-V1-101 to AFU-V1-106, FDM-V0-001 to FDM-V0-007.
  - Open tickets: none found.
  - Evidence today: `cmd/corvint-docs/docs_flows.go`, `internal/appflows/docs.go`,
    `internal/appflows/docs_maintain.go`, `docs/FLOW-DOCUMENTATION.md`.
  - Acceptance evidence at 1.0: generated flow documents for a real application, with a drift
    check recorded failing and then passing.

- `PROD-DOCS-006`: (accepted, decision 0596; V1-5272) Corvint MUST map an application's screens and
  routes to its flows and to the end-to-end tests that exercise them.
  - Source: owner, "E2E mapping".
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a screen is looked up.
    - **WHEN** the user asks which flows and tests reach one route
    - **THEN** the map names the flows, the variations and the tests, with the run that showed it
  - Implemented by: AMAP-V0-001 to AMAP-V0-004, ANL-V0-001 to ANL-V0-003 (decision 0518),
    MCJ-V0-001 to MCJ-V0-004.
  - Open tickets: none found.
  - Evidence today: `internal/appmap/`, `docs/APPLICATION-MAP.md`. One join requirement depends on
    a retired spec (`docs/specs/SPEC-HEALTH-2026-11-09.md`).
  - Acceptance evidence at 1.0: a map of one real application, with a hand check of a sample of
    routes against the tests that ran.

- `PROD-DOCS-007`: (accepted, decision 0596; V1-5272) `corvint how` MUST answer how to do a task in
  the application from the application map, giving the steps and the evidence for each step, under
  requirements in a rewritten spec.
  - Changed: (changed 2026-11-09, answer 8) Source was `PROPOSED`; answer 8 puts `corvint how` in
    1.0 and requires its spec to be rewritten. G5 keeps `how` in `corvint-docs`.
  - Source: answer 8; G5.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a goal is asked.
    - **WHEN** the user runs `corvint-docs how --map FILE --source e2e GOAL`
    - **THEN** the answer lists steps from the map, each with the recorded run that showed it
    - **AND** it says so when the map holds no evidence for the goal
  - Implemented by: no surviving capability requirement. Proposed: HOW-V1-001 to HOW-V1-010
    (`docs/specs/openspec/changes/how-rewrite/`).
  - Open tickets: a new ticket for the `how` spec rewrite (answer list of consequences).
  - Evidence today: `docs/CAPABILITIES.md` lists `corvint how` as experimental with profile
    `site-knowledge-how/0`, citing a retired contract; code under `internal/appmap/`.
  - Acceptance evidence at 1.0: the rewritten spec, and recorded answers on one real application.

- `PROD-DOCS-008`: (accepted, decision 0596; V1-5272) Flow and map evidence MUST come from recorded
  end-to-end runs, and be marked stale when the run was on another tree.
  - Changed: (changed 2026-11-09, answer 8, G24) Source was `PROPOSED`; answer 8 puts run evidence
    in 1.0. G24 removes `corvint runs` and `internal/playwrightruns`, which the first draft cited as
    implementation; the surviving path is `flows ingest` into `test-run-evidence/0`
    (`internal/appflows/runingest.go`) plus PROD-TEST-020. Owner decision R7 (2026-11-09) adds
    PROD-TEST-020 as the append-only run result record, amending answer 8.
  - Source: answer 8.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: evidence from an older commit.
    - **WHEN** the newest run of a test was on an earlier tree
    - **THEN** the flow and the map show that evidence as stale, with its commit
  - Implemented by: AFU-V1-011 to AFU-V1-014; PWR-V0-001 to PWR-V0-003 retire with
    `internal/playwrightruns` (G24).
  - Open tickets: none found.
  - Evidence today: `internal/appflows/runevidence.go`, `internal/appflows/runingest.go`,
    and the ingest command in `internal/command/flows/flows.go` (integ `9ec3a29d0`).
  - Acceptance evidence at 1.0: a recorded run ingested on a real application, then shown stale
    after a later commit.

- `PROD-DOCS-009`: (accepted, decision 0596; V1-5272) Corvint MUST let an agent search the project's
  documentation and read a cited passage that is verified against the pinned source.
  - Changed: (changed 2026-11-09, answer 8) Source was `PROPOSED`; answer 8 puts docs search for
    agents in 1.0.
  - Source: answer 8.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: an agent searches.
    - **WHEN** an agent searches the documentation for a term and opens one result
    - **THEN** it receives the passage with the file and commit it was read from
  - Implemented by: DCP-V1-001 to DCP-V1-003 (`documentation-corpus-v1.md`) and four companion
    specs, all intent `proposed`.
  - Open tickets: none found.
  - Evidence today: `internal/doccorpus/`, `cmd/corvint-docs-mcp`.
  - Acceptance evidence at 1.0: recorded searches on Corvint and on one outside repository, with
    every returned passage verified.

- `PROD-DOCS-010`: (accepted, decision 0596; V1-5272) Every documentation and flow answer MUST be
  available through MCP with the same content as the command line.
  - Source: owner, "reportable by the MCP and cli".
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: the same question two ways.
    - **WHEN** `corvint flows gaps` and the `corvint.flows.gaps` tool run on the same inputs
    - **THEN** both return the same records
  - Implemented by: MMR-V0-012 and the flow tool constants in `internal/mcp/bridge/flows.go`.
  - Open tickets: V1-5255, V1-5257.
  - Evidence today: `internal/capcatalog/mcp.go` lists no tool for `how` or for the flow document
    commands.
  - Acceptance evidence at 1.0: for each documentation and flow command, a test that compares the
    command's answer with the tool's answer.

- `PROD-DOCS-011`: (accepted, decision 0596; V1-5272) Flow authoring, discovery, proposals and `how`
  MUST live in `corvint-docs`. Flow reports that join flows with test evidence or results (gaps,
  status, impact, test-evidence) MUST be native in `corvint` on shared readers. One written boundary
  rule decides each verb's single home.
  - Changed: (changed 2026-11-09, G5) It said all documentation commands, including `flows`, move
    to `corvint-docs`. G5 keeps the test-joining reports in `corvint` and amends decision 0595,
    under which only `flows test-evidence` is native today (`cmd/corvint/flows_shim.go` on
    integ `9ec3a29d0`).
  - Source: decision 0594 items 3 and 8; G5.
  - Boundary rule (G5): joins with test evidence or results belong to testing; authoring and
    discovery belong to documentation.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: testing-only install.
    - **WHEN** only `corvint` is installed and the user runs `corvint flows gaps`
    - **THEN** the report is produced with no other binary
  - Implemented by: IRT-V0-001 to IRT-V0-006, CMS-V0-001 to CMS-V0-011, CMS-V0-912 to CMS-V0-916.
  - Open tickets: none found.
  - Evidence today: decision 0595 (integ); `tools/import-ratchet/areas.json`.
  - Acceptance evidence at 1.0: the boundary rule written; the release smoke test of PROD-INT-016.

- `PROD-DOCS-012`: (accepted, decision 0596; V1-5272) Corvint MUST propose flow declarations and
  test links with cited evidence (`flows propose --links`, with a bulk bootstrap), and a person MUST
  accept them per flow. A link record holds `test_id` (file path plus title path), `variation_id`,
  `evidence`, `accepted_by`, `accepted_at` and the pinned commit. Tags in test source are proposal
  evidence only, never authority to exclude.
  - Source: answer 10; G11; G25 item 6 (a model proposes flow declarations, links and doc drafts;
    Corvint verifies; the owner accepts).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: bootstrap.
    - **WHEN** the user runs `flows propose --links` on a repository with Playwright tests
    - **THEN** proposed links appear with evidence and none drives selection until accepted
  - Owner decision R12 (2026-11-09): each proposed link carries a confidence score, and only a
    person accepts a link in 1.0.
  - Implemented by: nothing yet. Planned: `flow-link-proposals`.
  - Open tickets: none found.
  - Evidence today: none found.
  - Acceptance evidence at 1.0: proposals and per-flow acceptance recorded on Corvint's own flows.

- `PROD-DOCS-013`: (accepted, decision 0596; V1-5272) `corvint-docs` MUST generate the feature map
  from `.corvint/flows.json`. The `feature`, `features` and `overview` verbs fold into flows and
  their regular-expression heuristics are removed.
  - Source: G25 item 4.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: feature map.
    - **WHEN** the user asks `corvint-docs` for the feature map
    - **THEN** each feature is an accepted flow with its variations, and no heuristic feature
      appears
  - Implemented by: FMP-V1-001 to FMP-V1-006 (`docs/specs/openspec/changes/feature-map-from-flows/`).
  - Open tickets: none found.
  - Evidence today: the heuristic verbs exist (decision 0594 Keeps line).
  - Acceptance evidence at 1.0: the generated map on Corvint; the old verbs absent from the
    capability catalogue.

### Task manager product

The task manager's code and specs are in `beamfall/corvint-tasks` (decision 0594, item 5). This
revision read nothing new in that repository. States marked "not verified" are carried from the
first draft. G9 froze Tasks scope for 1.0 at its nine requirements plus MCP. Owner decision R17
(2026-11-09) confirms the nine as PROD-TASK-001 to PROD-TASK-009 and removes the Tasks MCP server
from 1.0: Tasks is CLI only, and the Tasks repository publishes schema v1 and a supported-command
list.

- `PROD-TASK-001`: (accepted, decision 0596; V1-5272) Tasks MUST keep tickets in a local store in
  which every change is journaled and a ticket can be created, read and searched.
  - Source: owner, "task management". Wording PROPOSED; scope frozen by G9.
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: a bug is filed.
    - **WHEN** an agent creates a ticket and then searches for its title
    - **THEN** the search returns the ticket, and the journal holds one record of the creation
  - Implemented by: `corvint-tasks-store-init-v0.md`, `corvint-tasks-create-payload-v0.md`,
    `corvint-tasks-journal-segments-v0.md`, `native-taskman-planning-v0.md`.
  - Open tickets: none read.
  - Evidence today: this repository's `.taskman/` store is in daily use.
  - Acceptance evidence at 1.0: traced to a test on Tasks `main` (G9 conformance pack).

- `PROD-TASK-002`: (accepted, decision 0596; V1-5272) Tasks MUST let one agent claim a ticket and
  its file scope for a limited time, and refuse a claim that overlaps another.
  - Source: owner, "task management". Wording PROPOSED; G9 (8 concurrent agents never
    double-claim).
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: two agents want the same files.
    - **WHEN** a second agent claims a scope that overlaps a live claim
    - **THEN** the claim is refused with a code that names the holder
  - Implemented by: `corvint-tasks-agent-leases-v0.md`, `scope-lease-v0.md`.
  - Open tickets: none read.
  - Evidence today: none read beyond the spec file names.
  - Acceptance evidence at 1.0: at least 8 concurrent agents with no double claim (G9).

- `PROD-TASK-003`: (accepted, decision 0596; V1-5272) A ticket MUST be completed only through
  submit, a gate run that passes and a completion write; a manual completion carries recorded
  evidence.
  - Source: `AGENTS.md`, "Agent routing". Wording PROPOSED.
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: a gate fails.
    - **WHEN** the gate run for a submitted ticket fails
    - **THEN** the completion write is refused and the ticket stays open
  - Implemented by: `corvint-tasks-delivery-lifecycle-v0.md`, `tasks-criterion-binding-v0.md`,
    `corvint-tasks-attempt-runner-v0.md`.
  - Open tickets: none read.
  - Evidence today: none read beyond the spec file names.
  - Acceptance evidence at 1.0: one ticket completed through the gate and one refusal, both
    recorded.

- `PROD-TASK-004`: (accepted, decision 0596; V1-5272) Every change to the store MUST leave a
  receipt, and an audit command reports any change that has none.
  - Source: `AGENTS.md`. Wording PROPOSED; G9 (receipt audit 0 failures).
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: a record is edited by hand.
    - **WHEN** a ticket file is changed without the Tasks command
    - **THEN** the receipt audit reports that ticket
  - Implemented by: `corvint-tasks-basic-stability-v1.md`, `corvint-tasks-cli-v0.md`.
  - Open tickets: none read.
  - Evidence today: none read beyond the spec file names.
  - Acceptance evidence at 1.0: receipt audit with 0 failures on the real store at the RC; kill -9
    mid-write recovers (G9).

- `PROD-TASK-005`: (accepted, decision 0596; V1-5272) Tasks MUST answer what is open, what is next
  and what is blocked without changing the store.
  - Source: owner, "task management". Wording PROPOSED.
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: the roadmap is read.
    - **WHEN** the user runs the queue status and roadmap commands
    - **THEN** they list open tickets by priority, and the store's files are unchanged
  - Implemented by: `work-queue-observation-v0.md`, `corvint-tasks-lane-ticket-read-v0.md`,
    `corvint-tasks-cli-v0.md`.
  - Open tickets: none read.
  - Evidence today: `AGENTS.md` names `queue status`, `roadmap`, `ticket search`, `ticket show`.
  - Acceptance evidence at 1.0: read commands timed on the real store, with no write recorded.

- `PROD-TASK-006`: (accepted, decision 0596; V1-5272) A ticket MUST be able to name the requirement
  IDs it delivers and the evidence that closed it.
  - Source: PROPOSED, from "everything should be documented before being built".
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a ticket is traced.
    - **WHEN** a reader opens a completed ticket
    - **THEN** it names the requirement IDs it delivered and the evidence for each
  - Implemented by: `corvint-tasks-store-init-v0.md` (`requirementRefs`).
  - Open tickets: none read.
  - Evidence today: `.taskman/tickets/` records have `requirementRefs`; empty on V1-5251 to V1-5257.
  - Acceptance evidence at 1.0: every ticket closed for 1.0 names at least one requirement ID or
    states why none applies.

- `PROD-TASK-007`: (accepted, decision 0596; V1-5272) Agents MUST use Tasks through its CLI. The
  Tasks repository MUST publish schema v1 of its records and a supported-command list. There is no
  Tasks MCP server in 1.0, and `corvint mcp` has no tasks tools.
  - Changed: (changed 2026-11-09, answer 11, G9, R17) It required a Tasks-owned MCP server with CLI
    parity (answer 11, G9). Owner decision R17 (2026-11-09) removes the Tasks MCP server from 1.0
    and requires schema v1 and a supported-command list instead.
  - Source: decision 0594 item 5; answer 11; G9; R17.
  - Intent: proposed. Delivery: not-started. State: MISSING (not verified from this repository).
  - Scenario: an agent reads the queue.
    - **WHEN** an agent needs the open tickets
    - **THEN** it runs a command from the supported-command list and reads output that conforms
      to schema v1
  - Implemented by: Tasks repository specs, to be named there. `tasks-mcp-read-profile-v0.md`
    describes a group inside `corvint mcp` and is out of 1.0 (R17).
  - Open tickets: none read.
  - Evidence today: no Tasks-owned MCP server was found (first draft); none is needed for 1.0.
  - Acceptance evidence at 1.0: schema v1 and the supported-command list published on Tasks
    `main`, each listed command covered by the conformance pack (PROD-TASK-010).

- `PROD-TASK-008`: (accepted, decision 0596; V1-5272) Tasks MUST install, update and run without any
  Corvint binary, and Corvint runs without any Tasks binary.
  - Source: decision 0594 item 5; answer 13.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: Tasks alone.
    - **WHEN** a user installs only the Tasks release on a clean machine
    - **THEN** every Tasks command works
  - Implemented by: `corvint-tasks-release-membership-and-resume-v0.md`,
    `corvint-tasks-user-service-v0.md`.
  - Open tickets: none read.
  - Evidence today: `docs/CAPABILITIES.md`; `cmd/corvint-update` still accepts `--component tasks`.
  - Acceptance evidence at 1.0: an install of each product alone on a clean machine, recorded.

- `PROD-TASK-009`: (accepted, decision 0596; V1-5272) Tasks MUST take over a real project roadmap
  before stable: Corvint and one other repository (`beamfall/core`) run on the frozen RC binary for
  21 days with zero lost tickets, checked by ID and hash diff. A store format change restarts the
  clock.
  - Changed: (changed 2026-11-09, answer 12, G9, R18) The trial length was "an agreed period". Answer
    12 keeps the Beamfall takeover as a 1.0 condition; G9 sets 21 days on the frozen RC. Owner
    decision R18 (2026-11-09) names `beamfall/core` as the trial and conformance subject.
  - Source: PRS-V1-014, PRS-V1-016; answer 12; G9; R18.
  - Intent: proposed. Delivery: experimental. State: PARTIAL (not verified from this repository).
  - Scenario: trial.
    - **WHEN** the 21-day trial ends
    - **THEN** the ticket ID and hash diff shows no lost ticket
  - Implemented by: `corvint-tasks-basic-stability-v1.md`.
  - Open tickets: none read.
  - Evidence today: no takeover record read.
  - Acceptance evidence at 1.0: the trial record bound to the RC binaries.

- `PROD-TASK-010`: (accepted, decision 0596; V1-5272) Corvint 1.0 MUST NOT ship until corvint-tasks
  1.0 passes its own conformance pack: the nine requirements traced to tests on Tasks `main`, every
  command on the supported-command list checked against schema v1, at least 8 concurrent agents
  never double-claim, kill -9 mid-write recovers, receipt audit 0 failures. The conformance subject
  is `beamfall/core`. Tasks is tagged first.
  - Changed: (changed 2026-11-09, R17, R18) MCP parity with the CLI is removed (R17); the subject
    is `beamfall/core` (R18).
  - Source: G9; R17; R18.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: Tasks not ready.
    - **WHEN** the Tasks conformance pack has a failure
    - **THEN** the Corvint RC is not signed
  - Implemented by: Tasks repository specs (not read). Proposed: TIN-V1-001 to TIN-V1-003
    (`docs/specs/openspec/changes/tasks-integration/`). Planned: `tasks-conformance` (in the Tasks
    repository) for the pack and its record format.
  - Open tickets: none read.
  - Evidence today: none read.
  - Acceptance evidence at 1.0: the Tasks conformance record at its 1.0 tag.

- `PROD-TASK-011`: (accepted, decision 0596; V1-5272) Corvint and Tasks MUST keep separate semver
  lines and name one compatible pair (Corvint 1.0.0 with corvint-tasks 1.0.x). The file contract is
  ticket schema v1 plus receipt reference v1, with one shared fixture checked in both repositories'
  CI.
  - Source: G9; answer 13.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: fixture drift.
    - **WHEN** either repository changes the ticket schema without the shared fixture
    - **THEN** that repository's CI fails
  - Implemented by: TIN-V1-004 (`docs/specs/openspec/changes/tasks-integration/`).
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: the fixture passing in both CIs at the paired tags.

- `PROD-TASK-012`: (accepted, decision 0596; V1-5272) A ticket gate MUST be able to run `corvint
  affected` and `corvint test` and store the run's receipt ID, which must resolve on `receipt audit`
  (integration I1).
  - Source: G9.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: gate with tests.
    - **WHEN** a gate runs the affected tests for a ticket
    - **THEN** the ticket records the receipt ID and the audit resolves it
  - Implemented by: TIN-V1-005 (`docs/specs/openspec/changes/tasks-integration/`).
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: one recorded gate run on Corvint's own store.

- `PROD-TASK-013`: (accepted, decision 0596; V1-5272) A ticket MAY cite a flow or variation ID; the
  citation MUST resolve against `.corvint/flows.json` or the operation fails closed (integration
  I2).
  - Source: G9.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: dangling citation.
    - **WHEN** a ticket cites a variation ID that no longer exists
    - **THEN** the check names the ticket and the ID and fails
  - Implemented by: TIN-V1-007 (`docs/specs/openspec/changes/tasks-integration/`).
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: a recorded resolution and a recorded refusal.

### Integration and release

- `PROD-INT-001`: (accepted, decision 0596; V1-5272) The code MUST be divided into layers with a
  check that refuses a new wrong-way import, and 1.0 MUST have zero violations.
  - Changed: (changed 2026-11-09, answer 14) The acceptance allowed owner-accepted remaining
    entries; answer 14 requires zero violations at 1.0 and a check that blocks new ones.
  - Source: owner, "perfectly integrated and structured"; decision 0594 item 8; answer 14.
  - Note: the first draft named the layers shared, analyzers, tests, docs and release. G26 removes
    the analyzer binaries and packages, and the review settled that the analyzers layer goes,
    leaving four layers (checklist O18). Each removal batch records its verdicts in one build-log
    entry (owner decision R34 (2026-11-09)).
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a wrong-way import is added.
    - **WHEN** a commit makes a shared package import a package from the tests layer
    - **THEN** the import check fails and names both packages
  - Implemented by: IRT-V0-001 to IRT-V0-006.
  - Open tickets: none found.
  - Evidence today: (integ) `tools/import-ratchet/baseline.json` lists 17 edges (the first draft
    counted 39 at its base).
  - Acceptance evidence at 1.0: an empty baseline.

- `PROD-INT-002`: (accepted, decision 0596; V1-5272) Testing and documentation MUST read flows
  through one shared model and reader, so that both products give the same answer about a flow. The
  shared layer holds only the model and the reader.
  - Changed: (changed 2026-11-09, answer 14) Answer 14 adds the reader split.
  - Source: owner's decided structure; answer 14.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: one flow, two products.
    - **WHEN** the affected plan and the flow report read the same flow file
    - **THEN** both name the same flows, variations and tests
  - Implemented by: AFU-V1-001, AFU-V1-002, AFU-V1-018; APK-V0-007.
  - Open tickets: V1-5251.
  - Evidence today: `internal/appflows` in the shared area today holds more than model and reader
    (selection, impact, docs).
  - Acceptance evidence at 1.0: a test comparing the plan's flow selections with the flow report on
    the same inputs; the shared area holding only model and reader.

- `PROD-INT-003`: (accepted, decision 0596; V1-5272) Every command and MCP tool MUST appear in one
  capability catalogue that names the product that owns it.
  - Source: owner, "easy to track". Wording PROPOSED.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a command is added.
    - **WHEN** a commit adds a command without a catalogue entry
    - **THEN** the catalogue check fails
  - Implemented by: CAP-V0-001 to CAP-V0-004.
  - Open tickets: none found.
  - Evidence today: `internal/capcatalog/`, `docs/CAPABILITIES.md`; entries name no product.
  - Acceptance evidence at 1.0: the catalogue grouped by product, the check passing on the RC.

- `PROD-INT-004`: (accepted, decision 0596; V1-5272) Each of the three products MUST be installable
  and usable without the other two. They integrate loosely through files and IDs; no library is
  shared across repositories.
  - Changed: (changed 2026-11-09, answer 13) Source was `PROPOSED`; answer 13 adds the loose
    integration rule.
  - Source: answer 13.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: documentation alone.
    - **WHEN** a user installs only the documentation binaries
    - **THEN** the documentation commands and the documentation MCP server work
  - Implemented by: CMS-V0-001 to CMS-V0-011.
  - Open tickets: none found.
  - Evidence today: `cmd/corvint-docs`, `cmd/corvint-docs-mcp`; Tasks is a separate repository.
  - Acceptance evidence at 1.0: an install of each product alone on a clean machine, recorded.

- `PROD-INT-005`: (accepted, decision 0596; V1-5272) Every product requirement in this plan MUST
  trace to accepted capability requirements, and each of those to a test or a measured result.
  - Source: owner, "setup for success"; `docs/SPEC-DRIVEN-DEVELOPMENT.md`.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a requirement has no test.
    - **WHEN** a capability requirement is marked implemented and no test or measurement cites it
    - **THEN** the spec health report lists it, and 1.0 cannot claim it
  - Implemented by: `traceability-test-gate-v0.md`, `requirement-definition-gate-v0.md`,
    `spec-requirement-index-generator-v0.md`.
  - Open tickets: none found.
  - Evidence today: `docs/specs/REQUIREMENTS.tsv`, `docs/specs/SPEC-HEALTH-2026-11-09.md`.
  - Acceptance evidence at 1.0: for each `PROD` requirement, the accepted capability requirements
    and the test or measurement for each, listed in the acceptance decision (PROD-INT-012).

- `PROD-INT-006`: (accepted, decision 0596; V1-5272) A user MUST be able to install, update and roll
  back each product from verified release artifacts: archives for linux/amd64, linux/arm64 and
  darwin/arm64 with `SHA256SUMS`, a manifest, licences and a build attestation. No notarization,
  install script or Homebrew tap ships in 1.0.
  - Changed: (changed 2026-11-09, G15a, answer 16) It named no targets and no exclusions.
  - Source: owner, "successful 1.0 launch"; G15a; answer 16.
  - Notes: (proposed, not from an owner answer; checklist O27) Corvint sends no telemetry.
    Uninstall means deleting the binaries and `.corvint/`; `corvint-update rollback` restores the
    previous release first if wanted. The self-observation ledger under `.corvint/` stays; it is
    local-only and is documented as such (decided 2026-11-09 under R42, E8; low confidence).
  - Binaries per product (owner decision R14 (2026-11-09); read from
    `tools/release-binary/binaries.json`, integ 9ec3a29d0), each built for the three targets:
    testing `corvint`, `corvint-mcp`; documentation `corvint-docs`, `corvint-docs-mcp`; update
    `corvint-update`; Tasks binaries ship from the Tasks repository. The provider binaries
    (`corvint-test-runner`, `corvint-go-test-provider`, `corvint-js-test-provider`,
    `corvint-ruby-test-provider`) retire after their built-in runner contracts land (G23, R14) and
    are not 1.0 artifacts. `corvint-test-validity-mcp` is removed (G24) and the 16
    `corvint-analyzer-*` binaries are removed (G26). The binaries listed there with no 1.0 verdict
    yet are `corvint-cli` and `corvint-web-flows` (open), and the support-role release tools.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: an update is bad.
    - **WHEN** a user applies an update and then runs the rollback command
    - **THEN** the previous binaries run again, and their checksums match the earlier release
  - Implemented by: UPD-V0-001 to UPD-V0-004, ARTIFACT-V0-001 to ARTIFACT-V0-002, PUB-V0-001 to
    PUB-V0-003.
  - Open tickets: V1-0020.
  - Evidence today: `cmd/corvint-update`, `internal/update/`, `cmd/corvint-release-install`,
    `docs/INSTALL.md`. No release-publish workflow and no signing exist (panel fact check).
  - Acceptance evidence at 1.0: install, update and rollback recorded on Linux and macOS from the
    RC artifacts.

- `PROD-INT-007`: (accepted, decision 0596; V1-5272) The 1.0 wire formats and commands MUST be
  frozen and recorded, with a readiness record that states what was and was not qualified.
  - Source: owner, "setup for success". Wording PROPOSED.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a frozen format changes.
    - **WHEN** a commit changes a frozen wire format
    - **THEN** the freeze test fails
  - Implemented by: CCF-V1-001 to CCF-V1-003 (decision 0422), SRR-V1-001 to SRR-V1-003.
  - Open tickets: V1-0021.
  - Evidence today: `cmd/corvint/core_freeze_test.go`, `internal/releasecandidate/readiness_test.go`,
    `cmd/corvint-readiness-record`. The freeze spec speaks of "Core" and lists CEM formats that G8
    removes.
  - Acceptance evidence at 1.0: the freeze list restated for the three products (flow file profile,
    plan, test-run receipt, run evidence, ticket schema v1, receipt reference v1), and a readiness
    record for the RC.

- `PROD-INT-008`: (accepted, decision 0596; V1-5272) Before 1.0 the RC MUST be qualified on Corvint,
  on Beamfall, on seven pinned public repositories (one per blocking runner) and on four held-out
  repositories, one per family, drawn from sealed pools (owner decision H3 (2026-11-10)).
  - Changed: (changed 2026-11-09, answer 17, G12, G12a) It said "one repository not used during
    development". (changed 2026-11-09, owner decision R19 (2026-11-09)) Fault injection uses 100
    planted faults per held-out repository. (changed 2026-11-10, owner decision H3 (2026-11-10))
    The fixed list of owner decision R21 (2026-11-09), fastapi, solidus and payload, and the Go
    pick `helm/helm` with alternate `go-task/task` (R42, A14) are superseded. One held-out
    repository per family (Go, JS/TS, Python, Ruby) is drawn from that family's pool. Each pool
    lists at least five candidates, each checked against the held-out rule and the untouched
    search, and the pools are hashed and sealed before the first candidate tag is pushed
    (PRR-V1-011). The seed is the commit SHA of the first pushed candidate tag (REL-V1-002). The
    draw is deterministic from the seed and the sealed pool (PRR-V1-004; proposed detail in the
    `proof-repositories` design: pool sorted by slug, index from SHA-256 of the seed and the family
    name, modulo the pool size) and is never redrawn (REL-V1-014). A drawn repository that proves
    unusable is reported with the failed check, not replaced. The five names above may be pool
    candidates. Schedule risk: fault sets for the drawn repositories are built between the
    candidate tag and the freeze.
  - Source: PRS-V1-008, PRS-V1-017; answer 17; G12; G12a; owner decisions R19 and R21
    (2026-11-09); owner decision H3 (2026-11-10).
  - Rules (G12): a written selection rule is committed and hashed first (MIT, Apache-2.0 or BSD;
    at least 500 tests; full suite at most 30 minutes; default branch at least 85 percent green over
    90 days; at least 100 merged PRs in 12 months; the Playwright repository needs browser E2E
    against a running app). Named repositories (G12a, each checked against the rule first, any swap
    reported; NOT_VERIFIED): Go `prometheus/prometheus` (alternate `cli/cli`); Vitest `vuejs/core`
    (`vitejs/vite`); Jest `prettier/prettier` (`babel/babel`); Playwright
    `freeCodeCamp/freeCodeCamp` (`RocketChat/Rocket.Chat`); pytest `pydantic/pydantic`
    (`pallets/flask`); RSpec `rubocop/rubocop` (`thoughtbot/factory_bot`); Minitest `rails/rails`
    activesupport (`jekyll/jekyll`). Held-out pools are sealed before the M5a candidate tag is
    pushed, and the draw record is committed before any fault is generated for a drawn repository
    (H3); untouched means no Corvint run, config, flow or fix keyed to the
    repository before; no flows written there, so E2E runs unnarrowed. Fault injection (R19): 100
    planted faults per held-out repository; pass at most 3 misses, with the Wilson interval
    reported, each miss filed P1 and fixed by widening before the candidate (R42 (panel pick) A9);
    at most 5 percent of tests excluded; per-test durations in every runner's receipt.
    All repositories pinned by commit SHA, lockfiles, toolchain versions and image digest in a
    hashed manifest; source fetched, never vendored. Blocking: a fault result above the R19 limit
    or any confirmed miss outside fault injection; a refusal rate above 25 percent; a result
    differing across two identical reruns; a regression found on the RC, for each of the seven
    blocking runners, which blocks and never becomes a demotion of that runner (owner decision H2
    (2026-11-10); REL-V1-004, NMS-V1-007). Over-selection is reported, not blocking. Runs on the
    benchmark box on the M5a candidate (owner decision R2 (2026-11-09)) and again on the RC.
  - Notes: (proposed, not from an owner answer; checklist O23) Benchmark hosts: Linux runs on the
    WSL2 Linux of the Windows benchmark box and macOS on the owner's Mac, both reserved with no
    lanes from the freeze candidate to the RC. Unraid is not used (G1).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: the held-out repositories.
    - **WHEN** the M5a candidate is tagged
    - **THEN** the held-out pools were sealed before the tag, the draw seeded by the tag's commit
      SHA is committed, and each product's acceptance evidence is recorded there, with failures
      kept
  - Implemented by: PRS-V1-008, PRS-V1-017 (to be superseded).
  - Open tickets: V1-0019, V1-0020, V1-0021. V1-0014 closes as superseded (G8).
  - Evidence today: no qualification record for this scope.
  - Acceptance evidence at 1.0: the hashed manifest, the rule file, the sealed pool hashes and the
    draw record, each drawn held-out repository's verification record, and each repository's
    record bound to the M5a candidate and the RC.

- `PROD-INT-009`: (accepted, decision 0596; V1-5272) Corvint MUST use its three products on itself,
  recorded as use of the products, and each release records that use and the friction found. This
  replaces the CEM, OCM and dogfood ceremony.
  - Changed: (changed 2026-11-09, G8) It cited "Dogfood Corvint" and `docs/DOGFOOD.md`. G8 replaces
    the ceremony with recorded self-use and requires `AGENTS.md` to be amended in the same change.
  - Source: G8.
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: Corvint's own flows.
    - **WHEN** a change touches code behind one of Corvint's own end-to-end suites
    - **THEN** Corvint's own plan selects that suite through a flow
  - Implemented by: `corvint-self-development-v0.md` (to be revised).
  - Open tickets: V1-5195, V1-5200, V1-5215. V1-0014 and V1-1344 to V1-1348 close as superseded.
  - Evidence today: (integ) `.corvint/e2e-flows.json` exists.
  - Acceptance evidence at 1.0: Corvint's own flows accepted, and one release cycle recorded with
    all three products in use.

- `PROD-INT-010`: (accepted, decision 0596; V1-5272) Every surviving spec MUST name the product
  requirement it serves, or be retired in place with a status line, or be marked as outside 1.0.
  - Changed: (changed 2026-11-09, G26, G27) Retired specs are archived in place (status lines,
    files kept). Tasks specs are deleted here only after `corvint-tasks` PR #4 merges. (changed
    2026-11-09, owner decisions R37 and R41 (2026-11-09)) Out-of-scope specs are archived in place
    with spec health classes (R37). PRS-V1-018 (c), generated spec header rows, carries over to 1.0.
    The planned `spec-set-cleanup` change gives every old spec and process document a verdict (keep,
    rewrite, archive, delete, move), aligned with the removal batches and finished by 2026-12-01.
  - Source: owner, "you should rework the specs ..."; G27; owner decisions R37 and R41 (2026-11-09).
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a spec serves nothing.
    - **WHEN** a spec implements no requirement of this plan
    - **THEN** its header carries a retired or outside-1.0 status line and the index shows it
  - Implemented by: no capability requirement; `docs/specs/SPEC-HEALTH-2026-11-09.md` proposes
    classes. Planned: `spec-set-cleanup`.
  - Open tickets: none found.
  - Evidence today: the health report is a proposal.
  - Acceptance evidence at 1.0: every row of `docs/specs/README.md` showing a product requirement or
    a retired or outside-1.0 status.

- `PROD-INT-011`: (accepted, decision 0596; V1-5272) Every change MUST be documented before it is
  built: a committed change proposal in the OpenSpec change-folder layout (proposal, tasks, spec
  delta with ADDED, MODIFIED and REMOVED and scenarios) under `docs/specs/openspec/changes/`,
  checked by a Go check in CI. Repository spec files and requirement IDs stay the source of truth;
  the delta is a proposal only and the canonical spec is updated in the same change.
  - Changed: (changed 2026-11-09, G14) G14 fixes the layout and the check, rules out an npm tool, an
    `openspec/` root and any claim of OpenSpec CLI compatibility, and sets the acceptance mode: a
    proposal that only implements an accepted requirement is auto-accepted and logged; anything
    changing behaviour, a format or the release bar waits for the owner; unaccepted work stays
    `experimental`. (changed 2026-11-09, owner decisions R37 and R41 (2026-11-09)) Every change
    passes through the propose, apply and archive stages under `docs/specs/openspec/changes/`, and
    the planned `openspec-check` enforces the stages (R41). PRS-V1-018 (d), the recorded
    acceptance mode per requirement with an audit command, carries over to 1.0 (R37); (a), (b) and
    (e) drop. (changed 2026-11-10, owner decision H12 (2026-11-10)) H12 replaces the G14 clauses
    on the npm tool and the `openspec/` root: `docs/specs/openspec/` is an OpenSpec root, the
    pinned OpenSpec command runs the stages ("Where this file sits"), and `openspec validate --all`
    passes for every change folder (OSC-V1-012). The Go check and the product build still need no
    npm tool. (owner decision H9 (2026-11-10)) `external-spec-not-verified` is an accepted status
    of the check, reported and not failing (OSC-V1-005).
  - Source: owner, "everything should be documented before being built"; G14; owner decisions R37
    and R41 (2026-11-09); owner decisions H9 and H12 (2026-11-10).
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: a ticket is about to be dispatched.
    - **WHEN** a ticket has no committed proposal and cites no existing requirement ID
    - **THEN** it is not dispatched
  - Implemented by: `docs/SPEC-DRIVEN-DEVELOPMENT.md` (general); no enforcing requirement.
    Planned: `openspec-check` (the Go check and the stages) and `spec-set-cleanup` (one workflow
    document).
  - Open tickets: none found.
  - Evidence today: `docs/decisions/PROPOSED-openspec-workflow.md`; proposals under
    `docs/specs/openspec/changes/`.
  - Acceptance evidence at 1.0: the Go check in CI, a passing `openspec validate --all` run, the
    auto-accept log with rationale per entry, the acceptance-mode audit command, and one workflow
    document.

- `PROD-INT-012`: (accepted, decision 0596; V1-5272) 1.0 is done only when every `PROD` requirement
  is PASS with evidence or DEMOTED by a written owner note recorded before the freeze; there are
  zero open P0 or P1 tickets in `v1-0`; each product completes its end-to-end scenario on an
  untouched repository; and the RC soaks 7 days and at least 30 real PRs (merged Corvint and
  Beamfall PRs; held-out repositories have no live PRs, so they add only replayed merged PRs,
  reported separately, O21) with zero confirmed misses and zero new P0 or P1. Any code change in the
  RC restarts the soak and the RC-publish clock (owner decision H1 (2026-11-10); REL-V1-003).
  - Source: G10; owner decisions R1, R2 and R20 (2026-11-09). The signed decision file is the one
    acceptance record of R1.
  - Signature (G10): only the owner signs, twice (RC publish, stable promotion), in one decision file
    listing each requirement's evidence or demotion, the `NOT_RUN` and `NOT_OBSERVED` items, and the
    deferred P2 and P3 tickets.
  - Notes: Demotion verdicts at the freeze (checklist O20, owner decision R2 (2026-11-09)): the
    owner's demotion notes use the fault-injection (FIH) and measurement (NMS) verdicts recorded
    on the M5a qualification candidate, tagged about 2026-12-08 and rebuilt only for blockers.
    The RC run after the freeze confirms them; any regression from those verdicts is a blocking
    miss, for each of the seven blocking runners, and is never recorded as a demotion (owner
    decision H2 (2026-11-10); REL-V1-004). Soak count (checklist O21): PRs merged during the soak
    window in Corvint or Beamfall repositories, read from the GitHub API, plus held-out upstream
    PRs merged during the window and replayed on the RC; the two are reported as separate counts,
    and at least 20 of the 30 are live Corvint or Beamfall PRs (R42 (panel pick) C9). The soak
    also reports the PR
    agreement figure (owner decision R20 (2026-11-09)): the share of soak-window PRs where the
    narrowed run's pass or fail equals the full run's pass or fail on the same commit. Trial
    activity (checklist O22, R42 (panel pick) C9): at least 50 ticket writes and 5 completions per
    trial repository, read from the store event log.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: an unproven requirement at freeze.
    - **WHEN** a requirement has no evidence on 2026-12-15
    - **THEN** it ships as preview or runs the full suite with a printed reason, and the owner's
      demotion note is in the decision file
  - Implemented by: nothing yet. Planned: `release-process` and `acceptance-record`.
  - Open tickets: V1-0020, V1-0021.
  - Evidence today: none.
  - Acceptance evidence at 1.0: the signed decision file, with the PR agreement figure.

- `PROD-INT-013`: (accepted, decision 0596; V1-5272) The freeze date MUST be 2026-12-15. The RC is
  published after the owner's signature. Stable is not promoted before the latest of 2027-01-05, the
  end of the 21-day Tasks RC trial, the RC publish date plus 21 days and the end of the 7-day soak
  on the frozen RC (owner decision H1 (2026-11-10)).
  - Changed: (changed 2026-11-09, owner decision R2 (2026-11-09)) Qualification runs before the
    freeze, on the M5a candidate tagged about 2026-12-08 and rebuilt only for blockers.
  - Source: G10a; owner decision R2 (2026-11-09); owner decision H1 (2026-11-10).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a fix during soak.
    - **WHEN** a code change lands in the RC during the soak
    - **THEN** the soak and the RC-publish clock restart and the stable date moves; the Tasks trial
      restarts only on a store format change (TCF-V1-009)
  - Implemented by: nothing yet. Planned: `release-process`.
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: the dated M5a tag, the dated RC publish, and the trial and soak
    records.

- `PROD-INT-014`: (accepted, decision 0596; V1-5272) A pushed version tag MUST start a workflow that
  runs the full suite and builds the PROD-INT-006 artifacts, then stops. The publish job runs only
  after owner approval in a protected GitHub environment and is the only job with write permission.
  Tags are protected and actions pinned by SHA.
  - Changed: (changed 2026-11-09, owner decision R2 (2026-11-09)) The order is fixed: the RC tag
    is pushed first and builds the artifacts; the owner's approval at M8 publishes them. No
    publish precedes the tag.
  - Source: G15a; owner decision R2 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a tag without approval.
    - **WHEN** a tag is pushed and the owner does not approve
    - **THEN** artifacts are built and nothing is published
  - Implemented by: GHA-V1-040 to GHA-V1-043 (`docs/specs/openspec/changes/github-action/`;
    they stay there, decided 2026-11-09 under R42, D6, checklist O63) and cited by REL-V1-005
    (`docs/specs/openspec/changes/release-process/`).
  - Open tickets: none found.
  - Evidence today: no release-publish workflow (panel fact check).
  - Acceptance evidence at 1.0: the RC published through this path.

- `PROD-INT-015`: (accepted, decision 0596; V1-5272) Supported platforms MUST be Linux and macOS.
  Windows is supported only through WSL; native Windows is stated unsupported in 1.0. The exhaustive
  macOS test and vet MUST be a release gate on the freeze and release candidates. The per-merge
  macOS check stays advisory (REL-V1-015) until its flake tickets close and 20 consecutive main runs
  pass, and is then made required; if that has not happened by 2026-12-15 it stays advisory per
  merge for 1.0. (decided 2026-11-09 under R42, D7) MRS-V1-002 and MRS-V1-004 follow D7 and
  REL-V1-015; MRS-V1-004 keeps filing a P1 ticket per flake, and the release gate and the general
  open-P1 rule replace its own bar on RC approval (owner decision H11 (2026-11-10)). Native Windows
  vet and archive work leaves the 1.0 bar.
  - Notes: (owner decision R36 (2026-11-09)) `make windows-vet` runs once per removal batch,
    before that batch's push, instead of before every push (removal lanes only).
  - Source: answer 16 and its listed consequences; owner decision R36 (2026-11-09); owner decision
    H11 (2026-11-10).
  - Intent: proposed. Delivery: experimental. State: PARTIAL.
  - Scenario: native Windows.
    - **WHEN** a user runs the release on native Windows
    - **THEN** the documentation states it is unsupported and points to WSL
  - Implemented by: PRS-V1-004 (to be amended: it names darwin/arm64 and linux/amd64 only).
    Planned: `macos-required-shards`.
  - Open tickets: hosted-macOS flake tickets (IDs not collected).
  - Evidence today: macOS shards are advisory today (operator practice; not re-checked here).
  - Acceptance evidence at 1.0: the macOS release gate (exhaustive test and vet) green on the freeze
    candidate and the RC; the WSL path recorded once.

- `PROD-INT-016`: (accepted, decision 0596; V1-5272) The flow file MUST carry a schema version and
  every reader MUST refuse an unknown version. A release smoke test MUST run the testing product
  with `corvint` installed alone, including the flow reports of PROD-DOCS-011.
  - Source: G5.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: future schema.
    - **WHEN** `.corvint/flows.json` declares a schema version newer than the reader knows
    - **THEN** the reader refuses and names the version
  - Implemented by: nothing yet. Planned: `flow-schema-version`.
  - Open tickets: none found.
  - Evidence today: none.
  - Acceptance evidence at 1.0: the smoke test on the RC.

- `PROD-INT-017`: (accepted, decision 0596; V1-5272) Release gates MUST be the affected plan, the
  exhaustive `go test` and `go vet` in both modules, and one scripted end-to-end qualification per
  product. The CEM, OCM and dogfood ceremony is superseded by one decision with a carry-over table.
  - Changed: (changed 2026-11-09, owner decisions R3 and R34 (2026-11-09)) Milestone M0
    re-scopes the `v1-0` release object: each open `v1-0` ticket is mapped to a plan requirement
    or moved to `post-1-0`, each move recorded. Tasks queue policy v15 drops `cemRequired`,
    `ocmRequired`, `cem-verdict` and `context-focused` (R3). Removal verdicts and proof are
    recorded as one build-log entry per removal batch, and the superseding decision stays short
    (R34).
  - Source: G8; owner decisions R3 and R34 (2026-11-09).
  - Notes: (proposed, not from an owner answer; checklist O29) Each product's scripted
    qualification is the PROD-INT-004 "install alone" scenario plus that product's primary verb
    on a pinned repository.
  - Notes: (decided 2026-11-09 under R42, D7, D9, D14, M5 and M7) The macOS exhaustive test and
    vet are a release gate on the freeze and release candidates, while the per-merge macOS check
    stays advisory (REL-V1-015). `greptile-gate` stays required on main through stable 1.0 and is
    not a release gate (REL-V1-012). The nightly gate and the archive install smoke for the
    documentation product are release gates (REL-V1-012), named in the release record
    (REL-V1-013). `init` and
    `adopt` are 1.0 first-run verbs; their first-run behaviour is product gap (a), served by
    change `user-docs` (R42, E8).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a release candidate.
    - **WHEN** an RC is cut
    - **THEN** the three gates run and their records are cited in the signed decision file
  - Implemented by: nothing yet; see `source-removal` for the ceremony removal. Planned:
    `release-process` (M0 and the gates) and `tasks-policy-v15`.
  - Open tickets: V1-0014, V1-1344 to V1-1348 close as superseded.
  - Evidence today: `docs/DOGFOOD.md` and `make dogfood-*` targets still govern.
  - Acceptance evidence at 1.0: the three scripted product qualifications.

### GitHub Actions

Owner instruction: "a world class and full corvint integrated CI with github actions". Decisions:
G16 (lean core blocks), G17 (new public repository `beamfall/corvint-action`), G18 (Corvint's own CI
runs the action alongside, then cuts over). The change proposal is
`docs/specs/openspec/changes/github-action/` (written separately; this plan only links it).

Today (CI survey of the integration checkout, spot-checked only): there is no `action.yml` and no
`workflow_call` workflow. Adopters copy `script/adopter-ci-advisory.sh` and an advisory snippet from
`docs/AUTOMATION.md`. `ci.yml` selection is bespoke (`.github/forward-rows`, `tools/ci-reuse-plan`,
`tools/docs-ci-plan`). The product CLI emits no job summary, annotation or PR comment.

- `PROD-INT-018`: (accepted, decision 0596; V1-5272) One GitHub Action in `beamfall/corvint-action`
  MUST, with no account, no secrets and read-only permissions: download a checksum-verified Corvint
  binary; resolve the base commit on `pull_request`, `merge_group` and `push`; output a plan and a
  per-runner matrix with shards; run the selected tests; and fall back to running everything when
  unsure.
  - Changed: (changed 2026-11-09, owner decisions R23, R24 and R25 (2026-11-09)) The action
    narrows on `pull_request` and `push`; fork pull requests always run everything (R23). Actions
    are pinned by SHA; permissions are `contents: read`, plus `id-token: write` only for
    attestation; PR code is never checked out under `pull_request_target` (R24). The licence is
    Apache-2.0 (R25).
  - Source: G16, G17; answer 6; owner decisions R23, R24 and R25 (2026-11-09).
  - Notes: the repository carries the action and starter workflows under Apache-2.0 (R25) with
    tags `v1` and `v1.x.y`. The repository is created when its proposal is committed (G17
    approval). Proposed (checklist O28): `SECURITY.md` is copied into `beamfall/corvint-action`
    and `beamfall/corvint-agents`.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: unresolved base.
    - **WHEN** the action cannot resolve a base commit
    - **THEN** it runs every test and says why in the job summary
  - Implemented by: `docs/specs/openspec/changes/github-action/`.
  - Acceptance evidence at 1.0: the action run on each of the seven pinned repositories and on
    Corvint, with recorded plans and fallbacks.

- `PROD-INT-019`: (accepted, decision 0596; V1-5272) The action MUST honour a kill-switch repository
  variable equivalent to `CORVINT_SELECTION=full`, provide one always-running result job usable as
  the single required check, write a job summary with each skip reason, and run the full suite on a
  periodic schedule.
  - Changed: (changed 2026-11-09, owner decision R13 (2026-11-09)) The variable uses the same
    grammar as the CLI, including `full:<runner>` (NMS-V1-008); an unknown value fails the job
    with exit 2 and a clear message.
  - Source: G16; answer 6; owner decision R13 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: kill switch.
    - **WHEN** the repository variable is set to full
    - **THEN** the next run selects everything and the summary names the switch
  - Implemented by: `docs/specs/openspec/changes/github-action/`.
  - Acceptance evidence at 1.0: the switch and the periodic full run recorded on Corvint's CI.

- `PROD-INT-020`: (accepted, decision 0596; V1-5272) A change to Corvint's own config or narrowing
  map MUST run everything; fork pull requests MUST read config from the base ref; the documentation
  MUST forbid `pull_request_target` with head checkout.
  - Source: owner decisions R23 and R24 (2026-11-09): fork pull requests always run everything,
    and PR code is never checked out under `pull_request_target`. Originally carried from the G16
    security lens.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: fork edits the map.
    - **WHEN** a fork PR changes the narrowing map
    - **THEN** the action reads the base ref's map and runs everything
  - Implemented by: `docs/specs/openspec/changes/github-action/`.
  - Acceptance evidence at 1.0: both cases recorded on a test repository.

- `PROD-INT-021`: (accepted, decision 0596; V1-5272) Corvint's own CI MUST run the action in shadow
  next to the bespoke scripts until it matches them on 30 PRs with no would-be miss, then replace
  them before the freeze. If not matched by 2026-12-15, the scripts stay for 1.0 and the cut-over is
  the first 1.0.x change. Shadow mode is therefore built before the freeze even though it is preview
  for adopters.
  - Notes: (owner decisions R15 and R22 (2026-11-09)) The action produces one plan artifact with
    a digest; shards filter it by runner; a final job verifies that every shard used the same plan
    (R22). Several instances of one runner are keyed by root directory, and sharding happens only
    in the action (R15).
  - Source: G18; owner decisions R15 and R22 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: shadow mismatch.
    - **WHEN** the shadow plan would have skipped a test that the bespoke path ran and that failed
    - **THEN** it is recorded as a would-be miss and the 30-PR count restarts
  - Implemented by: `docs/specs/openspec/changes/github-action/`; at cut-over, `gate-ledger`,
    `gate-receipt`, `go-archive-gate` and `adopter-ci-advisory` are replaced (G27).
  - Acceptance evidence at 1.0: the 30-PR shadow record, or the recorded decision to keep the
    scripts.

- `PROD-INT-022`: (accepted, decision 0596; V1-5272) These action features MUST be specified at 1.0
  and labelled preview, not release-blocking: shadow mode with a would-be-miss report for adopters,
  a docs drift step, a flow-gaps report, a Tasks ticket-gate step, starter workflows per runner, a
  PR comment and index cache reuse. Out of 1.0: GitLab and other CI systems, Windows runners, a
  hosted dashboard, JavaScript or Docker action forms.
  - Notes: (owner decision R4 (2026-11-09)) The docs drift step is advisory by default, with an
    opt-in setting that makes it blocking.
  - Source: G16; owner decision R4 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: preview feature fails.
    - **WHEN** the PR comment step fails
    - **THEN** the release is not blocked and the feature stays preview
  - Implemented by: `docs/specs/openspec/changes/github-action/`.
  - Acceptance evidence at 1.0: each preview feature specified with its label.

### Agent support

Owner instruction: "support for claude, codex, opencode and pi ... world class and industry
leading". Decisions: G19 (two block, two preview), G20 (new public repository
`beamfall/corvint-agents`; rebuild, logic in the binary), G21 (pinned gate, weekly latest). The
change proposal is `docs/specs/openspec/changes/agent-packages/` (written separately; this plan only
links it). This amends decision 0594 (which removed hooks and packages) and PRS-V1-006 by a
superseding decision.

- `PROD-INT-023`: (accepted, decision 0596; V1-5272) Claude Code and Codex MUST meet the full agent
  bar for 1.0. OpenCode and Pi ship the same generated package labelled preview until they pass, and
  never block. Pi is opt-in.
  - Source: G19.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: preview agent fails.
    - **WHEN** the OpenCode qualification fails at the freeze
    - **THEN** OpenCode ships labelled preview and the release is not blocked
  - Implemented by: `docs/specs/openspec/changes/agent-packages/`.
  - Acceptance evidence at 1.0: PROD-INT-026 passing for Claude Code and Codex.

- `PROD-INT-024`: (accepted, decision 0596; V1-5272) The full agent bar MUST be: one install command
  and `corvint agents doctor`; MCP registered (a CLI-backed tool where the agent has no MCP client);
  three generated skills (tests, docs, tasks); an after-edit hook that records touched paths and at
  end of turn reports the affected tests and the command, says "UNKNOWN, run all" when unsure, and
  flags edits whose tests never ran; a session-start readiness check; after compaction, restating
  only tests not yet run; a read-only instruction-file check. The hook only reads, never runs tests,
  never blocks the edit, has a bounded timeout, and emits no repository content or secrets. No
  context injection.
  - Changed: (changed 2026-11-09, owner decisions R4 and R5 (2026-11-09)) The end-of-turn report
    is a blocking Stop decision with a reason, at most one nudge per turn (loop guard); the
    restatement after compaction uses SessionStart with source `compact` instead of PostCompact
    (R5). The edit itself is still never blocked. A docs review finding in the report is advisory
    by default, with an opt-in setting that makes it blocking (R4).
  - Source: G19; owner decisions R4 and R5 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: hook timeout.
    - **WHEN** the after-edit hook exceeds its timeout
    - **THEN** the edit proceeds and the end-of-turn report says the affected set is UNKNOWN
  - Implemented by: `docs/specs/openspec/changes/agent-packages/`.
  - Acceptance evidence at 1.0: PROD-INT-026 transcripts.

- `PROD-INT-025`: (accepted, decision 0596; V1-5272) Agent logic MUST live in the `corvint` binary
  (`corvint agent hook <event> --host X`, `corvint agent manifest --json`). Thin per-agent packages
  are generated from one source against a pinned Corvint release in `beamfall/corvint-agents`, with
  a CI drift check. Each package declares its supported Corvint range and degrades visibly outside
  it. npm packages use trusted publishing with provenance, zero runtime dependencies and no install
  scripts. `CORVINT_HOOKS=off` turns the hooks off. Only event mappings, fixtures and the
  qualification harness may be reused from `841a58cbd^`.
  - Source: G20.
  - Note: G19 and G20 use both `corvint agents` and `corvint agent` as the verb; the proposal must
    pick one (open, checklist O33).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: out-of-range Corvint.
    - **WHEN** a package runs against a Corvint version outside its declared range
    - **THEN** it reports the mismatch visibly and does not pretend to work
  - Implemented by: `docs/specs/openspec/changes/agent-packages/`.
  - Acceptance evidence at 1.0: the drift check passing; packages published with provenance.

- `PROD-INT-026`: (accepted, decision 0596; V1-5272) "Supported" MUST mean a retained pass for the
  exact (agent version, package version, Corvint version, OS) on a scripted session with the real
  agent binary and the released package: install; MCP tools listed; an edit after which the hook
  names exactly the `corvint affected` tests and the agent runs them; unsure leads to run all; a
  hook crash or timeout does not block the edit; a docs query; a task claim on a scratch store;
  compaction restates pending tests; clean uninstall. Assertions are on hook and tool transcripts;
  retries are logged.
  - Changed: (changed 2026-11-09, owner decisions R27, R39 and R40 (2026-11-09)) Each agent host
    uses one pinned model; the model ID and agent version are recorded (R27). Qualification uses
    the owner's existing subscription logins, with no API keys and no spend cap; it runs on the
    owner's machines (logged-in CLIs on the Mac, WSL or Unraid hosts, or a self-hosted runner
    there), never on GitHub-hosted runners, and its result is a release evidence file (R39, which
    amends R26 and AGT-V1-029; the reading is confirmed in the acceptance record). Codex
    qualification uses the logged-in Codex CLI, not a Codex API key (R40).
  - Source: G21; owner decisions R27, R39 and R40 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: a new agent release breaks.
    - **WHEN** the weekly run against an agent's newest release fails
    - **THEN** that version is marked untested, the last pass stays advertised, a ticket is filed,
      and doctor warns
  - Implemented by: `docs/specs/openspec/changes/agent-packages/`.
  - Open: upstream facts are not verified: Codex hook, compaction and stop events; Pi's MCP
    client; OpenCode plugin events.
  - Acceptance evidence at 1.0: retained passes for versions pinned at the freeze, in the release
    evidence file, with model ID and agent version.

- `PROD-INT-027`: (accepted, decision 0596; V1-5272) The agent qualification MUST rerun on every
  package or Corvint change and weekly against each agent's newest release. 1.0 is gated on the
  versions pinned at the freeze.
  - Changed: (changed 2026-11-09, owner decision R39 (2026-11-09)) The per-change and weekly runs
    happen on the owner's machines with subscription logins, not on GitHub-hosted runners.
  - Source: G21; owner decision R39 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: Corvint change.
    - **WHEN** a Corvint commit lands that changes the hook output
    - **THEN** the qualification runs for each supported tuple before release
  - Implemented by: `docs/specs/openspec/changes/agent-packages/`.
  - Acceptance evidence at 1.0: the schedule running, with one weekly record.

### Revived preview verbs (G25)

The owner chose "Revive as proposed" for the context and guidance verbs. Items 1 and 2 are labelled
preview for 1.0 and never block the release. Each needs a committed proposal before any build. None
is designed in detail or checked against code yet.

- `PROD-INT-028`: (accepted, decision 0596; V1-5272) `corvint impact` MUST become the cross-product
  change report: tests to run, flows and variations touched, docs made stale, tickets and
  requirements touched, built on the `affected` engine plus the flow and docs joins. The separate
  impact engine in `contextindex` is removed. Preview; not release-blocking.
  - Source: G25 item 1.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: one change, four answers.
    - **WHEN** the user runs `corvint impact --base FULL_SHA`
    - **THEN** one report lists the tests, flows and variations, stale docs and touched tickets
  - Notes: (owner decision H8 (2026-11-10), marked low confidence by the owner) With `--base`
    omitted, `impact` uses the merge base of HEAD with the default branch, reports the base it
    chose, and refuses when it cannot determine one (CRI-V1-001).
  - Implemented by: CRI-V1-001 to CRI-V1-009 (`docs/specs/openspec/changes/change-report-impact/`).
  - Acceptance evidence at 1.0: the report on Corvint, labelled preview.

- `PROD-INT-029`: (accepted, decision 0596; V1-5272) `corvint review` MUST become a deterministic
  pre-merge evidence check: changed lines whose selected tests did not run, changed flows with an
  untested variation, unlinked E2E tests, stale docs, and ticket acceptance criteria without
  evidence. Its consumers are the Action PR comment, the agent end-of-turn report and the Tasks
  completion gate. Preview; not release-blocking.
  - Notes: (owner decision R4 (2026-11-09)) In the blocking consumers (the Tasks gate, the
    Action and the agent hooks) the docs findings are advisory by default, with an opt-in setting
    that makes them blocking.
  - Source: G25 item 2; owner decision R4 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: an unrun test.
    - **WHEN** a selected test for a changed line has no recorded result
    - **THEN** `corvint review` lists it and exits non-zero
  - Implemented by: the nine `review-evidence-check` requirements, numbered REC-V1-001 to
    REC-V1-009 in that proposal; the prefix is settled as RVE-V1 (C14)
    (`docs/specs/openspec/changes/review-evidence-check/`).
  - Note: depends on PROD-TEST-020 for recorded results.
  - Acceptance evidence at 1.0: the check on Corvint PRs, labelled preview.

- `PROD-INT-030`: (accepted, decision 0596; V1-5272) `corvint context` MUST become a ticket starting
  brief built from declared links only, with no ranking. It ships as redesigned, non-blocking, with
  no claim of measured benefit; the measurement of agent tool calls on a fixed task set runs in
  1.0.x.
  - Changed: (changed 2026-11-09, owner decisions R28 and R29 (2026-11-09)) It said the verb is
    kept only if it measurably cuts agent tool calls, and is otherwise removed. The keep-or-remove
    measurement moves to 1.0.x.
  - Source: G25 item 3; owner decisions R28 and R29 (2026-11-09).
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: no benefit claim.
    - **WHEN** the 1.0 documentation describes `corvint context`
    - **THEN** it states that the verb is non-blocking and makes no claim of measured benefit
  - Implemented by: TBC-V1-001 to TBC-V1-007 (`docs/specs/openspec/changes/ticket-brief-context/`).
  - Acceptance evidence at 1.0: the redesigned verb on Corvint, labelled non-blocking; the frozen
    task set for the 1.0.x measurement.

- `PROD-INT-031`: (accepted, decision 0596; V1-5272) `corvint index` MUST survive only as the warm
  cache for `corvint watch` if it helps meet the 1.0 s watch bar; otherwise it is removed, together
  with the snapshot path if watch does not need it.
  - Source: G25 item 5.
  - Intent: proposed. Delivery: not-started. State: MISSING.
  - Scenario: cache helps.
    - **WHEN** watch measured with and without the index differs enough to matter for the bar
    - **THEN** the index stays as watch's cache, and otherwise it is removed
  - Implemented by: RMV-V1 requirements in `docs/specs/openspec/changes/source-removal/` and
    WCH-V1 requirements in `docs/specs/openspec/changes/watch-seven-runners/`.
  - Acceptance evidence at 1.0: the with-and-without measurement.

## Proposal traceability

Each proposal under `docs/specs/openspec/changes/` states in its header which product requirements it serves. The
third column lists the plan requirements whose "Implemented by" names the proposal. Where the two
differ, both sides are reconciled before acceptance (checklist O31, Q1). Nothing here is accepted.

| Proposal | Header says it serves | Plan requirements that name it |
|---|---|---|
| `affected-plan-kinds` | PROD-TEST-003, 004 | PROD-TEST-003 |
| `agent-packages` (AGT-V1-001 to 030) | PROD-TEST-009, PROD-DOCS-001, PROD-TASK-007 | PROD-INT-023 to 027 |
| `change-report-impact` (CRI-V1-001 to 009) | PROD-TEST-003, 010; PROD-INT-001, 002 | PROD-INT-028 |
| `e2e-inventory-discovery` (AFU-V1-140 to 149) | PROD-TEST-004 | PROD-TEST-017 |
| `fault-injection-harness` (FIH-V1-001 to 015) | PROD-TEST-002, 011; PROD-INT-008 | PROD-TEST-014 |
| `feature-map-from-flows` (FMP-V1-001 to 007) | PROD-DOCS-004, 006, 011 | PROD-DOCS-013 |
| `flows-status-report` (AFL-V1-001 to 008) | PROD-TEST-010 | PROD-TEST-010 |
| `flows-unlinked-tests-report` (AFU-V1-155 to 159, 166, 167) | PROD-TEST-005 | PROD-TEST-005, 018 |
| `github-action` (GHA-V1, 36 requirements) | PROD-TEST-012; PROD-INT-006 | PROD-TEST-013; PROD-INT-014, 018 to 022 |
| `how-rewrite` (HOW-V1-001 to 012) | PROD-DOCS-007; PROD-INT-004 | PROD-DOCS-007 |
| `narrowing-measurement` (NMS-V1-001 to 011) | PROD-TEST-002, 008, 011; PROD-INT-007, 008 | PROD-TEST-013, 015, 016 |
| `non-code-map` (NCM-V1-001 to 009) | PROD-TEST-002, 008, 012 | PROD-TEST-019 |
| `proof-repositories` (PRR-V1-001 to 011) | PROD-TEST-002, 011; PROD-INT-008 | PROD-TEST-016 |
| `pytest-provider` (PYR-V1-001 to 010) | PROD-TEST-006, 008, 011 | PROD-TEST-006 |
| `review-evidence-check` (prefix settled as RVE-V1) | PROD-TEST-010, 012; PROD-INT-001 | PROD-INT-029 |
| `ruby-providers` (RBR-V1-001 to 012) | PROD-TEST-006, 008, 011 | PROD-TEST-006 |
| `runner-contract` (RNC-V1-001 to 018) | PROD-TEST-006, 011 | PROD-TEST-006, 013 |
| `source-removal` (RMV-V1-001 to 037) | PROD-TEST-002; PROD-INT-009, 010 | PROD-INT-017, 031; "Removed from 1.0" |
| `tasks-integration` (TIN-V1) | PROD-TASK-003, 004, 006 to 013; PROD-INT-004 | PROD-TASK-010 to 013 |
| `ticket-brief-context` (TBC-V1-001 to 008) | PROD-TASK-006; PROD-INT-002 | PROD-INT-030 |
| `watch-seven-runners` (WCH-V1-001 to 014) | PROD-TEST-007 | PROD-TEST-007, 013; PROD-INT-031 |

`narrowing-measurement` owns the `CORVINT_SELECTION` grammar, the refusal and the reason text
(NMS-V1-008); the other proposals cite it, including RNC-V1-008, WCH-V1-009 and GHA-V1-009. Owner
decision R13 (2026-11-09) fixes the behaviour for an unknown value (exit 2 in the CLI and CI; run
all tests in agent hooks and MCP) and the grammar `full:<runner>` everywhere; the MCP side is
carried by the planned `mcp-tool-table` (checklist O25).

Planned change folders (owner decisions R35 and R38 (2026-11-09), and the review fix list). Some
are written alongside this revision; each is proposed only. The order and prerequisites are in
"Proposal dependency table".

| Planned proposal | Serves | Owner decisions |
|---|---|---|
| `canonical-test-id` | PROD-TEST-006 | R11 (RNC-V1-012, fresh IDs), R38 |
| `test-inventory` | PROD-TEST-017 | R10, R38 |
| `run-evidence` | PROD-TEST-020, 010 | R7, R38 |
| `line-evidence` | PROD-TEST-008 | R8, R9, R38 |
| `flow-link-proposals` | PROD-DOCS-012; PROD-TEST-018 | R12, R38 |
| `jest-provider` | PROD-TEST-006, 008, 011 | R7, R8 |
| `per-test-durations` | PROD-TEST-016 | R19 |
| `release-process` | PROD-INT-012, 013, 017; milestones M0 and M5a | R2, R3 |
| `openspec-check` | PROD-INT-011 | G14, R41, H9, H12 |
| `flow-schema-version` | PROD-INT-016 | G5 |
| `macos-required-shards` | PROD-INT-015 | answer 16, R36, H11 |
| `mcp-tool-table` | PROD-DOCS-001, 010; PROD-TEST-009 | R13, R30 |
| `user-docs` | product gaps (a) to (f); PROD-INT-004, 017 | R42 B6 (O30, O64) |
| `compatibility-promise` (CMP-V1-001 to 006) | product gap (g) | R42 D10, D10a (O30, O64) |
| `tasks-conformance` (Tasks repository) | PROD-TASK-010 | R17, R18 |
| `tasks-policy-v15` | milestone M0 | R3 |
| `acceptance-record` | the one acceptance record (checklist O19) | R1, R9 |
| `spec-set-cleanup` | PROD-INT-010, 011 | R37, R41 |

## Product gaps (open)

These user-facing items have no requirement here. Q35 (NEW-PROC-1 to 3; checklist O30, O64) is
decided by R42 (panel pick) B6: all items ship as pages in the repository, with no site. Gaps (a)
to (f) are planned in the `user-docs` change folder ("Proposal dependency table"); the content of
the compatibility statement (g) was decided 2026-11-09 under R42, D10 and D10a. Gap (g) is served
by change `compatibility-promise`.

| Gap | Item | Owner question |
|---|---|---|
| (a) | First-run behaviour of each product | Q35, NEW-PROC-1 |
| (b) | Configuration reference: every file and variable | Q35, NEW-PROC-1 |
| (c) | Upgrade from pre-1.0: removed verbs and old files | Q35, NEW-PROC-2 |
| (d) | Error and exit-code catalogue | Q35, NEW-PROC-1 |
| (e) | Release notes | Q35, NEW-PROC-1 |
| (f) | Documentation site or in-repository pages | Q35, NEW-PROC-1 |
| (g) | 1.x compatibility promise | Q35, NEW-PROC-3 |

Gaps (h) to (k) are proposed in PROD-INT-006 (telemetry, uninstall), PROD-INT-018
(`SECURITY.md`) and PROD-INT-017 (per-product scenario).

## Removed from 1.0

The owner asked for everything else in source to be confirmed or planned for removal. The removal
plan is `docs/specs/openspec/changes/source-removal/` (written separately; this plan only links it).
Each removal fails closed: a path whose owner was removed becomes UNKNOWN, never silently unselected.
Panel file counts are estimates not yet checked by a build.

| Answer | Removed | Kept or moved | Decision it supersedes |
|---|---|---|---|
| G8 | CEM, OCM and dogfood ceremony; `corvint cem` verbs | Release gates of PROD-INT-017; recorded self-use (PROD-INT-009) | 0594 item 4 (kept `corvint cem`); `AGENTS.md` "Dogfood Corvint" section; invariant 4 amended by the change `acceptance-record` (ACR-V1-012, owner decision H5 (2026-11-10)); invariant 8 is not reworded (owner decision H4 (2026-11-10)) |
| G22 | `vscode-extension.yml` (separate `.github` PR), Makefile `host-adapter-test` and `vscode-check-install-test`, `conformance/vscode-extension-v0`, `tools/lsp-editors`, three `script/*lsp*.py` | `extensions/vscode` moves to the new public repository `beamfall/corvint-vscode` (owner decision R33 (2026-11-09), creation approved), frozen and unpublished; orphaned code is deleted and tracked in a table; a thin editor client on `corvint watch` is the first post-1.0 item | none named |
| G22 (panel, fold into the proposal) | third-party parts of external evidence (`examples/evidence-provider`, `docs/EXTERNAL-EVIDENCE-PROVIDERS.md`, MCP and command sources), `protocol/integrations/speckit`, `conformance/cli-parity-v0` (repoint `internal/gokernel/repository.go:390@1726b502`); archive the deployment-neutral index spec and `docs/EXTENSION-ECOSYSTEM.md` | `internal/extevidence` as an internal format | `AGENTS.md` invariant 7 reworded by the change `acceptance-record` (ACR-V1-012; owner decisions H4 and H5 (2026-11-10)) |
| G23 | `internal/runnerprotocol`, `runnerpolicy`, `runnerqualification`, `tools/runner-*` | compiled-in `registry.RunnerPlugin`, `runnerselection`, `testrunner`; one compiled-in contract for seven runners | external-protocol part of 0592 (RSP-V0-025, RSP-V0-026) |
| G24 | `corvint test-plan`, `tests lint`, `tests duplicates`, `internal/testplan`; `corvint runs`, `internal/playwrightruns`; `test-validity` and `tests validity`, `cmd/corvint-test-validity-mcp` (and its row in `tools/release-binary/binaries.json`), `internal/mcp/testvaliditybridge`, the `test-validity` MCP group, `internal/testsource/validity*.go`, tcq code unused outside tcq | `internal/testvalidity` projection types and state constants, tcq `Flaky` and result constants, the testvaliditydoc receipt decoder (`read.go` moves to `internal/pinnedread`), `internal/testevidence`, `corvint test-evidence`, `flows test-evidence` and its MCP group (preview) | 0594 Keeps line for these verbs |
| G25 | context ranking files, the old impact engine, feature heuristics, the snapshot path if watch does not need it | revived verbs of PROD-INT-028 to 031, PROD-DOCS-013 | 0594 Keeps line |
| G26 | all 16 `corvint-analyzer-*` binaries, analyzer packages, `analyzerenvelope`, `experimental/analyzers/sqlnative`; cfamily, dotnet, java, kotlin, rust, swift liveverify providers and their plugin files; swiftimpact, csharpregions, nativelinecov, rustsyntax, shaderdialect, the sql-native ratchet and toolchain receipt, pymutate, rbmutate | shell provider and Go; `testrunner/native` split keeping go-test; specs retired in place; `docs/CAPABILITIES.md` amended | 0594 Keeps line (needs a superseding decision). Accepted cost: beamfall-apple (Swift) and beamfall-android (Kotlin) run all tests until a provider is rebuilt after 1.0. Open: orphaned Go `liveverify/mutate` gets its own ticket |
| G27 | about 13 lane-process lints; lane and batch steps out of CI now; lane scripts, `LANE-ORCHESTRATION.md` and the lane-flow skill after the layering branch merges; `gate-ledger`, `gate-receipt`, `go-archive-gate`, `adopter-ci-advisory` at the action cut-over | about 15 product-protecting lints; `goos-filenames`, `mkfifo-build-tags`, `windows-main` folded into cross-vet; perf-budget kept, enforced on a quiet host only | none named |
| Answer 16 | native Windows vet and archive work from the 1.0 bar | WSL path | PRS-V1-004 wording |
| R6 | Cypress as a 1.0 runner (no change folder) | listed post-1.0; Cypress projects run all tests with a named reason | answer 2 and G1 (Cypress conditional) |
| R14 | Ruby and JS provider binaries and `corvint-test-runner` as separate 1.0 artifacts | each retires after its built-in runner contract lands (G23) and is folded into `corvint` | none named |
| R17 | a Tasks MCP server in 1.0 | Tasks is CLI only; the Tasks repository publishes schema v1 and a supported-command list | answer 11 |

## Milestones and order

Order follows G27, with testing at milestones only (no constant testing between them). Owner dates:
the M5a qualification candidate about 2026-12-08 (owner decision R2 (2026-11-09)), the freeze
2026-12-15 (G10a), and stable no earlier than 2027-01-05 (G10a, R2). Removals finish by 2026-12-01
(settled in review, completeness F6; confirm in the record). Removal verdicts and proof are one
build-log entry per removal batch (owner decision R34 (2026-11-09)), and `make windows-vet` runs
once per removal batch before its push (owner decision R36 (2026-11-09)). The build order of the
change folders is in "Proposal dependency table".

| No. | Milestone | Contents | Test at this milestone |
|---|---|---|---|
| M0 | Re-scope the `v1-0` release object (owner decision R3 (2026-11-09)) | Each open `v1-0` ticket is mapped to a plan requirement or moved to `post-1-0`, each move recorded; Tasks queue policy v15 without `cemRequired`, `ocmRequired`, `cem-verdict` and `context-focused` (`tasks-policy-v15`, `release-process`); the fresh-store cutover (`tasks-store-cutover`), run only after this plan and the ticket scheme (`ticket-taxonomy`) are accepted (R42, C11; TSC-V1-001) | The policy update read back; `receipt audit` on the store |
| M1 | Cut PR | `claude/bigcut-keepdocs` lands unchanged; no removals added to it. It already carries the decision 0586 licence text in `AGENTS.md` (see "Corrections", item 3). | PR CI on the cut PR |
| M2 | `.github`-only PR | Items 0594 deferred, `vscode-extension.yml`, `cem-release-gate.yml`, `intake-containment.yml`, the dangling `.github/workflows/release-gates.yml:90@3aa27b74` target, cishards entries; lane and batch steps out of CI; required-check list checked and the ruleset updated in the same window | Required CI checks |
| M3 | Removal lanes on the integration branch | G8, G23, G24, G26 removals and lint trims as the first lanes, before directory moves; compile-check only. Removals complete by 2026-12-01; a batch not merged by then moves to 1.0.x. `spec-set-cleanup` verdicts land with the matching removal batch (R41). Per batch: `go vet ./...` in both modules, `make windows-vet` once before the batch's push (R36), `make -n gate doc-gates shell-test release-binaries-check`, a zero-hit `git grep -F` for each deleted path over live paths, and one build-log entry (R34) | Per-batch reference closure (no tests) |
| M4 | Layering and release merge (about 2026-12-08), landed before the M5a tag (owner decision H7 (2026-11-10)) | The layering integration branch merges with the 1.0 feature lanes; lane tooling moves to a private operator repository after it, in batch B8b (R42, E9; RMV-V1-025) | `corvint affected --base <pre-batch SHA>` with the selected tests, plus required CI |
| M5 | Action cut-over | After 30 shadow PRs with no would-be miss (PROD-INT-021): first a `.github` PR, then the non-`.github` tools; ideally before the freeze, otherwise first 1.0.x change | Shadow record |
| M5a | Qualification candidate tag, about 2026-12-08 (owner decision R2 (2026-11-09)) | A qualification candidate is tagged from the integration result; it is rebuilt only for blockers. The Tasks RC is tagged the same day (R42, C22); publishing that tag is the owner's act. Tagged only after M4 lands (owner decision H7 (2026-11-10)). The held-out pools are sealed before the tag, and the tag's commit SHA seeds the draw (owner decision H3 (2026-11-10); PROD-INT-008). If the Playwright flow links are not signed by 2026-12-04, M5a slips until they are; Playwright is not demoted (owner decision H6 (2026-11-10)) | Tag recorded; candidate builds |
| M6 | Freeze, 2026-12-15 | Owner records demotions from the FIH and NMS verdicts on the M5a candidate (PROD-INT-012 notes, R2); nothing removed in the two weeks before except the action cut-over (removal panel r5) and the two late removal batches: B8b (by 2026-12-10, or the first 1.0.x) and B11 (by 2026-12-15) (R42, E5; REL-V1-009) | none |
| M7 | Per-product end-to-end qualification on the M5a candidate, before the freeze (R2) | PROD-TEST-014 to 016 per runner (R19); PROD-INT-008 pinned and held-out repositories; PROD-INT-017 scripted qualification per product; agent qualification on the owner's machines (R39, R40); Tasks conformance pack and tag (PROD-TASK-010, beamfall/core per R18) | Full qualification on the benchmark box, Linux and macOS (hosts proposed in PROD-INT-008 notes, O23); confirmed again on the RC |
| M8 | RC tag, then approve and publish | The RC tag is pushed first and the tag workflow builds the artifacts; owner signature 1 is the approval in the protected environment that publishes them (PROD-INT-014, R2) | Full suite in the tag workflow |
| M9 | Soak and trial | 7 days and at least 30 real PRs; 21-day Tasks trial on the frozen RC, 2026-12-08 to 2026-12-29 (R42, C22). A code change in the Corvint RC restarts the soak and the RC-publish clock; the Tasks trial restarts only on a store format change (TCF-V1-009); B8b and B11 restart a clock only if they change a shipped binary (owner decision H1 (2026-11-10); REL-V1-003) | Soak records |
| M10 | Stable, not before the latest of 2027-01-05, the end of the Tasks trial, the RC publish plus 21 days and the end of the soak (owner decision H1 (2026-11-10)) | Owner signature 2 | none beyond M9 |

Also on the path: Tasks specs are deleted from this repository only after `corvint-tasks` PR #4
merges (G27). The analyzer roster removal needs its own superseding decision (removal panel r5).

## Proposal dependency table

Owner decision R35 (2026-11-09) asks for one table with every change folder under
`docs/specs/openspec/changes/`, existing and planned. Two orders are fixed by the owner: the testing
build order `canonical-test-id`, `test-inventory`, `run-evidence`, `line-evidence`,
`flow-link-proposals`, with lanes overlapping where files do not collide (owner decision R38
(2026-11-09)); and `spec-set-cleanup` after `openspec-check` and `source-removal` (owner decision
R41 (2026-11-09)). Every other edge is a proposed order, checked again when each proposal is
written. Cypress has no change folder (owner decision R6 (2026-11-09)). Every folder is proposed;
none is accepted before the acceptance record (R1). State "existing" means the folder is on disk
with `proposal.md`, `design.md`, `tasks.md` and a spec delta; "planned" means this plan names it
and nothing is written yet. The states were refreshed on 2026-11-09: every folder in the table is
now on disk, so none is "planned". The four `-hardening` rows were added in the same refresh; their
edges come from the review verdict files and are proposed. The `compatibility-promise`,
`ticket-taxonomy` and `tasks-store-cutover` rows were added on 2026-11-10. The table has one row
for each of the 45 folders on disk, and `docs/specs/openspec/README.md` lists the same folders.

| Change folder | State | Needs first | Unblocks |
|---|---|---|---|
| `acceptance-record` | existing | this plan reconciled with the proposal headers (O31) | acceptance of every other folder; the R9 Python release exception |
| `tasks-policy-v15` | existing | none | `release-process` (M0) |
| `release-process` | existing | `tasks-policy-v15` for M0; `proof-repositories`, `fault-injection-harness`, `narrowing-measurement` and `per-test-durations` for M5a qualification | M5a, M8 publish; the signed decision file |
| `openspec-check` | existing | none | `spec-set-cleanup`; the archive stage of every later change |
| `source-removal` | existing | none (RMV-V1-034 alone waits for SLF-V1-007 and is in no batch B0 to B11) | `spec-set-cleanup`; `runner-contract` (external protocol removed, G23); `user-docs` (upgrade notes) |
| `spec-set-cleanup` | existing | `openspec-check`, `source-removal` | one workflow document; finished by 2026-12-01 |
| `runner-contract` | existing | none | `canonical-test-id`, `pytest-provider`, `ruby-providers`, `jest-provider`, `watch-seven-runners`, `github-action` |
| `canonical-test-id` | existing | `runner-contract` (it adds RNC-V1-012) | `test-inventory`, `pytest-provider`, `ruby-providers`, `jest-provider` |
| `test-inventory` | existing | `canonical-test-id` | `run-evidence`, `e2e-inventory-discovery`, `flows-unlinked-tests-report`, `watch-seven-runners` |
| `run-evidence` | existing | `test-inventory` | `line-evidence`, `per-test-durations`, `flows-status-report`, `review-evidence-check` |
| `line-evidence` | existing | `run-evidence` | `flow-link-proposals`, `narrowing-measurement` |
| `flow-link-proposals` | existing | `line-evidence`, `flows-unlinked-tests-report`, `flow-schema-version` | none |
| `pytest-provider` | existing | `runner-contract`, `canonical-test-id` | `line-evidence` (pytest at file level), `watch-seven-runners` |
| `ruby-providers` | existing | `runner-contract`, `canonical-test-id` | `line-evidence`, `watch-seven-runners` |
| `jest-provider` | existing | `runner-contract`, `canonical-test-id` | `line-evidence`, `watch-seven-runners` |
| `per-test-durations` | existing | `run-evidence` | `fault-injection-harness`, `narrowing-measurement` |
| `e2e-inventory-discovery` | existing | `test-inventory` (to be reconciled with it; fresh IDs per R11) | `flows-unlinked-tests-report` |
| `flows-unlinked-tests-report` | existing | `test-inventory`, `e2e-inventory-discovery` | `flow-link-proposals` |
| `flow-schema-version` | existing | none | `flows-status-report`, `feature-map-from-flows`, `flow-link-proposals` |
| `affected-plan-kinds` | existing | none | `flows-status-report`, `change-report-impact`, `review-evidence-check` |
| `flows-status-report` | existing | `affected-plan-kinds`, `run-evidence`, `flow-schema-version` | `review-evidence-check`, `change-report-impact` |
| `feature-map-from-flows` | existing | `flow-schema-version` | none |
| `review-evidence-check` | existing | `run-evidence`, `flows-status-report` | `agent-packages` (end-of-turn report) |
| `change-report-impact` | existing | `affected-plan-kinds`, `flows-status-report` | none |
| `non-code-map` | existing | none | `narrowing-measurement` |
| `proof-repositories` | existing | none | `fault-injection-harness`, `narrowing-measurement`, `release-process` |
| `fault-injection-harness` | existing | `proof-repositories`, `per-test-durations` | `narrowing-measurement`, `release-process` |
| `narrowing-measurement` | existing | `fault-injection-harness`, `line-evidence`, `non-code-map` | `release-process` (demotion verdicts), `github-action`, `mcp-tool-table` (kill switch) |
| `watch-seven-runners` | existing | `runner-contract`, `test-inventory`, `pytest-provider`, `ruby-providers`, `jest-provider` | none |
| `mcp-tool-table` | existing | none for the table; `narrowing-measurement` for the kill-switch rows | `agent-packages` |
| `agent-packages` | existing | `mcp-tool-table`, `review-evidence-check` | agent qualification (M7) |
| `github-action` | existing | `runner-contract`, `narrowing-measurement` | the action cut-over (M5) |
| `macos-required-shards` | existing | none (a `.github`-only PR) | the macOS release gate on the freeze and release candidates (REL-V1-015). MRS-V1-002 and MRS-V1-004 follow R42, D7 and REL-V1-015: the per-merge check stays advisory until the flake tickets close and 20 consecutive main runs pass, then is required, and stays advisory per merge for 1.0 if that has not happened by 2026-12-15 (owner decision H11 (2026-11-10)) |
| `how-rewrite` | existing | none | `user-docs` |
| `user-docs` | existing | `how-rewrite`, `source-removal`, `mcp-tool-table` | release notes for the RC (`release-process`) |
| `tasks-integration` | existing | none | `tasks-conformance`, `ticket-brief-context` |
| `tasks-conformance` | existing (built in the Tasks repository) | `tasks-integration` | the 21-day trial on beamfall/core (M9) |
| `ticket-brief-context` | existing | `tasks-integration` | none |
| `selection-hardening` | existing | `fault-injection-harness` (its SLF-V1-006 gate) | `affected-plan-kinds` (one vocabulary for unknowns), `source-removal` (RMV-V1-034 only: the second Go attribution; no other RMV requirement waits) |
| `watch-flows-hardening` | existing | `affected-plan-kinds`, `flow-link-proposals` | `watch-seven-runners` (warm-state invalidation, WFF-V1-002) |
| `docs-mcp-hardening` | existing | `mcp-tool-table`; `how-rewrite` now owns the former DMF-V1-012 as HOW-V1-012 (R42, B1) | `change-report-impact` (docs check report), `user-docs` |
| `tasks-hardening` | existing (built in the Tasks repository) | none | `tasks-conformance` (schema v1 freeze) |
| `ticket-taxonomy` | existing (built in the Tasks repository) | `tasks-hardening` (build items B1 to B3 land with it by 2026-11-27; R42, C18) | `tasks-store-cutover` (TSC-V1-001); `tasks-conformance` (schema v1 freeze, 2026-12-01) |
| `tasks-store-cutover` | existing | `ticket-taxonomy` and this plan accepted (R42, C11; TSC-V1-001) | M0; the Tasks trial store |
| `compatibility-promise` | existing | none | `user-docs` (the compatibility page) |

## Corrections to the first draft

The first draft of this file (commit `1d9dc4c90`) contained these errors or omissions.

1. Watch latency budget. The draft said `internal/perfbudget/budgets.json` "holds no budget for"
   watch. That is true at the draft's base `32140bb3b` and on `origin/main`, where `watch-replan`
   does not occur. It is false on the integration branch `claude/layers-integ` (integ, `9ec3a29d0`):
   `internal/perfbudget/budgets.json` defines scenario `watch-replan` (advisory p95 target
   1000 ms, PRB-V0-1014; enforced provisional ceilings 2500 ms p95 and 64 MiB peak RSS) and
   `watch-replan-self`, followed by the baseline samples. The budget was added by
   commits `2df03c7d8`, `0a5e38d42` and `5ce5029b6`. Gaps remaining against G7: target advisory not
   enforced, no p50 target, Go fixtures only.
2. Watch is a `corvint` command and Go-only. The draft said the loop was the tool `linecov watch`.
   On the integration branch it is `corvint watch` (`cmd/corvint/watch.go`), Go-only (the command
   refuses a non-Go index), running tests by default unless `--dry-run`, with the macOS kqueue
   source not observed live (`docs/specs/live-proof-carrying-verification-v0.md:14@3bb12604`;
   the first draft cited line 1285, which is wrong). G7 needs all seven runners and
   list-by-default.
3. `AGENTS.md` licence text. `LICENSING.md:5-10@6536f97b` says the whole repository is Apache-2.0 since
   decision 0586 (`docs/decisions/0586-relicense-apache-2-0-2026-10-29.md`). The pushed branch
   `claude/bigcut-keepdocs`, `origin/main`, this worktree (`AGENTS.md:161@b8bc96f4`) and the
   integration branch all carry the 0586 text in their `AGENTS.md` licence paragraph. Only the
   primary checkout's stale local `AGENTS.md` still shows the AGPL wording; it is not a branch head.
   Required amendment: the cut PR (M1) must not regress `AGENTS.md` to the AGPL text; the G8, G22
   amendments (Dogfood section, invariants 3, 4, 5 and 7, the invariant edits made by the change
   `acceptance-record` under ACR-V1-012) land on top of the 0586 text; invariant 8 is not
   reworded (owner decisions H4 and H5 (2026-11-10)). Related: the
   agents panel's statement that a permissive licence "needs decision 0002 amendment if in the main
   repo" rests on the stale text; the main repository is already Apache-2.0.
4. "Last recorded result" has no writer. PROD-TEST-010 and the flows status proposal report each
   test's last recorded result, but: `flows status` reads `test-run-evidence/0` records only from
   files passed with `--evidence` (the result source section of the `flows-status-report`
   proposal); `flows ingest` writes records to standard output only (the ingest command in
   `internal/command/flows/flows.go`, integ); `corvint test` writes `corvint-test-run/0`
   receipts under `<git-dir>/corvint/` (`cmd/corvint/test.go`, integ), which no flow reader
   consumes. No component persists results where a report finds them by default. PROD-TEST-020 adds
   the missing writer.
5. Conflicts with PRS-V1-001 and PRS-V1-018 were not listed. See the next section.

## Conflicts with accepted text

Each needs a resolution in the acceptance decision of answer 18, which supersedes the pre-cut 1.0
contract and carries over what is still valid. Rows that a review-round owner decision now settles
name it; the acceptance record still restates them.

| Accepted text | What it says | Conflict | Resolution needed |
|---|---|---|---|
| PRS-V1-001 (`docs/specs/corvint-1.0-product-and-release-v1.md:324@220578f8`) | 1.0 Core is exactly the local change-evidence loop, the CEM, OCM and frontier proof wire, the dogfood loop, the three Core jobs of decision 0332, and the native release artifact | G8 removes the ceremony and `corvint cem`; decision 0594 and answer 13 define three products, not Core plus companions | Supersede with the three-product definition of "The 1.0 bar"; carry over the native release artifact and lifecycle clause into PROD-INT-006 and PROD-INT-014 |
| PRS-V1-018 (`docs/specs/corvint-1.0-product-and-release-v1.md:384@1f9a7848`) | Stable 1.0 ships five SDD improvements, V1-1344 to V1-1348: (a) `record` citation class, (b) dogfood-check OCM coverage line, (c) generated spec header rows, (d) recorded acceptance mode per requirement with an audit command, (e) `dogfood finish-lane` | G8 closes V1-1344 to V1-1348 as superseded; (b) and (e) depend on the removed ceremony | Supersede. (c) and (d) carry over to 1.0 and (a), (b), (e) drop (owner decision R37 (2026-11-09)); see PROD-INT-010 and 011 |
| PRS-V1-004 | Core platforms darwin/arm64 and linux/amd64; Windows unsupported | G15a builds linux/arm64 too; answer 16 adds WSL | Amend platform list; keep Windows native unsupported |
| PRS-V1-006 | Host scope is plain CLI plus Codex CLI and Claude Code at FALLBACK | G19 sets a full bar for Claude Code and Codex, preview for OpenCode and Pi | Supersede by the agent decision (PROD-INT-023 to 027) |
| PRS-V1-007 | No CEM, OCM or frontier interoperability claim unless V1-0014 passes | G8 closes V1-0014 | Drop; no interoperability claim in 1.0 |
| PRS-V1-008 | Core candidate passes three Core jobs on Corvint, Beamfall and one untouched repository | G12 sets seven pinned plus held-out repositories (four drawn, one per family, owner decision H3 (2026-11-10)) | Supersede by PROD-INT-008 |
| PRS-V1-013 | Qualified Flows, navigation, flow docs and MCP tools | Consistent in direction; G5 and G6 change file and command homes | Carry over, pointing to PROD-TEST-004, PROD-DOCS-004 to 006, 011 |
| PRS-V1-014, PRS-V1-016 | Tasks replaces Beamfall's roadmap workflow | Consistent with answer 12; G9 adds the 21-day trial and coupling | Carry over, pointing to PROD-TASK-009 to 013 |
| PRS-V1-015 | Flow docs never accept intent or publish themselves | Consistent with answer 7 | Carry over into PROD-DOCS-002 and 005 |
| Decision 0433 (`docs/decisions/0433-rc2-stable-signing-and-support-policy-2026-09-29.md`) | No signing | G15a adds a build attestation; G20 adds npm provenance | Supersede the no-signing choice for the build attestation (G15a) and npm provenance (G20); keep its support window and its rule against an agent-held signing identity (settled in review; checklist O26) |
| Decision 0594 item 4 | Keeps `corvint cem` verbs | G8 removes them | Amend in the G8 decision |
| Decision 0594 Keeps line | Keeps test-quality verbs, analyzers, context verbs | G24, G25, G26 | Superseding decisions per removal |
| Decision 0595 | Flows and `how` move to docs; only `flows test-evidence` native | G5 makes the test-joining reports native | Amend 0595 |
| Decision 0592 | Runner plugins in both compiled and external forms | G23 drops the external protocol | Supersede the external part |
| `tasks-mcp-read-profile-v0.md` (Tasks repository) | Tasks tools served by `corvint mcp` | Answer 11; owner decision R17 (2026-11-09): no Tasks MCP server in 1.0 | Retire or mark outside 1.0 in the Tasks repository |
| `live-proof-carrying-verification-v0.md` header | Delivery `not-started` | Parts implemented (integ) | Correct the header |
| Answer 3 against G2 | Answer 3 required Playwright to be line-level | G2 narrows browser E2E by flows plus reachability only | G2 amends answer 3 for Playwright; line-exact set is Go, RSpec, Minitest, Vitest and Jest, with pytest at file level (owner decisions R7 and R8 (2026-11-09)); Cypress is dropped (R6) |
| Answer 8 against G24 | Answer 8 keeps run evidence from recorded E2E | G24 removes `corvint runs` and `internal/playwrightruns` | Run evidence is kept through `flows ingest` and the append-only PROD-TEST-020 record (owner decision R7 (2026-11-09)) |
| Answer 9 against proposals for V1-5255 and V1-5257 | Covered = linked test of any kind | The proposals assume E2E-only links | Amend both proposals |

## Work in flight

The table from the first draft is replaced by this summary, read on 2026-11-09. Owner decision R1
(2026-11-09): in-flight work continues as experimental, and the acceptance record states this.
Nothing may be described as delivered before that record exists.

| Work | Requirements | Where it is |
|---|---|---|
| V1-5251 one plan with kinds and flows | PROD-TEST-003, 004 | Merged into `claude/layers-integ` (`8cd570acb`); needs the G13 `kindSource` and G6 file name |
| V1-5255, V1-5257 flow reports | PROD-TEST-005, 010 | Merged into `claude/layers-integ` (`3c4cda623`); needs answer 9 amendment and PROD-TEST-020 |
| V1-5254 MCP affected | PROD-TEST-009 | `corvint.affected` present on `claude/layers-integ` |
| V1-4372, V1-5253 watch | PROD-TEST-007 | `corvint watch` and budget on `claude/layers-integ`; Go only |
| Miss kill switch lane (`misskill`) | PROD-TEST-002 | Merged into `claude/layers-integ` (`e3e159adf`) |
| V1-5252 E2E inventory | PROD-TEST-017 | Proposal only |
| V1-5256 multi-runner `corvint test` | PROD-TEST-006 | Lane state not re-read |
| Flows move to docs | PROD-DOCS-011 | Decision 0595 on `claude/layers-integ`; needs G5 amendment |

## State counts

The counts follow the State field of each requirement above.

| Group | Requirements | DELIVERED | PARTIAL | MISSING |
|---|---|---|---|---|
| Testing | 20 | 0 | 15 | 5 |
| Documentation | 13 | 0 | 11 | 2 |
| Task manager | 13 | 0 | 8 | 5 |
| Integration and release (001 to 017) | 17 | 0 | 11 | 6 |
| GitHub Actions (018 to 022) | 5 | 0 | 0 | 5 |
| Agent support (023 to 027) | 5 | 0 | 0 | 5 |
| Revived preview verbs (028 to 031) | 4 | 0 | 0 | 4 |
| Total | 77 | 0 | 45 | 32 |

The task manager states marked PARTIAL were not verified from this repository.

## Owner acceptance checklist

Answer 18: one decision record accepts this plan and supersedes the pre-cut 1.0 contract. The rows
below separate what the owner has already decided on 2026-11-09 (to be restated in that record)
from what is still open. Nothing is accepted until the record exists.

### Decided by owner answers (restate in the acceptance record)

| No. | Statement | Answer |
|---|---|---|
| 1 | 1.0 is three products plus integration and release; each works alone; loose integration by files and IDs | Owner goals; answer 13 |
| 2 | Watch answers within 1 second (p95 1.0 s, p50 300 ms) for all seven blocking runners | Answer 1; G7 |
| 3 | Line-exact narrowing in 1.0 for Go, RSpec, Minitest, Vitest and Jest; pytest selects by file with an explicit tier label, recorded as a release exception; browser E2E narrowed by flows plus reachability | Answer 3; G2; R8; R9 |
| 4 | Seven blocking runners; Cypress dropped from 1.0 and listed post-1.0; others full run with a reason | Answer 2; G1; R6; R7 |
| 5 | CI narrowing is a product feature for GitHub Actions only, with kill switch and periodic full runs | Answer 6 |
| 6 | Drift detection, run evidence, `corvint how` with a rewritten spec, and docs search are in 1.0 | Answer 8 |
| 7 | Flow reports joining test evidence are native in `corvint`; authoring, discovery and `how` in `corvint-docs` | G5 (amends 0595) |
| 8 | Tasks is CLI only in 1.0, with no Tasks MCP server; `corvint mcp` has no tasks tools | Answer 11; R17 |
| 9 | Beamfall takeover stays a 1.0 condition; coupled release; 21-day trial on `beamfall/core` | Answer 12; G9; R18 |
| 10 | Zero layer violations at 1.0; flow reader split | Answer 14 |
| 11 | Pinned public repositories per runner plus four held-out repositories, one per family, drawn from sealed pools and never redrawn (the R21 names and the A14 Go pick may be pool candidates), under a hashed rule | Answer 17; G12; G12a; R21; owner decision H3 (2026-11-10) |
| 12 | Separate inventory step; `affected` only reads it | Answer 4 |
| 13 | Accept the plan once; auto-accept implementing proposals; OpenSpec change folders with propose, apply and archive stages run with the pinned OpenSpec command, `openspec validate --all` passing, and a Go check | G14; R1; R41; owner decision H12 (2026-11-10) |
| 14 | Zero-miss by fault injection; replay reports size only | G3 |
| 15 | Tiered narrowing bar | G4 |
| 16 | One flow file `.corvint/flows.json`; flow selects, variation shown | G6 |
| 17 | CEM, OCM and dogfood ceremony replaced; `corvint cem` removed | G8 |
| 18 | Definition of done, qualification candidate (M5a) about 2026-12-08, freeze 2026-12-15, stable no earlier than 2027-01-05, owner signs twice | G10; G10a; R2 |
| 19 | Unlinked E2E tests always run; links proposed and accepted | G11 |
| 20 | Test kind enum and kind source, reporting only | G13 |
| 21 | Tag-started release with protected owner approval; no notarization, install script or Homebrew | G15a |
| 22 | Non-code files: full by default, proven map entries narrow | G15b |
| 23 | Lean GitHub Action blocks; preview list; new repository `beamfall/corvint-action`; shadow then cut-over | G16; G17; G18 |
| 24 | Claude Code and Codex block, OpenCode and Pi preview; new repository `beamfall/corvint-agents`; pinned gate, weekly latest | G19; G20; G21 |
| 25 | Removals of G22, G23, G24, G26, G27; revival of `impact`, `review`, `context` per G25 | G22 to G27 |
| 26 | Platforms Linux and macOS; Windows through WSL only | Answer 16 |
| 27 | No work dispatched without a committed proposal or a cited existing requirement | G14; owner goals |

### Still open (needs an owner yes, no or value)

The "Answered by" column records the owner decision of 2026-11-09 that answers a row; each named
decision is restated in the acceptance record (R1). "Settled in review" marks a reviewer settlement
the acceptance record must confirm; it is not an owner answer. "Open" rows still need the owner.

| No. | Statement to accept, reject or fill | If rejected or left open | Answered by |
|---|---|---|---|
| O1 | This file, as rewritten, states the decided items correctly (re-read the changed requirements) | The plan is corrected before the acceptance record | Owner decision R1 (2026-11-09): one record accepts the plan as amended and names the wording to re-read |
| O2 | The nine Tasks requirements frozen by G9 are PROD-TASK-001 to 009 | The Tasks scope list is restated | Owner decision R17 (2026-11-09): confirmed |
| O3 | "JS" in answer 3 means Vitest and Jest | The line-level mandatory set is restated | Owner decision R8 (2026-11-09): Vitest and Jest are line-exact |
| O4 | G2 amends answer 3 for Playwright and Cypress | Playwright line-level stays a release blocker | Owner decisions R7 and R8 (2026-11-09) for Playwright; Cypress dropped by R6 |
| O5 | Run evidence survives G24 through `flows ingest` plus a new result writer (PROD-TEST-020) | Answer 8's run evidence has no path in 1.0 | Owner decision R7 (2026-11-09): PROD-TEST-020 append-only run result record |
| O6 | Add PROD-TEST-020 (result writer) | Flow status shows `NOT_OBSERVED` unless files are passed by hand; `review` (PROD-INT-029) lacks input | Owner decision R7 (2026-11-09): added |
| O7 | Which of PRS-V1-018 (c) and (d) carry over | All of PRS-V1-018 drops | Owner decision R37 (2026-11-09): (c) and (d) carry over; (a), (b), (e) drop |
| O8 | Add linux/arm64 to the platform list in place of PRS-V1-004 | The release builds only what PRS-V1-004 names | Settled in review: three platforms including linux/arm64; confirm in the record |
| O9 | The G16 trust rules (PROD-INT-020) | The action proposal leaves fork and config trust unstated | Owner decisions R23 and R24 (2026-11-09) |
| O10 | Licence of `beamfall/corvint-action` and `beamfall/corvint-agents` (Apache-2.0 or MIT) | The repositories are not created | Owner decision R25 (2026-11-09): Apache-2.0 |
| O11 | Agent API keys and CI spend for G21 | Agent qualification cannot run in CI; Claude Code and Codex cannot pass PROD-INT-026 | Owner decisions R39 and R40 (2026-11-09): subscription logins and the Codex CLI on the owner's machines, no API keys or spend cap (amends R26) |
| O12 | Name and visibility of the VS Code extension repository (G22) | `extensions/vscode` stays in place, unpublished | Owner decision R33 (2026-11-09): public `beamfall/corvint-vscode` |
| O13 | Swaps among the G12a named repositories after the rule check | The named list stands only where it passes the rule | Partly: owner decision H3 (2026-11-10) for the held-out repositories (drawn from sealed pools, never swapped or redrawn; it supersedes R21's fixed list); swaps among the other G12a repositories are still reported to the owner |
| O14 | Behaviour for a repository that declares a removed runner (G26) | Fail-closed UNKNOWN is assumed | Owner decision R32 (2026-11-09): run all tests with a named reason |
| O15 | `context` is not release-blocking until measured (PROD-INT-030) | `context` needs its own bar | Owner decisions R28 and R29 (2026-11-09): non-blocking, measured in 1.0.x |
| O16 | Work in flight on `claude/layers-integ` may continue as experimental until acceptance | Those lanes stop until the record exists | Owner decision R1 (2026-11-09): in-flight work continues as experimental |
| O17 | The spec health classes and archive-in-place are the direction for the spec set | All surviving specs stay in scope | Owner decision R37 (2026-11-09); cleanup by `spec-set-cleanup` (R41) |
| O18 | Whether an "analyzers" layer remains after G26 | The layer list in PROD-INT-001 is restated | Settled in review: four layers; confirm in the record |
| O19 | The acceptance decision record itself (answer 18): this file becomes the 1.0 product contract | This file stays a proposal and governs nothing | Owner decision R1 (2026-11-09) fixes its form; the record is drafted, proposed and unsigned, at `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (change `acceptance-record`) |
| O20 | Demotion verdicts at the freeze come from the FIH and NMS verdicts on a named freeze-candidate SHA; a later regression is a blocking miss (PROD-INT-012 notes; timing waits for Q2) | Demotions rest on the owner's judgement alone | Owner decision R2 (2026-11-09): verdicts come from the M5a qualification candidate |
| O21 | Soak count: GitHub API count of Corvint and Beamfall PRs merged in the window, plus replayed held-out PRs, reported separately; whether replays count toward 30 (Q19) | The 30-PR count has no stated source | Settled: owner decision R20 (2026-11-09) fixes the agreement figure; R42 (panel pick) C9: replays count, reported separately, at least 20 of 30 live |
| O22 | Trial minimum: 50 ticket writes and 5 completions per trial repository, from the store event log | The 21-day trial has no activity floor | Settled: R42 (panel pick) C9 |
| O23 | Benchmark hosts: WSL2 Linux on the Windows benchmark box and the owner's Mac, reserved with no lanes from freeze candidate to RC (Q20) | The benchmark host stays unnamed | Settled: decided 2026-11-09 under R42, D8 (REL-V1-011). The hosts are reserved only during declared measurement sessions, which replaces "no lanes from freeze candidate to RC"; a result is NOT_VERIFIED if a lane ran |
| O24 | M3 removals complete by 2026-12-01; later batches move to 1.0.x | Removals may run into the freeze window | Settled in review; confirm in the record |
| O25 | `narrowing-measurement` owns the `CORVINT_SELECTION` grammar, refusal and reason text; an MCP kill-switch requirement is missing; unknown values stay open (Q13) | Each proposal defines its own kill switch | Owner decision R13 (2026-11-09): exit 2 in CLI and CI, run all in hooks and MCP, `full:<runner>` everywhere |
| O26 | Decision 0433's no-signing choice is superseded for the build attestation and npm provenance only | Decision 0433 conflicts with G15a and G20 | Settled in review; confirm in the record |
| O27 | No telemetry; uninstall by deleting binaries and `.corvint/` (PROD-INT-006 notes) | Telemetry and uninstall stay unstated | Settled in review; confirm in the record |
| O28 | `SECURITY.md` copied into `corvint-action` and `corvint-agents` | The new repositories have no security policy | Settled in review; confirm in the record |
| O29 | Per-product qualification is the "install alone" scenario plus the primary verb on a pinned repository | PROD-INT-017's scenario stays undefined | Settled: decided 2026-11-09 under R42, D9 and M7 (REL-V1-012); the repository is untouched and pinned |
| O30 | Product gaps (a) to (g) | Decided under Q35 | Settled: R42 (panel pick) B6, all ship as repository pages; (g) decided 2026-11-09 under R42, D10 and D10a: a narrow promise in `docs/COMPATIBILITY.md`, change `compatibility-promise` |
| O31 | Proposal headers and plan "Implemented by" lines are reconciled ("Proposal traceability"; Q1) | Traceability stays inconsistent | Partly: owner decision R35 (2026-11-09) adds the dependency table and fixed-commit citations; header reconciliation still to do |

### Open items with no row before this revision

Each row maps an open item to the owner question that covers it and, where one exists, to the
owner decision of 2026-11-09 that answers it. Rows marked "settled without the owner" record a
reviewer settlement for the owner to confirm.

| No. | Open item | Owner question | Answered by |
|---|---|---|---|
| O32 | C5: RC and stable calendar against the 21-day trial | Q2 | Owner decision R2 (2026-11-09) |
| O33 | C7: `agents doctor` (G19) against `agent hook` and `agent manifest` (G20) naming | Q27 | Partly: owner decision R27 (2026-11-09) pins one model per agent host; verb naming decided 2026-11-09 under R42, D4: `corvint agent doctor`, `agent hook`, `agent manifest`, no alias |
| O34 | C8: G27 retires `gate-ledger` and `gate-receipt`, which `make gate` uses; decisions 0224 and 0352 need superseding. The interim handling is settled without the owner | No question number yet | Settled by G27 and G18 (panel E2, R42): 0224 and 0352 stay in force until the B11 action cut-over, the change that lands B11 supersedes them, and both stay if B11 slips past 1.0. No change folder carries the superseding text yet |
| O35 | C9: invariant 4 against agent-hook session state; the `--learn-slot-weights` exception; invariant 7 wording | Q28 | Settled: decided 2026-11-09 under R42, D17. Invariant 3 drops "and learned task traces"; invariant 4 drops the `--learn-slot-weights` clause and states that agent hook session state in the user cache directory is not repository or trace state; invariant 5 becomes "Corvint 1.0 learns nothing from use". Every edit of invariants 3, 4, 5, 7 and 8 is made by the change `acceptance-record` (ACR-V1-012, owner decision H5 (2026-11-10)), not `agent-packages`; invariant 7 is reworded and invariant 8 is not (owner decision H4 (2026-11-10)) |
| O36 | C10: Cypress must-run (answer 2) against conditional (G1) | Q6 | Owner decision R6 (2026-11-09): Cypress dropped from 1.0 |
| O37 | C11: `review` is preview (G25) while review evidence blocks completion (G9) | Q4 | Owner decision R4 (2026-11-09): advisory by default, opt-in blocking |
| O38 | C12: G5 partly reverses decision 0595 and breaks CMS-V0-915/916 layer tests | No question number yet | Decision settled by G5 (panel B7, R42). Paperwork open: one superseding decision file for 0595, and CMS-V0-915/916 and the layer test rewritten with `flows-status-report` |
| O39 | C13: `context` on 0594's Keeps line; release smoke uses `corvint context`; settled without the owner | Confirm in the record | Confirm in the record |
| O40 | C14: the review prefix is RVE-V1; settled without the owner | Confirm in the record | Confirm in the record |
| O41 | D1: `corvint test` skips changes in removed languages | Q32 | Owner decision R32 (2026-11-09): run all tests with a named reason |
| O42 | D2: keep or remove `internal/docviews` | Q33 | Owner decision R33 (2026-11-09): orphaned code is deleted and tracked in a table |
| O43 | D5: kill-switch syntax per runner | Q13 | Owner decision R13 (2026-11-09): `full:<runner>` |
| O44 | D6: minimum PR count for non-Go replay measurement; cost cap | Q19 | Owner decision R19 (2026-11-09) |
| O45 | D8: retire or keep the Ruby and JS provider binaries and `corvint-test-runner` | Q14 | Owner decision R14 (2026-11-09): retire each after its built-in runner contract lands |
| O46 | D9: Minitest results from the rubylinecov hook; settled without the owner | Confirm in the record | Confirm in the record |
| O47 | D10: fold read declarations into `data-map.json` | Q12 | Owner decision R12 (2026-11-09) for flow links; the low-confidence D10 fold stays for the proposal |
| O48 | D11: PROD-TASK-001 to 009 mapping to the Tasks repository; schema and receipt reference names; test-run receipt ID | Q17 (receipt ID partly Q3) | Owner decision R17 (2026-11-09): schema v1 and a supported-command list |
| O49 | D12: second 21-day trial repository | Q18 | Owner decision R18 (2026-11-09): `beamfall/core` |
| O50 | D13: `how` rewrite, re-spec or re-implement | Q30 | Owner decision R30 (2026-11-09): one `corvint-docs mcp` server with one tool table |
| O51 | D14: Codex non-interactive install and hook trust; Pi MCP client | Q27 | Partly: owner decisions R27 and R40 (2026-11-09); Pi decided 2026-11-09 under R42, D5: Pi registers MCP if its client is confirmed usable, else the CLI-backed tool |
| O52 | D17: action fail-closed rules beyond O9 | Q24 | Owner decision R24 (2026-11-09) |
| O53 | D18: 30-PR shadow match as PROD-TEST-012 acceptance; bespoke CI selection to replace | Q22 | Owner decision R22 (2026-11-09) |
| O54 | D19: when to drop the `e2e-flows.json` read; inventory refresh and `flows ingest` locations | Q10 | Owner decision R10 (2026-11-09): one inventory, `corvint inventory refresh` |
| O55 | D20: watch targets, reference machine and repository | Q16 | Settled: owner decision R16 (2026-11-09) fixes the tiers; R42 (panel pick) A11 names the machines and repositories |
| O56 | D21: impact ticket matching; acceptance-criteria IDs; ticket-brief keep threshold | Q29 | Settled: owner decisions R28 and R29 (2026-11-09); R42 (panel pick) C10 fixes ticket matching |
| O57 | D22: the parts of D22 beyond O3 | Q7 | Owner decision R7 (2026-11-09) |
| O58 | D24: only Go, Vitest and Playwright run through `corvint test` today; build order | Q38 | Owner decision R38 (2026-11-09) |
| O59 | D25: add-only rule for provider files, receipts and inventories; settled without the owner | Confirm in the record | Confirm in the record |
| O60 | PROD-INT-003 and PROD-INT-007 wording (marked PROPOSED) | Q1; PROD-INT-007 also Q35 | Owner decision R1 (2026-11-09): the record names the wording |
| O61 | PROD-TASK-006 source (PROPOSED) and PROD-TASK-001 to 005 wording | Q17 | Owner decision R17 (2026-11-09) |
| O62 | Feature map is preview as the output of `docs flows generate`; settled without the owner | Confirm in the record | Confirm in the record |
| O63 | Where GHA-V1-040 to 043 live: `github-action` or `public-release-v0.md` | No question number yet | Settled: decided 2026-11-09 under R42, D6: change `github-action` |
| O64 | NEW-PROC-1 to 3: user documents in the bar, upgrade behaviour, 1.x compatibility | Q35 | Settled: R42 (panel pick) B6 (R35 answered the dependency table and citations); (g) decided 2026-11-09 under R42, D10 and D10a: a narrow promise in `docs/COMPATIBILITY.md`, change `compatibility-promise` |
| O65 | NEW-PROC-4: keep or remove `make windows-vet` and its CI step after answer 16 | Q36 | Owner decision R36 (2026-11-09): once per removal batch, before its push |

| O66 | A4: the `e2e-safe` selection profile and the appmap navigation verdict leave the public surface in 1.0 (RMV-V1-037). This supersedes the Core classification in decision 0385 (AFU-V1-019 to 024) and the `corvint impact`, `affected` row of `corvint-1.0-product-and-release-v1.md` | No question number | Settled: decided 2026-11-09 under R42, A4; the supersession takes effect only when the owner accepts this record |
| O67 | E10: the "Implemented by" lines and the dependency table are generated from proposal headers, and a check fails on mismatch | O31 | Settled as a fix under R42, E10; the check is built by the change `openspec-check`; until then the table is hand-kept and may lag |

O19 is on the critical path for every proposal: no proposal is accepted until the record exists.
