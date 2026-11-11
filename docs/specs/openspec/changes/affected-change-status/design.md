# Design: affected-change-status

## Status input

`corvint affected --base SHA` already knows the base. The command computes one status per dirty
path from the base tree and the head (or working) tree:

| Status | Meaning |
| --- | --- |
| `added` | absent at the base, present now (an untracked file included) |
| `deleted` | present at the base, absent now |
| `type-changed` | present at both, and the entry kind differs (file, directory, symlink, submodule), or the entry is a symlink or a submodule at either side and its content differs |
| `modified` | present at both as a regular file |
| `unknown` | anything else: no base, an unreadable tree, a path the status source did not report |

A rename is reported as `deleted` for the old path and `added` for the new one. Similarity
detection is off.

The status reaches the planner as a parallel input to the dirty list. The existing
`Select(graph, dirty)` keeps its signature and passes `unknown` for every path, so callers that
do not have a base behave as they do today. The lane chooses the Go shape (a new function, or an
options value); the contract is only that an absent status is `unknown`.

`listingCanChange(status)` is true for every status except `modified`.

## Name-listing reads

The shell plugin classifies a listing or walking command as name-only when the command and all
of its flags are in a closed list. The list in this change:

- `ls` with no flags, or with only flags from `-1`, `-a`, `-A`, and with no `-R`: direct children
  of each operand.
- `ls -R` with only those flags: every descendant.
- `find` whose expression is empty, or holds only `-name`, `-iname`, `-path`, `-type`,
  `-maxdepth`, `-mindepth`, `-print` and `-print0` with literal arguments: descendants, bounded
  by a literal `-maxdepth` when present.
- `tree` with no flags: every descendant.

`-type` reads the entry kind, which is why `type-changed` can change a listing. Everything else
is a content read: any other flag, `du` (sizes), recursive `grep`, `rg`, `ag`, `ack`, a computed
flag or operand, and any command reached through `xargs`, `-exec` or a helper.

A name-only command contributes a name-listing read to the unit, in a field separate from
`PathTokens`. A unit that has both a name-listing read and a content read of the same directory
keeps the content read; the content read decides.

## Selection

For a dirty path `p` with status `s`:

- A content read below which `p` lies selects the unit, as today.
- A name-listing read of directory `d` selects the unit when `p` is within the read's depth below
  `d` and `listingCanChange(s)`.
- An unbounded reader with the reason `shell:tree-walk` whose every walking command is name-only
  is selected for `p` only when `listingCanChange(s)`. One content-searching walker in the unit
  makes it selected on every path, as today.

A dirty directory entry (a submodule or a symlink to a directory) is a path like any other.

## Cache

The graph cache stores the name-listing reads. Its version changes, so a cache from an older
binary is refused and rebuilt (the existing version check). The status is a plan input, not a
graph property, and is never cached.

## Why this is safe

The only new non-selection is: status `modified`, same entry kind, read by a command whose output
is a function of names and kinds alone. Every uncertainty (status, command form, flag, operand)
falls back to the current behaviour.

## Alternatives rejected

- Deriving "added" from the absence of the path in the index: there is no base tree in the
  planner today, and a staged new file is in the index.
- Treating all `ls` and `find` forms as name-only: `ls -l` prints sizes and times, and
  `find -newer`, `-size` and `-exec` read content or metadata.

## Implementation notes (lane affstatus, 2026-11-11)

- Go shape: `Graph.WithChangeStatus(map[string]ChangeStatus)` returns a plan-input copy of the
  graph, as `WithGoChanges` does; `Select(graph, dirty)` is unchanged and reads every path as
  `unknown`. `affected.ChangeStatuses` reads the base with `git ls-tree --literal-pathspecs` and
  the head through the parent directory's listing; only an exact name match counts on either
  side, so a case-folding file system cannot turn a rename into `modified`.
- `Unit.NameListings` is the subset of `PathTokens` that only name-listing reads hold, so every
  other consumer of `PathTokens` (record readers, `Readers`, `Namers`, placement) sees the same
  tokens as before; only the path-literal reader rule drops them for a `modified` path.
  `Unit.UnboundedNamesOnly` marks a name-only `shell:tree-walk`; a unit a root locator also
  reaches is never narrowed.
- Unit gate (fail closed). Today a listing's `/dir/` token, or a walker's unbounded reads, also
  select the test for computed reads that no token holds (`cat "script/$f"`, `cd script`, a
  helper, `git ls-files | xargs cat`). An admitted form therefore records a name-listing read
  only when every later stage of its pipeline is a standard-input filter and every other command
  of the test is a closed-list builtin or a closed-list reader (`cat`, `head`, `tail`, `wc`,
  `sort`, `cmp`, a non-recursive `grep`) of literal path-shaped operands, and the test sets no
  upper-case name and runs no `export`. `diff` and the checksum tools are not readers here: a
  directory operand or a check mode reads files no token names (review repair, 2026-11-11). This
  is stricter than the closed list alone and is written into AFP-V0-9961. A tracked symbolic
  link operand is a known limit: the plugin's source interface carries no link targets.
- Depth. A name-listing read selects on every non-`modified` path below the directory, at any
  depth: an added `script/sub/new.sh` creates the `sub` entry that `ls script` lists. Depth never
  narrows; a `modified` path cannot change a listing at any depth.
