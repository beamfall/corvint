# Proposed: use OpenSpec's change workflow next to the Corvint spec set

- Date: 2026-11-09
- Status: proposed. Not accepted. This file has no decision number and no row in
  `docs/decisions/README.md`. Only the owner can accept it, in whole or in part.
- Recommendation: Option B in "Options and recommendation" below.
- Asked for by the owner on 2026-11-09: "we should make sure we are taking advantage of flows in
  OpenSpec to ensure we are doing this in an industry standard way", and later "we really want to
  make sure we use OpenSpec and SDD to make sure we keep on top of everything. everything should
  be documented before being built".
- Read with: `docs/specs/PRODUCT-PLAN-1.0.md` (the requirement named PROD-INT-011 and item 22 of
  its acceptance checklist), `docs/specs/SPEC-HEALTH-2026-11-09.md` and
  `docs/specs/openspec/README.md`.

## The proposal in short

1. Every change starts with one committed change proposal in OpenSpec's form. Nothing is
   dispatched before that file exists.
2. The capability specs under `docs/specs/` stay the contract of record, with their numbered
   requirement IDs, generated index files and checks unchanged.
3. The two are joined by one rule: each requirement name in a proposal begins with the Corvint
   requirement ID it will carry in the capability spec.
4. A proposal is archived only after the owner's acceptance is recorded in the capability spec.
5. The existing specs are not converted to OpenSpec's format.

## What was read

OpenSpec was read from its own repository, `Fission-AI/OpenSpec`, through the GitHub API on
2026-11-09 at commit `579fcac57ae00c2dff38f971b6d88b6f1f098b8a`. The latest release on that day
was v1.17.0. The package is `@fission-ai/openspec` on npm, needs Node 22.12.0 or later and is
under the MIT licence. Corvint pins version 1.16.0 in `interop/spec-toolchains/versions.json`.

The pages read, all under
`https://github.com/Fission-AI/OpenSpec/blob/579fcac57ae00c2dff38f971b6d88b6f1f098b8a/`:

- `README.md`
- `docs/concepts.md`, `docs/writing-specs.md`, `docs/cli.md`, `docs/opsx.md`,
  `docs/existing-projects.md`, `docs/agent-contract.md` and `docs/glossary.md`
- `openspec/specs/openspec-conventions/spec.md` and `openspec/specs/cli-validate/spec.md`
- `src/core/validation/constants.ts`
- `schemas/spec-driven/templates/proposal.md`, `spec.md`, `tasks.md` and `design.md`
- `openspec/config.yaml`

The `openspec` command is not installed on this machine and installing software was outside this
task. Nothing below was checked by running it. Every statement about how the command behaves comes
from the pages above.

## What OpenSpec is

OpenSpec keeps two things in a repository: the specs that describe current behaviour, and one
folder per change that is being proposed or built.

- Current behaviour: `openspec/specs/<capability>/spec.md`. The file has a `## Purpose` section
  and a `## Requirements` section. Each requirement is a `### Requirement: <name>` heading, a
  statement that uses SHALL or MUST, and at least one `#### Scenario: <name>` block with WHEN and
  THEN bullets. The heading text is the requirement's identity. Requirements are not numbered.
- A change: `openspec/changes/<change-name>/` with `proposal.md`, `tasks.md`, an optional
  `design.md`, a small `.openspec.yaml`, and delta files under `specs/<capability>/spec.md`.
- A proposal has four sections: `## Why`, `## What Changes`, `## Capabilities` (new and modified)
  and `## Impact`.
- A delta file lists changes to a capability under `## ADDED Requirements`,
  `## MODIFIED Requirements`, `## REMOVED Requirements` and `## RENAMED Requirements`.
- A task file holds numbered groups of checkboxes. A design file holds context, goals and
  non-goals, decisions, and risks and trade-offs.
- The order of work is proposal, specs, design, tasks, implementation, archive. Archiving applies
  the deltas to the current specs and moves the folder to
  `openspec/changes/archive/YYYY-MM-DD-<change-name>/`.
- `openspec validate --strict` checks structure. A change must hold at least one delta unless its
  `.openspec.yaml` sets `skip_specs: true`. A requirement must use SHALL or MUST and have a
  scenario. The validator's limits for the `## Why` section are 50 and 1000 characters.
  Requirement text longer than 500 characters is flagged, and so is a change with more than 10
  deltas.
