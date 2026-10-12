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
  `["-p", "1", "-timeout", "30m", "-count=1"]`, and seven languages declared `{"none": reason}`:
  `dotnet`, `kotlin`, `rust`, `swift` (the only sources are testdata fixtures under
  `tools/beamfall-shadow`, `internal/analyzerkotlinandroid`, `internal/liveverify/affected` and
  `internal/analyzerswift`; no project file or test command), `python` and `ruby` (benchmark and
  qualification scripts, runner reporters and affected-graph fixtures; no gate step,
  `script/gate-affected.sh`, `tools/corvint-pr-tests` or CI job runs a suite, no pytest, unittest,
  Gemfile or Rakefile), and `typescript` (the `extensions/vscode` npm test and the
  `integrations/pi-protected` and `tools/web-flows` `node --test` suites are not run by `make
  gate`, the drivers or CI; `make gate`'s `host-adapter-test` runs the `integrations/pi` JS suites
  as a repository check, not a language full suite). Evidence for each reason: a search of
  `.github/workflows`, the Makefile, `script/gate-affected.sh` and `tools/corvint-pr-tests` for
  `npm`, `node --test`, `pytest`, `python -m unittest`, `ruby`, `cargo`, `swift test`, `gradle` and
  `dotnet` found no suite invocation.
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
  owner posts): runs `make -s -n go-test` with `PATH` only, requires one command line, compares
  the leading `NAME=VALUE` words as a set with `go.env`, requires the rest to equal
  `go test <go.flags> ./...` exactly, and dry-runs every declared language command whose
  `argv[0]` is `make`. It is not in `tools/gate-ledger`'s scoped map, so it runs on every change.
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

Command, with the slice-2 driver binaries rebuilt from `bee5a3886`:

```sh
str-parity -repo <checkout> -corpus <checkout>/tools/str-parity/testdata/pairs-645304a2.txt \
  -corvint <bin>/corvint -selector <bin>/selector -pr-tests <bin>/corvint-pr-tests \
  -script <checkout>/script/gate-affected.sh -source bee5a388682121053206f04e1591d5e8a76f0083 -work <new dir>
```

- Exit 0 in 1026 seconds. The `corvint-str-parity/0` report is 218,169 bytes, sha256
  `2b8ad62eedd0d4f29fa7da257aefa69ee26cab4dbe4a8fad983b06a5a7beea35`; it records `corpus`
  `a4d9da4b02496167e395f3da827f20d28630e356ea0742d3111241dcd54b8b7a` (the sha256 of the committed
  corpus file) and `declarationDigest`
  `bfc9eee0c3d13e9aa58646fb2cfa9f4abff8cb069273024e7c5f8c29dcf153dd` (the sha256 of the committed
  `.github/cishards/test-commands.json` bytes; the manifest's `declaredDigest` digests the parsed
  form and is a different value). The report is not committed; the command regenerates it.
- 257 differences in five categories, none `UNEXPLAINED`, against 277 in six categories on
  2026-10-11 (`docs/build-log/2026-10-11-session-test-runner-parity.md`). The 20 that vanished
  are the whole `str-v0-004-undeclared-language` category: ten pairs each selected one unit of a
  language with no adapter (typescript in eight pairs, python in one, ruby in one) and were
  `INCOMPLETE` in both profiles; the declaration now asserts `none` for those languages, so the
  unit is a `noSuite` entry and the plan is `NARROWED` (or `FULL`) without an `INCOMPLETE`. The
  other five categories keep their counts: `ci-admission-no-qualification` 95,
  `ci-closed-environment` 50, `ci-exec-confine` 50, `local-json-flag` 50,
  `str-v0-003-unknown-widening` 12.
- Per pair the corvint local scope and package count equal the 2026-10-11 run in all 50 pairs:
  46 `NARROWED`/`NARROWED`, 3 `FULL`/`NARROWED` (the STR-V0-003 widenings) and 1 `FULL`/`FULL`.
- The parity binaries predate the round-1 Codex fix (`e2be5b36f`), which only changes which
  malformed declarations the reader refuses; the committed declaration is accepted by both
  builds (`make test-commands-check` and the real dry run below pass on the fixed tree with the
  same invocation), so the plans are not affected. This is an inference from the fix's scope,
  not a rerun.

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
- Round 3: see the line appended below when it finishes.

## Left for slice 3

The call-site switch (`make gate-affected`, then the CI call site under AFP-V0-016 consent and a
new forward epoch), whether the ci profile reads its own declared profile, the AFP-V0-029 shadow
bar before CI narrowing, the balanced-placement 20% measurement, and the LPCV-V0-021 capability
manifest per adapter.
