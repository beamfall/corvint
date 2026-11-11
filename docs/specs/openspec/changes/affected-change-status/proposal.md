# Change proposal: base change status as a plan input, so name-listing shell readers stop selecting on edits

Change name: `affected-change-status`
Tickets: V1-5193, V1-5194 (milestone v1-0, P1). Both are open with their recall half delivered.
Product requirement served: PROD-TEST-001 (a change runs only the tests it can affect).
Owning capability spec: `docs/specs/affected-plan-selection-v0.md` (prefix `AFP-V0`). The change
amends AFP-V0-9133 and AFP-V0-9134 and adds AFP-V0-9960 to AFP-V0-9964.
Requirement IDs this change adds: AFP-V0-9960 to AFP-V0-9964. No `AFP-V0-996x` ID exists in
`docs/specs` on `origin/main` at `4c3c20333` or on any fetched lane branch (checked 2026-11-11).
Intent status: proposed (written 2026-11-11; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/affected-change-status/spec.md`).

## Why

AFP-V0-9133 and AFP-V0-9134 made shell tests that list a directory or walk the tree recall-safe:
a file added to or deleted from the directory selects the test. Both state their own precision
gap: the plan reads dirty paths without their base status, so an edit of an existing file selects
the same tests, although an edit cannot change a name listing.

The audit of `origin/main` at `4c3c20333` (2026-11-11) confirms the gap against the tickets:

- V1-5193 criterion 2 ("not selected for an edit of an existing file") is not met.
  `Select(graph, dirty []string)` at `internal/liveverify/affected/select.go:315` takes bare
  paths. The shell plugin puts the `/dir/` fragment in `PathTokens`, which a content read matches
  too.
- V1-5194 criterion 2 ("edits that cannot change a name listing do not select name-only
  unbounded walkers") is not met. Every `shell:tree-walk` reader is selected on every dirty path.
- The measured criteria of both tickets (selection-baseline misses and narrow count, the pr-39
  replay, mutation evidence) were never run.

Every repository edit therefore selects each test that counts or lists files, which is a fixed
cost on every plan.

## What Changes

- The planner takes each dirty path's change status against the base: added, deleted, renamed,
  type-changed, modified or unknown (AFP-V0-9960).
- The shell plugin records a name-listing read apart from a content read, for a closed list of
  command forms whose output depends only on names (AFP-V0-9961).
- A name-listing read selects its test only for a dirty path whose status can change a listing.
  Modified is the only status that cannot (AFP-V0-9962).
- The same rule applies to an unbounded walker whose every walking command is name-only
  (AFP-V0-9963).
- The graph cache carries the new field under a new cache version (AFP-V0-9964).

## Non-goals

- Deleted-script admission (AFP-V0-2751, V1-5191). It is on an unmerged branch and stays there.
  V1-5193 criterion 3 depends on it and is not delivered by this change.
- Any change to how content reads select. A content read still selects on every dirty path below
  it.
- Change status for languages other than shell.
- Rename detection by similarity. A rename is an addition plus a deletion.

## Failure modes

- No base, or a status that cannot be established: the path is `unknown`, and `unknown` selects
  (fail closed). The plan never treats a missing status as `modified`.
- A command form outside the closed list (for example `ls -l`, `find -newer`, `find -exec`,
  recursive `grep`): it stays a content read and selects on edits, as today.
- A cache written before this change: refused by version and rebuilt. It is never read as "no
  name-listing reads".
- A caller of `Select` that passes no status: every path is `unknown`, so the result equals
  today's.

## Acceptance evidence

- Unit and fixture tests named in `tasks.md`, each asserting both selection and non-selection.
- Mutation evidence for the status test: each mutant that weakens it (modified treated as able to
  change a listing is harmless; unknown or added treated as modified is not) is killed by a named
  test.
- `selection-baseline`: misses 0, and the narrow count not below the recorded floor.
- The pr-39 replay: no `FALLBACK` row.
- A build-log entry with the measured table.

## Rollback

Revert the change's commits. With the name-listing field absent the plugin emits content reads
again, which selects more, never less. The cache version change makes an old binary rebuild its
cache.