- The commands that draft and apply a change (`/opsx:propose`, `/opsx:apply`, `/opsx:verify`,
  `/opsx:archive` and others) run inside a coding agent. As the pages describe it, the verify step
  is carried out by the agent. It is not a deterministic check.
- The command sends usage telemetry unless `OPENSPEC_TELEMETRY=0` or `DO_NOT_TRACK=1` is set.

For a project that already has documentation, OpenSpec's own guidance is not to convert it in
bulk. The page `docs/existing-projects.md` says: "Treat existing docs as source material for
exploration, not as specs to convert".

## What "flows in OpenSpec" most likely means

OpenSpec has no feature called flows. The closest thing is its change workflow: the fixed order
of proposal, specs, design, tasks, implementation and archive. This proposal reads the owner's
phrase as that workflow. This is an inference, and the owner should confirm it. It is unrelated
to Corvint's own flows, which are features of an application with variations and tests.

## How Corvint's process maps onto OpenSpec

| Corvint today | OpenSpec | Proposed treatment |
|---|---|---|
| Capability spec: one file per capability under `docs/specs/`, with status header lines and an agent digest | `openspec/specs/<capability>/spec.md` with a purpose and requirements | Keep Corvint's file as the contract of record. |
| Requirement ID: numbered, stable, never reused, listed in `docs/specs/REQUIREMENTS.tsv` | The text of the `### Requirement:` heading. A rename is a `RENAMED` delta. | Adapt. The heading begins with the Corvint ID. |
| Acceptance per requirement (`owner`, `standing`, `unrecorded`) and the intent status axis | Nothing. A change is either active or archived. | Keep Corvint's. Archive only after recorded acceptance. |
| Delivery status axis | Task checkboxes and the move to the archive | Keep Corvint's. |
| Non-goals, failure modes and rollback in each spec | Goals and non-goals and risks in `design.md`. No failure-mode or rollback section. | Adapt. The proposal carries these three sections. |
| Acceptance evidence: the table that names tests, checked by `make traceability-tests-check` | Scenarios in WHEN and THEN form. No link from a requirement to a test. | Adopt scenarios. Keep Corvint's check. |
| Generated `REQUIREMENTS.tsv`, `INDEX.json` and the table in `docs/specs/README.md` | `openspec list` and `openspec show --json`. No committed index. | Keep Corvint's. |
| Implementation plan | `tasks.md` and `design.md` | Adopt as the task list at the end of the proposal. |
| Evidence record: one file per entry under `docs/build-log/` | The archived change folder | Keep the build log. The archive records which proposal was accepted. |
| Decision records under `docs/decisions/` | A decisions section in one change's `design.md` | Keep Corvint's. |
| Tickets, claims, gates and completion in `.taskman/` | Checkboxes in `tasks.md` | Keep `.taskman/`. One ticket per change. The task list is a checklist, not a queue. |
| Change loop in `docs/SPEC-DRIVEN-DEVELOPMENT.md`: the spec changes before or with the code | Proposal, specs, design, tasks, implementation, archive | Adopt. The proposal exists before dispatch. |

### Adopt as it is

- One folder per change, named after the change, holding the proposal.
- The proposal sections `## Why`, `## What Changes`, `## Capabilities` and `## Impact`.
- `## ADDED Requirements` and the other three delta headings to say what a change does to a
  capability.
- `### Requirement:` with at least one `#### Scenario:` in WHEN and THEN form for every new
  requirement.
- A numbered task list with checkboxes.
- The archive folder name `YYYY-MM-DD-<change-name>`.

### Adapt

- The requirement name begins with the Corvint requirement ID, for example "AFU-V1-150 Gaps lists
  tests with no variation". OpenSpec treats the whole heading as the name, so this is valid
  OpenSpec and keeps Corvint's ID as the stable handle.
- The proposal also carries non-goals, failure modes, acceptance evidence and rollback, because
  product invariant 8 in `AGENTS.md` requires them and OpenSpec's templates do not.
- The proposal names its ticket, the product requirement it serves and the owning capability spec.
- One file per change for now, instead of OpenSpec's three files. The section names are OpenSpec's,
  so the file can be split later without rewriting. See stage 2.
- The directory is `docs/specs/openspec/` for now, not `openspec/` at the repository root. See
  stage 2.

### Do not adopt

