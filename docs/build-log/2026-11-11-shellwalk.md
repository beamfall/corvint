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

The ticket title names four gaps; the fourth is not readable from the title and is NOT_DONE.

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

Differential check (a throwaway test, not committed): every one of the 250 repository shell units
was observed with the base and the head plugin. No unit loses an import, path token or glob; 28
gain tokens (mostly AFP-V0-9977: tests that assign `PATH` keep their `mkdir`, `cp` and `mv`
operands), 16 gain `shell:tree-walk`, and imports, computed names, frontiers and test roles are
unchanged.

A first version of `holds_test.go` carried an error string spelled `"script/env.sh: ..."`, which
the foreign-caller scan reads as a constructed `script/` path and so invalidated
`.corvint/shell-foreign-callers.json` for the whole repository; the message was reworded.

`TestHeavyShEditKeepsShellReaders_V14170` asserted that a docs edit does not select
`script/analyzer-schema_test.sh`; it now admits that selection only through the test's own
`shell:tree-walk` read and checks both units' import chains to `script/heavy.sh` directly.

Rollback: revert the lane commits and drop the 16 units from `.corvint/unbounded-readers.json`
and the two `docs/build-log/` record-reader entries.
