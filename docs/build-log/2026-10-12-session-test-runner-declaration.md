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
  is redundant); the reader keeps its own validation. It is not in `tools/gate-ledger`'s scoped
  map, so it runs on every change.
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
  the test process's directory is the package, not `--root`). The decision 0526 condition
  (`VERDICT: PASS`) is checked by rounds 5 and 6 below.

## Left for slice 3

The call-site switch (`make gate-affected`, then the CI call site under AFP-V0-016 consent and a
new forward epoch), whether the ci profile reads its own declared profile, the AFP-V0-029 shadow
bar before CI narrowing, the balanced-placement 20% measurement, and the LPCV-V0-021 capability
manifest per adapter.
