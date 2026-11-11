<!-- SPDX-License-Identifier: Apache-2.0 -->
# Lane shellwalk: shell walker fail-closed gaps (2026-11-11)

Branch `claude/shellwalk`, based on `b55635262`. Scope: the shell plugin
(`internal/liveverify/affected/shell`) and the V1-5203 unbounded-reader audit. New requirements
AFP-V0-9970 to AFP-V0-9977 live in `docs/specs/affected-plan-selection-v0.md` (184 obligations
before, 192 after), which already holds the shell walker requirements AFP-V0-9129, AFP-V0-9133
and AFP-V0-9134. Every rule only adds a hold, an edge or a token.

The lane host could not read the ticket bodies: none of V1-5203, V1-5217, V1-5226 or V1-5382 is
in the committed `.taskman` projection or in the only staged queue snapshot (2026-11-04). The
work follows the charter's ticket titles; the gaps named below are the ones the lane observed.

## V1-5382: carriage returns and empty here-document delimiters (AFP-V0-9970, AFP-V0-9971)

Confirmed on origin/main. In a scratch repository whose test is
`printf 'echo a\r#;cat script/golden.txt\n'`, an edit of `script/golden.txt` selected nothing
(`scope=UNKNOWN selected=0`, the edit an `UNOWNED_DIRTY_PATH`), while bash 5 and dash both print
`a\r#` and then read the file. The lexer counted `\r` as a blank, so `#` started a comment that
dropped `cat script/golden.txt`. A bare `golden` (no `/` or `.`) is no path token on either side,
so the probe needs a path-shaped name.

Empty delimiters: `cat <<''` (and `<<""`, `<<-''`) opened no here-document, so its body was lexed
as commands. A plain read in the body was still token-scanned, but a body line such as
`tmp=$(mktemp -d)` became a temp-root assignment, which dropped the token of a later
`cat "$tmp/script/golden.txt"`; both shells treat the line as data, so `$tmp` comes from the
environment. The `EOF\r` delimiter trim misparsed the same way in the other direction.

Fix: `\r` is word text; delimiter lines match byte for byte; a quoted empty delimiter opens a body
ended by the first empty line. `trapAllowed` no longer splits at `\r` (`exit\r` was an INT exit
handler) and the discarded-redirection pattern no longer takes `\r`. Tests:
`TestCarriageReturnIsWordText_AFPV09970`, `TestEmptyHereDocumentDelimiter_AFPV09971`, and
`TestCarriageReturnAndEmptyDelimiterAgreeWithShells_AFPV09970`, which runs each fixture under
bash 5.2 and dash 0.5 on the host and checks both read the file.

## V1-5217: tree-walk classifier gaps (AFP-V0-9972 to AFP-V0-9974)

- Option values: `walkerPaths` read every non-option word as an operand, so `grep -rm 1 FIXME`
  took `FIXME` as its path and `du -d 1` took `1`, and neither was a `shell:tree-walk`. Both
  readings of option values are now tried (AFP-V0-9972).
- git path output: `git ls-files`, `status`, `diff`, `log`/`reflog`/`show` with path output and
  the like listed the tree with no token. They are tree walks unless the repository is a proven
  private temporary root (AFP-V0-9973).
- awk options: `awk -f prog` and `sed -f script` read programs whose `getline`, redirections or
  `r` commands no word names; such a test is now `shell:runs-toolchain` (AFP-V0-9974).

Forms delivered and their tests: awk options, `TestAwkProgramFiles_AFPV09974`; git log and
reflog path output, the `log_names` and `reflog` cases of `TestGitPathOutputWalks_AFPV09973`;
walker option values, `TestWalkerOptionValues_AFPV09972`. NOT_DONE: `node --import` (a node
preload; a shell test running node is already `shell:runs-toolchain`, but the form itself has no
rule or test) and any other form the ticket body specifies, which the lane could not read.

## V1-5226: wrappers, startup names and write detection (AFP-V0-9975 to AFP-V0-9977)

- `commandWord` skips a prefix's options but not their values, so `sudo -u u bash x.sh` read `u`
  as the command and `env -u X find .` read `X`; `nice`, `stdbuf`, `flock`, `xargs` and other
  runners were not prefixes at all. The first known command after a runner is now classified in
  addition to the base command word (AFP-V0-9975). A first version replaced the command word
  instead and dropped `script/affected-zero-unknown-replay.sh`'s edge to `script/heavy.sh`
  (`exec script/heavy.sh ... go ...`); the differential check below caught it.
- No interpreter startup name was modelled: `BASH_ENV=hook.sh bash t.sh`,
  `env BASH_ENV=... bash`, `export PERL5LIB=lib` or `sudo LD_PRELOAD=./x.so` loads a file no word
  links. Such a file is now a `shell:dynamic-source` (AFP-V0-9976); an empty or absolute literal
  value is not counted, as a `source` of an absolute literal links nothing.
- NSA-V0-016 dropped the written operands of `cp`, `mv` and `mkdir` by name even when the file
  defined `mv() { cat "$2"; }`, aliased `cp`, hashed it or assigned `PATH` (AFP-V0-9977).

