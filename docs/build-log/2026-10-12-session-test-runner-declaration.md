# V1-0040 redesign: the test-commands declaration replaces the Makefile reader

Date: 2026-10-12. Branch `claude/v1-0040-impl`, started at
`bed653e92` (slice 1 after fourteen Codex rounds), merged with `claude/v1-0040-slice2`
(`7d3d773b2`, merge `1bf048759`, no conflicts). Authority: decision 0526 (two owner decisions in
chat, 2026-10-12). Scope: `docs/specs/session-test-runner-v0.md` STR-V0-003, 004, 006, 013 and the
STR-V0-015 wording, IDs stable. Delivery stays experimental; no call site switched.

## Why

Slice 1 typed the local Go full-suite command by interpreting the Makefile. The reader in
`internal/sessiontest/declared.go` reached 1451 lines (conditionals, `include`, prerequisites,
target-specific variables, `.SHELLFLAGS`, byte-level recipe parsing), and fourteen Codex review
rounds on slice 1 went to GNU-make semantics the reader modelled or refused rather than to the
runner itself: round 13 closed "read-time fatals and shell start-up paths", round 14 "prerequisite,
target, ifdef, byte and SCCS paths". The lane's independent review recommended replacing the
interpreter with a declaration the project owns plus a gate that keeps the declaration honest.
The owner decided so in chat on 2026-10-12.

## What changed

- `.github/cishards/test-commands.json`, profile `corvint-test-commands/0`:
  `go.env` `{GOCACHE: /tmp/corvint-go-build-cache, GOTOOLCHAIN: local}`, `go.flags`
  `["-p", "1", "-timeout", "30m", "-count=1"]`, `typescript` declared as
  `["make", "host-adapter-test"]` and six languages declared `{"none": reason}`: `dotnet`,
  `kotlin`, `rust`, `swift` (the only sources are testdata fixtures under `tools/beamfall-shadow`,
  `internal/analyzerkotlinandroid`, `internal/liveverify/affected` and `internal/analyzerswift`;
  no project file), `python` and `ruby` (benchmark and qualification scripts, runner reporters and
  affected-graph fixtures; no pytest, unittest, Gemfile or Rakefile). After the round-4 finding,
  `none` is gate-scoped: each reason states that no `make gate` step, `script/gate-affected.sh`,
  `tools/corvint-pr-tests` or CI job runs a suite for the language. Evidence: the dry run of all
  38 `GATE_STEPS` recipes (`make -s -n -f Makefile <steps>`, 43 lines) names a `none`-language
  runner nowhere, the only non-Go runner word in it is `host-adapter-test`'s `node --test` over
  ten `integrations/pi/*.test.mjs` files, and a search of `.github/workflows`,
  `script/gate-affected.sh` and `tools/corvint-pr-tests` for `npm`, `node --test`, `pytest`,
  `python -m unittest`, `ruby`, `cargo`, `swift test`, `gradle` and `dotnet` found no suite
  invocation (CI runs `node --version` and `npm install` of Codex only). `integrations/pi`'s
  `.mjs` files map to the affected graph's `typescript` language
  (`internal/liveverify/affected/typescript`), so `typescript` declares exactly the gate step that
  runs them; `host-adapter-test` also runs a `cmd/corvint` Go subset (`-run
  '^(TestDogfoodEvent|TestCodexHostAdapter|...)'`), which is redundant with the Go adapter but is
  what the gate step is. Suites outside the gate (`extensions/vscode` npm test,
  `integrations/pi-protected` and `tools/web-flows` `node --test`, `make pi-protected-test`) are
  recorded limits this runner does not run.
- `internal/sessiontest/declared.go` is 178 lines: `ReadDeclared` opens the file, reads at most
  `MaxDocumentBytes`+1, decodes with `encoding/json/v2` and `RejectUnknownMembers` (which also
  refuses duplicate members), requires the profile and a `go` member, validates environment names
  and argv words, and sorts the result; any problem is `Unreadable`. The manifest digests the
  parsed form, so member order and whitespace do not move `declaredDigest`.
- `plan.go`: an unreadable declaration is `INCOMPLETE` in every scope and profile with no units;
  no-plan `FULL` and `FULL` with an empty Go universe run `declared:go`; `declaredLanguage` maps a
  declared argv to `declared:<language>`, `none` to a `noSuite` entry, and an undeclared language
  to `INCOMPLETE`. The CI-profile `INCOMPLETE` that `addDeclaredGo` returned is gone.
- `run.go`: `declared:go` runs `go test <declared flags> ./...` under the declared environment
  locally and `go test -exec <confine> <ciGoFlags> ./...` under the CI constants in the ci
  profile; another declared unit runs its argv with an empty environment; runner discovery is per
  `argv[0]` (`go env GOVERSION` for `go`, `--version` otherwise); a manifest unit the declaration
  no longer names is `INCOMPLETE` ("names no command for it").
- `manifest.go`: new `noSuite` member; `cmd/corvint/test.go` reads the declaration independently
  of the plan and prints `noSuite` in the dry-run document.
