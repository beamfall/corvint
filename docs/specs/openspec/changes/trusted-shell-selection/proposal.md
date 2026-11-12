# Change proposal: shell-test selection and the doc-gates target list come from the base

Change name: `trusted-shell-selection`
Ticket: V1-5097 (P1 BUG, milestone `v1-0`): "CI shell-test selection and doc-gates target list are
built from the candidate tree, so a PR can weaken its own checks."
Owning capability spec: `docs/specs/ci-narrowing-v0.md`, prefix `CIN-V0`
Requirement IDs this change adds: CIN-V0-8724 to CIN-V0-8727 (inside the block CIN-V0-8722 to
CIN-V0-8739 given to lane ghci2; CIN-V0-8722 and CIN-V0-8723 are used by V1-5258 and V1-5349)
Requirement IDs this change modifies: CIN-V0-2320 (the `shell-test-selected` job), AFP-V0-2401
(`make shell-test-affected`) in `affected-plan-selection-v0.md`, by narrowing who builds the planner
Owner answers this change implements: none yet; the ticket's acceptance criteria are the input
Intent status: proposed (written 2026-11-11 by lane ghci2; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No code for this change exists; lane ghci2 stopped at this proposal.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/trusted-shell-selection/spec.md`.

## Why

On a pull request GitHub runs the merge commit's own `ci.yml`, so every step executes candidate
code unless it builds its tools from a trusted commit. A change under `.github/` is held by the
`ci-control-plane` waiver (only an approver's status on the exact head lets it merge), and the Go
shard narrowing builds its planner, selector and driver from a trusted `main` commit
(`corvint-pr-bin`, CIN-V0-011). Two decisions that choose what runs do neither:

- `shell-test-selected` runs `make shell-test-affected`, whose recipe in the candidate `Makefile`
  builds the candidate `cmd/corvint` and `tools/shell-test-select` and maps the candidate plan to
  the candidate `shell-test:` prerequisite list. A pull request that edits the selector to select
  nothing, or drops a step from that list, runs fewer shell tests, and nothing under `.github/`
  changed, so no waiver is asked for.
- `doc-gates` runs `make -s doc-gates`, which runs `DOC_GATE_TARGETS` of the candidate `Makefile`.
  A pull request that removes a gate from that list skips it in its own run.

## What exists today

Verified on `b55635262788687c3eea9a08a94378e4ab8856a4` (origin/main, 2026-11-11):

- `ci.yml` job `doc-gates` (key at line 141): checkout of the merge commit, then
  `make -s doc-gates` (line 180) and `make -s ai-coauthor-trailers-check`.
- `ci.yml` job `shell-test-selected` (line 190): checkout, then
  `make -s shell-test-affected BASE="$(git merge-base HEAD "$BASE_SHA")"` (line 221).
- `Makefile` `shell-test-affected` (line 1026): `go build ./cmd/corvint ./tools/shell-test-select`
  from the checkout, `corvint affected --base`, `shell-test-select --plan --makefile Makefile`,
  then `make --keep-going` of the printed targets. A failed plan is reported and the selector
  prints every step (FULL, AFP-V0-2401).
- `Makefile` `DOC_GATE_TARGETS` (line 1153) and `doc-gates` (line 1206):
  `doc-gates-summary.sh ... make --keep-going -j $(addprefix doc-gate/,$(DOC_GATE_TARGETS))`,
  then `go run ./conformance/use-cases-v0`. `doc-gate/%` (line 1198) runs
  `script/doc-gate-timed.sh NAME make NAME`.
- `tools/shell-test-select/main.go`: `ReadSteps` parses the `shell-test:` prerequisite list and
  each step's recipe; FULL when the plan is unreadable, names the Makefile as dirty or carries an
  unknown outside the shell allowlist.
- Trusted builds today: `corvint-pr-bin` (`ci.yml` lines 614-633) from `steps.pin-main.outputs.tool`
  or `CORVINT_PR_TOOL_SOURCE`; `linecov-corvint` (`ci.yml` line 1363) from `tools`, the merge base
  of the event base and `main`.

## What changes

1. `shell-test-selected` builds `corvint` and `shell-test-select` from the event base in a worktree
   outside the tested checkout and plans the tested tree with them (CIN-V0-8724).
2. The shell steps run are the union of what that selector picks from the base `Makefile`'s
   `shell-test:` list and from the candidate's list; a base step the candidate no longer defines
   forces FULL with a visible reason (CIN-V0-8725).
3. `doc-gates` also runs every gate of the base `DOC_GATE_TARGETS` that the candidate list leaves
   out; a base gate the candidate no longer defines is a visible reason (CIN-V0-8726).
4. An extract-and-run fault-injection test proves both without a hosted run (CIN-V0-8727).

## Impact

- Files: `.github/workflows/ci.yml` (the `shell-test-selected` and `doc-gates` steps) and a new
  extract-and-run test under `.github/forward-rows/`, landed as one `.github`-only pull request;
  `docs/specs/ci-narrowing-v0.md` (CIN-V0-8724 to CIN-V0-8727, CIN-V0-2320 amended) and
  `docs/specs/affected-plan-selection-v0.md` (AFP-V0-2401 amended) in a separate code pull request.
- Behaviour: a pull request can no longer narrow its own shell steps or doc gates by editing the
  selector, the `shell-test:` list or `DOC_GATE_TARGETS`; a removed step forces FULL with a reason
  and a removed gate is named.
- Cost: one extra base build of `corvint` and `shell-test-select` per shell-test job, and one extra
  `make` call in `doc-gates` only when the lists differ.

## Non-goals

- Running the base revision's gate or test *implementations*. A gate or shell test checks the
  candidate content and may legitimately change with it; editing a gate script is a visible diff of
  that script. This change protects only which gates and steps are chosen.
- Moving the lists under `.github/` (the `ci-control-plane` waiver would then hold them, but every
  ordinary gate addition would need an admin waiver).
- Changing the Go shard narrowing, `docs-plan`, the cadence classifier or any job's required status.
- Blocking a pull request that legitimately removes a step or gate: it gets FULL and a reason, not a
  failure (owner review of the visible reason is the control).

## Failure modes

- The event base does not build its selector: the job runs FULL over both lists, named in the
  summary; it never falls back to the candidate's selector.
- The base-built planner on the candidate tree reports `self-binary-stale`: expected (the candidate
  changes Go source) and kept as a named degradation, as CIN-V0-8722 does; any other degradation is
  FULL.
- A step or gate renamed in one change is "removed" from the base view and "added" in the candidate:
  FULL plus a reason for the shell steps; both names are visible for the gates.
- A candidate can still redefine a target's recipe to do nothing; that is an edit of the gate
  itself, outside the selection this change protects (see non-goals).
- Cost: one extra `go build` of two binaries at the base per shell-test job (about a minute on a
  hosted runner), and one extra `make` invocation in `doc-gates` only when the lists differ.

## Acceptance evidence

- CIN-V0-8727's test: a fixture repository whose candidate edits the selector to print nothing still
  runs the base-selected step; a candidate that drops a step from `shell-test:` still runs it; a
  candidate that drops a gate from `DOC_GATE_TARGETS` still runs it; a candidate that deletes a
  step prints the FULL reason; a base that does not build runs FULL. Run in `go-static`.
- Workflow binding tests for the base build and the union, as for CIN-V0-8722.
- One hosted pull-request run whose summary names the base-built selector (NOT_RUN until merged).

## Rollback

Revert the `ci.yml` steps and the test; `shell-test-selected` again runs `make shell-test-affected`
from the candidate and `doc-gates` runs only the candidate list. No state migrates.
