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
  upper-case name and runs no `export`. (Superseded by the closed grammar below: the gate no
  longer judges which names are environment-style.) `diff` and the checksum tools are not readers here: a
  directory operand or a check mode reads files no token names (review repair, 2026-11-11). This
  is stricter than the closed list alone and is written into AFP-V0-9961. A tracked symbolic
  link operand is a known limit: the plugin's source interface carries no link targets.
- Depth. A name-listing read selects on every non-`modified` path below the directory, at any
  depth: an added `script/sub/new.sh` creates the `sub` entry that `ls script` lists. Depth never
  narrows; a `modified` path cannot change a listing at any depth.

## Gate redesign: a closed grammar (lane affgate, 2026-11-11)

Two independent reviews found fail-open paths in the unit gate, the second after the first
repair (`printf -vPATH`, a computed name `v=PATH; read -r $v`, `set -a` with
`$((CLICOLOR_FORCE=1))`). The cause was the shape of the rule: it admitted every "output
builtin" (`set`, `shift`, `local`, `readonly`, `unset`, `read`, `for` among them) and then tried
to deny each way of writing an environment-style variable. Bash has too many. The gate is now a
closed grammar: it states what it admits, and refuses everything else. A refused unit keeps
today's content read and is selected, which only costs precision; 0 of 250 shell units in this
repository qualified even under the permissive gate.

### What is admitted

| Level | Admitted | Why it cannot change a read |
| --- | --- | --- |
| File | no function definition, here-document, `case` statement or `((` arithmetic command | the lexer folds each of these away without a command (a `case` header and its patterns, an arithmetic command, a here-document body), so their words would escape the word rules |
| Word | not an assignment word in any position or case; not `{name}`; no backtick, `<(`, `>(`; every `$` is `$name`, `${name}`, `${name:-text}`, `${name-text}` (literal text), `$#`, `$?`, `$0`-`$9` or `$(…)` | every remaining expansion only reads; `$((`, `$[`, `${name=…}`, `${name:=…}` and `{name}>` write a variable; a backtick's inner text is lexed approximately, so it is refused rather than trusted; `$(…)`'s inner commands are commands of the unit and pass the same gate |
| Command | control words and redirections alone (`if then else elif fi while until do done ! { } ( )` joined by `; & && \|\| \|`) | they sequence commands and write nothing |
| | `echo`, `true`, `false`, `:`, `exit` | print or set a status |
| | `printf` with a literal first argument not beginning `-` | bash's `printf` parses options only before its first operand, so `-v` can only be the first argument; a literal one cannot expand to it |
| | `test`, `[`: literal dash arguments from `-e -f -d -h -L -z -n -eq -ne -lt -le -gt -ge -a -o`; an expanded argument only as a double-quoted single field in `[ X ]`, `[ ! X ]`, `[ -z X ]`, `[ -n X ]`, `[ X OP Y ]` with OP from `= == != -eq -ne -lt -le -gt -ge` | `test` picks operators by argument count and position after expansion; in these shapes an expansion is always an operand, never an operator such as `-s` or `-nt`, and never a file operand (a computed existence test reads a name no token holds); `test` compares integers without arithmetic evaluation |
| | `[[` with the same unary operators and string comparisons; no `-eq` family, no `=~`; an expansion only with no file operator | `[[` evaluates the operands of `-eq` and the rest as arithmetic, which can assign (`[[ x++ -eq 0 ]]`); `=~` sets `BASH_REMATCH`; operators are fixed before expansion |
| | `cat head tail wc sort cmp grep egrep fgrep`: short options judged by unquoted value (no grep `-r -R -d -e -f`), digit counts, literal grep pattern, literal path-shaped operands that do not end in `/` or `/.` and do not name a directory a listing of the unit names | each reads only its operands or standard input |
| Position | an expansion only in an argument of `echo`, `printf` (after the first), `test`, `[`, `[[`, `:` or `exit`; a redirection target `1 2 &1 &2 - /dev/null` or a literal path-shaped word | an expansion elsewhere could become a command, an option, an operand or a target |
| Pipeline | an admitted listing form whose later stages are filters (`wc`, `sort`, `head`, `tail`, non-recursive `grep` with a literal pattern) | unchanged from the first version |

Removed against the first version: `set`, `shift`, `local`, `readonly`, `unset`, `read`, `for`,
`return` and `esac` (with `case`). Nothing beyond the starting list (`echo`, `printf`, `true`,
`false`, `:`, `exit`, the test builtins and the control words) is kept, and `case`/`esac` was
dropped from it because the lexer discards a `case` header and its patterns, which bash expands.
No variable can be written by any admitted construct, so `assignsEnvironment`,
`environmentName` and `environmentDefault` are gone: there is one mechanism.

### Directory operands of readers (round-2 item 4)

A reader operand or input redirection target that ends in `/` or `/.`, or whose cleaned path
(leading `./` and trailing `/` removed) equals a directory a listing of the test names, is
refused. An operand naming some other directory cannot be recognised statically; it is kept,
because every admitted reader (GNU and BSD `cat`, `head`, `tail`, `wc`, `sort`, `cmp`, and `grep`
without a recursion or `-d` option) fails on a directory operand and reads nothing below it. That
reliance is stated in AFP-V0-9961.

### What the gate does not model

- The runner's environment. A variable exported before the unit starts (`PATH`, `BASH_ENV`,
  `SHELLOPTS`, `CLICOLOR_FORCE`, `GREP_OPTIONS`) or a positional parameter is outside the unit's
  words. A runner that sets one changes reads the gate cannot see; the gate only guarantees that
  the unit itself changes nothing.
- Shell-maintained variables (`_`, `?`, `PIPESTATUS`, `LINENO`, `RANDOM`'s seed): every shell
  writes them, no admitted word names them as a write, and no admitted utility reads them.
- The tracked-symlink operand limit, as documented in the first version.
- A write redirection (`>` to a literal path) may create or overwrite a file that a later
  listing names; what the listing then prints depends on the unit's own write, not on the
  repository, so it does not observe a repository edit.

### Proof plan

Table rows in `TestNameListingForms_AFPV09961` name each refused construct, the four round-2
items and the base rows; a generator in the same test composes every refused construct before
and after an admitted pipeline and inside `if`, a subshell and a brace group and asserts refusal
in every position. Mutation evidence for each class is in `docs/build-log/2026-11-11-affstatus.md`.
