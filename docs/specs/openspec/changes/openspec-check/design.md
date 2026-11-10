# Design: `openspec-check`

## Decisions

- Language and place. A Go program under `tools/openspec-check`, run by a Make target
  `openspec-check` that `doc-gates` and the `focused-docs` gate include. The Go program needs no
  network access, no npm and no OpenSpec command (G14).
- The OpenSpec command (owner decision H12 (2026-11-10); OSC-V1-012). A second Make target,
  `openspec-validate`, runs `openspec validate --all` from `docs/specs` with `@fission-ai/openspec`
  at the version pinned in the Makefile (1.17.0 on 2026-11-10) and `OPENSPEC_TELEMETRY=0`. A folder
  that fails validation fails the target. Strict-mode warnings are printed and do not fail it. When
  the command is absent or at another version, the target reports NOT_RUN locally and fails in CI.
  The CI step that installs the pinned command is added in its own `.github` pull request. The
  command is a developer and CI tool: no product binary calls it and the product build gains no
  npm dependency.
- Active and archived folders. An active change is a directory directly under
  `docs/specs/openspec/changes/` other than `archive/`. An archived change is a directory
  `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/`. Names are lowercase words joined by
  hyphens and are unique across active and archived folders.
- Layout (G14; settled without the owner: split into proposal, design, tasks and spec delta). Each
  active folder holds:
  - `proposal.md` with the headings Why, What Changes, Impact and Non-goals, and the header lines
    "Intent status" and "Delivery status";
  - `design.md` with the headings Decisions, Failure modes, Acceptance evidence and Rollback;
  - `tasks.md` with numbered checkbox items (`- [ ] N text`, the form the OpenSpec command counts),
    each naming at least one requirement ID and a proof;
  - at least one `specs/<capability>/spec.md` with at least one of the headings "ADDED
    Requirements", "MODIFIED Requirements" or "REMOVED Requirements", each requirement written as
    `### Requirement: <ID> <title>` with at least one `#### Scenario:` block (a REMOVED requirement
    needs a reason line instead of a scenario).
  These are the SDD minimum: intent, numbered requirements, non-goals, failure modes, acceptance
  evidence and rollback (`docs/SPEC-DRIVEN-DEVELOPMENT.md`). A folder may also hold the
  `.openspec.yaml` that `openspec new change` writes; the check neither requires nor refuses it.
- Requirement IDs. An ADDED ID must not exist in `REQUIREMENTS.tsv` or in any other active or
  archived change folder. A MODIFIED or REMOVED ID must exist in `REQUIREMENTS.tsv`. A retired ID is
  never ADDED again.