- Converting the existing specs. The numbers are in "Options and recommendation".
- Archiving as the act of acceptance. In OpenSpec, archiving changes the current specs. In Corvint
  only the owner accepts intent, and the acceptance is recorded on the requirement line.
- The agent-run verify step as evidence. Corvint's evidence is a named test or a measured result.
- A task list as a work queue. Claims, gates and completion stay in `.taskman/`.

## Conflicts with the product invariants

The invariants are the numbered list in `AGENTS.md`.

- Invariant 6 says Corvint integrates with existing specs and agents and does not require a new
  spec language. Option B does not breach it: the Corvint product still asks nothing of a user's
  repository, and this repository keeps its own spec format. Option C would make this repository's
  checks depend on OpenSpec's format, and Corvint no longer has code that reads that format (see
  the next section).
- Invariant 8 requires recorded human intent, numbered requirements, non-goals, failure modes,
  acceptance evidence and rollback, and says generated documentation can propose intent but
  cannot accept it. OpenSpec has no numbered requirements, no failure modes and no rollback
  section. Its propose command drafts with an agent, and its archive command changes the current
  specs with no record of who accepted the change. The adaptations above close each of these gaps.
- Invariants 1 and 2 require pinned evidence and stated uncertainty. A scenario is a description
  of expected behaviour. It is not evidence that the behaviour holds. Corvint's check that every
  cited test exists stays in force.
- Invariant 7 keeps the default product to one Go binary with no network dependency. The `openspec`
  command is a Node package that sends telemetry by default. It can only be a development tool for
  this repository, pinned and checksummed the way `interop/spec-toolchains/versions.json` already
  does, and run with telemetry off. It must never be something the Corvint binary needs.
- Requirement text. OpenSpec flags requirement text over 500 characters. A simple parse of the
  surviving specs finds about 5820 requirement definitions, and about 3570 of them are longer
  than that. This matters only for text placed in OpenSpec files, so under Option B it applies to
  new proposals and not to the existing specs.

## Can Corvint read OpenSpec layouts today?

Only in one narrow way.

- What still works: `internal/contextindex/toolchain_status.go` recognises paths under a root
  `openspec/` directory and labels them as current specs, active changes or archived changes. It
  labels paths. It does not read requirements.
- What was removed: the reader that turned OpenSpec requirements into obligations. It was reached
  through `corvint ocm prepare --intent-form`. The spec for the reader,
  `docs/specs/ocm-intent-forms-v0.md`, is retired, and its header says that decision 0594 deleted
  the command group and the code. At this head the directory `internal/lrfrepo` is absent and a
  text search for `intent-form` finds nothing under `cmd/` or `internal/`.
- What is left over and now out of date:
  - `docs/specs/spec-toolchain-adapters-v0.md` still claims the reader, with intent `accepted` and
    delivery `experimental`. The spec health report proposes retiring it.
  - `protocol/integrations/openspec/README.md` has a section on reading OpenSpec specs that
    describes the removed command.
  - `docs/SPEC-TOOLCHAIN-INTEGRATION.md` describes the integration.
  - `script/spec-toolchain-conformance.sh` (`make spec-toolchain-conformance`) calls the removed
    command in its third step, so it cannot pass as written. It was not run for this proposal. It
    needs the network and installs packages.
  - `.github/workflows/spec-toolchain-conformance.yml` runs that script and still lists paths
    under `internal/lrfrepo` among its triggers.
  - `interop/spec-toolchains/versions.json` pins OpenSpec 1.16.0, one release behind.
- What is not affected: `protocol/integrations/openspec/config.yaml` and
  `protocol/integrations/openspec/skills/corvint-openspec/SKILL.md` tell an OpenSpec user's agent
  to run `corvint context`, `corvint affected` and `corvint review`. Decision 0594 lists all three
  among the kept commands. They were not run for this proposal.
- A directory under `docs/specs/openspec/` is not recognised by the path labelling, because the
  code looks only at the repository root.

What it would take:

- For Option B, nothing in Go. The capability specs stay where the generator reads them. The
  generator, `script/gen-spec-requirements.sh`, reads only files directly under `docs/specs/`, so
  it ignores the proposals.
- For this repository to treat OpenSpec files as the source of requirements (Option C, or stage 3
  below): a change to the generator so that `### Requirement:` blocks produce rows in
  `REQUIREMENTS.tsv`, a matching rule in the traceability check, and an equivalent of the status
  header and agent digest. Each is a script or Go change that needs its own accepted requirements
  first.
