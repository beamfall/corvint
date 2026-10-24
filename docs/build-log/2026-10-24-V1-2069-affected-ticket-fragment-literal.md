# V1-2069: a fragment path literal does not name a Tasks ticket record

Owner input: ticket V1-2069 (V1-2049 residual; no GitHub issue). PR #1314 measured that every
ticket-only Corvint diff kept each `.taskman/` record as `UNOWNED_DIRTY_PATH`, because a generic
literal such as `/tickets/` in `internal/tasks/transaction` counted as naming every ticket
record under AFP-V0-088 (2). AFP-V0-361 gave `internal/tasks/transaction` an empty declared read
scope, but the same shape stayed in other packages. AFP-V0-088 (2) is amended in
`docs/specs/affected-plan-v0.md` under the owner's standing auto-accept of lane requirements
(2026-10-11). No new requirement ID was added.

## Decision

For the AFP-V0-088 (2) "names it" test, a path literal names a projection file only when one of
its component runs ends at the file's whole name. A run that ends at a directory component
(`/tickets/`, `.taskman/tickets/`, `/.taskman`) or at a prefix of the file name (the lexer keeps
`.taskman/tickets/V1-` from `".taskman/tickets/V1-%04d.json"`) names a family of records, not the
one that changed. Selection is unchanged: the reader rules still select such a literal's unit as
a `PATH_LITERAL_READER`, so no test that ran before is dropped. Only the path's classification
changes, from an `UNOWNED_DIRTY_PATH` unknown to the `TASK_STORE_PROJECTION` `nonSource` group.

We did not add more declared read scopes. That would fix one package at a time and needs a
Landlock-confined full CI run for each new declaration. The literal rule fixes the shape wherever
it occurs. A whole-name literal (`.taskman/tickets/V1-0001.json`, `V1-0001.json`,
`.taskman/queue.json`) still keeps its path unknown, as before.

## Measurements

These were taken on Corvint at base 7dc89ccff (PR #1725 head), darwin, using the branch binary
before and after the change. Three record files were dirtied:
`.taskman/tickets/V1-2069.json`, `.taskman/tickets/CORE-MERGE-PRESERVE-ANCESTRY-1.json` and
`.taskman/releases/v1-0.json`.

- Before: 3 `UNOWNED_DIRTY_PATH` entries, no `nonSource` member, 106 selected units, 97
  `LANGUAGE_FRONTIER` entries.
- After: both ticket records are in the `TASK_STORE_PROJECTION` group. 1 `UNOWNED_DIRTY_PATH`
  entry remains, for `.taskman/releases/v1-0.json`, because `internal/contextindex` and
  `internal/liveverify/affected` tests carry that whole path as a literal and
  `internal/tasks/cli` carries `v1-0.json`. Still 106 selected units and 97 frontier entries.

The ticket-record literals that the old rule counted were these: `/.taskman/`
(`internal/cemchain`), `.taskman/tickets/V1-` (`internal/contextindex`), `/tickets/` (an
`internal/liveverify/affected` test fixture), `/.taskman` (`internal/opencodequalification`) and
`.taskman/tickets/` (`internal/tasks/cli`).

The plan stays `UNKNOWN`. That comes from the language frontiers of the rule (d) unbounded readers
that every dirty path selects (the V1-0081 floor), not from the ticket paths.

## Residual

- A test that builds a record path from a fragment and reads this repository's real store keeps
  its selection, but the record no longer keeps the plan unknown. This is the same trust the
  AFP-V0-021 data-file rule already extends.
- `queue.json`, `policy.json` and `releases/v1-0.json` are named by whole-name literals in this
  repository, so a diff that touches them stays `UNKNOWN` by AFP-V0-088 (2) as designed.

Rollback: in `internal/liveverify/affected/taskstate.go`, make `named` call `namesAny` instead of
`namesFileAny`, delete `namesFileAny` and `pathToken.namesFile`, and revert the AFP-V0-088 (2)
sentence.
