# V1-5236: release each product binary independently from one tag pipeline

Experimental, under the owner's standing lane auto-accept; base
`0e99e1e6a` (the decision 0594 cut branch), branch `claude/release-per-binary-5236`. Decision
0594 item 8 keeps one engine repository organised as layers and asks that each binary be
released on its own, so a docs-only fix ships without rebuilding or re-qualifying the analyzers.
This entry records the design first, then what was built and measured.

## Design

1. **Catalog.** `tools/release-binary/binaries.json` names every `cmd/` executable exactly once
   as `product` (released) or `support` (release tooling, never archived), cites the decision
   that admits it, and assigns each product binary to a version set or leaves it independent.
   The `core` set is `corvint`, `corvint-docs`, `corvint-cli`, `corvint-mcp`, `corvint-docs-mcp`,
   `corvint-test-validity-mcp`, `corvint-web-flows` and `corvint-test-runner`: `corvint` execs
   `corvint-docs`, and the rest link the same internal packages. Analyzers, test providers and
   `corvint-update` are independent because they speak versioned envelopes.
2. **Per-binary build entry.** `release-binary build --binary NAME --version V --output DIR`
   builds that one package for the five release targets from a clean `HEAD`, stamps
   `main.build` and `main.sourceCommit`, binds each binary's embedded build info to the commit,
   and assembles `NAME_<goos>_<goarch>.tar.gz` (`.zip` on Windows) with the same root, legal
   files, inner `SHA256SUMS`, modes and zero timestamps as the existing `corvint` archives,
   through the existing `archivebuild` encoders. It also writes `NAME.SHA256SUMS` and
   `NAME.release-entry.json`. Nothing else is compiled.
3. **Manifest.** `release-binary manifest --output DIR [--base FILE]` merges the entries in DIR
   over the previous release's `release-manifest.json`, so a one-binary release carries the
   other binaries' recorded version, source commit, build number, Go version and archive
   digests forward unchanged, and writes `release-manifest.json` with `release-manifest.sha256`.
4. **Skew rule.** Members of one version set present in the manifest must agree on version and
   source commit; a disagreement refuses with `version-skew` naming the set and the binaries,
   unless `--declare-skew SET=REASON` records it under `declaredSkew`. Independent binaries
   never skew. `verify` re-runs the check and rechecks archive digests without repairing.
5. **Selection.** `release-binary select --tag TAG` maps `vX.Y.Z` to every product binary,
   `SET/vX.Y.Z` to a version set and `NAME/vX.Y.Z` to one binary, refusing support binaries and
   unknown names. The workflow (a separate `.github/` change) fans one build job out per
   selected binary and publishes only from an owner-approved job; this tool has no network,
   tag or upload code.

## What changed

- `tools/release-binary/` (new, release layer): `binaries.json` (the catalog: 32 `cmd/`
  executables, 28 product, 4 support, one version set), `binaries.go`, `build.go`, `manifest.go`,
  `select.go`, `main.go` and `release_binary_test.go`. The tool imports the existing
  `conformance/release-artifact-v0/archivebuild` and `archivewire` packages; it adds no network,
  tag, signing or transfer code (`TestNoPublicationCode` scans for it).
- `docs/specs/release-artifact-integrity-v0.md`: new section "Per-binary release profile" with
  `ARTIFACT-BIN-V0-001..007` and its traceability table; digest `Exists` and `Read next` lines.
- `AGENTS.md` invariant 7 names the product binaries and the catalog.
- `Makefile`: `release-binaries-check` (in `GATE_STEPS`), `release-binary` and
  `release-manifest`; four line citations repinned after the one-line shift.

## Measured on this host (linux/amd64, go1.27.3, `--host-only`)

Commands run at `81b63716b`, the clean committed tree, through
`go run ./tools/release-binary` with a reduced three-binary catalog so `manifest` could accept
host-only entries (the committed catalog requires all five targets per entry):