- For the Corvint product to read a user's OpenSpec project as intent again: the removed reader
  would have to return under a new accepted spec. That is a product decision outside this
  proposal.

## Options and recommendation

Option A: keep Corvint's format and process, and borrow only the scenario style. Nothing new to
learn and nothing to install. It does not give the owner the standard workflow that was asked for,
and it leaves "documented before built" without a named document to check for.

Option B: use OpenSpec's change workflow and requirement format for every new change, and keep
`docs/specs/` as the contract of record. This is the recommendation. It adds one file per change
and one rule at dispatch. It changes no generated file and no existing check. The four proposals
already committed under `docs/specs/openspec/changes/` show what it looks like.

Option C: move the spec set to OpenSpec's layout and format. Not recommended.

- `REQUIREMENTS.tsv` holds 9005 rows at this head. 5879 are in the 168 surviving specs. Each would
  need a name and at least one scenario.
- 3868 of the 9005 IDs are named in at least one test file (a textual count). OpenSpec identifies
  a requirement by its heading text, so every one of those references would depend on the heading
  keeping the ID.
- `REQUIREMENTS.tsv`, `INDEX.json`, the README table and the four checks named in stage 0 would
  all need rewriting.
- OpenSpec's own guidance for existing projects is not to do this.

The cost of Option B is that there are two places to look: the proposal while a change is open and
the capability spec once its lines are copied in. The ID at the start of each requirement name is
what keeps them from drifting apart, and stage 2 adds a check for it.

## Document before build

The rule, in the owner's words: "everything should be documented before being built".

1. No lane or subagent is dispatched for a change until a committed change proposal exists.
2. The proposal holds the intent, numbered requirements with scenarios, non-goals, failure modes,
   acceptance evidence, rollback and a task list. The four committed proposals have exactly these
   sections.
3. The proposal names the owning capability spec and the exact requirement IDs it adds, so a lane
   can be dispatched from the proposal alone.
4. A bug fix against an existing requirement needs no proposal. Its ticket cites the requirement
   ID.
5. One proposal file per change. No second queue: the ticket stays in `.taskman/` and its
   `requirementRefs` field names the IDs.
6. The lane's first commit copies the requirement lines into the capability spec as numbered
   definition lines that begin `(proposed DATE; TICKET; delivery experimental)`. From that commit
   on, the generated index knows the IDs.
7. The intent stays `proposed` and the delivery stays `experimental` until the owner accepts.

How it is checked. Three points, from cheapest to strongest.

- At dispatch, by reading the ticket. A ticket is dispatched only if its `requirementRefs` is not
  empty or it names a proposal path. This needs no new code. In the task store on 2026-11-09,
  seven of the eight in-flight 1.0 tickets (V1-5251 to V1-5257) have an empty `requirementRefs`.
  The eighth, V1-4372, names one ID.
- At merge, by a gate. No existing gate refuses a code change that cites a requirement ID with no
  spec text. `make requirement-definitions-check` (`script/check-requirement-definitions.sh`,
  specified in `docs/specs/requirement-definition-gate-v0.md`) checks the other direction: every
  row of `REQUIREMENTS.tsv` has a definition line. At this head 15 IDs with known prefixes are
  named in test files and have no definition line (a textual count). The proposal is to add one
  rule to that gate: within a commit range, every requirement ID with a known prefix that appears
  on an added line of a code or test file must have a row in `REQUIREMENTS.tsv` at the head of the
  range. Limiting the rule to a range means the 15 existing cases are reported and do not block.
  The range form already exists in the traceability check (`--range BASE..HEAD`, the requirement
  named TTG-V0-950 in `docs/specs/traceability-test-gate-v0.md`). This is a script change. It
  needs its own requirement in the gate's spec and its own tests, so it is proposed here and not
  made.
- For the proposal's own shape, `openspec validate --strict` in stage 2.

What no check can do. The merge rule catches code that cites an ID that does not exist. It cannot
catch code that cites no ID at all. That case is left to the dispatch rule and to review. A
stricter rule, such as refusing a pull request that names neither a proposal nor a requirement ID,
is possible later and is not proposed now.

To keep this light: a change to prose alone needs no proposal, and neither does a repair that
makes the implementation meet an existing accepted requirement.

## Staged migration

Stage 0 is done by the commits that carry this file. It changes no script and no check.

