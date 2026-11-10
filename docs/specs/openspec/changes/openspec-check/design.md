# Design: `openspec-check`

## Decisions

- Language and place. A Go program under `tools/openspec-check`, run by a Make target
  `openspec-check` that `doc-gates` and the `focused-docs` gate include. No network access, no npm
  (G14).
- Active and archived folders. An active change is a directory directly under
  `docs/specs/openspec/changes/` other than `archive/`. An archived change is a directory
  `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/`. Names are lowercase words joined by
  hyphens and are unique across active and archived folders.
- Layout (G14; settled without the owner: split into proposal, design, tasks and spec delta). Each
  active folder holds:
  - `proposal.md` with the headings Why, What Changes, Impact and Non-goals, and the header lines
    "Intent status" and "Delivery status";
  - `design.md` with the headings Decisions, Failure modes, Acceptance evidence and Rollback;
  - `tasks.md` with numbered checkbox items, each naming at least one requirement ID and a proof;
  - at least one `specs/<capability>/spec.md` with at least one of the headings "ADDED
    Requirements", "MODIFIED Requirements" or "REMOVED Requirements", each requirement written as
    `### Requirement: <ID> <title>` with at least one `#### Scenario:` block (a REMOVED requirement
    needs a reason line instead of a scenario).
  These are the SDD minimum: intent, numbered requirements, non-goals, failure modes, acceptance
  evidence and rollback (`docs/SPEC-DRIVEN-DEVELOPMENT.md`).
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
  marked retired there, in the same range as the code commits for that change.
- Archive stage (owner decision R41). A change whose tasks are all ticked and whose every delta ID is
  present in the canonical specs is finished. A finished change still under `changes/` at the range
  head fails. Moving it to `archive/YYYY-MM-DD-<name>/` is a pure rename; the date is the archive
  commit's date in UTC. Any later commit that changes a file under an archived folder fails.
- Boundaries (G14). The check fails if the repository root holds an `openspec/` directory or an
  `.openspec.yaml` file, or if a Makefile target or workflow under `.github/workflows/` runs an
  `openspec` command. `interop/spec-toolchains/` is excluded: it describes other repositories.
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

## Acceptance evidence

- Fixture repositories, one per rule, each with a passing and a failing history, run as Go tests.
- The check passing on the repository and the baseline being empty at the RC.
- `make spec-acceptance-audit` output showing the acceptance modes per requirement (PRS-V1-018 (d)
  carried over), and `make spec-requirements-check` passing (PRS-V1-018 (c) carried over).

## Rollback

- Remove the `openspec-check` target from `doc-gates` and from the `focused-docs` gate; the tool
  stays and can still be run by hand. Restoring the baseline file to its first version returns the
  existing folders to report mode.
- The health class header line is inert for every other reader; removing the generator projection
  restores the previous `INDEX.json` shape.
- PRS-V1-018: revert the spec edit.
