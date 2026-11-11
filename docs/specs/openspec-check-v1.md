# OpenSpec Check V1 — change folders, stages, boundaries and health classes

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-5399)
Delivery status: experimental
Health: KEEP
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-INT-010, PROD-INT-011), decision 0596 (owner answers G14,
R37, R41, H9, H12 and H13), and the change folder `docs/specs/openspec/changes/openspec-check/`
(the proposal, design and spec delta this file was created from).

## Agent digest
- Claim: A Go check enforces the OpenSpec change-folder layout, the propose, apply and archive stages, one OpenSpec root and spec health classes.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-5399) / experimental
- Exists: this spec; the change folder `docs/specs/openspec/changes/openspec-check/`.
- Blocked on: the Go check, the baseline, the Health projection and the `openspec-validate` target.
- Read next: Requirements; Decisions; Failure modes; Acceptance evidence.

## User and measurable job

The owner needs the rule "documented before built" (PROD-INT-011) and the change-folder layout of
G14 enforced by a check, not by review alone, and needs a finished change moved out of `changes/`
so it no longer reads as pending work. Every spec needs a health class so the index stops reading
as if all 288 specs were live (PROD-INT-010).

Measured by: the check fails each fixture history that breaks a rule and passes each that keeps
it; it passes on the repository with every remaining failure listed in a baseline that only
shrinks; `make openspec-validate` passes at the pinned OpenSpec command version.

## Current state

On 2026-11-11 at `b55635262`: `docs/specs/openspec/` holds `config.yaml` (schema `spec-driven`),
`acceptance-log.tsv` and 46 change folders, none archived. No Go check reads the folders; the
OpenSpec command (`@fission-ai/openspec` 1.17.0) was run by hand on 2026-11-10 and passed for every
folder. No spec header carries a `Health:` line. Commits on `origin/main` carry the trailer
`Corvint-Ticket:`; none of the last 2000 carries `Change:` or `Requirement:`.

## Scope and non-goals

In scope: the Go check `tools/openspec-check`, its baseline, the Make targets `openspec-check` and
`openspec-validate`, the `Health:` projection of `make spec-requirements`, and the PRS-V1-018
amendment.

Non-goals:

- No repository-root `openspec/` directory and no `docs/specs/openspec/specs/` tree (G14; H13).
- The Go check does not call the OpenSpec command, the network or npm; no shipped binary calls the
  command and the product build gains no npm dependency (AGENTS.md invariant 7).
- No native spec merge: `openspec archive` is always run with `--skip-specs` (H13).
- Strict-mode warnings of the OpenSpec command fail nothing.
- No semantic review of requirement text: the check reads structure, IDs and history only.
- Writing the `Health:` lines and archiving specs belong to the change `spec-set-cleanup`.
- No change to `interop/spec-toolchains/versions.json`, which describes other repositories.

## Requirements

- `OSC-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) Each
  active change folder directly under `docs/specs/openspec/changes/` (other than `archive/`) MUST
  hold `proposal.md` (headings Why, What Changes, Impact and Non-goals, and the `Intent status:`
  and `Delivery status:` lines), `design.md` (headings Decisions, Failure modes, Acceptance
  evidence and Rollback), `tasks.md` (numbered checkbox items, each naming at least one
  requirement ID, or `Requirements: all`, and a `Proof:`) and at least one
  `specs/<capability>/spec.md` with an ADDED, MODIFIED or REMOVED Requirements heading, each
  requirement written as `### Requirement: <ID> <title>` with at least one `#### Scenario:` block
  or, for REMOVED, a reason line. The folder MAY hold `.openspec.yaml`. Whether the folder passes
  the OpenSpec command is OSC-V1-012, not this rule. (G14; PROD-INT-011; H12)
