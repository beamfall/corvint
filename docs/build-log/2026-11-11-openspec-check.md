# 2026-11-11: Go check of the OpenSpec change folders and their stages (V1-5399)

Change `openspec-check`, tasks 1.2 and 2.1 to 2.10 and the repository run of task 3.1.
Requirements OSC-V1-001 to OSC-V1-012 in `docs/specs/openspec-check-v1.md`, and the PRS-V1-018
amendment. Owner answers implemented: G14, R37, R41, H9, H12 and H13 (decision 0596).

## What changed

- `docs/specs/openspec-check-v1.md` is the canonical spec, created from the change's delta with
  the IDs unchanged and marked with the standing auto-accept. PRS-V1-018 now requires only
  clauses (c) and (d), traced to SRG-V0-012, SRG-V0-009 and SRG-V0-010 (OSC-V1-010). The
  acceptance log gains the change's `auto` row; the proposal names ticket V1-5399.
- `tools/openspec-check` (root module, no network, no npm, no OpenSpec command) runs the tree
  rules on every invocation: layout (OSC-V1-001), IDs against `REQUIREMENTS.tsv` at the parent
  of the commit that added the folder (002), tick order and the canonical spec of ticked tasks,
  with `external-spec-not-verified` reports (005), archive state, pure-rename archive history
  and the no-edit rule (006), one OpenSpec root (007), `Health:` lines (008) and the acceptance
  link (009). `--base REV` adds commit binding (003) and the propose stage (004) over REV..HEAD.
  A shallow clone, a missing base or no repository refuses with `history-unavailable`; a stale
  `REQUIREMENTS.tsv` refuses with `requirements-stale` (011).
- `make openspec-check` (in `DOC_GATE_TARGETS`), a new `make focused-docs`, and
  `tools/openspec-check/baseline.txt`, which only shrinks.
- `make spec-requirements` projects a `Health:` class into an optional `INDEX.json` key `health`
  and a `; health CLASS` suffix of the README Contract cell. `internal/specindex` accepts the
  optional key.
- `make openspec-validate` (`script/openspec-validate.sh`, pin 1.17.0, `OPENSPEC_TELEMETRY=0`)
  with `script/openspec-validate_test.sh`, registered in `GATE_STEPS` and `shell-test`.
- The SDD authoring template carries `Health: KEEP`; `docs/specs/openspec/README.md` names the
  two targets.

## Decisions made in the lane

- Commit binding. Commits on this tree carry `Corvint-Ticket:` only (`git log --format=%B -200`
  shows 20 `Corvint-Ticket:` trailers and no `Change:` or `Requirement:`). A `Corvint-Ticket:`
  that names the ticket in a proposal's `Ticket:` header binds the commit to that change, beside
  the design's `Change:` and `Requirement:` trailers. A ticket that no proposal names binds
  nothing, so a range of ordinary history fails OSC-V1-003. Merge commits are exempt; no other
  exemption is decided (open for the owner).
- The commit rules run only with `--base`. `doc-gates` passes no base: a default of
  `origin/main` would fail every lane whose ticket no proposal names. The CI range belongs to the
  separate `.github` pull request with task 2.11.
- `focused-docs` is a command list in `.github/workflows/release-gates.yml` and in the Tasks gate
  of the same name, not a Make target. The lane adds a `focused-docs` Make target holding the
  same five checks plus `openspec-check` and changes nothing under `.github/`.
- Task numbers accept `2.3a`-style suffixes, which existing folders use. `Requirements: all`
  names every delta ID. `X-NNN to NNN` and the reservation form `` `X` NNN..NNN `` name ranges.
- The baseline holds `change <name>` and `spec <path>` subjects, so spec health failures are
  baselined like folders. An entry that no longer fails fails the check, and with `--base` an
  entry added since the base fails.
- Heading matching ignores case and accepts a suffix (`## Rollback (TSC-V1-020)`).
- A REMOVED ID counts as retired when its `REQUIREMENTS.tsv` text or its spec's delivery status
  says retired. RETIRE and OUT-OF-1.0 need `Delivery status: retired`, the marking of
  `spec-set-cleanup`.
- The PRODUCT-PLAN conflict row for PRS-V1-018 cited `...release-v1.md:387@1f9a7848`, the
  text before the amendment. A new anchor would claim the old sentence still holds, so the cell
  now cites the file at commit `b55635262`, before the amendment.

## Where the tree differs from the proposal (verified at integ 9ec3a29d0)

- 46 change folders exist (the proposal and the README bar say 45).
- No `docs/specs/openspec/specs/` tree, no root `openspec/` and no archived folder exist.
- `script/acceptance-plan-pin-check.sh` already reports STALE_PLAN on `origin/main` (b55635262)
  before this lane's edit.

## Repository run (task 3.1)

- Before: 374 findings in 320 subjects (32 change folders, 288 specs). After reading the
  reservation range form, `runner-contract` and `watch-seven-runners` pass and leave the
  baseline: 318 subjects (30 change folders, 288 specs), 363 findings, PASS.
- What remains: no spec carries a `Health:` line, so all 288 stay until `spec-set-cleanup` task
  2.4 writes them. The classes are owner decisions this change does not take. The 30 folders
  fail on real gaps in their own text: tasks with no requirement ID (22 folders, mostly
  administrative tasks), missing design or proposal headings (`hub-read-scopes`,
  `source-removal`, `tasks-store-cutover`), a delta requirement with no ID (`source-removal`),
  ticks out of order, and a ticked external task naming no commit (`ticket-taxonomy`).
  `openspec-check` itself stays baselined until the orchestrator ticks task 1.1.
- Task 3.1 stays unticked because its precondition, the `Health:` lines, is not met.

## Evidence

- `go test -count=1 ./tools/openspec-check/`: 13 tests, ok.
- `script/gen-spec-requirements_test.sh`: ok, with the OSC-V1-008 projection case.
- `go test -count=1 -run 'TestIndex|Superseded|StatusDrift|Requirements' ./internal/specindex/`: ok.
- `script/openspec-validate_test.sh`: ok; its real-command case printed NOT_RUN.
- `make openspec-check`: PASS. `make openspec-check OPENSPEC_BASE=origin/main`: PASS (this
  lane's code commits bind through `Corvint-Ticket: V1-5399`).
- A depth-1 clone of this branch: `go run ./tools/openspec-check --root CLONE --base HEAD~10`
  printed `REFUSE history-unavailable` and exited 2.

## NOT_RUN

- `make openspec-validate` on the repository and the real-command case of its test: this host
  has no `openspec` command. They are to be run on a host with `@fission-ai/openspec` 1.17.0:
  `make openspec-validate openspec-validate-test`. Task 2.10 stays unticked until then.
- Task 2.11 (CI install step and range) and task 3.2 (Tasks gate) belong to the orchestrator.
