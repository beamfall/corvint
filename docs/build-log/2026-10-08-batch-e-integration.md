# 2026-10-08: Batch E integration (V1-0598, V1-0600/0601, V1-0624, V1-0687, V1-0529)

## Intent

Integrate five finished, independently reviewed lanes onto `origin/main`
`2a93b5a182966f72836cd28bf569dc44c0f5220c` as one batch branch,
`claude/batch-e-2026-10-08`. Each lane is merged with `git merge --no-ff`, so its reviewed commits
stay intact; the integration adds only conflict resolution and requirement renumbering.

## Merges

| Order | Lane branch | Lane head | Lane base | Merge commit | Conflicts |
| --- | --- | --- | --- | --- | --- |
| 1 | `claude/v1-0598-mocha` | `91662aa8` | `722904f9` | `bcf0f03e` | none |
| 2 | `claude/v1-0600-0601-dotnet` | `70c5f8ac` | `722904f9` | `cf1bb03a` | `docs/specs/test-runner-execution-v0.md`, `docs/specs/REQUIREMENTS.tsv` |
| 3 | `claude/v1-0624-fifo-admission` | `089910c1` | `2a93b5a1` | `c4dfde49` | `docs/specs/test-runner-execution-v0.md`, `docs/specs/REQUIREMENTS.tsv` |
| 4 | `claude/v1-0687-go-json-pass` | `e9973ca2` | `722904f9` | `1d819075` | none |
| 5 | `claude/v1-0529-mcp-preview-parity` | `06269b08` | `2a93b5a1` | `6f630640` | none |

## Requirement renumbering

Main already held `TRE-V0-021..023` (decision 0454). Three lanes each appended new requirements to
`docs/specs/test-runner-execution-v0.md` starting at `TRE-V0-024`.

| Lane | Ticket | Lane ID | Integrated ID |
| --- | --- | --- | --- |
| Mocha selection reconciliation | V1-0598 | `TRE-V0-024` | `TRE-V0-024` (unchanged) |
| Mocha runner/profile registration | V1-0598 | `TRE-V0-025` | `TRE-V0-025` (unchanged) |
| .NET TRX failure evidence | V1-0600 | `TRE-V0-024` | `TRE-V0-026` |
| MTP socket path admission | V1-0601 | `TRE-V0-025` | `TRE-V0-027` |
| Nonregular runner document admission | V1-0624 | `TRE-V0-024` | `TRE-V0-028` |
| Go JSON PASS event marker | V1-0687 | `LTA-V0-015` | `LTA-V0-015` (no collision) |

Every lane reference was renumbered: the spec requirement bullets, traceability rows and recorded
limits; the lane build-log entries `2026-10-08-v1-0600-trx-failure-evidence.md`,
`2026-10-08-v1-0601-mtp-socket-path-admission.md` and `2026-10-08-runner-fifo-admission.md`; and the
code comments in `internal/testrunner/native/mtp_live_test.go`, `mtp_test.go` and
`trx_adversarial_test.go`. No test function name encoded a TRE ID. No lane changed
`docs/specs/README.md`, `docs/specs/INDEX.json` or the spec header, because the new requirements are
proposed and the accepted range `TRE-V0-021..023` is unchanged; those files need no edit.

## Conflict resolutions

- `docs/specs/test-runner-execution-v0.md`: each conflict was two lanes appending a new section at
  the end of the file. Both sides were kept in merge order (Mocha, .NET TRX, MTP socket path,
  nonregular document admission), with the incoming side renumbered as above.
- `docs/specs/REQUIREMENTS.tsv`: generated. The conflict was resolved by taking the current side,
  staging the spec, and regenerating with `make -s spec-requirements`.
- `internal/testrunner/execute_unix.go` auto-merged: V1-0601 changes the executor `TMPDIR` to
  `ExecutionTempDir(r.ReportDir)` and V1-0624 changes the pinned-file opens; the hunks are disjoint
  and the package builds.
- Analyzer schema pin: V1-0687 bumps `corvint-analyzer/111` to `/112` with a new audit digest
  because `internal/secretscreen` is a pinned input. Main had not moved the pin, the merge was
  clean, and `TestAnalyzerSchemaInputs` passes on the integrated tree, so the digest was not
  regenerated.

## Verification

After each merge: `make -s spec-requirements`, `git add -A`, the doc gates
(`spec-requirements-check requirement-definitions-check traceability-tests-check
decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check
use-case-receipts-check diagnostic-coverage-check`) passed, `go test ./internal/specindex` passed,
no conflict markers remained and `INDEX.json` parsed.

Focused tests after all merges (`GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m`):
all 18 packages passed: `cmd/corvint-test-runner`, `internal/testrunner` and its `dynamic`,
`mobile`, `native`, `platform`, `registry` and `sql` subpackages, `internal/secretscreen`,
`internal/localcompletion`, `internal/trace`, `internal/obscorpus`, `internal/mcp/bridge`,
`internal/cemcandidate`, `cmd/corvint-cem-candidate`, `internal/specindex`,
`internal/contextindex` (holds the analyzer schema pin) and
`internal/liveverify/affected/typescript`. `go vet` on the same packages passed, and `gofmt -l`
over the 21 Go files the batch adds or modifies reported nothing.

`corvint affected --base 2a93b5a1` (Corvint 1.0.0-rc.2, build 360) returned 163 advisory Go
packages, because `internal/secretscreen` and `internal/contextindex` are widely imported, plus the
mandatory repository gate commands. The focused run above covers the packages each lane changed and
their direct consumers named by the lanes.

## NOT_RUN

- The other advisory packages from the affected plan, including `cmd/corvint` (about 10 minutes
  alone on a quiet host), `./...`, `make gate` and `interop/cem01-go`: not run by policy for this
  scoped integration.
- Lane live qualifications (real Mocha, .NET SDK, FIFO process runs, live `dogfood verify`) were not
  repeated; their evidence stays in each lane's build-log entry.

## Rollback

Revert the merge commit of the lane to remove, then rerun `make -s spec-requirements`. Reverting a
lane leaves a gap in the `TRE-V0` numbering; IDs are not reused.
