# Affstatus: base change status narrows name-listing shell readers (V1-5193, V1-5194)

Owner intent: the affstatus lane charter and the committed proposal
`docs/specs/openspec/changes/affected-change-status/` (intent `proposed`; only the owner accepts
it). Requirements AFP-V0-9960 to AFP-V0-9964, all proposed, delivery experimental. Base
`origin/main` `4c3c203336a6cc4a9772af39b1a771f34236138e`.

## What changed

- AFP-V0-9960. `affected.ChangeStatuses` classifies each changed path against the base commit:
  `git --literal-pathspecs ls-tree -z --full-tree BASE -- PATHS` for the base side (batches of
  256) and the parent directory's listing for the head side. Only an exact name match counts on
  either side, so a case-folding or normalizing file system reads a rename as deleted plus
  added, never modified. `modified` needs a regular file (100644 or 100755) at the base and a
  regular file now; anything else present at both sides is `type-changed`; an unreadable side,
  a path present at neither side, a trailing-slash path and an unreadable base are `unknown`.
  `Graph.WithChangeStatus` carries the result as a plan input (never cached); `Select(graph,
  dirty)` keeps its signature and reads every path as `unknown`. `corvint affected --base`
  (and the review snapshot plan, which shares `planAffectedGraph`) passes it; `--path` and the
  worktree-only form do not.
- AFP-V0-9961. `Unit.NameListings` is the subset of `PathTokens` held only by name-listing
  reads. Every other consumer of `PathTokens` sees the tokens it saw before. The shell plugin
  admits the closed list from the design, and only when every later stage of the command's
  pipeline is a standard-input filter (`wc`, `sort`, `head`, `tail`, non-recursive `grep`, each
  with short flags only) and the file passes a unit gate (below).
- AFP-V0-9962. The path-literal reader rule matches a `modified` path against the unit's tokens
  less its `NameListings`; every other status uses all tokens, so `unknown` plans exactly as
  before.
- AFP-V0-9963. `Unit.UnboundedNamesOnly` marks a `shell:tree-walk` test whose every walking
  command is an admitted name-only form. Rule (d) selects it only for a dirty path whose status
  can change a listing, witnessed by the smallest such path. A unit a root locator also reaches
  is never narrowed.
- AFP-V0-9964. Both fields are in the graph digest (omitted when empty, so graphs without them
  keep their digest) and the cache entry; the cache profile is `corvint-affected-graph-cache/4`.

## Unit gate (a fail-closed tightening of AFP-V0-9961)

