# Spec delta: affected-change-status

The lane copies each statement below into `affected-plan-selection-v0.md`, inside its
requirements section, as a numbered definition line that begins
`(proposed 2026-11-11; delivery not-started; V1-5193, V1-5194)`. It also replaces the sentence in
AFP-V0-9133 that says narrowing "is not admitted here" with a pointer to AFP-V0-9962, and the
words "selected on every dirty path" in AFP-V0-9134 with a pointer to AFP-V0-9963.

## ADDED Requirements

### Requirement: AFP-V0-9960 Change status is a plan input

The affected planner MUST accept, for each dirty path, one change status against the base:
`added`, `deleted`, `type-changed`, `modified` or `unknown`. A rename MUST be reported as
`deleted` for the old path and `added` for the new path. `modified` MUST mean a regular file at
both sides; a changed symlink or submodule entry, which can redirect a listing, MUST be
`type-changed`. A path whose status is absent or cannot
be established MUST be `unknown`. `corvint affected --base` MUST compute the status from the base
tree and pass it to the planner.

#### Scenario: Untracked file

- **GIVEN** a file that is absent at the base and untracked in the worktree
- **WHEN** `corvint affected --base` plans the change
- **THEN** the path's status is `added`

#### Scenario: No base

- **GIVEN** a caller that passes dirty paths with no status
- **WHEN** the planner selects
- **THEN** every path's status is `unknown` and the plan equals the plan before this change

### Requirement: AFP-V0-9961 Name-listing reads are recorded apart from content reads

The shell plugin MUST record a name-listing read, separate from a content read, for a
directory operand of a command whose output depends only on entry names and kinds. The
admitted forms are a closed list: `ls` with no flags or only `-1`, `-a`, `-A` (direct
children); the same with `-R` (all descendants); `find` with an empty expression or only
`-name`, `-iname`, `-path`, `-type`, `-maxdepth`, `-mindepth`, `-print`, `-print0` with
literal arguments; and `tree` with no flags. Every other form MUST stay a content read. When
a unit holds a content read and a name-listing read of the same directory, the content read
MUST decide. An admitted form MUST also stay a content read unless every later stage of its
pipeline is a standard-input filter (`wc`, `sort`, `head`, `tail`, a non-recursive `grep`
with a literal pattern) and the whole test passes a closed grammar: the gate admits only
what follows and MUST refuse every construct it does not recognise. (1) The file defines no
function and holds no here-document, `case` statement or `((` arithmetic command. (2) No
word of any command is an assignment word (`x=1`, `x+=1`, `x[0]=1`, in any position and of
any case), a `{name}` redirection word, a backtick or process substitution; and every `$` in
a word is `$name`, `${name}`, `${name:-text}`, `${name-text}` with literal text, `$#`, `$?`,
a single-digit positional parameter, or a `$(…)` substitution whose inner commands are
commands of the test held to this same grammar. (3) Every command outside an admitted
pipeline is one of: control words and redirections alone (`if`, `then`, `else`, `elif`,
`fi`, `while`, `until`, `do`, `done`, `!`, `{ }`, `( )` joined by `;`, `&`, `&&`, `||` or
`|`); `echo`, `true`, `false`, `:` or `exit`; `printf` whose first argument is literal and
does not begin with `-`; `test` or `[` whose literal arguments beginning with `-` are
`-e -f -d -h -L -z -n -eq -ne -lt -le -gt -ge -a -o`, where an argument holding an expansion
is a double-quoted single field and appears only as `[ X ]`, `[ ! X ]`, `[ -z X ]`,
`[ -n X ]` or `[ X OP Y ]` with OP one of `= == != -eq -ne -lt -le -gt -ge`; `[[` with the
same unary operators and string comparisons but no integer operator (whose operands `[[`
evaluates arithmetically) and no `=~` (which sets `BASH_REMATCH`), and with an expansion
only when no file operator is present; or `cat`, `head`, `tail`, `wc`, `sort`, `cmp`, `grep`,
`egrep` or `fgrep` with short options only (no `-r`, `-R`, `-d`, `-e` or `-f` for a grep,
judged by the unquoted value), a digit count after `head` or `tail` `-n` or `-c`, a literal
grep pattern, and literal path-shaped operands. (4) An expansion appears only in an argument
of `echo`, `printf` after the first, `test`, `[`, `[[`, `:` or `exit`; a redirection target
is `1`, `2`, `&1`, `&2`, `-`, `/dev/null` or a literal path-shaped word. (5) A reader
operand or input redirection target that ends in `/` or `/.`, or that names a directory a
listing of the test names, MUST be refused. Under this grammar no command can write a
variable, change the working directory, run a program other than the named utilities, or
open a file by a name no literal token holds, so no write needs to be judged
environment-style. `diff`, a checksum tool, `set`, `read`, `for`, `cd` and every other
command are refused as unrecognised. Not modelled, with the limit each leaves: the
environment the test runner exports before the unit starts (`PATH`, `BASH_ENV`,
`SHELLOPTS`, `CLICOLOR_FORCE`, `GREP_OPTIONS`, positional parameters) is outside the unit's
words, so a runner that exports one changes reads this gate cannot see; variables the shell
itself maintains (`_`, `?`, `PIPESTATUS`, `LINENO`) are written by every shell and no word
of the unit names them as a write; a reader operand naming a directory without a trailing
slash that no listing of the test names is admitted because every admitted reader fails on a
directory operand and reads nothing below it; and an operand that is a tracked symbolic
link is matched by its own path and not by its target's, so a link into a listed directory
is not seen as a content read of it.