- Binding commits to a change. A commit belongs to a change when it carries a `Change: <name>`
  trailer. A commit that touches paths outside `docs/` and carries no `Change:` trailer must carry a
  `Requirement: <ID>` trailer naming an existing accepted requirement (the G14 "only implements an
  accepted requirement" case, which also needs an auto-accept log entry). Merge commits are exempt.
  Other exemptions are not decided and are listed in the hand-back as open.
- Propose stage (owner decision R41). For each commit in the range that belongs to a change and
  touches paths outside `docs/`, the change folder must exist (active) in that commit's first
  parent. A folder added in the same commit as code fails.
- Apply stage (owner decision R41). In `tasks.md`, a ticked item (`- [x]`) may not follow an
  unticked item (`- [ ]`). For every ticked item, each ADDED or MODIFIED requirement ID it names
  must appear in a canonical spec under `docs/specs/` at the range head, and each REMOVED ID must be
  marked retired there, in the same range as the code commits for that change. When the proposal
  header names a canonical spec in another repository, the item is reported as
  `external-spec-not-verified`; that status is listed, does not fail the check and is never counted
  as a pass (owner decision H9 (2026-11-10)).
- Archive stage (owner decision R41). A change whose tasks are all ticked and whose every delta ID is
  present in the canonical specs is finished. A finished change still under `changes/` at the range
  head fails. The move is made with `openspec archive <name> --skip-specs` from `docs/specs`, which
  is a pure rename to `archive/YYYY-MM-DD-<name>/`. The command writes the local date, so the check
  accepts a date within one day of the archive commit's UTC date. `--yes` is never used to pass
  over an unticked task: an archived folder with an unticked task fails. Any later commit that
  changes a file under an archived folder fails.
- Boundaries (G14; owner decision H12 (2026-11-10)). There is one OpenSpec root,
  `docs/specs/openspec/`, and its `config.yaml` must exist and name the schema `spec-driven`. The
  check fails if the repository root holds an `openspec/` directory or an `.openspec.yaml` file, or
  if `docs/specs/openspec/specs/` exists. `interop/spec-toolchains/` is excluded: it describes
  other repositories.
- One difference from stock OpenSpec, kept from G14. The canonical specs are the flat files under
  `docs/specs/`, not `openspec/specs/<capability>/spec.md`. Apply copies each requirement into the
  owning flat spec, and archive skips the command's own spec merge. The strict-mode note that a
  MODIFIED requirement's target spec does not exist is therefore expected.
- Health classes (owner decision R37). Each canonical spec header carries a line
  `Health: KEEP | MERGE | RETIRE | OUT-OF-1.0` (classes from `docs/specs/SPEC-HEALTH-2026-11-09.md`).
  RETIRE and OUT-OF-1.0 specs also carry a retired or outside-1.0 status line (PROD-INT-010). The
  generator projects the class into `INDEX.json` and the README table. This check enforces the lines;
  writing them is not part of this change.
- Acceptance link. A proposal whose header says "Intent status: accepted" must have a matching entry
  in the acceptance log defined by `acceptance-record`; otherwise its intent must read "proposed"
  and its delivery "experimental" or "not-started" (G14).
- Roll-out. The check starts in report mode for existing folders (it prints failures and exits 0
  for folders listed in a committed baseline file) and is blocking for every folder not in the
  baseline. Each folder leaves the baseline when it passes; the baseline only shrinks.

## Failure modes

- Shallow clone: the check cannot find the commit that added a folder. It refuses with
  `history-unavailable` and exits non-zero; it never passes by default. CI checks out with enough
  depth for the range (full history for the release gate).
- Range not given: the check runs only the layout, ID, boundary and health rules on the tree and says
  that the stage rules were NOT_RUN.
- A code commit with neither trailer: fails, naming the commit and the paths.
- A rename that also edits archived content: fails as an edit of an archived change.
- Two folders with the same name (one active, one archived): fails.
- A malformed `tasks.md` line (no number or no checkbox): fails, naming the line.
- `REQUIREMENTS.tsv` stale: the check refuses and points to `make spec-requirements`, since its ID
  rules depend on the generated file.
- The OpenSpec command is absent or at another version: `openspec-validate` reports NOT_RUN
  locally and fails in CI; it never reports a pass it did not run.
- `openspec archive` run without `--skip-specs`: it creates `docs/specs/openspec/specs/`, which
  the boundary rule refuses.
- A new command version changes validation: the pin moves only in a commit that also shows
  `openspec validate --all` passing at the new version.

## Acceptance evidence

- Fixture repositories, one per rule, each with a passing and a failing history, run as Go tests.
- The check passing on the repository and the baseline being empty at the RC.
- `make openspec-validate` output at the pinned version: every change folder valid, with the
  strict-mode warning counts recorded.
- `make spec-acceptance-audit` output showing the acceptance modes per requirement (PRS-V1-018 (d)
  carried over), and `make spec-requirements-check` passing (PRS-V1-018 (c) carried over).

## Rollback

- Remove the `openspec-check` target from `doc-gates` and from the `focused-docs` gate; the tool
  stays and can still be run by hand. Restoring the baseline file to its first version returns the
  existing folders to report mode.
- The health class header line is inert for every other reader; removing the generator projection
  restores the previous `INDEX.json` shape.
- PRS-V1-018: revert the spec edit.
- The OpenSpec command: remove the `openspec-validate` target and its CI step. `config.yaml` and
  any `.openspec.yaml` files are inert for every other reader and can stay.