While writing the tests I found that the closed list alone drops recall that today's plan has.
`ls script | wc -l` beside `for f in a b; do cat "script/$f"; done` is selected on an edit of
`script/a` today only because of the listing's `/script/` token: the computed read leaves the
token `script/`, which names only a file called `script` (V1-0290). Likewise `cd script && cat
a.sh`, a helper run, or `find . -name '*.md' | wc -l` beside `git ls-files '*.md' | ... cat
"$f"` (selected today only by the walker's unbounded reads). So a file records name-listing
reads only when every command outside an admitted pipeline is an output builtin, an existence or
kind test (`-e -f -d -h -L` and value comparisons; not `-s`, `-nt`, `-x` or other attribute
tests), or an operand reader (`cat`, `head`, `tail`, `wc`, `sort`, `cmp`, non-recursive `grep`
without `-e`/`-f`) whose operands and input redirections are literal path-shaped words, and the
file sets no environment-style name. A sourced file, a function call, a directory change,
`exec`, `run`, `env`, a program or a computed operand anywhere keeps every listing a content
read. The sentence is added to AFP-V0-9961 in the spec and the delta.

### Review repair (2026-11-11)

The independent review found the unit gate fail-open, and failing rows confirmed it before the
repair. The first version admitted `diff` without `-r` and the checksum tools as operand
readers. `diff script/ golden/` reads every file below its directory operands, and
`sha256sum -c sums` reads the files the sums file names; neither read has a token below the
listed directory, so with `ls script | wc -l` beside it a `modified` `script/a.sh` dropped the
unit (`diff_dir`, `diff_dot`, `diff_file_dir`, `sha_check`, `shasum_check`, `md5_check`), and a
name-only walker beside one stayed names-only (`walk_diff`, `walk_sha`). Both tool families are
now programs to the gate, in every form. The gate also refused only a `PATH` assignment;
`CLICOLOR_FORCE=1` makes BSD `ls` colour by mode bits and `GREP_OPTIONS=-r` makes a filter
recurse. It now refuses `export` and every way of setting an upper-case name (an assignment
word, `read NAME`, `for NAME in`, `printf -v NAME`, a `${NAME:=...}` expansion).

Known limit, not repaired: the shell plugin matches an operand that is a tracked symbolic link
by the link's own path. `cat fixture.txt`, where `fixture.txt` links to `script/a.sh`, beside
`ls script | wc -l` is therefore admitted, and a `modified` `script/a.sh` does not select the
unit. Before this capability the listing's content read covered that case by accident. The
plugin's source interface carries no link targets; this is ticketed, not fixed here.

Depth never narrows: an added `script/sub/new.sh` creates the `sub` entry `ls script` lists, so
a name-listing read selects on every non-`modified` path below its directory.

## Tests

- `internal/liveverify/affected`: `TestClassifyChange_AFPV09960`, `TestChangeStatuses_AFPV09960`
  (real Git: added, committed addition, deleted, renamed, file to directory, changed symlink,
  modified, untracked, absent at both sides, no base, unreadable base),
  `TestGraphCacheNameListings_AFPV09964` (round trip, digest, plans under both statuses, a `/3`
  entry rebuilt).
- `internal/liveverify/affected/shell`: `TestNameListingForms_AFPV09961` (15 admitted rows, 38
  refused rows: `ls -l`, `--all`, `find -newer`, `-exec`, `-size`, `-o`, `-L`, a computed
  `-name`, `tree -s`, `du`, `grep -r`, a computed operand, `xargs`, `wc --files0-from`, a loop, a
  `while read`, a group, a captured value, a redirection, `tee`, `run`, `env`, a function, a
  shadowed filter, `PATH`, a sourced file, `exec`, a glob, and the gate rows),
  `TestNameListingContentDecides_AFPV09961`, `TestNameListingSelection_AFPV09962`,
  `TestNameOnlyTreeWalker_AFPV09963`; the AFP-V0-9133 and 9134 tests pass unchanged.
- `cmd/corvint`: `TestAffectedChangeStatusCLI_AFPV09960` (edit with `--base`: lister and walker
  not selected, `ls -l` selected; without `--base`: all selected; untracked addition: all
  selected).

Existing assertion changed: `TestTestEmbedGraphCacheRejectsOldProfile_AFPV03352` checked the
literal `corvint-affected-graph-cache/3 ` header before injecting `/1`. AFP-V0-9964 requires the
profile to change, so the literal is now `affected.GraphCacheProfile`; the test's claim (an old
profile is rebuilt) is unchanged. AFP-V0-3303 and AFP-V0-3352 still name `/3` as the profile
their facts need; `/4` carries those facts too, and their text was left for the owner.

## Mutation evidence

Each mutant was applied to the source, its test run once through `script/heavy.sh env
GOTOOLCHAIN=local go test -count=1 -run '^NAME$' PKG`, and the source restored (driver:
`/tmp/affstatus-mut/run.py`, not committed). Earlier mutants M11 (captured output admitted), M13
(function shadow ignored) and M14 (sourcing test admitted) targeted checks that the unit gate
made redundant; the checks were removed rather than kept as dead code.