- `tools/test-commands-check` behind `make test-commands-check` (added to `GATE_STEPS`, `.PHONY`
  and the CI doc-gates list in `.github/workflows/ci.yml`, the last under AFP-V0-016 consent the
  owner posts). After round 4 it no longer reads recipe text: it copies itself to `<tmp>/bin/go`,
  runs the real `make -s -f Makefile go-test` from the root with the environment exactly
  `PATH=<tmp>/bin:/usr/bin:/bin` and `CORVINT_TEST_COMMANDS_RECORD=<tmp>/records`, and the shim
  (the same program, entered through `shim()` before flag parsing) writes `len(argv)`, each
  argument and each environment entry NUL-terminated to `<records>/<pid>.record` and exits 0. The
  gate fails when make exits nonzero, when no record exists ("never ran go from PATH": an
  absolute-path `go` or a recipe that never reaches `go`), when more than one exists, when the
  argv is not `test <go.flags> ./...`, when a passed name (`PATH`, the record variable) arrives
  changed, or when the remaining environment, sorted, is not `go.env` after dropping only the
  closed set `makeIntroduced`. That set was measured on this host (GNU Make 3.81 behind Apple's
  `/usr/bin/make` xcrun shim, `/bin/sh` = bash 3.2.57): the shim saw `GOCACHE GOTOOLCHAIN
  MAKEFLAGS MAKELEVEL MANPATH MFLAGS PATH PWD SHLVL _ __CF_USER_TEXT_ENCODING`; make itself adds
  `MAKEFLAGS MFLAGS MAKELEVEL` (also under the direct Xcode `make`), the xcrun shim adds `MANPATH`
  and `__CF_USER_TEXT_ENCODING` (the direct Xcode `make` does not), bash adds `PWD SHLVL _` and
  `/bin/dash` only `PWD`; GNU make 4.x documents `MAKE_RESTARTS`, `MAKE_TERMOUT` and
  `MAKE_TERMERR` as the remaining additions. That closed set settles the deferred question of which
  environment names make introduces; an unlisted name is drift. The gate also dry-runs every
  declared `make` language command and, for the `none` languages, reads `GATE_STEPS` from
  `make -p` and dry-runs the steps (together first, per step only to name a violation), failing
  when a recipe word's basename is in that language's runner list (`languageRunners`: `node`,
  `npm`, `bun`, `pytest`, `python3`, `cargo`, `swift`, `dotnet`, `gradle` and the rest) or when a
  `none` language has no runner list; it inspects only the words make prints, not what a script
  those words name runs (`./tools/python-tests` would pass), which is the recorded limit of that
  check. `compareGo` and `plainWord` are gone (the observation is exact, so recipe-text validation
  is redundant); the reader keeps its own validation. After round 5 (`5fc504ffe`) the shim also
  records its working directory, which must resolve to the repository root; every passed name
  must be present as well as unchanged; the make-introduced names are checked rather than
  trusted: a control recipe of the same shape (`GOCACHE=... GOTOOLCHAIN=local go
  __corvint_test_commands_control`, added through a second `-f` makefile so make runs it through
  the same shell; a bare `go` word would take make's direct-exec path and show no `PWD`, `SHLVL`
  or `_`) is observed under the same make, and the make-introduced names and values `go-test`'s
  `go` saw must equal the control's exactly, so `MAKEFLAGS=--just-print go test ...` fails; and
  the recipe's shape is checked because the shim cannot see a `go` that is not it: `make -s -n
  go-test` must print one line with exactly one word `go`, no other word whose basename is `go`
  and none of `;|&()<>{}` `` ` `` `$\"'`, so a second recipe line, a chained absolute-path `go`, a
  substitution, a redirection, a quoted word or a wrapper script is refused. `runnerWords` trims
  quotes, so `"python3"` in a gate step is the python runner. After round 6 (`c07788ba7`) the
  shape rule is `NAME=VALUE` words then `go` (round 5's claim that `nice go test` would pass was
  wrong in both directions: bash's `_` rejected it by accident, and a wrapper under a shell that
  sets no `_` passed); the control makefile is read alone (`make -s -f control.mk <target>`), so
  a Makefile `export MAKEFLAGS := --keep-going` now differs (`sk` against `s`) instead of being
  inherited; and the gate parses `make -s -n -p go-test` (`readDatabase`/`parseDatabase`:
  `NAME = value`, `NAME := value`, `define`/`endef`, the `go-test:` block's `# NAME = value`
  target-specific comments and its tab-indented recipe) so `plainRecipe` can refuse any `$` in
  the recipe or a variable it expands that is not a `$(NAME)`/`${NAME}` reference to a plain
  variable (`$(shell ...)`, `$$`, `$@`, substitution references, a `:=` value still holding `$`)
  and any `SHELL`/`.SHELLFLAGS` whose origin is not `default`. After round 7 (`parseDatabase`
  reads both database forms: 3.81's `#  commands to execute` and 4.x's `#  recipe to execute`,
  one recipe prefix character stripped per line, 4.x's `go-test: NAME := value` lines before the
  target, `+=` and `$(subst ,,)` values, and the `# Pattern-specific Variable Values` section
  kept for patterns matching `go-test` through `matchesTarget`), `plainRecipe` also refuses
  `SHELL`/`.SHELLFLAGS` set for `go-test` itself or for a matching pattern. The limit that remains: a make
  function expanded while the Makefile is read (`CORVINT_GOCACHE := $(shell ...)`) runs on every
  make invocation before the recipe and is not observed; the repository's `$(shell git ...)`
  lookups are of that kind. It is not in `tools/gate-ledger`'s scoped map, so it runs on every
  change. Superseded on 2026-10-12: the shim, the control recipe and the database parser are
  deleted and the gate is a Makefile text check; see "Round 8 and the redesign" below.