| Step | Result |
|---|---|
| `check` against the committed catalog and `cmd/` | PASS, 32 binaries, 28 product, 1 version set |
| `build corvint-cli`, `corvint-docs`, `corvint-analyzer-shell` at 1.0.1 | 1 s, 3 s, under 1 s with a warm build cache (16 s for `corvint-docs` cold); each wrote one archive, `NAME.SHA256SUMS` and `NAME.release-entry.json` |
| `manifest` then `verify --output` | PASS, 3 binaries recorded, 3 archive sets rechecked |
| `make release-binary RELEASE_BINARY=corvint-docs RELEASE_VERSION=1.0.1 RELEASE_HOST_ONLY=1` | entry stamped `sourceCommit 81b63716b…`, build 183 |
| `corvint-docs` only at 1.0.2 over release 1 | `REFUSE version-skew: set core members disagree: corvint-cli at 1.0.1@…; corvint-docs at 1.0.2@…`; no manifest written |
| same with `--declare-skew "core=docs-only fix, corvint-cli unchanged"` | PASS; `declaredSkew` records the set and reason; `corvint-cli` and the analyzer carried forward unchanged |
| `corvint-analyzer-shell` only at 1.0.3 over release 2 | PASS without re-declaring; the declaration carries while `core` still disagrees |
| one byte of an archive changed, `verify --output` | `REFUSE archive-mismatch: corvint-analyzer-shell_linux_amd64.tar.gz: sha256 differs from the manifest` |
| extracted `corvint-docs` archive | `sha256sum -c SHA256SUMS` OK; `./corvint-docs --version` prints `corvint-docs 1.0.0-rc.3.dev` |

The last row is a finding: only `cmd/corvint` declares `main.build` and `main.sourceCommit`, so
the linker drops the stamp for every other binary and they cannot self-report the commit the
manifest records. `build` therefore refuses a `--version` that differs from the `VERSION` file
(`version-mismatch`), so the manifest never labels a binary with a version it does not compile
in; the per-binary stamp and the `CMS-V0-002` beside-mismatched-version refusal are follow-ups.
The full five-target build of every binary, the real tag-driven workflow run and the
`version-mismatch` case on the real tree (`VERSION` is `1.0.0-rc.3`, so a `1.0.1` release of the
real catalog refuses until `VERSION` is bumped) are `NOT_RUN` in this lane.

## Checks

- `GOTOOLCHAIN=local go build ./...`, `go vet ./tools/release-binary/`: clean.
- `GOTOOLCHAIN=local go test -count=1 ./tools/release-binary/`: 7 tests PASS, 3.2 s (the fixture
  module is built four times with the local toolchain).
- `make release-binaries-check`: PASS.
- `make -s -k spec-status-check spec-requirements-check unbounded-readers-check
  traceability-tests-check line-citations-check`: PASS after moving the new IDs off the frozen
  digest Status line, dropping the `--show-toplevel` literal that made the tool an unbounded
  reader, and repinning four citations.
- `make changed-checks BASE=0e99e1e6a…`: ok. `make windows-vet`: exit 0.
- Not run: whole-repository tests, `make gate`, any five-target build, any network or tag.

## Non-goals and rollback

Single build per target (the `corvint` archive gate keeps the double-build and offline-verifier
proof); no smoke of the built binary; no installer or `docs/INSTALL.md` change (V1-5237's
`corvint docs` NOT_INSTALLED gap is closed only in that `corvint-docs` now has its own archive
in the core set and must ship with `corvint`); the `.github/` workflow is a separate change.
Rollback: revert the lane commits; the `corvint` archive gate and its witnesses are untouched.

## Review fixes (lane relfix, 2026-11-10)

An independent review of the tool reported twelve findings. Each was checked against the code
before anything changed; every real one has a test that failed before its fix (the pre-fix tool
built from `dd34f74f9` was run against the same scenarios by hand where the new test could not
compile against the old types). Spec text changed in the same change
(`docs/specs/release-artifact-integrity-v0.md`, `ARTIFACT-BIN-V0-001..007`, `-508`).

