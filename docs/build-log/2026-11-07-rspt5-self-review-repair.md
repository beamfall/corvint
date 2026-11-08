# Go runner and implicit interpreter self-review repair

V1-5107: RSP-V0-002, RSP-V0-003, RSP-V0-006 and RSP-V0-036.
V1-5106: RSP-V0-007, RSP-V0-021 and RSP-V0-022.
Repair base: `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.
The three supplied findings require preservation of completed failures, implicit interpreter
reads for outside executables, and configuration identity stable across checkout revisions.

## Decisions and closed forms

A damaged native report must retain every successfully decoded terminal result before its
failure point. The decoder admits only start, run, pause, cont, output, pass, skip and fail,
for the configured package before its package terminal event. Unknown actions, malformed
frames, invalid package/order, truncation and byte/event caps still return the existing errors.
The parser returns the known results with that error; a skip without a reason contributes no
terminal result and preserves other completed results. No malformed or unknown suffix can
certify the report complete. A failed test remains failed after a passing FULL retry.
`TestRSPV0003GoDamagedReportRetainsFailureAcrossRetry` covers the reported truncation, malformed
JSON, an unknown action, a reasonless skip and a capped suffix, including the actual session
reconciliation and passing retry.

Executable location cannot prove the absence of implicit repository inputs. Every executable
is inspected under the closed ELF64, little-endian, x86-64, EXEC/DYN allowlist. PT_INTERP must
be a bounded, single NUL-terminated absolute path with resolvable ordinary filesystem links;
unsupported executable/path forms mark the trace incomplete using existing reasons. The
interpreter target and each repository symlink dependency are retained even when the executable
and initial loader alias are under a declared outside toolchain root. Ordinary outside symlinks
use resolved-path classification, consistent with ordinary reads; repository symlinks retain
their own identities. This was necessary because the first complete package run exposed the
system `/lib64` alias into the declared `/usr` toolchain as an undeclared read. Existing tests
and assertions were preserved; the complete package passed after the classification repair.
`TestRSPV0021OutsideExecutableRepositoryInterpreter` changes only the repository interpreter
and observes exit 7 then 9 while retaining the target and both repository links.
`TestRSPV0022OutsideUnknownExecutableFormatIncomplete` rejects unknown bytes and scripts in
outside roots, proving the default cannot claim completeness.

The Go configuration digest clears Checkout in a private copy before hashing. InventoryKey
still carries the original checkout and rejects cross-checkout capture binding. All execution
configuration fields remain hashed. `TestRSPV0036GoCheckoutDoesNotChangeConfigurationDigest`
pins both properties; existing configuration-change tests retain their assertions. This
changes no JSON fields, public shapes, error codes, exit codes, stderr strings or goldens.

## Verification and limits

The required affected plan at the repair base selects all four lane packages at tier 0:
`internal/testrunner/golang`, `internal/proctrace`, `internal/linecov`, and
`internal/liveverify/mutate`. Its installed-binary stale-source notice and unknowns remain
visible in the external receipt; the plan does not claim complete repository verification.
Whole-package commands run through `script/heavy.sh`, one package per command, without test
filters or exported commit identity. The existing Go 1.27.3 toolchain check passes.
The staged citation repin finds no moved citation; the spec and line-citation checks pass.
No existing assertion or golden was weakened or changed.

Independent review, Darwin execution, live runner qualification, full repository gates,
Windows vet, publication, sealing and native task completion are NOT_RUN/NOT_PRODUCED under
this repair charter. Pre-change context is reused; there is no new pre-change capture.
Qualification is still absent; these fixes do not enable narrowing or resolve the previously
retained Ginkgo inventory integration question. Host qualification and ticket completion
remain outside this round. Raw receipts live under `/home/russell/lanes/rspt5-evidence/`.

Rollback: revert this repair's parser, digest and executable/link-walk changes together with
its new tests and traceability rows. No store migration or wire-format change is involved.

## Retained verification digests

| External receipt | Result | SHA-256 |
|---|---|---|
| `repair-affected.txt` | exit 0; tier 0 includes all four tested packages; stale-source notice retained | `e718ef735ab2b17d9d3a528db599a5c9608980783275e67c5f424e8342d96bf6` |
| `repair-golang-test.log` | ok   github.com/Beamfall/corvint/internal/testrunner/golang 0.089s | `32b08510c311848088b8fedac5c6226c423c06272b6eaf75ac46f12c07726ac4` |
| `repair-proctrace-test.log` | ok   github.com/Beamfall/corvint/internal/proctrace 3.557s | `8ab926e697b2222f1723a17b901f164463b21de28bee187d229fe56d29aabae0` |
| `repair-linecov-test.log` | ok   github.com/Beamfall/corvint/internal/linecov 26.417s | `1a1e1f66f74960b9b960baeffed07538ac85c0b340edb2d17e5d9353d7664a3a` |
| `repair-mutate-test.log` | ok   github.com/Beamfall/corvint/internal/liveverify/mutate 89.835s | `e88fa2cb747f3557a08b471fea25217546f33ea5b2261da57820cdbb8c1b6488` |
| `repair-repin-staged.log` | exit 0 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `repair-spec-citations.log` | exit 0 | `2acbda26237de996980fba77a3786887943e95d1745a65a10723b9764ce515f0` |