- `run.go` `discover` (round 4): a declared `argv[0]` containing a path separator is resolved
  against `filepath.Abs(Root)` before `exec.LookPath`, so `./tools/python-tests` is the
  repository's file when `corvint --root R test` runs from another directory; a bare name is
  still a `PATH` lookup, and `launch` already ran every command with `cmd.Dir = Root`.
- `tools/str-parity`: a `declared` difference whose commands the `-declaration` file names is
  the category `str-v0-004-declared-language` (`languageCommands` reads the argv lists out of the
  declaration bytes; `allDeclared` requires every command corvint also runs to be declared).
- `tools/str-parity`: `-corpus` reads `tools/str-parity/testdata/pairs-645304a2.txt` and refuses a
  corpus that differs from `git rev-list --first-parent --parents` over the same end and count;
  `-declaration` (default: the `-repo` file) is written into every checkout after `git clean`,
  kept out of the dirty set by `.git/info/exclude`, and removed before the cleanliness check;
  the report records the corpus and declaration sha256; `undeclaredOnly` matches the new
  `INCOMPLETE` reason text.

## What was deleted

- The GNU-make reader: `declared.go` went from 1451 to 178 lines (`git show bee5a3886 --stat`:
  115 insertions, 1389 deletions in that file).
- Its tests: `TestSTRV0006ReaderGrammar`, `TestSTRV0006RepositoryMakefileTypesGoTest` and
  `TestSTRV0006RepositoryMakefileDependsOnConditional` (`plan_test.go` lost 841 lines net); the
  `Makefile` entry of `internal/sessiontest` in `.corvint/test-read-scopes.json`.
- `addDeclaredGo` and its ci-profile `INCOMPLETE`; the `usesAdapter`/`withoutAdapter` helpers,
  replaced by `runnerKey`/`withoutRunner`.

Nothing else was unified. The CI constants in `run.go` (`ciGoEnv`; `ciGoFlags`
`-p 1 -count=1 -race -timeout 50m`) differ from the local declaration by design: AFP-V0-028
fixes the CI invocation and the declaration describes the local full suite. Slice 3 decides
whether the CI call site reads a second declared profile.

## Evidence

Logs under the lane scratch directory
(`scratchpad/redesign/`), each with its recorded rc:

- `gofmt -l` on the four touched packages: empty, rc 0. `go vet` on `internal/sessiontest`,
  `internal/gateselect`, `cmd/corvint`, `tools/test-commands-check`, `tools/str-parity`: rc 0.
- `go test -count=1 -timeout 30m ./internal/sessiontest ./internal/gateselect ./tools/str-parity
  ./tools/test-commands-check`: ok, rc 0. `go test ./cmd/corvint -run 'TestSTRV00|Help|UseCase'`:
  ok in 72 s, rc 0.
- `make -s test-commands-check`: rc 0 (`GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go
  test -p 1 -timeout 30m -count=1 ./...`). `TestSTRV0006DeclarationMatchesMakefile` fails each of
  seven mutations: flag value, flag order, dropped flag, extra environment variable, changed
  environment value, declared `make` target that does not exist, absent file.
- Real dry run from the committed tree against `origin/main`
  `ddd559754614e7b7bd4e7527eb7fbb9500bfaf71`: rc 0, `FULL` (`UNINDEXED_SOURCE_PATH
  cmd/corvint-corpus-mcp/legacycases.go`), one `go` invocation with the declared environment and
  flags, `incomplete []`, seven `noSuite` entries, manifest digest
  `sha256:d1649e54332952f4832ac510b7442400c1162dd2fcf1795f1b84844a033aa7e6`.
- Doc gates: `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`,
  `use-case-receipts-test`: rc 0 each after the `.corvint/test-read-scopes.json` entry for
  `tools/test-commands-check` (first `unbounded-readers-check` run: rc 2, the package was
  undeclared).