| Mutant | Change | Killing test | Result |
| --- | --- | --- | --- |
| M1 | `ListingCanChange`: `added` treated as `modified` | `TestClassifyChange_AFPV09960`, `TestNameListingSelection_AFPV09962` | killed |
| M2 | `ListingCanChange`: `unknown` treated as `modified` | `TestNameListingSelection_AFPV09962` | killed |
| M3 | `ListingCanChange` always false | `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M4 | `ListingCanChange` always true (harmless for recall) | `TestNameListingSelection_AFPV09962` | killed |
| M5 | an absent status defaults to `modified` | `TestNameListingSelection_AFPV09962` | killed |
| M6 | one regular side is `modified` | `TestClassifyChange_AFPV09960` | killed |
| M7 | `ls -l` admitted | `TestNameListingForms_AFPV09961` | killed |
| M8 | `find -newer` admitted | `TestNameListingForms_AFPV09961` | killed |
| M9 | any `find` primary admitted (`-exec`, `-size`, `-o`, `-L`) | `TestNameListingForms_AFPV09961` | killed |
| M10 | `xargs` accepted as a filter | `TestNameListingForms_AFPV09961` | killed |
| M12 | a non-control prefix (`run`, `env`) skipped | `TestNameListingForms_AFPV09961` | killed |
| M15 | a non-admitted walker not marked a content walk | `TestNameOnlyTreeWalker_AFPV09963` | survived: equivalent, the unit gate already refuses every walker that is not admitted |
| M16 | reader rule ignores status | `TestNameListingSelection_AFPV09962` | killed |
| M17 | reader rule drops listings for every status | `TestNameListingSelection_AFPV09962` | killed |
| M18 | rule (d) ignores status for name-only walkers | `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M19 | a content read does not decide | `TestNameListingContentDecides_AFPV09961` | killed |
| M20 | cache profile left at `/3` | `TestGraphCacheNameListings_AFPV09964` | killed |
| M21 | cache drops `NameListings` | `TestGraphCacheNameListings_AFPV09964` | killed |
| M22 | a non-bare operand admitted | `TestNameListingForms_AFPV09961` | killed |
| M23 | a path absent now classified `modified` | `TestChangeStatuses_AFPV09960` | killed |
| M24 | unit gate off | `TestNameListingForms_AFPV09961`, `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M25 | `test -s` admitted | `TestNameListingForms_AFPV09961` | killed |
| M26 | a computed operand admitted by the gate | `TestNameListingForms_AFPV09961` | killed |
| M27 | `cd` admitted by the gate | `TestNameListingForms_AFPV09961` | killed |
| M28 | `corvint affected --base` passes no status | `TestAffectedChangeStatusCLI_AFPV09960` | killed |
| M29 | a `PATH` assignment ignored | `TestNameListingForms_AFPV09961` | killed |
| M30 | a pipeline stage need not be a filter | `TestNameListingForms_AFPV09961` | killed |
| M31 | a helper unit admitted | `TestNameListingForms_AFPV09961` | killed |

## Measurements

All runs on this WSL2 host, through `script/heavy.sh`, foreground unless stated.

| Measurement | Before (base `4c3c20333`) | After (this lane) | Exit |
| --- | --- | --- | --- |
| `selection-baseline` narrow merged / misses | NOT_RUN (see below); recorded floor 33, misses 0 | 36/43 (rate 0.837), misses 0, `OK floor narrow>=33 misses<=0` | 0 |
| `selection-baseline` other summary | - | median cost 0.752, whole 6966, sliced 2, test-misses 0; FALLBACK rows: `pr-39`, `ci-af9d23f80`; `MOVED pr-14 ci FULL -> NARROW` against the recorded verdict | 0 |
| pr-39 replay | `plan=UNKNOWN verdict=FALLBACK ci=FULL`, 589 dirty, 546 selected, 133 unknown | identical: `FALLBACK`, 589 / 546 / 133, same unknown kinds | 0 / 0 |
| Shell tests in this repository with a name-listing read or a name-only walk | - | 0 of 250 shell units (0 tests even use an admitted form on a repository directory; their listings name temporary directories) | - |

Commands:

- After: `script/heavy.sh make selection-baseline-check
  SELECTION_BASELINE_OUT=/tmp/affstatus-baseline-after.json
  BASE=4c3c203336a6cc4a9772af39b1a771f34236138e` (run=1187.0 s, rc=0). It passed the tool's
  600 s foreground limit and was moved to the background by the agent harness; it was not
  started twice. Its planner was built from the worktree whose Go sources the commit
  `Narrow name-listing shell readers by base change status` holds (results `planner`:
  `commit 9086507cb..., modified`).
- Before, full corpus: NOT_RUN. A second 20-minute replay with the base binaries cannot finish
  inside the 600 s foreground limit, and the charter forbids background jobs. The recorded floor
  (`benchmarks/selection-baseline/changes.json`) is 33 narrowed, 0 misses; the last recorded
  replay (`docs/build-log/2026-11-09-selbase-guard.md`, an older base) was 35/43.
- pr-39: a change set holding only `pr-39` (`/tmp/affstatus-pr39/changes.json`, filtered from
  the corpus), binaries built `-trimpath -buildvcs=false` from `git archive 4c3c20333` (before)
  and from this tree (after), then `script/heavy.sh /tmp/affstatus-pr39/after/selection-baseline
  --root . --changes /tmp/affstatus-pr39/changes.json --corvint SIDE/corvint --selector
  SIDE/gate-affected-select --out SIDE.json` (102.3 s and 83.4 s, rc=0 each; the tool's
  one-change floor line `FAIL narrowed merged changes 0 below the floor 33` is expected for a
  one-change set and `--check` was not passed).

Reading: the change narrows nothing in this repository's own plans, so the baseline cannot show
a precision gain here, and it shows no recall loss (misses 0, test-misses 0). The pr-39
`FALLBACK` comes from an `UNKNOWN` plan of a 589-path change with 133 unknown entries, none of
them a name-listing read; it is unchanged by this lane, so the proposal's "no FALLBACK row"
criterion is NOT_MET.

## Gate redesign (2026-11-11)

Lane affgate, on top of the reviewed affstatus head `5d828908593ee8be070ef710ea56c758fb6c3a5e`.
Two independent reviews found the unit gate fail-open, the second after the first repair. The
cause was its shape: it admitted every "output builtin" (`set`, `shift`, `local`, `readonly`,
`unset`, `read`, `for`) and then denied each way of writing an environment-style name. The gate
is now a closed grammar, stated as what it admits (AFP-V0-9961 text; the table is in
`docs/specs/openspec/changes/affected-change-status/design.md`, "Gate redesign"). The design was
committed before the code. `assignsEnvironment`, `environmentName`, `environmentDefault`,
`outputBuiltins` and `plainCommand` are gone; `nameListingAdmitted`, `gateWord`,
`scanExpansions`, `gateCommand`, `testArguments`, `conditionalArguments`, `plainOperands` and
`literalOperand` in `internal/liveverify/affected/shell/namelisting.go` are the one mechanism.
The lexer now counts the two constructs it folds away without a command, `case` statements and
`((` arithmetic commands (`script.cases`, `script.arithmetic` in `lex.go`); nothing else in the
lexer, the selection side or the planner changed.

Decisions:

- Nothing is kept beyond the starting list (`echo`, `printf`, `true`, `false`, `:`, `exit`, the
  test builtins, the control words), and `case`/`esac` and `return` are dropped from it: the
  lexer discards a `case` header and its patterns, which bash expands, and with no function a
  `return` has no use. No unit in this repository needs more (count below).
- `printf`'s first argument must be fixed and not begin with `-`: bash's `printf` reads options
  only before its first operand, so this covers `-v`, `-vNAME`, `"-v"` and a substituted
  `"$(echo -vNAME)"`.
- `test` and `[` place an expanded argument only where it is an operand (`[ X ]`, `[ ! X ]`,
  `[ -z X ]`, `[ -n X ]`, `[ X OP Y ]`); `[[` refuses its integer operators, whose operands it
  evaluates arithmetically (`[[ x++ -eq 0 ]]` assigns), and `=~`, which sets `BASH_REMATCH`.
- Round-2 item 4, decided as refusal: a reader operand or redirection target ending in `/` or
  `/.`, or naming a directory a listing of the test names, is refused. Another directory operand
  stays admitted on the stated reliance that every admitted reader fails on a directory.
- A backtick is refused outright rather than lexed: the lexer unescapes only `` \` `` inside it.
- The base row `plain_rest` held `set -eu` and a `while read` loop; it moved to the refused table
  as `base_plain_rest` with a note, and an admitted `plain_rest` keeps the rest of it.

The position generator found one leak while the code was written: `[[ $x =~ a ]]` was admitted
because `=~` holds `~`, which made the word read as computed and skipped the operator check.
`=~` is now judged first.

### Fail-open paths and the refusing code

| Path | Row(s) | Refused at |
| --- | --- | --- |
| Round 1: `diff`, checksum tools | `diff_dir`, `diff_dot`, `diff_file_dir`, `diff_files`, `sha_check`, `shasum_check`, `md5_check`, `sha_file`, `grammar_diff`, `grammar_checksum` | `namelisting.go:399` (unrecognised command) |
| Round 1: `export`, upper-case assignment | `clicolor`, `clicolor_bare`, `clicolor_cmd`, `grep_options`, `export_name`, `path`, `grammar_export`, `grammar_assign_upper` | `namelisting.go:243` (assignment word), `:399` |
| Round 1: `read NAME`, `for NAME in`, `printf -v NAME` | `read_env`, `for_env`, `printf_env` | `:399` (`read`, `for`), `:390` (`printf`) |
| Round 1: `${NAME:=x}` | `default_env`, `grammar_default_assign_colon` | `:288` (readExpansion) |
| 1: `printf -vPATH` | `r2_printf_vPATH`, `grammar_printf_v_joined` | `:390` |
| 2: computed name | `r2_read_computed`, `r2_read_computed_quoted`, `r2_printf_computed` | `:243` (`v=PATH`), `:399` (`read`), `:390` (`printf -v`) |
| 3: allexport | `r2_allexport_arith`, `r2_allexport_o_arith`, `r2_allexport_read`, `r2_allexport_printf` | `:399` (`set`), `:247` (`$((`), `:205` (here-document), `:243`, `:390` |
| 4: directory operand | `r2_dir_cat`, `r2_dir_wc`, `r2_dir_grep`, `r2_dir_cmp`, `r2_dir_bare`, `grammar_dir_slash_unlisted`, `grammar_dir_dot_unlisted` | `:553` (listed directory), `:550` (trailing `/` or `/.`) |

### Mutation evidence

Driver `/tmp/affgate-mut/run.py` (not committed): each mutant is one exact string replacement in
`namelisting.go`, run once through `script/heavy.sh env GOTOOLCHAIN=local GOMAXPROCS=2 go test
-count=1 -run 'NameListing|NameOnly' ./internal/liveverify/affected/shell/`, and the source is
restored in a `finally`. Final runs at the committed code.

| Mutant | Change | Result (killing rows, abridged) |
| --- | --- | --- |
| M1 | re-admit `set` | killed: `grammar_set_a`, `grammar_set_allexport`, `grammar_set_e`, `grammar_set_args` |
| M2 | re-admit `read` | killed: `grammar_read`, `read_env` |
| M3 | re-admit `for` | killed: `grammar_for`, `for_env` |
| M4 | drop the `printf` `-` prefix check | killed: `grammar_printf_v`, `_v_joined`, `_v_quoted`, `printf_env`, `walk_printf_v` |
| M5 | drop the `$((` check | killed: `grammar_arith_expansion`, `_quoted` |
| M6 | drop the assignment-word check | killed: `grammar_assign`, `_upper`, `_append`, `_index`, `_prefix`, `_after_if` |
| M7 | re-admit `cd` | killed: `grammar_cd`, `cd`, `find_cd`, `grammar_substitution_refused` |
| M8 | drop the `{name}` check | killed: `grammar_fd_name` |
| M9 | drop the backtick check | killed: `grammar_backtick` |
| M10 | drop the process-substitution check | killed: `grammar_process_substitution` |
| M11 | admit function definitions | killed: `grammar_function`, `grammar_function_keyword` |
| M12 | admit here-documents | killed: `grammar_heredoc` |
| M13 | admit `case` | killed: `grammar_case`, `grammar_case_arith` |
| M14 | admit `((` arithmetic commands | killed: `grammar_arith_command` |
| M15 | admit every braced expansion | killed: `grammar_default_assign`, `_colon`, `grammar_indirect`, `default_env` |
| M16 | admit a computed `printf` format | killed: `grammar_printf_computed_format`, `_substituted_format` |
| M17 | drop the `test` argument-count shapes | killed: `grammar_test_computed_operator`, `_computed_file`, `_four_computed` |
| M18 | drop the `test` single-field check | killed: `grammar_test_unquoted` |
| M19 | admit `[[` integer operators | killed: `grammar_cond_arith` |
| M20 | admit `[[ =~` | killed: `grammar_cond_regex` |
| M21 | admit a `[[` computed file operand | killed: `grammar_cond_computed_file` |
| M22 | admit a trailing-slash reader operand | killed: `grammar_dir_slash_unlisted`, `grammar_dir_dot_unlisted` |
| M23 | admit a listed-directory reader operand | killed: `grammar_dir_listed`, `grammar_dir_input`, `r2_dir_bare` |
| M24 | admit a glob reader pattern | killed: `grammar_reader_pattern_glob` |
| M25 | admit an expansion in reader arguments | killed: `grammar_reader_pattern_computed` |
| M26 | skip redirection-target checks | killed: `grammar_reader_input_computed`, `_write_computed`, `_herestring_computed`, `_dir_input` |
| M27 | judge reader options by their quoted form | killed: `grammar_reader_quoted_recursive` |
| M28 | drop the fixed `test` operator list | killed: `grammar_test_size`, `_size_quoted`, `_newer`, `_variable`, `size_test` |
| M29 | admit a computed or glob filter pattern | killed: `filter_glob`, `filter_computed` |
| M31 | a dangling redirection operator does not refuse | survived: equivalent, bash rejects `cmd >` as a syntax error, so no such test runs |
| M32 | admit every unrecognised command | killed: 107 distinct row names across three tests |
| M33 | admit every `$` form | killed: `grammar_ansi_c`, `_arith_bracket`, `_indirect`, `_subscript` |

M30 (`fi`/`done` with arguments admitted) survived as equivalent, since bash rejects such a
line; the check was removed rather than kept as dead strictness. Rows no single mutant kills,
because two independent rules refuse each: `grammar_alias` (an assignment-shaped `ls=cat` and an
unrecognised command), `grammar_for_arith` (`for` and `((`), `grammar_heredoc_read` and
`grammar_herestring_read` (`read` and a here-document), `grammar_process_substitution_write`
(the substitution and its target), `grammar_reader_computed` (an expansion and a non-literal
operand), `grammar_test_glob` (an operator expansions may not follow, and a glob),
`grammar_cond_size` (`-s` is outside the `[[` list too), the `grammar_dir_*` rows naming the
listed `script` (trailing `/` and the listed name) and every `r2_` row except `r2_printf_vPATH`
and `r2_dir_bare` (each holds two or more refused constructs). The base's refused listing-form
rows (`ls_l`, `find_exec`, `xargs` and the rest) test `nameListingForm`, which this lane did
not change.

### Qualifying units

Method: a throwaway test in the shell package (not committed) ran `New().Units` on this
checkout and, for every unit, counted `NameListings`, `UnboundedNamesOnly`, an admitted listing
pipeline (`nameListingCommand`) and the gate verdict; the same probe ran against a detached
worktree of the base commit over the same files.

| | Base gate | Closed grammar |
| --- | --- | --- |
| Shell units | 250 | 250 |
| Units with a name-listing read or a name-only walk | 0 | 0 |
| Files with an admitted listing pipeline | 1 (`.github/forward-rows/mq-narrow.sh`, gate-refused) | 1 (same, gate-refused) |
| Files the gate admits (with or without a listing) | 16 | 12 |

No unit changed side. The four files the grammar now refuses hold no listing:
`internal/liveverify/affected/testdata/fixture/shell/lib/core.sh` and `solo.sh` (a function
definition), `tools/cw-trial/testdata/fake-agent.sh` (a here-document) and
`tools/cw-trial/testdata/abstain-agent.sh` (a backtick inside a single-quoted `printf` format, a
precision loss the stricter rule accepts).

### Tests and results

All through `script/heavy.sh env GOTOOLCHAIN=local GOMAXPROCS=2`, one package per command, rc 0:
`go test -count=1 ./internal/liveverify/affected/shell/`, `go test -count=1 -timeout 30m
./internal/liveverify/affected/`, `go test -count=1 -run '^TestAffectedChangeStatusCLI_AFPV09960$'
./cmd/corvint/`, and `go vet` on the two affected packages; `gofmt -l internal/liveverify/affected/`
prints nothing. `TestNameListingForms_AFPV09961` now holds 23 admitted rows (the base's 14, the
rewritten `plain_rest` and 8 new) and 175 refused rows (the base's 55, `base_plain_rest`,
`filter_glob`, `filter_computed`, 104 `grammar_` rows and 13 `r2_` rows);
`TestNameListingGrammarPositions_AFPV09961` composes the 104 constructs in 8 positions and a
command substitution, with an `echo ok` control per position; `TestNameOnlyTreeWalker_AFPV09963`
gains `walk_printf_v`, `walk_read` and `walk_allexport`. `make shell-foreign-callers-check` failed at the
base already: the AZU-V0-8901 completeness matcher reads the fixture spelling `\"script/$f\"`
(a quote right before `script/`, then `$`) as a constructed path under `script/`. Those four base
rows (`loop`, `while`, `captured`, `computed_read`) and the new `test_computed_file` row are
respelled `script/\"$f\"`, the same shell word, so each row tests what it tested and the check
passes; no assertion changed. The other doc gates pass; `make doc-gates` as a whole was not run
because it includes `selection-baseline-check`. The selection baseline and the pr-39
replay were not rerun in this lane (see the handback); the gate only refuses more, and the count
above shows no unit of this repository on either side of it.

## Rollback

Revert the lane commits. Without `NameListings` and `UnboundedNamesOnly` every listing is a
content read again, which selects more, never less; the profile change makes an older binary
rebuild its cache.