#### Scenario: Long listing is a content read

- **GIVEN** a shell test that runs `ls -l script | wc -l`
- **WHEN** the plugin builds the unit
- **THEN** the unit has a content read of `script/` and no name-listing read

#### Scenario: Plain listing is a name-listing read

- **GIVEN** a shell test that runs `ls script | wc -l`
- **WHEN** the plugin builds the unit
- **THEN** the unit has a name-listing read of the direct children of `script/`

#### Scenario: A construct outside the grammar keeps the content read

- **GIVEN** a shell test that runs `v=PATH`, then `printf -v "$v" '%s' stub`, then `ls script | wc -l`
- **WHEN** the plugin builds the unit
- **THEN** the gate refuses the assignment word and the `printf -v`, and the unit has a content
  read of `script/` and no name-listing read

### Requirement: AFP-V0-9962 A name-listing read selects only on a status that can change a listing

A name-listing read MUST select its unit for a dirty path within the read's depth whose status is
`added`, `deleted`, `type-changed` or `unknown`, and MUST NOT select it for a path whose status
is `modified`.

#### Scenario: Edit of an existing file

- **GIVEN** a test whose only read of `script/` is `ls script | wc -l`
- **WHEN** an existing file in `script/` is edited
- **THEN** the test is not selected by that read

#### Scenario: Added file

- **GIVEN** the same test
- **WHEN** a file is added to `script/`
- **THEN** the test is selected

#### Scenario: Unknown status

- **GIVEN** the same test and a dirty path in `script/` with status `unknown`
- **WHEN** the planner selects
- **THEN** the test is selected

### Requirement: AFP-V0-9963 A name-only unbounded walker follows the same rule

An unbounded reader with the reason `shell:tree-walk` whose every walking command is an admitted
name-only form MUST be selected for a dirty path only when the path's status is `added`,
`deleted`, `type-changed` or `unknown`. A unit with at least one content-searching walker MUST be
selected on every dirty path.

#### Scenario: Name walker and an edit

- **GIVEN** a test whose only walker is `find . -name '*.md'`
- **WHEN** an existing Go file is edited
- **THEN** the test is not selected by the walker

#### Scenario: Content searcher and an edit

- **GIVEN** a test that runs `grep -rn TODO .`
- **WHEN** an existing Go file is edited
- **THEN** the test is selected

### Requirement: AFP-V0-9964 The graph cache carries name-listing reads under a new version

The graph cache MUST store name-listing reads, and its version MUST change with this field. A
cache written under an older version MUST be rebuilt and MUST NOT be read as holding no
name-listing reads. Change status MUST NOT be cached.

#### Scenario: Older cache

- **GIVEN** a graph cache written before this change
- **WHEN** the planner loads it
- **THEN** the cache is refused by version and the graph is rebuilt