| # | Finding | Disposition |
|---|---|---|
| 1 | A carried skew declaration covered any later skew in the set | FIXED. `SkewDeclaration` gains `members`, the exact `version@commit` of every recorded member when declared; `mergeManifest` binds this release's declaration to the merged snapshot and keeps a base declaration only while the set still records that snapshot; `validate` (so `manifest` and `verify`) refuses `stale-skew-declaration` for any other snapshot and `version-skew` for the uncovered set. The release-candidate mirror `appSkew` carries the field. The old tool accepted alpha moving to 1.0.2 under release 2's declaration; the new test refuses it. |
| 2 | `verify --output` rechecked only binaries with an entry file | FIXED. `verifyOutput` selects by the manifest's records and the archive files present, requires `release-manifest.json.sha256` to bind the verified bytes, rechecks every archive and `NAME.SHA256SUMS`, refuses a partial archive set, an unrecorded archive and `nothing-rechecked`. The old tool passed an archives-only directory with a tampered archive ("0 archive sets rechecked"). |
| 3 | `version-mismatch` compared every binary with `VERSION`; non-core binaries carried no stamp | FIXED, building on the packaging work (`version_stamp.go`, `cliapp.AnswerStamped`, `ARTIFACT-BIN-V0-508`). The `VERSION` comparison applies to the core set only. An independent binary is stamped `-X main.version=X.Y.Z`; the four non-analyzer independents (`corvint-update`, the three test providers) declare `version` and answer `--version` through the new `cliapp.AnswerReleased`, so `corvint-update 1.4.2 (build 183, commit …)` is what a stamped build prints (verified by building it). The catalog marks the sixteen analyzers `stamp: none` (their `--version` is a protocol receipt; the manifest alone records their version). `build` parses the main package and refuses `stamp-unbound` when a stamped variable is not declared, since the linker drops an `-X` silently. The old tool refused an independent fixture binary at its own version. |
| 4 | Failed writes left partial output | FIXED. `outputs` reserves every path before the first write, writes each file to a temporary sibling and renames it, and `discard` removes what a failed run wrote; `runManifest` reserves both files before writing either. The old tool left `release-manifest.json` behind when `.sha256` pre-existed. |
| 5 | Output inside the repository was not refused | FIXED. `refuseOutputInsideRepository` refuses `output-inside-repository` for a DIR at or under the root in `build`, and in `manifest` when the root (`--root`, default the working directory) holds `.git`. The old tool built into `ROOT/dist` and left three untracked files. |
| 6 | Ignored files escaped the clean check; `GOWORK` unset | FIXED. `GOWORK=off` in the closed environment; the build runs from `exportHead`, a shared local clone of the repository checked out at the commit (155 MB, under one second on this host, nothing written into the repository), so only committed bytes reach the compiler while `vcs.revision` still binds. The old tool compiled an ignored `ignored.go` with a panicking `init` into the archive. |
| 7 | `TestNoPublicationCode` was a string search | FIXED. The test parses the source with `go/parser`: imports must be standard library outside `net`/`crypto/tls` or the two archive packages; every `exec` call is `exec.CommandContext` with a literal `go build`/`go env` or `git` inside `gitOutput`; every `gitOutput` caller passes a literal subcommand on the allowlist `rev-parse status rev-list clone checkout`; no string literal names a URL or a release client. A probe file with `gitOutput(ctx, root, "push")` and a `curl` call failed on four counts. |
| 8 | The Windows `.zip` branch was untested off Windows | FIXED. `TestAssembleArchiveWindowsZip` builds `corvint-alpha_windows_amd64.zip` directly: four members, `.exe` at 0755, legal files and inner `SHA256SUMS` at 0644, stored, canonical 1980 timestamp. |
| 9 | Untested requirements | FIXED. `buildinfo-mismatch` (another commit, another target) on the archived fixture binary; `loadEntries` refusals (file name, non-product, version set, archive count, archive name, target); `verify` refusing undeclared skew in a stored manifest. The duplicate-entry branch was unreachable (the file name is the binary's) and is removed; the spec names `entry-name-mismatch` as the refusal. |
| 10 | `REQUIREMENTS.tsv` rows held the traceability table row | FIXED. The twelve bullets (`001..007`, `508..512`) are now rank-0 definition lines (`` - `ID`: (note) text ``) inside the `## Requirements` section under two `###` subheadings, which the generator requires (SRG-V0-891; the frozen outside-section baseline may not grow). `make spec-requirements` now records each with status `standing` and its date and ticket. The TSV title is cut at 80 characters (SRG-V0-005), so the acceptance note fills it; the normative text is on the definition line. |
| 11 | A version set may share a binary's name | FIXED. `Catalog.validate` refuses a version set named like a binary, so `NAME/vX.Y.Z` is never ambiguous; tested through `select`. |
| 12 | `AGENTS.md` invariant 7 unchecked against the catalog | FIXED. `TestAgentsInvariantNamesTheCatalogProductBinaries` reads the "The product binaries are …" sentence, expands `corvint-analyzer-*`, and requires set equality with the catalog's product binaries. |

Checks on this host: `go build ./...`, `go vet` over `tools/release-binary`, `internal/cliapp`,
`internal/releasecandidate` and the four independent `cmd/` packages clean;
`go test -count=1 ./tools/release-binary/` 17 s PASS (including the installed-set smoke),
`./internal/releasecandidate/`, `./internal/cliapp/`, `./cmd/corvint-update/`,
`./cmd/corvint-ruby-test-provider/`, `./cmd/corvint-go-test-provider/` (68 s) PASS;
`./cmd/corvint-js-test-provider/` fails only on the pre-existing
`TestDocsHelpProducerExampleUsesE2EFlags` (opens `../corvint/docs.go`, absent since decision
0594), as the packdocs handback recorded. `make -s -k spec-status-check spec-requirements-check
unbounded-readers-check traceability-tests-check line-citations-check release-binaries-check`
PASS; `make changed-checks BASE=dd34f74f9ff44518ee1f016f813788dceb3f1337` ok; `make windows-vet`
exit 0. Not run: whole-repository tests, `make gate`, any five-target build, any network or tag.