- STR-V0-013 parity over the committed corpus: see the parity section below.
- After the round-4 fixes (`96bed1f5e`): `gofmt -l` empty on `tools`, `internal/sessiontest`,
  `cmd/corvint`; `go vet` rc 0 on `internal/sessiontest`, `cmd/corvint`, `tools/test-commands-check`,
  `tools/str-parity`; `go test -count=1` rc 0 for `internal/sessiontest`, `tools/str-parity`,
  `tools/test-commands-check` (5.2 s: the repository pass, eight declaration mutations, seven
  Makefile mutations, a copied unmodified Makefile passing, `compareEnv` and record cases),
  `internal/gateselect`, and `./cmd/corvint -run 'TestSTRV00|Help|UseCase'` (45 s).
  `TestSTRV0003DeclaredRelativeCommandResolvesAgainstRoot` fails on the pre-fix `run.go`
  (`receipt INCOMPLETE`, the runner looked up `./tools/fake-tests` in the caller's directory) and
  passes after it. `make -s test-commands-check` rc 0 and the direct `go run` rc 0 printing
  `equals make go-test (GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1
  -timeout 30m -count=1 ./...); 1 declared make targets dry-run; no make gate step recipe runs a
  suite of [dotnet kotlin python ruby rust swift]`. Real dry run from the committed tree against
  the same `origin/main`: rc 0, `FULL`, the `go` invocation with the declared environment and
  flags plus the `declared` invocation `make host-adapter-test`, `incomplete []`, six `noSuite`
  entries, manifest digest
  `sha256:2df05a9f715bbf6d9ac3d47907f2bdfa9fad616b8bf51bfd7d89c87a2134a2f1`. `make -s
  spec-requirements` rc 0; `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check` rc 0; `use-case-receipts-check`,
  `use-case-receipts-test` rc 0.
- After the round-5 fixes (`5fc504ffe`, gate only; `corvint test` is unchanged, so the dry run
  and its manifest digest above stand): `gofmt -l` empty; `go vet` rc 0 on
  `tools/test-commands-check`; `go test -count=1 ./tools/test-commands-check` rc 0 (8.2 s:
  thirteen Makefile mutations, nine declaration cases, `compareEnv`, `compareIntroduced` and
  record cases); `make -s test-commands-check` rc 0. The three round-5 Makefile scenarios (`cd
  internal/sessiontest; unset OLDPWD; ...`, `MAKEFLAGS=--just-print ...`, a second recipe line
  `/usr/bin/true test -race ./...`) were run against a build of the `96bed1f5e` gate and of
  `5fc504ffe` on the same mutated Makefile copies: rc 0, 0, 0 before and 1, 1, 1 after.
- After the round-6 fixes (`c07788ba7`, gate only; `corvint test` unchanged): `gofmt -l` empty;
  `go vet ./tools/...` rc 0; `go test -count=1 ./tools/test-commands-check` rc 0 (11.2 s: twenty
  Makefile mutations, nine declaration cases, `TestSTRV0006PlainRecipe`, `compareEnv`,
  `compareIntroduced` and record cases); `make -s test-commands-check` rc 0. Eight Makefile
  scenarios were run against builds of the `5fc504ffe` and `c07788ba7` gates: a `/bin/sh
  wrapper go test ...` recipe with `SHELL := /bin/sh`, `$(shell true)$(GO_TEST_COMMAND)`, `export
  MAKEFLAGS := --keep-going`, `nice go test`, `go-test: GO_TEST_FLAGS = $(shell true)...`,
  `GO_TEST_FLAGS = $(shell true)...`, `SHELL := /bin/bash` and `CORVINT_GOCACHE := $(shell echo
  ...)`. The old gate refused only the two bash-`_` cases (wrapper, `nice`); the new gate refuses
  the first seven and still passes the eighth, the recorded parse-time limit.
- After the round-7 fixes (gate only; `corvint test` unchanged): `gofmt -l` empty; `go vet
  ./tools/test-commands-check` rc 0; `go test -count=1 ./tools/test-commands-check` rc 0
  (14.6 s: twenty-two Makefile mutations, `TestSTRV0006DatabaseForms` over a captured GNU make
  3.81 excerpt and a 4.3-form excerpt written from `print_file`, `print_variable` and
  `print_commands` in that release's `src/file.c`, `src/variable.c` and `src/commands.c`, plus
  the earlier cases); `make -s test-commands-check` rc 0. Four Makefile scenarios were run
  against builds of the `c07788ba7` and round-7 gates: `go-test: SHELL := /usr/bin/nice
  /bin/sh`, `%: .SHELLFLAGS := -ec` and `go-%: SHELL := /bin/bash` passed the old gate (rc 0)
  and fail the new one (rc 1, naming the target or pattern); `%.o: SHELL := /bin/bash`, which
  does not match `go-test`, passes both. No GNU make 4.x is installed on this host (`gmake`
  absent), so the 4.x parse is pinned by the fixture and by the cited source, not by a live run:
  NOT_OBSERVED until the CI doc-gates job (ubuntu, GNU make 4.3) runs the gate.
- After the round-1 Codex fixes (`e2be5b36f`): `gofmt -l` empty and `go vet` rc 0 on the five
  packages; `go test` rc 0 for `internal/sessiontest`, `tools/test-commands-check`,
  `internal/gateselect`, `tools/str-parity` and the focused `cmd/corvint` run (74 s);
  `make -s test-commands-check` rc 0; `spec-requirements` regenerated (line shifts only) and
  `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `line-citations-check`, `decision-numbers-check`, `error-code-ownership-check`,
  `unbounded-readers-check` rc 0; real dry run from the fixed tree against the same
  `origin/main`: rc 0, `FULL`, one `go` invocation with the declared environment and flags,
  `incomplete []`, seven `noSuite` entries, manifest digest
  `sha256:d533686750a705c60b2830da26b4fe9d988f75498cabcb6e8678321eb442a739` (the digest moved
  with the tree, not the declaration).

## Parity (STR-V0-013) over the committed corpus

Command, with the driver binaries rebuilt from `96bed1f5e` (the round-4 tree; the earlier
slice-2 run from `bee5a3886` is kept below for the comparison):

```sh
str-parity -repo <checkout> -corpus <checkout>/tools/str-parity/testdata/pairs-645304a2.txt \
  -corvint <bin>/corvint -selector <bin>/selector -pr-tests <bin>/corvint-pr-tests \
  -script <checkout>/script/gate-affected.sh -declaration <checkout>/.github/cishards/test-commands.json \
  -source 96bed1f5e9cc5d09301bf0311ffed78f400d8921 -work <new dir>
```

- Exit 0 in 787 seconds. The `corvint-str-parity/0` report is 224,096 bytes, sha256
  `153442aadb7ea36a0b41c6fe093e569e047db2a20c1f4761561d8a66df5a7bb3`; it records `corpus`
  `a4d9da4b02496167e395f3da827f20d28630e356ea0742d3111241dcd54b8b7a` (the sha256 of the committed
  corpus file) and `declarationDigest`
  `9bcc950fd9af177ba2349dc7165b3ec9da16057053f4eb9d58d95891395cd302` (the sha256 of the committed
  `.github/cishards/test-commands.json` bytes after the round-4 rewording and the `typescript`
  command; the manifest's `declaredDigest` digests the parsed form and is a different value).
  The report is not committed; the command regenerates it.
- 283 differences in six categories, none `UNEXPLAINED`. Five categories keep the counts of the
  `bee5a3886` run (257 in five categories: `ci-admission-no-qualification` 95,
  `ci-closed-environment` 50, `ci-exec-confine` 50, `local-json-flag` 50,
  `str-v0-003-unknown-widening` 12). The 26 new differences are the category
  `str-v0-004-declared-language`, one per profile in 13 pairs: corvint also runs the declared
  `make host-adapter-test` where neither driver runs anything outside Go. The 13 are the ten
  pairs that were `str-v0-004-undeclared-language` on 2026-10-11 (nine `NARROWED` pairs whose
  selected units include a `typescript` unit under `integrations/`, `extensions/vscode`,
  `internal/doccorpus/testdata` or `internal/appmap/testdata`, plus the `FULL` pair 50) and the
  other three `FULL` pairs (30, 33, 49), because a `FULL` plan runs every declared language
  command. Per pair the corvint local scope and package count equal both earlier runs in all 50
  pairs: 46 `NARROWED`/`NARROWED`, 3 `FULL`/`NARROWED` (the STR-V0-003 widenings) and 1
  `FULL`/`FULL`; no pair is `INCOMPLETE`.
- The earlier run from `bee5a3886` (seven `none` languages, no declared command): exit 0 in 1026
  seconds, report 218,169 bytes, sha256
  `2b8ad62eedd0d4f29fa7da257aefa69ee26cab4dbe4a8fad983b06a5a7beea35`, `declarationDigest`
  `bfc9eee0c3d13e9aa58646fb2cfa9f4abff8cb069273024e7c5f8c29dcf153dd`; 257 differences against 277
  in six categories on 2026-10-11 (`docs/build-log/2026-10-11-session-test-runner-parity.md`),
  the 20 that vanished being the whole `str-v0-004-undeclared-language` category (ten pairs
  `INCOMPLETE` in both profiles for a typescript, python or ruby unit).
- `str-parity` itself (`96bed1f5e`) explains a `declared` difference only when every command
  corvint runs appears verbatim in the `-declaration` file's `languages` argv lists; a command
  outside the declaration, or a mixed list, stays `UNEXPLAINED` (`TestSTRV0013ParityReading`, cases "declared and undeclared commands" and "undeclared command").

## Independent review

Codex (`gpt-6-astra`, read-only, up to three rounds) over `bed653e92..HEAD`.

- Round 1 (`bee5a3886`): `CHANGES_REQUESTED`, three findings, each verified before fixing.
  [P1] the gate compared the recipe's shell spelling with the declaration, so a quoted word such
  as `-run='TestA|TestB'` matched although make hands the recipe to a shell and `corvint test`
  passes the word literally. [P2] the gate runs make with `PATH` only, so a developer
  `GO_TEST_TIMEOUT` or `CORVINT_GOCACHE` override is invisible to it. [P2] the reader accepted
  `\r`, NUL and other control or Unicode-space runes in environment names and values, flags,
  argv words and language names. Fixed in `e2be5b36f`: `compareGo` requires every recipe and
  declared word to be a plain word (letters, digits, `_ - . / = : , + @ %`), with a
  same-spelling quoted flag and three recipe cases in `TestSTRV0006CompareGo`; STR-V0-006 and
  the gate's comment state the defaults-only contract; `plain()` refuses any `unicode.IsSpace`
  or `IsControl` rune, with six new reader mutations.
- Round 2 (`2cde4d485`): `CHANGES_REQUESTED`, two findings, both verified. [P2] a leading
  recipe word such as `A-B=1` passed as an environment word on both sides although `/bin/sh`
  runs it as a command while exec sets it as a variable. [P2] `strings.Fields` split the recipe
  on U+00A0 and other Unicode whitespace before the plain-word check, so `-p<U+00A0>1` matched
  `["-p", "1"]` although the shell passes it as one argument. Fixed in the next commit:
  `sessiontest.EnvName` requires `[A-Za-z_][A-Za-z0-9_]*` in the reader (two more mutations) and
  for every leading assignment in the gate; the gate rejects any whitespace or control rune other
  than a space or tab in the raw recipe line before splitting on exactly those two; cases for a
  dashed name, U+00A0, `\r\n` and tab separators. Codex confirmed the corpus triples, both input
  digests, the requirement locators and the 257 arithmetic; it could not run Go tests in its
  read-only sandbox and did not verify the uncommitted parity report.
- Round 3 (`a4fd32143`, the last authorised round): `CHANGES_REQUESTED`, two findings, both
  verified. [P2] `compareGo` dropped a recipe line that `strings.TrimSpace` emptied before the
  new rune check, so a U+00A0-only line passed although make itself fails on it. [P2] two drift
  mutations in `TestSTRV0006DeclarationMatchesMakefile` failed in the reader, not in the
  comparison they named: `"GOFLAGS": ""` has an empty value (refused since round 1) and the
  `rust` member was a duplicate. Fixed after the round, without a further Codex pass: every raw
  recipe line is checked before filtering and only space/tab lines are blank (two cases); the
  mutations use `"-mod=mod"` and a `zig` language, the test asserts each mutation is readable and
  names the diagnostic its comparison prints (`go.flags give`, `go.env is`,
  `languages.zig names make no-such-target`). Codex again confirmed the corpus triples, input
  digests, category arithmetic, status records and requirement locators. The decision 0526
  condition (`VERDICT: PASS`) is therefore not met by this lane; the owner or the orchestrator
  decides whether a fourth round is run.
- Round 4 (`5dedd8684`, run by the coordinator): `CHANGES_REQUESTED`, three findings, all
  verified. [P1] drift in make's effective environment passed the gate: `go-test: export
  GOFLAGS := -race`, a global `export` or `.EXPORT_ALL_VARIABLES` leaves `make -s -n go-test`
  identical while the test process gets the variable. [P2] `none` asserted the wrong absence:
  STR-V0-004 and decision 0526 said "has no suite" while the declaration itself acknowledged npm
  and node suites, and excused `integrations/pi` as a "repository check" although `make gate`'s
  `host-adapter-test` runs its JS suites. [P2] `discover` resolved a repository-relative declared
  executable with `exec.LookPath` against the caller's working directory. Fixed in `96bed1f5e`:
  the shim-observing gate, the closed `makeIntroduced` set and the `GATE_STEPS` runner-word check
  described above (`TestSTRV0006MakefileDriftReachesGo`: target-specific export, global export,
  `.EXPORT_ALL_VARIABLES` with `EXTRA := 1`, absolute-path `go` via `/usr/bin/true`, two `go`
  calls, `&& false` after `go`, a changed flag; `TestSTRV0006DeclarationMatchesMakefile` keeps the
  six declaration mutations and adds a `none` typescript the gate runs and a `none` zig without a
  runner list); gate-scoped `none` in STR-V0-004, decision 0526 and the six reasons, with
  `typescript` declared `["make", "host-adapter-test"]`; root-relative resolution in `discover`
  with a runner test under `t.Chdir` and a CLI test (`TestSTRV0003CLIDeclaredRelativeCommandRunsFromRoot`,
  the test process's directory is the package, not `--root`).
- Round 5 (`3f5dab6ca`, run by this lane, prompt asking whether the shim gate is exact):
  `CHANGES_REQUESTED`, five findings, all verified against the `96bed1f5e` gate. [P1] a recipe
  that changes directory (`cd internal/sessiontest; ...`) passed: the shim recorded no working
  directory and `PWD` was excluded. [P1] a second `go` through an absolute path beside the
  `PATH` one passed: the count covered shim records only. [P2] a passed name removed from the
  environment (`env -u PATH`) passed: only observed entries were compared. [P2]
  `MAKEFLAGS=--just-print` prefixed to the recipe passed: the exclusion was by name. [P2]
  `"python3"` in a gate step evaded the `none` check: quotes were not trimmed. Fixed in
  `5fc504ffe` as described above (working directory in the record, passed names required
  present, the control recipe for the make-introduced values, the one-plain-line recipe shape,
  quote trimming), with six new Makefile mutations in `TestSTRV0006MakefileDriftReachesGo`,
  `TestSTRV0006CompareIntroduced`, and `dropped PATH`/`dropped both` cases in
  `TestSTRV0006CompareEnv`. The answer to the exactness question is bounded: the gate is exact
  about the one `go` the recipe runs from `PATH` (argv, environment, directory, make's exit) and
  refuses every recipe shape under which something else could run beside it; it does not and
  cannot audit an arbitrary program the Makefile could name, which is why the shape rule admits
  only a plain `go` line.
- Round 6 (`53ff3bebd`, run by this lane, same exactness question): `CHANGES_REQUESTED`, four
  findings. [P1] `SHELL := /bin/dash` with a recipe `<go.env> /bin/dash tools/wrapper go test
  ...` (wrapper `exec "$@"`) passed the shape rule: one `go` word, no shell syntax, and dash sets
  no `_` for the control to differ on. [P1] `$(shell ./tools/pre-test)$(GO_TEST_COMMAND) ./...`
  ran the script during the dry run and the observation while printing the plain line. [P2] a
  global `export MAKEFLAGS := --keep-going` was inherited by the control through `-f Makefile`
  (both saw `sk`). [P2] the spec's claim that `nice go test` passes was false: bash's `_` is
  `/usr/bin/nice` for the recipe and the shim's path for the control. Fixed in `c07788ba7` as
  described above (strict `NAME=VALUE... go` shape, standalone control makefile, the database
  parse refusing make functions the recipe expands and a Makefile `SHELL`), with seven more
  mutations in `TestSTRV0006MakefileDriftReachesGo` and `TestSTRV0006PlainRecipe`. The bounded
  answer stands with one named hole: make functions expanded while the Makefile is read are not
  observed.
- Round 7 (`7561537fc`, run by the coordinator over `bed653e92..7561537fc`, threat model: the
  Makefile is trusted and the gate is a drift alarm for plausible maintainer edits):
  `CHANGES_REQUESTED`, two findings. [P1] `parseDatabase` matched only make 3.81's `#  commands
  to execute`; GNU make 4.x prints `#  recipe to execute`, so on the ubuntu CI runner `db.recipe`
  stayed nil and the unchanged Makefile would have failed with `make -p prints no go-test:
  recipe`. [P1] the shell check read only the global `SHELL`/`.SHELLFLAGS` origin, so a
  target-specific `go-test: SHELL := /usr/bin/nice /bin/sh` wrapped the run while the printed
  recipe, the shim's argv, environment and directory and the global origin were all unchanged.
  Fixed in the round-7 commit: both headers accepted, 4.x's pre-target `go-test: NAME op value`
  lines, `+=`, `$(subst ,,)`, doubled `$` and the recipe prefix character handled, the
  pattern-specific section parsed with `matchesTarget`, and `plainRecipe` refusing
  `SHELL`/`.SHELLFLAGS` globally, on `go-test` itself and on any matching pattern (a prerequisite
  cannot carry a recipe: the dry run would print a second line). STR-V0-006, decision 0526 and
  the test counts were updated to match. Live GNU make 4.x remains unobserved on this host; the
  decision 0526 condition (`VERDICT: PASS` from an independent review) is not yet met and awaits
  the coordinator's round 8.
- Round 8 (`6ecae5897`, run by the coordinator): `CHANGES_REQUESTED`, one finding, verified.
  [P1] `expandsPlainly` read only the global and `go-test` target-specific values of a variable
  the recipe expands, so a pattern-specific `go-%: GO_TEST_FLAGS := $(shell ./tools/pre-test)`
  (any `%` pattern matching `go-test`) ran a make function during expansion while the printed
  recipe, the shim's observation and the global value were unchanged. A fix was written (every
  scope consulted for every expanded name and its siblings, four more mutations) and discarded
  uncommitted when the owner ended the loop; no further Codex round ran on the make-observing
  gate.