- Four change proposals under `docs/specs/openspec/changes/<change-name>/proposal.md` and a short
  `docs/specs/openspec/README.md`.
- The product plan gives each of its 43 requirements a scenario in WHEN and THEN form.
- The proposals add no row to `REQUIREMENTS.tsv`, `INDEX.json` or the README table, because the
  generator reads only files directly under `docs/specs/`.
- `make spec-status-check`, `make spec-requirements-check`, `make traceability-tests-check` and
  `make line-citations-check` pass with these files present.

Stage 1 is the process rule. It needs the owner's acceptance and no tool.

- The document-before-build rule above is in force for every 1.0 ticket.
- A lane's first commit copies the proposal's requirement lines into the capability spec and runs
  `make spec-requirements`. The new rows carry the acceptance value `unrecorded`.
- The ticket's `requirementRefs` is filled in when the proposal is written.
- When the owner accepts, the requirement lines record the decision, and the change folder moves
  to `docs/specs/openspec/changes/archive/YYYY-MM-DD-<change-name>/`.

Stage 2 adds the tool. It needs the owner's decision on each point.

- Move the directory to `openspec/` at the repository root, so that the `openspec` command and the
  path labelling in `internal/contextindex/toolchain_status.go` both see it, and add it to
  `.corvint/documentation-paths.json`.
- Split each proposal into OpenSpec's `proposal.md`, `tasks.md` and `specs/<capability>/spec.md`.
  The validator reads deltas from the `specs/` folder, so the single file is not expected to pass
  it. This expectation was not tested.
- Update the pin in `interop/spec-toolchains/versions.json` and run
  `openspec validate --all --strict` with telemetry off in the existing conformance job. That job
  must first be cut down to what still exists, because its third step calls a removed command.
- Add the range rule to `make requirement-definitions-check`, and a small check that every
  `### Requirement:` heading in an open change begins with an ID that is either free or defined
  with the same title.

Stage 3 is optional and has no date. Generate `openspec/specs/<capability>/spec.md` from the
capability specs as a read-only copy, for tools that expect OpenSpec's layout. Do this only when
something needs it.

## Rollback

- Stage 0: remove `docs/specs/openspec/` and the sentences that cite it in the product plan and
  the spec health report. No generated file changes.
- Stage 1: stop applying the rule. Requirement lines already copied into capability specs stay,
  because requirement IDs are not deleted or reused. A line whose intent the owner rejects is
  marked rejected in place.
- Stage 2: move the directory back, restore the pin and the job, and remove the added gate rule.
- No stage changes the Corvint binary, so there is nothing to roll back in the product.

## The worked example

The brief for this work asked for the new testing requirements as one change proposal in
OpenSpec's format. Four were written, one for each ticket that had not been dispatched:

| Change | Ticket | Requirement IDs it adds |
|---|---|---|
| `affected-plan-kinds` | V1-5251 | APK-V0-001 to APK-V0-007, in a new capability spec |
| `e2e-inventory-discovery` | V1-5252 | AFU-V1-140 to AFU-V1-145 |
| `flows-unlinked-tests-report` | V1-5255 | AFU-V1-150 to AFU-V1-154 |
| `flows-status-report` | V1-5257 | AFU-V1-160 to AFU-V1-165 |

They differ from OpenSpec's layout in two ways that this file has already named: one file instead
of three, and a directory under `docs/specs/`. They have not been validated with the `openspec`
command. No existing spec was converted. The ID blocks were checked free when the proposals were
written and have not been reserved in the lane ledger.

## What the owner decides

1. Whether Option B is adopted, or Option A or C instead.
2. Whether "flows in OpenSpec" means the change workflow, as this proposal assumes.
3. Whether the document-before-build rule is in force now, with the two exemptions named above.
4. Whether the gate rule for undefined requirement IDs is added.
5. Whether the proposals move to `openspec/` at the repository root, and whether each becomes
   three files.
6. Whether the `openspec` command is installed in CI, at which version, with telemetry off.
7. What happens to the out-of-date integration files: retire them with
   `docs/specs/spec-toolchain-adapters-v0.md`, or rebuild the reader under a new spec.

## What this proposal did not do

- It did not run the `openspec` command or any validator.
- It did not change a script, a check, a workflow file or any Go code.
- It did not convert, retire, merge or split any existing spec.
- It did not mark anything accepted.