- `OSC-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) An ADDED
  requirement ID MUST NOT exist in `docs/specs/REQUIREMENTS.tsv` at the first parent of the commit
  that first added the change folder, nor be ADDED by any other active or archived change folder;
  a definition the change itself adds during apply is allowed. A MODIFIED or REMOVED ID MUST exist
  in `REQUIREMENTS.tsv`. A retired ID, which keeps its `REQUIREMENTS.tsv` row, is therefore never
  ADDED again. (G14; R11)
- `OSC-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) Every
  non-merge commit in the checked range that touches a path outside `docs/` MUST bind to a change
  or name an accepted requirement: a `Change: <name>` trailer naming an active or archived change
  folder, a `Corvint-Ticket: <ID>` trailer naming the ticket of a change's `Ticket:` proposal
  header, or a `Requirement: <ID>` trailer naming a requirement whose `REQUIREMENTS.tsv`
  acceptance is `owner` or `standing`. Otherwise the check fails naming the commit and its paths.
  (R41; PROD-INT-011)
- `OSC-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) For every
  commit in the range that binds to a change and touches a path outside `docs/`, the change's
  `proposal.md` MUST exist as an active folder in that commit's first parent; a folder added in the
  same commit as code fails. (R41; G14)
- `OSC-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) In
  `tasks.md` a ticked item MUST NOT follow an unticked item. For every ticked item, each ADDED or
  MODIFIED delta ID it names MUST be defined by a canonical spec directly under `docs/specs/`
  (a `REQUIREMENTS.tsv` row) at the checked head, and each REMOVED ID MUST be marked retired there.
  When the proposal's `Owning capability spec:` header names a spec in another repository
  (`beamfall/<repository>`), the check MUST NOT look for the IDs here: each ticked item MUST name a
  commit of that repository (a 7 to 40 digit hexadecimal object name) and is reported as
  `external-spec-not-verified` with that commit, never counted as a pass; a ticked item naming no
  commit fails. (R41; G14; H9)
- `OSC-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) A change
  whose tasks are all ticked and whose every delta ID is in the canonical specs is finished and
  MUST be moved to `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/` by a pure rename
  (`openspec archive <name> --skip-specs`, run from `docs/specs`): the archived tree equals the
  active tree of the commit's first parent, and its date is within one day of the archive commit's
  UTC committer date. The check MUST fail for a finished change still active at the head, an
  archived folder holding an unticked task, an archive commit that is not a pure rename, and any
  later commit that changes a file in an archived folder. Names are lowercase hyphenated words,
  unique across active and archived folders. (R41; H12)
- `OSC-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) The
  repository MUST hold exactly one OpenSpec root, `docs/specs/openspec/`, whose `config.yaml` names
  the schema `spec-driven`. The check MUST fail when that file is missing or names another schema,
  when the repository root holds an `openspec/` directory or an `.openspec.yaml` file, and when
  `docs/specs/openspec/specs/` exists, since the canonical specs are the flat files directly under
  `docs/specs/`. `interop/spec-toolchains/` is not read. (G14; H12; H13)
- `OSC-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) Each spec
  directly under `docs/specs/` other than `README.md` MUST carry a header line `Health:` with one
  of `KEEP`, `MERGE`, `RETIRE` or `OUT-OF-1.0` before its first `## ` heading; a `RETIRE` or
  `OUT-OF-1.0` spec MUST also read `Delivery status: retired` and stays in place.
  `make spec-requirements` MUST project the class into `INDEX.json` (a `health` key, present only
  for a spec with the line) and into the README table (a `; health CLASS` suffix of the Contract
  cell). (R37; PROD-INT-010)
- `OSC-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) A proposal
  whose `Intent status:` starts with `accepted` MUST have a `change` row for its folder name in
  `docs/specs/openspec/acceptance-log.tsv` with mode `owner` or `auto`. Without such a row, the
  intent MUST start with `proposed` and the delivery with `not-started` or `experimental`. (G14)
- `OSC-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399)
  `REQUIREMENTS.tsv`, `INDEX.json` and the README table MUST stay generated from spec headers and
  checked for staleness (SRG-V0-012), and each requirement MUST keep its recorded acceptance mode
  with the audit command `make spec-acceptance-audit` (SRG-V0-009, SRG-V0-010). (R37: PRS-V1-018
  (c) and (d))