## Round 8 and the redesign (2026-10-12)

Why: rounds 4 to 8 each found a new make semantic because the gate reverse-engineered make. The
recipe-text comparison (rounds 1 to 3) gave way to a `go` shim (round 4), which needed the
working directory, a record count, a control recipe for the make-introduced names and a recipe
shape rule (round 5), then a `make -p` database parse to refuse make functions and `SHELL`
(round 6), then the 4.x database header and target- and pattern-specific `SHELL` (round 7), then
the pattern-specific values of every expanded variable (round 8). Each fix was right and each
left the next scope unread, because the gate was modelling what make would do with a Makefile
that restates the declaration. The owner's decision (2026-10-12, decision 0526 item 3): the
Makefile runs the declaration, so there is nothing to compare.

What changed, in one commit from `6ecae5897`:

- `tools/test-commands-run` (new; standard library and `internal/sessiontest` only): reads the
  declaration through the strict reader, runs `go test <go.flags> ./...` from the repository
  root with each `go.env` entry replacing the inherited value of that name, passes output through
  and returns `go`'s exit status; refuses arguments, an unreadable or absent declaration and a
  directory without `go.mod` with exit 2. `TestSTRV0006RunnerRunsTheDeclaration` proves the argv,
  directory, environment precedence (`GOCACHE` overridden, an inherited `GOFLAGS=-race` kept) and
  exit 3 under a fake `go` first on `PATH`.