## V1-5203: the 15 units behind `anyDirtyPath` stay unbounded

On a docs-only path (`docs/build-log/README.md`) at `b55635262`, 34 selections carry an
`UNBOUNDED_READER` witness with `anyDirtyPath`, from 15 distinct cause units (the ticket said 14).
Each was read for a provable bound; none was narrowed:

| Cause unit | Read | Verdict |
|---|---|---|
| `.github/testconfine/cmd` | `os.Getwd`, reads `$CORVINT_CONFINE_ROOT` scopes | own tests already bound (`[]` read scope); the production locator stays for its runner |
| `internal/perfbudget` | `"../models/"` is Ruby text written to a fixture | own tests declared; the token is a classifier fact, pinned by a liverepo test (AFP-V0-2952) |
| `cmd/corvint` | `os.Getwd` and `--show-toplevel` in several production files | keep: whole-repository CLI reads |
| `internal/jstestprovider` | `git rev-parse --show-toplevel`, then `log`, `rev-list`, `status` | keep for importers; own tests declared |
| `tools/cem-stable-regen` | `"../manifest.json"` is Markdown text; reads `protocol/cem-1.0` | own tests already bound to `protocol/cem-1.0/` |
| `internal/liveverify/parentverify` | `ls-files`, `status` over a configured root | own tests bound to `[]`; locator stays for importers |
| `tools/test-commands-run` | runs `go test ./...` from the working directory | keep by design (liverepo pin) |
| `cmd/corvint-docs` | runs `corvint docs` on committed HEAD, builds `../corvint` | keep |
| `internal/analyzerhtmlcss` | reads a pinned commit from `.git` | keep: no grant for `.git` under Landlock |
| `internal/analyzerpython` | `git status`/`show` on sibling checkouts | keep: confined run failed (recorded reason) |
| `internal/contextindex` | another test walks every Go file and `.git` | keep (not this lane's files) |
| `internal/doccorpus/selfcorpus` | builds the corpus at HEAD from `.git` | keep |
| `tools/release-binary` | `go build` of the catalog's commands from the root | keep: bound is data-driven |
| `tools/test-exec-writes-check` | `git ls-files -- *_test.go` over the root | keep: reads the index, which AFP-V0-9000 does not cover and Landlock denies |
| `tools/test-read-scopes-check` | `go list -deps` on fixture modules (GOROOT, module cache) | keep: the recorded toolchain-read reason needs a confined CI run to overturn |

Six of the 15 causes already have their own tests bounded by `.corvint/test-read-scopes.json`;
what remains selected is importers inheriting `LocatesRoot`, which only
`.corvint/gated-root-locators.json` clears (an absolute literal gate path none of them has), and
genuine whole-tree or `.git` readers. No bound here is provable without a confined run, so the
set stays fail-closed (AFP-V0-108, AFP-V0-109).

## Census and precision cost

AFP-V0-9973 makes 16 repository shell tests `shell:tree-walk` readers selected on every change:
14 run git in a fixture repository the plugin cannot prove private (the root is populated by
`mv`, `ln`, `git clone` or `git worktree`, reassigned, or reached by a `cd` inside a function or
subshell), one (`script/batch-assemble_test.sh`) reassigns its root, and
`script/register-shell-test.sh` reads the real repository's `git diff --name-only`. Each is in
`.corvint/unbounded-readers.json` with its reason; `script/check-decision-numbers_test.sh` and
`script/repin-use-case-receipts_test.sh` are also in `.corvint/record-readers.json` under
`docs/build-log/` (AFP-V0-2745). No other rule adds an always-selected unit in this repository.
`run-case.sh` under `conformance/perf-v0/results/` was a dynamic source through its
`export PYTHONPATH=/private/tmp/...` until the absolute-value exemption of AFP-V0-9976, which
`TestRepositoryInboundCallers_AZU_V0_8901` required; it stays bounded.

Differential check: `TestShellUnitDifferential_AFPV09970` (committed, skipped unless its
environment names the inputs) observes every repository shell file with the base plugin and the
head plugin on the same input bytes and fails on any lost import, glob or path token. Procedure
from the head checkout, with `D` a private scratch directory:

```sh
git ls-files '*.sh' '*.bash' '*.bats' '.github/*.sh' > "$D/files"
git archive b55635262788687c3eea9a08a94378e4ab8856a4 | tar -x -C "$D/base"
cp internal/liveverify/affected/shell/differential_test.go "$D/base/internal/liveverify/affected/shell/"
(cd "$D/base" && script/heavy.sh env GOTOOLCHAIN=local CORVINT_SHELL_DIFF_ROOT="$HEAD" \
  CORVINT_SHELL_DIFF_FILES="$D/files" CORVINT_SHELL_DIFF_OUT="$D/base.json" \
  go test -count=1 -run TestShellUnitDifferential ./internal/liveverify/affected/shell/)
script/heavy.sh env GOTOOLCHAIN=local CORVINT_SHELL_DIFF_ROOT="$PWD" CORVINT_SHELL_DIFF_FILES="$D/files" \
  CORVINT_SHELL_DIFF_OUT="$D/head.json" CORVINT_SHELL_DIFF_BASE="$D/base.json" \
  go test -count=1 -run TestShellUnitDifferential -v ./internal/liveverify/affected/shell/
```

Retained output at the fix-round head (PASS):

```text
differential: 250 units, 28 gain tokens, 0 gain imports, 16 change unbounded reads, 0 change frontier, names, tests or bound
```

The 16 changed units are the `shell:tree-walk` tests listed above. Most token gains come from
AFP-V0-9977: tests that assign `PATH` keep their `mkdir`, `cp` and `mv` operands. An earlier cut
rule of the fix round (end a body at any line with an unbalanced `)`) made
`script/wsl-keepalive-check.sh` a 17th change, a false hold on a correctly closed body, which this
check caught; the rule now follows bash (below).

A first version of `holds_test.go` carried an error string spelled `"script/env.sh: ..."`, which
the foreign-caller scan reads as a constructed `script/` path and so invalidated
`.corvint/shell-foreign-callers.json` for the whole repository; the message was reworded.

`TestHeavyShEditKeepsShellReaders_V14170` asserted that a docs edit does not select
`script/analyzer-schema_test.sh`; it now admits that selection only through the test's own
`shell:tree-walk` read and checks both units' import chains to `script/heavy.sh` directly.

## Fix round 1 (independent review: SHIP-WITH-FIXES)

- P1: AFP-V0-9971's empty delimiter swallowed the rest of a file whose here-document sat in a
  command substitution: `echo $(cat <<''\n)\n. "$X"` and `x=$(cat <<''\nbody\n)\n. "$X"` lost
  the base `shell:dynamic-source` hold. bash 5.3 ends such a body at a line that starts with the
  delimiter and whose rest closes the substitution (`)` for an empty delimiter, `EOF)` or
  `EOF )` otherwise) and runs what follows; a body line such as `body ) x` stays data; dash
  refuses all three forms. The lexer now ends the body there and lexes the rest, and every
  here-document no delimiter line ends (cut by the substitution, at end of file, an operator on
  the last line or on its substitution's closing line) holds the
  file as a dynamic source, in command strings and trap handlers too. Tests:
  `TestIncompleteHereDocumentsHold_AFPV09971`, `TestHereDocumentFixturesAgreeWithBash_AFPV09971`.
- P3: `<<$'EOF'` and `<<$''` end only at a literal `$EOF` (or `$`) line, while bash decodes
  them. Round 1 decoded `$'...'` and `$"..."`; fix round 2 reverted that decode (see below), so
  this is NOT_DONE.
- V1-5226 AC1: `TestWrappedStartupAssignments_AFPV09976` covers `timeout`, `nice`, `nohup`,
  `time`, `exec` and `command` before a `BASH_ENV` assignment, directly and through `env` (it
  passes at the first-round head, whose any-word assignment rule already held these; origin/main
  has no startup-name rule). AC2: `PHPRC`, `PHP_INI_SCAN_DIR`, `TCLLIBPATH` and `LUA_PATH`,
  `LUA_CPATH`, `LUA_INIT` in every versioned form are startup names
  (`TestMoreInterpreterStartupNames_AFPV09976`).
- Each new hold test failed at the first-round head `35af15b47` except the two noted above.

## Fix round 2 (re-review: one new P1 from the round-1 decode)

- P1: the round-1 ANSI-C/locale decode removed a `$` before any quote in the raw delimiter, so
  `<<'$''EOF'`, `<<"$"'EOF'` and `<<\$'EOF'`, whose delimiter is the literal `$EOF` in bash 5.3
  and dash, ended at a later `EOF` line instead and swallowed `. "$X"`, losing the hold. The decode
  is reverted; the delimiter is the base `unquote(raw)` again. Regression test:
  `TestQuotedDollarDelimiter_AFPV09971` (four-line files; each case failed at `a9a033a6b`). The
  round-1 decode-only assertions (a decoded `<<$'EOF'` body and a closed `<<$'EOF'` control) are
  removed; `TestAnsiCQuotedDelimiterWidens_AFPV09971` keeps the hold rows, now held by the
  end-of-file rule.
- NOT_DONE, `<<$'EOF'` decoding (pre-existing on base): the delimiter stays `$EOF`. With no later
  `$EOF` line the body reaches the end of the text and holds the file; a file that also holds a
  `$EOF` line reads the commands between the `EOF` and `$EOF` lines as data, as on the base.
- NOT_DONE, pre-existing P2s on which base and head agree: `unquote` deletes every quote
  character (`shell.go`, `unquote`), and `((x<<2))` opens a here-document (`lex.go`, the `<<`
  case of `parse`).
- The bash oracle `TestHereDocumentFixturesAgreeWithBash_AFPV09971` runs in the heavy slot
  (`command -v bash` there is `/usr/bin/bash`, bash 5.3.9) and logs the binary; without bash it
  skips with a reason that `-v` prints.
- The differential rerun at the fix-round-2 tree: the same summary line as above, PASS.

Rollback: revert the lane commits and drop the 16 units from `.corvint/unbounded-readers.json`
and the two `docs/build-log/` record-reader entries.
