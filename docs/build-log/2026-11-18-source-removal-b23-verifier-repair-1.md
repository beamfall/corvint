# Source removal B2/B3: verifier repair 1

Ticket: V1-5694. Repair parent: `097824447063c94918ea5b3d96950ee69dadbca3`.

RMV-V1-008/010 removed test-quality commands and runner-protocol packages, but 141
traceability citations in 11 active specs still presented 116 deleted test functions as live
witnesses. Every name was checked against a deleted declaration in the base-to-parent diff
from `9f4d5d89ab0648425db40e617f3d890598f9ff79`. Mark those citations `(removed)` under
TTG-V0-952; retain historical names and surviving witnesses. The 24 affected rows with no
surviving test name explicitly state why they are untested under TTG-V0-963.

UCV0-004/005: use the existing receipt repinner to refresh the two implementation receipts
for UC-TASK-ORIENTATION and UC-CHANGE-CONSEQUENCE, and their ledger digests. Their changed
subject is `cmd/corvint/main.go`, pinned at the repair parent. No receipt shape, claim,
attestation, result, or other subject digest changes. This is the host-requested pin repair;
no frozen benchmark receipt, golden, runtime contract, or test assertion changes.

Failure class: stale evidence references after authorized source removal, not an analysis
soundness gap. No form allowlist or unknown-form regression applies. The traceability checker
is unchanged: unresolved names still fail, and a removed marker on a declared test fails.
Receipt pins still require exact subject and receipt bytes.

## Verification and limits

All commands use `TMPDIR=/home/russell/lanes/sr-b23.tmp` without commit identity exports.
The affected plan was captured before tests with the retained current binary:
`corvint-final affected --base 9f4d5d89ab0648425db40e617f3d890598f9ff79 --format json`.
It selected both `conformance/use-cases-v0` and `internal/specindex`. Scope remains UNKNOWN:
447 selected, 248 excluded, 105 unknowns, eight unresolved packages. Unknown reasons:
55 LANGUAGE_FRONTIER, one NO_SELECTABLE_TEST, five UNINDEXED_SOURCE_PATH,
42 UNOWNED_DIRTY_PATH and two UNSELECTED_BINARY_CONSUMER. Broad fallback is NOT_RUN.
The 418303-byte raw plan is outside Git at `~/lanes/sr-b23-evidence/repair1-affected.json`;
SHA-256 `abd491799de0ac4be748713999b59d9c4d3637adea035055ccf3debcdf0437c0`.
The removed-declaration audit is retained at `~/lanes/sr-b23.tmp/repair1-removed-tests.json`;
SHA-256 `b85ed9a2165390254c1ec16fb5f5810818a97eb3e202b5615d421f0128c6717b`.

- `make -s traceability-tests-check use-case-receipts-check`: PASS; 1001 removed markers,
  one planned test, 129 retired specs, two subtest citations; all 11 receipts current.
- `script/check-line-citations.sh --repin`: PASS, no moved citations; 184 basename-only skips.
- `make -s spec-requirements-check line-citations-check`: PASS; the existing whole
  `internal/specindex` package passed through `script/heavy.sh` in 0.993 seconds.
- `script/heavy.sh go test -count=1 -timeout 30m ./conformance/use-cases-v0`: PASS;
  existing whole package, no name filter, 0.210 seconds.
- Local Go version is `go1.27.4`; staged whitespace check passes.

Heavy semaphore admission took 59.1 seconds for the spec check and 57.1 seconds for the
use-case tests; both used the configured lock directory without overrides.
No Go file, module file, checker, or test script changed in this repair. Broad gates,
selection-baseline-check, live platform qualification and independent review remain host-owned.
Prior code-test evidence and its limits are unchanged; this round adds no runtime edit.

CEM binding is NOT_PRODUCED: decision 0594 and `docs/DOGFOOD.md` retire the command families,
make targets, scripts and bind/check/seal loop. No current map exists to bind. The final
citation, verify and intents files remain in `~/lanes/sr-b23.tmp/`; the citation plan records
this absence. No capture, seal, push, PR or task-store write is authorized in this round.
The orchestrator must resolve the obsolete finish requirement; no passing check is claimed.

Rollback: revert this repair commit to restore the previous citations and receipt pins.
