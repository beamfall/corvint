# Native replay import boundary repair

Date: 2026-10-02. Issue #395; V1-0542. Experimental refusal slice only.

PR #489 CI tested merge `9ed78ba20083ef00e6e2f0c16fd1ebf57677d165`
(tree `4cf2208d3655a937dd94127ed89526e4d4abd2aa`) and failed
`internal/tasks.TestImportDirection`: native replay imported Tasks' safe opening
implementation, contrary to decision 0397. The prior two-package dependency
comparison did not validate this repository-wide import-direction obligation.

Native replay now owns package-private file-opening helpers derived from the
exact Tasks implementation at `29a6db884ed795f7694c316433896d190e1ab508`.
The seven call sites retain their arguments and caller checks. Component-wise
atomic no-follow opens, nonblocking special-file handling, descriptor lifetime,
bridge identity checks, Linux traversal-only ancestor handles, exclusive writes,
unsupported-platform refusal and error labels remain equivalent. The existing
Core observer helper is read-only; the CEM publisher resolves root symlinks, so
neither is an equivalent drop-in. Tasks and the boundary test remain unchanged.
The helper files retain derivation and AGPL-3.0-or-later notices.

PMR-V1-001 regression evidence includes deterministic directory/file swaps,
FIFO refusal, pinned-root and descriptor lifetime, bridge failure, and exclusive
creation against existing files, hardlinks and symlinks. Existing native tests
retain identity aliases, redaction, strict inputs and evidence rechecks. Linux
traversal-only ancestor tests are retained for Linux execution.

Focused local verification on Go 1.27.1, Darwin arm64 passed:

- `go test -count=1 -timeout 30m ./internal/postmergeworkflow ./cmd/corvint-postmerge-workflow ./internal/tasks`
  (workflow 49.495s; import boundary package 0.414s; command has no test files,
  with CLI execution exercised by workflow tests).
- `go vet ./internal/postmergeworkflow ./cmd/corvint-postmerge-workflow`.
- Production source parity after private identifier renames and removal of the
  unused supported-platform constant; actual two-package closure contains 148
  packages and no Tasks package.

An earlier sandboxed legacy run returned `adapter-process-failed`; its failed
output is retained alongside the successful outside-sandbox run in
`/private/tmp/corvint-395-preflight-20261002/repair2-actual/`. No source change was
needed for that rerun. The earlier focused native/security/boundary run passed.
Corvint affected evidence against the unchanged workflow base retains language,
build-variant and unowned-path unknowns. The full repository gate was not run
under the owner's scoped-check policy.

Independent bounded Repair 2 review passed with no actionable P1/P2 findings.
The replacement reviewer independently checked donor parity, all seven call sites,
all eight frozen source hashes and 36 retained artifact hashes, and read the actual
raw test evidence without rerunning tests or changing source. The report is
`/private/tmp/corvint-395-repair2-independent-source-review.md`, SHA-256
`97cad6bf66c5b1fcadf2e7b0febf833b913953857bb3ebe055398ad223680094`.
New CEM binding, same-enrollment terminal checks, Linux hosted execution and
publication remain pending at this source commit.
Original workflow base, key and frozen plan remain unchanged. The full positive
workflow and its owner decision remain pending; this repair grants no new
workflow qualification or ticket completion. Rollback is reverting this bounded
helper/call-site change, which restores the known import-boundary failure and
therefore cannot qualify the prior PR for merge.