- `OSC-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) The check
  MUST run as the Make target `openspec-check` inside `doc-gates` and the `focused-docs` target.
  When it cannot read the history it needs (a shallow clone, a range endpoint that is not a commit,
  no Git repository) it MUST refuse with `history-unavailable` and a non-zero exit. Without a range
  it MUST report the commit rules (OSC-V1-003, OSC-V1-004) as NOT_RUN. A stale
  `REQUIREMENTS.tsv` is refused, naming `make spec-requirements`. Failures of a change folder or
  spec listed in the committed baseline `tools/openspec-check/baseline.txt` are reported and do
  not fail; an entry that no longer fails, and an entry added since the range base, fail, so the
  baseline only shrinks. (G14)
- `OSC-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5399) The Make
  target `openspec-validate` MUST run `openspec validate --all` from `docs/specs` with
  `@fission-ai/openspec` at the one version pinned in the Makefile and `OPENSPEC_TELEMETRY=0`, and
  MUST fail when any change folder fails validation; `--strict` warnings are printed and do not
  fail it. Outside CI an absent command or another version reports NOT_RUN with the reason and
  exits zero; in CI (`CI` set to `true`) either fails. The Go check needs neither the command, the
  network nor npm, and no shipped binary calls the command. (H12; G14)

## Decisions

- Binding (OSC-V1-003). The design names `Change:` and `Requirement:`. Commits on this tree carry
  only `Corvint-Ticket:`, so a `Corvint-Ticket:` naming the ticket in a proposal's `Ticket:` header
  also binds the commit to that change; a ticket named by no proposal binds nothing. Merge commits
  are exempt. No other exemption is decided.
- Task numbers. An item is `- [ ] N text` or `- [x] N text` where `N` is digits with optional
  `.digits` groups and an optional lowercase letter (`2.3a`), the forms the folders already use
  and the OpenSpec command counts. A line starting `- [` in another form is malformed.
  `Requirements: all` names every delta ID of the folder. `X-NNN to NNN` and `X-NNN to X-NNN`
  name the whole range. Ticket IDs (`V1-NNNN`) are not requirement IDs.
- Commit rules run only with a range (`--base REV`, checked as `REV..HEAD`). The tree rules
  (layout, IDs, tick order, canonical spec, archive state and history, boundaries, health,
  acceptance) run on every invocation; the ID and archive rules read history, so every run needs a
  full clone.
- Baseline. One subject per line: `change <name>` or `spec docs/specs/<file>.md`, with `#`
  comments. Boundary and commit failures have no subject and are never baselined.
- Health projection. The README table keeps its columns: the class becomes a `; health CLASS`
  suffix of the Contract cell, and the `INDEX.json` key is optional, so no generated file changes
  until a spec carries the line.
- `focused-docs`. That gate is a command list in `.github/workflows/release-gates.yml`; this change
  adds a Make target `focused-docs` holding the same checks plus `openspec-check`, for the separate
  `.github` pull request to call.

## Scenarios

- Scenario for OSC-V1-001: an active folder without `design.md` fails naming the folder and the
  file; an ADDED requirement with no `#### Scenario:` block fails naming the ID.
- Scenario for OSC-V1-002: a delta that adds an ID present in `REQUIREMENTS.tsv` fails naming the ID
  and the spec that holds it.
- Scenario for OSC-V1-003: a range holding a commit that changes a Go file with no binding trailer
  fails naming the commit and the paths.
- Scenario for OSC-V1-004: a commit that adds `docs/specs/openspec/changes/x/` and a Go file with
  `Change: x` fails; a folder committed first and code in a later commit passes.
- Scenario for OSC-V1-005: `tasks.md` with 2.1 unticked and 2.2 ticked fails naming 2.2; a ticked
  task whose ADDED ID no canonical spec defines fails naming the ID; a ticked task of a change whose
  spec is in `beamfall/corvint-tasks` and that names a commit there is reported as
  `external-spec-not-verified` and exits zero when nothing else fails.