- `Makefile`: `go-test:` is the one line `go run ./tools/test-commands-run`; `make -n go-test`
  prints exactly that. Nothing else changed but two comments. `GO_TEST_TIMEOUT`, `GO_TEST_FLAGS`
  and `GO_TEST_COMMAND` stay: `ledger/go-test` (GL-V0-004), `gate-affected`, and the specindex,
  cem and tasks targets use them, so the declaration governs `make go-test` only (recorded limit
  in STR-V0-006; slice 3 moves the call sites). No CI workflow changed.
- `tools/test-commands-check`: `shim`, `observeGoTest`, the control recipe, `recipeShape`,
  `readDatabase`/`parseDatabase`, `plainRecipe`, `expandsPlainly`, `matchesTarget` and their
  tests are deleted (810 to 514 lines). The gate is the text check of STR-V0-006 (a) to
  (e): logical lines with continuations joined, `#` comments stripped outside recipes, each line
  classified by make's own rule (the first `=`-operator or rule colon), `go-test:` exact with one
  exact recipe line, no recipe on any rule whose target could match `go-test`,
  `SHELL`/`.SHELLFLAGS`/`MAKEFLAGS`/`.RECIPEPREFIX` refused in every scope, `export MAKEFLAGS`,
  bare `export` and `.EXPORT_ALL_VARIABLES` refused, target- and pattern-specific variables that
  could reach `go-test` refused, `$(eval)`, `define`, computed and missing includes refused,
  literal includes followed four deep, `GATE_STEPS` read as one literal line, the `none`-language
  dry-run kept. One deviation from the brief's "no prerequisites": a rule whose target could
  match `go-test` may list prerequisites without a recipe, because the repository's own
  GOC-V0-010 line `$(GATE_STEPS) $(addprefix ledger/,$(GATE_STEPS)): | gate-receipt-clear` gives
  `go-test` an order-only prerequisite; the `go-test:` line itself still takes none.
