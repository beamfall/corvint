# Verifier repair 1: deterministic fixture identity and shell isolation

Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Repair starts at `ef3ff4af94fddd3ad7c88694f8b9bf5a153fa422` on `claude/mac-dogfix`.
Only the host verifier's two failed selected units and binding failure are in scope.

## Decisions and proof

The context fixture configured `user.name=t` and `user.email=t@x`, but inherited
`GIT_AUTHOR_*` and `GIT_COMMITTER_*` variables. The owner's required seal identity
overrode that configuration, changing the packet's `commit` while its tree and all
other golden bytes stayed the same. Pin author and committer identity alongside
the existing dates in `gitFixture`. `TCP-V0-024` and `TCP-V0-150` remain unchanged.
The init/adopt fixture had the same issue: pin its existing `Corvint Test` /
`corvint@example.test` identity in `newActivationFixture`. The three reported
`CCF-V1-002` init/adopt golden cases then pass with Russell's identity exported.
All exact-byte and structural assertions, committed goldens, wire shapes, codes
and diagnostic text are unchanged. These are fixture repairs; no product contract
or new requirement is needed.

The second failed selected unit, declared shell tests, stopped in the existing
`SRG-V0-923` regression. macOS `/bin/sh` retains a variable assignment before a
function invocation: the earlier private-index case left `GIT_INDEX_FILE` set,
so subsequent worktree checks skipped. Reproduced the exact reported failure,
then contained that one case in a subshell. Every existing assertion still runs,
including the negative unstaged-generated-file assertion and clean-index checks.
The full `make spec-requirements-test` target passes (46.1 seconds).

The host's dogfood failure happened during its impact abstention replay. The
original binding records sandbox-only `unsupported-impact-range`, with error
`checkout Git directory unreadable`. A host read can differ from that result.
Refresh the binding through the existing tree binary, retaining the original
pre-change capture; no new `make dogfood-change` capture, output-change exception,
verifier override, manual artifact editing or seal is authorized. A sandbox PASS
cannot establish cross-environment agreement: host replay still requires its own
fresh binding if the impact result differs.

## Verification and limits

`corvint affected --base` selected `cmd/corvint` through `DIRECT_TEST_CHANGE`;
its overall scope remains UNKNOWN, retaining Go build-constraint, nested-module
and foreign-shell invocation frontiers. Corvint context was used, with omitted
lexical/test rows retained. Installed Core warns that it is stale relative to
this checkout; the checkout binary was built for final binding. Optional update
checks for both components fail the sandbox process-cleanup/version probe;
official release freshness remains UNKNOWN and no executable was switched.
No account usage telemetry is available; costs and live model controls remain
NOT_OBSERVED. The existing session handles this bounded fixture repair without
model or effort changes.

The precise context golden tests and frozen init/adopt cases pass under the
owner's exported identity. The complete original golden command and the broader
frozen-profile command were also rerun: they retain sandbox Git ancestry/revision
read refusals in flow/range cases. No assertions or golden files were changed to
hide those failures. Whole-package and final binding outcomes are recorded in
the lane handback. Focused Go vet and Windows vet pass.

The host selected-run receipt at the starting head reports whole-package PASS
for all four original ticket packages: `internal/dogfoodflow`,
`internal/cem/workflow`, `tools/retrieval-bench`, and `internal/tasks/cli`.
Their source is unchanged in this repair. This supersedes the earlier local
runtime uncertainty for those checks; historical macOS TCC attribution remains
NOT_OBSERVED. Original requirement dispositions otherwise remain unchanged.

Raw evidence and command ledger stay outside the repository under
`/Users/russelllewis/lanes/mac-dogfix-evidence/`. No Tasks write, publication,
model-backed reviewer, lock-directory override or background shell job was used.
Independent review belongs to the orchestrator. Rollback reverts these three
fixture changes and rebinds the CEM; no runtime, store or remote state changes.

## Evidence digests

| Log | rc | SHA-256 | Observation |
| --- | --- | --- | --- |
| `repair1-identity.log` | 0 | `72dbff50c47c056c213477951b6aeddbb11789482c91715bf913839e2ce98d79` | Reported context golden cases pass, 1.090 seconds. |
| `repair1-activation.log` | 0 | `02f1a6c9e24d209e2fdde01fe7105868a53fa9c57be658384e4e171995e9f149` | Reported init/adopt frozen cases pass, 0.866 seconds. |
| `repair1-spec-worktree.log` | 1 | `9c3c57f4050323b1cb913f8a021b49eff3b14f21205e5de23a570f8d0d5be73b` | Exact shell failure reproduced before isolation. |
| `repair1-spec-worktree-fixed.log` | 0 | `a2405a3f81db2d97f2819760b198259bb41e21f7b91a4a74736d5e7432750d35` | Same shell regression passes after isolation. |
| `repair1-spec-suite.log` | 0 | `0285dceedac64b7fc8dc683d57ce880ea153932148dc5e81caae3fdad9dd984d` | Entire spec-requirements-test target passes, 46.1 seconds. |
| `repair1-golden.log` | 1 | `51efc2750a8ec11678c3ae52e66aa7e1ed6bcc5e7471454864eac9bde4884722` | Original verifier golden command rerun; sandbox flow revision reads refuse. |
| `repair1-frozen.log` | 1 | `5660c5221f99dee2e6ce1f1f86fdc3e096a93aafba2a1038b35c887eb7940298` | Extended frozen run retains sandbox range/revision refusals. |
| `repair1-vet-cmd.log` | 0 | `5323d0c5587394337725de6e25c95c4d876455691c996192e7229377a935f57a` | Focused Go vet passes. |
| `repair1-windows-vet.log` | 0 | `fde2107ff45158d1d5a8e400b6a9147be386831f4bc84ebcb7a010cb993a1c62` | Windows vet passes for repository and interop. |
| `repair1-host-selected-receipt.json` | 1 | `fa6bebfb5eed180ada143d1d81210fdaff2382a69963b6a3bc350e524d8d5cae` | Host receipt retains four original ticket whole-package PASS outcomes; overall 100/102 before repair. |
| `repair1-whole-cmd-admitted.log` | 1 | `3e86786ed47222f008891da9f3b9f73fea2c70b99fc81a267c853f6449bf693c` | Whole cmd/corvint, no filter: 772.402 seconds, 214 failing top-level tests; sandbox Git-read/process-supervision/residue refusals retained. |

The required whole `cmd/corvint` run finished with rc 1. No reported context identity regression failed in that run. Representative remaining failures include flow fixture revision reads, frozen committed-range ancestry reads, native adapter process-cleanup failures and work-final-check process residue. Their full diagnostics remain in the external log; no product guard or assertion was bypassed. Host rerun is required before claiming this package green. The initial whole-package admission refusal (missing HEAVY_WHOLE_REASON, rc 2) was followed by the admitted owner-mandated whole-package run, without changing the lock directory or remote host.

## Batch integration (claude/batch-1105a, 2026-11-05)

The docperf lane landed the same fixture identity pins in `gitFixture` and
`newActivationFixture` first, so this change keeps docperf's comments there and adds
no further test-file bytes. Its spec-requirements worktree check isolation lands as written.
