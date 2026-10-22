# V1-2327: the structural label is lifted by one go1.27.1 corpus

Ticket V1-2327. Decision 0551, owner-approved in chat on 2026-10-21. Requirements `CEM-V1-369`
(corpus packet), `CEM-V1-370` (parser-closure admission) and `CEM-V1-371` (label), with `CEM-V1-050`
and `CEM-V1-091` amended.

## What changed

- `internal/cem/verify/stable.go`, `interop/cem01-go/cem.go` and
  `interop/cem01-go/stable_repository.go` report `runtime.structuralQualification`
  `QUALIFIED_EXACT_TUPLE`, not `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`, on an admitted tuple.
  The value was already in the result schema's enum.
- `protocol/cem-1.0/structural-corpus/` seals the qualifying corpus:
  - `CORPUS.json` names the repository selection and the observation and perturbation rules;
  - `cases/` holds 25 of the 26 version-sensitive cases, with the go1.27.1 observations of all
    26 in `CASES.observations.jsonl`. The zero-byte `empty-file.go.txt` is declared, not
    committed: CEM 1.0 refuses an empty-file creation (`metadata-only-change-unsupported`,
    `CEM-V1-300`), so both harnesses observe it as zero bytes at its sorted path;
  - `QUALIFICATION.json` records every run, host and digest;
  - `manifest.json` binds all of them.
- `tools/cem-structural-closure` computes the parser-closure digest. Both tuple packets record it
  under `parserClosure`.
- Two tests compare `structuralTuples` with `QUALIFICATION.json` `admittedTuples`, one per verifier
  module. `TestRunningToolchainClosure` checks the running toolchain's closure against the
  qualified one.
- The frozen packet's `runtime` values were re-emitted by `make cem-stable-regen-replace`:
  - a go1.27.3 capture covering 54 cases and 29 cells, so 83 values;
  - the two go1.24.13 cases passed unchanged through `--values`;
  - 7 unmatched cells passed through `--cell-values`.

  `goVersion` stays `go1.27.1`. `TestReplaceRuntimeKeepsFrozenTuple` now exercises the rollback
  direction on the re-emitted packet.
- The stable `ALGORITHMS.md` and `OPERATION.json` prose now names go1.27.3. It had omitted that
  tuple since decision 0547.

## Corpus

- **Repository.** Corvint at `ef2a01db52d5436c262725d136060c9eb95e8580`: 4824 tracked `.go` files
  plus the 12 `internal/cem/verify/testdata/structural` fixtures, 4836 files.
  - Path list sha256 `9a4b0fd0…2dde57`.
  - Extracted with `git archive`.
  - It yields 19190 perturbation pairs (rename, move, import reorder, formatter-only and
    identity-derived). Each pair is judged in both directions.
- **Cases.** 26 files, yielding 85 pairs.

## Runs

Each digest is the sha256 of one observation JSONL. `QUALIFICATION.json` holds the full values,
the uname strings and the binary digests.

| Host | Toolchain | Repository observations | Case observations |
| --- | --- | --- | --- |
| darwin/arm64 | go1.27.1 (official) | `beb9a5b2…7eda39` | `55792ead…8a9a41` |
| darwin/arm64 | go1.27.2 (official) | `beb9a5b2…7eda39` | `55792ead…8a9a41` |
| darwin/arm64 | go1.27.3 (Homebrew) | `beb9a5b2…7eda39` | `55792ead…8a9a41` |
| darwin/arm64 | go1.24.13 (official) | `f8e866b9…c5b773` | `afe35c56…ad718c` |
| linux/arm64 | go1.27.1 | `beb9a5b2…7eda39` | `55792ead…8a9a41` |
| linux/arm64 | go1.24.13 | `f8e866b9…c5b773` | `afe35c56…ad718c` |

- **Darwin host.** Mac Studio, Darwin 25.6.0, arm64.
- **Linux host.** A colima VM on that Mac: Ubuntu 24.04.4, kernel 6.8.0-117-generic aarch64,
  docker 29.5.2. The run used image `node:24-alpine` (`sha256:44fce696…`, linux/arm64) with
  `--network none`. The Linux test binaries are static `GOOS=linux GOARCH=arm64` builds,
  cross-compiled on darwin by the same toolchains.
- **Native parity.** On both hosts, the native verifier built with go1.27.2 and with go1.27.3 reaches
  the portable go1.27.1 verdicts on every pair: 19190 repository pairs and 85 case pairs.
- **go1.24.13.** It differs from go1.27.1 only in the parse digest: 19 of 26 cases and 4772 of
  4836 repository files. The cause is the new `ast.BasicLit.ValueEnd` field. 7 of 3571 rename pairs
  are rejected under go1.24.13, each correctly. The comparison shows the corpus detects a parser
  change, and go1.24.13's closure digest differs, so it is not admitted.
- **Runtime re-emission.** `make cem-stable-regen-check` passes after the re-emission and the tuple
  packet edits: 56 cases, 23 keys, packet manifest `c6a5cbe0…5834f4`, 2 structural tuples.

## Limits

- Windows and linux/amd64: NOT_RUN.
- No native Linux toolchain built the Linux binaries, and no CI host ran the corpus.
- The frozen `cancelledContextExpectedResult` snapshot in the two envelope cell files keeps the old
  label, as decision 0551 records.
- A later Go release is admitted only by a new tuple packet whose parser-closure digest equals
  `83e19e1d…877ec7`. A release that changes the closure needs a new corpus run.
- The full-corpus parity runs read observation files outside the repository. Their completeness rests
  on the observation digests recorded in `QUALIFICATION.json`, not on a count check in the harness.

## Verification

- Focused tests pass: `internal/cem/verify`, `internal/cem/workflow`, `internal/cem/cli`,
  `internal/doccorpus`, `internal/witness`, `internal/witnesscollapse`, `internal/console`,
  `tools/cem-stable-regen`, `tools/cem-structural-closure`, `tools/test-read-scopes-check` and
  `interop/cem01-go`, plus `go vet` in both modules.
- After the zero-byte case became a declared case, the cases still observe `55792ead…8a9a41` on
  go1.27.1 and `afe35c56…ad718c` on go1.24.13. Native case parity passes on go1.27.2 and
  go1.27.3.
- The local doc-gates, `gofmt -l` and `make windows-vet` pass. The dogfood binding is the CEM
  sealed with this change. The repository-wide `make gate` was not run.

## Review

One Codex round (`gpt-6-astra`, read-only) over `ef2a01db5..HEAD` raised four findings. All are
fixed in this change:

1. **High.** The closure excluded `path/filepath` and `internal/filepathlite`, but `go/scanner`
   uses them on `//line` file names. Both are now in the closure. The digest became
   `83e19e1d…877ec7` (35 packages, 323 files), and go1.27.0 through go1.27.3 still share it. The
   corpus observations are unaffected, since the closure only governs admission.
2. **Medium.** An empty corpus or observation set passed. Both full-corpus harnesses now fail on
   zero files or pairs.
3. **Medium.** `TestRunningToolchainClosure` resolved `go` from PATH and skipped on a mismatch. It
   now digests the `GOROOT` that `go test` exports for the building toolchain, and fails if that
   GOROOT cannot be found or names another version.
4. **Medium.** `CORPUS.json` described the verdict bits as four forward then four reverse. The
   harnesses interleave forward and reverse per predicate, and the text now says so.