- Docs: STR-V0-006 rewritten (delegation, the text rules, the limits; every claim about
  observing make execution removed) with its Boundaries row, Acceptance bullets and Traceability
  row; decision 0526 items 3 to 5 and its Enforcement and Rollback; `REQUIREMENTS.tsv`
  regenerated.

Review and gates under the owner's 2026-10-12 decisions: Codex review is capped at one round on
this result, then ship; acceptance follows that round with each finding fixed or recorded as a
limit, not a `VERDICT: PASS` (decision 0526 item 4). `make gate` is waived for V1-0040: the
ticket's `requiredGates` full gate is `NOT_RUN` under the waiver, replaced by the focused tests,
the CI doc-gates list run locally, the use-case receipts checks and the CI run on #815 (item 5).

Evidence (this commit; host GNU Make 3.81, go1.27.2, `GOTOOLCHAIN=local`): `gofmt -l` empty on
both tools; `go vet` rc 0 on `tools/test-commands-check`, `tools/test-commands-run`,
`internal/sessiontest`; `go test -count=1` on the same three packages rc 0 (2.1 s, 1.0 s, 5.2 s);
`make -s test-commands-check` rc 0, printing "Makefile go-test is exactly `go run
./tools/test-commands-run`, which runs .github/cishards/test-commands.json (GOCACHE=/tmp/corvint-go-build-cache
GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./... in the repository root); no SHELL,
.SHELLFLAGS, MAKEFLAGS or go-test variable in the Makefile; 1 declared make targets dry-run; no
make gate step recipe runs a suite of [dotnet kotlin python ruby rust swift]"; `make -n go-test`
prints the one line `go run ./tools/test-commands-run`, rc 0 (the full run was not repeated here:
the fake-`go` test proves the argv, directory and environment). CI doc-gates list
(`spec-requirements-check requirement-definitions-check traceability-tests-check
decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check`)
rc 0 after the Makefile comment was kept at four lines so the `Makefile:42`, `:111` and `:130`
citations in other specs still resolve (the first attempt, two lines longer, broke all three) and
`tools/test-commands-run` was declared with no outside reads in `.corvint/test-read-scopes.json`
(the first `unbounded-readers-check` run flagged the new package, rc 2);
`make -s use-case-receipts-check use-case-receipts-test` rc 0 (18 receipts current, 5 cases).
Nothing under `cmd/` changed, so `cmd/corvint` tests were not run. `make gate`: `NOT_RUN` (owner
waiver, above). Live GNU make 4.x is no longer a dependency of the gate: it reads text and
dry-runs only `make -s -n` for the declared targets and gate steps.

## Round 9, the one capped round (2026-10-12)

Codex reviewed `21af64b58` once (decision 0526 item 4) and returned six P2 findings. The owner
then said to fix them without waiting for another round, so there was no round 10.

1. `make gate` runs Go tests through `ledger/go-test`, whose `go test $(GO_TEST_FLAGS)` was a
   second statement of the command that the declaration did not govern. Fixed: STR-V0-006 rule (f)
   pins that rule's one recipe line, the single global assignments of `GATE_LEDGER`,
   `GO_TEST_FLAGS`, `GO_TEST_TIMEOUT` and `CORVINT_GOCACHE`, and their expansion against the
   declared `go.env` and `go.flags` (gate-ledger adds `-count=1` itself only for unresolved
   packages). TestSTRV0006LedgerMatchesDeclaration has twelve refusals, and removing the rule
   passes because `make gate` then reaches `go-test` through `ledger/%`.
2. The check still dry-runs `make -s -n` for the declared targets and `GATE_STEPS`, so "reads text
   only" was not true. Recorded: STR-V0-006 now names rules (d) and (e) as dry-runs that expand the
   Makefile, and the rest as text rules.
3. A tab line inside an `ifeq` branch after the `go-test` recipe was not counted against it, because
   the conditional directive reset recipe ownership. Fixed: conditional directives no longer end a
   recipe, so a hidden second line is refused.
4. `.IGNORE` would let a failing `go test` succeed. Fixed: naming it as a target, with or without
   prerequisites, is refused.
5. A declared `go.env` `PATH` cannot select which `go` runs, because exec resolves `go` on the
   caller's `PATH`. Fixed: the reader refuses it (STR-V0-003).
6. Decision 0526 said the gate refuses a stale `none`; it refuses one only when a direct
   `GATE_STEPS` recipe word names that language's runner. Narrowed in 0526 and STR-V0-006.

Still recorded limits, not fixed: the environment make starts with (`GOFLAGS`, or an override of
a `?=` variable such as `GO_TEST_TIMEOUT`) reaches `go`; a Makefile `export GOFLAGS` is not
refused; `gate-affected` uses `GO_TEST_COMMAND` until slice 3.

Evidence: `gofmt -l` empty; `go vet` rc 0 and `go test -count=1 ./tools/test-commands-check
./tools/test-commands-run ./internal/sessiontest` ok; `make -s test-commands-check` rc 0 on the
committed Makefile.

## Left for slice 3

The call-site switch (`make gate-affected`, then the CI call site under AFP-V0-016 consent and a
new forward epoch), whether the ci profile reads its own declared profile, the AFP-V0-029 shadow
bar before CI narrowing, the balanced-placement 20% measurement, and the LPCV-V0-021 capability
manifest per adapter.