- Scenario for OSC-V1-006: a finished active folder fails naming its archive path; a commit after
  the archive that edits its `tasks.md` fails naming the commit; an archived folder with an unticked
  item fails naming the folder and the task.
- Scenario for OSC-V1-007: a root `openspec/changes/x/proposal.md` fails pointing to
  `docs/specs/openspec/changes/`; a missing `config.yaml` fails naming it; a
  `docs/specs/openspec/specs/watch/spec.md` fails saying the canonical spec belongs directly under
  `docs/specs/`.
- Scenario for OSC-V1-008: a spec header without `Health:` fails naming the spec.
- Scenario for OSC-V1-009: a proposal reading `Intent status: accepted` with no log row fails naming
  it.
- Scenario for OSC-V1-010: an `INDEX.json` edited by hand fails `make spec-requirements-check`.
- Scenario for OSC-V1-011: a depth-1 clone checked over a range exits non-zero with
  `history-unavailable`.
- Scenario for OSC-V1-012: a folder whose delta has a requirement without a scenario fails the
  target with the command installed; without the command outside CI it prints NOT_RUN and exits
  zero; in CI it exits non-zero.

## Failure modes

- Shallow clone, missing range endpoint or no Git repository: refused with `history-unavailable`,
  exit 2; never a pass.
- No range: the commit rules report NOT_RUN; every other rule runs.
- A stale `REQUIREMENTS.tsv`: refused with `requirements-stale`, exit 2, naming
  `make spec-requirements`; without `script/gen-spec-requirements.sh` the staleness check reports
  NOT_RUN.
- A malformed `tasks.md` line: fails naming the line.
- Two folders with one name: fails.
- A rename that also edits archived content: fails as an archive that is not a pure rename.
- `openspec archive` without `--skip-specs`: creates `docs/specs/openspec/specs/`, which
  OSC-V1-007 refuses.
- The OpenSpec command absent or at another version: NOT_RUN locally, failure in CI; never a pass
  it did not run. The pin moves only in a commit showing `openspec validate --all` passing at the
  new version.

## Acceptance evidence

| Requirement | Evidence | State |
|---|---|---|
| OSC-V1-001 | `TestOSCV1001Layout` | fixture-tested |
| OSC-V1-002 | `TestOSCV1002RequirementIDs` | fixture-tested |
| OSC-V1-003 | `TestOSCV1003CommitBinding` | fixture-tested |
| OSC-V1-004 | `TestOSCV1004ProposeStage` | fixture-tested |
| OSC-V1-005 | `TestOSCV1005ApplyStage` | fixture-tested |
| OSC-V1-006 | `TestOSCV1006ArchiveStage` | fixture-tested |
| OSC-V1-007 | `TestOSCV1007Boundaries` | fixture-tested |
| OSC-V1-008 | `TestOSCV1008HealthClass`; `script/gen-spec-requirements_test.sh` case OSC-V1-008 | check and projection fixture-tested; lines not written |
| OSC-V1-009 | `TestOSCV1009AcceptanceLink` | fixture-tested |
| OSC-V1-010 | `script/gen-spec-requirements_test.sh` cases 6, 7 and 8 (SRG-V0-009, 010, 012); `TestRequirementsAcceptanceColumns`; `make spec-requirements-check` | carried over, delivered |
| OSC-V1-011 | `TestOSCV1011Baseline`; `TestOSCV1011HistoryUnavailable`; `TestOSCV1011StaleRequirements` | fixture-tested; repository run in build log |
| OSC-V1-012 | `script/openspec-validate_test.sh` | NOT_RUN path and fake-command paths tested; pinned run `NOT_RUN` on hosts without the command |

## Rollback

Remove `openspec-check` from `DOC_GATE_TARGETS` and from the `focused-docs` target; the tool stays
and runs by hand. Restoring `tools/openspec-check/baseline.txt` to its first version returns the
existing folders to report mode. Remove the `openspec-validate` target. The `Health:` line is
inert for other readers; removing its projection in `script/gen-spec-requirements.sh` restores the
previous `INDEX.json` shape. For PRS-V1-018, revert its definition edit.
