<!-- SPDX-License-Identifier: Apache-2.0 -->
# Pibrow: Pi host discovery, workflow fencing and coverage evidence

Date: 2026-11-04. Base: `39c03198086681fb24f509fbd00b34696d3268c4`.
Human intent: the supplied lane charter and V1-4662, V1-4663, V1-3624,
V1-4089, V1-4430 and V1-4431 ticket snapshots. Delivery remains experimental;
new requirement rows record technical intent without claiming owner read-back.

## Decisions and ticket disposition

| Ticket | Local disposition | Contract and retained evidence |
|---|---|---|
| V1-4662 | PARTIAL: entry points implemented; hosted criterion unavailable | PWV-V0-8300, PWV-V0-009/010. The normal adapter target discovers both complete Pi suites; absent tools/artifacts skip by name and required Pi qualification fails if prerequisites are missing. Installed failures still fail. The Linux job installs Pi 0.99.1 and source-built native candidates. Its remote run and ci-control-plane admin status are NOT_PRODUCED: this lane cannot publish. The charter's mcp-scope/dependencies test files are absent at BASE; wildcard discovery will include them when their owning slice is integrated. |
| V1-4663 | PARTIAL: present race fixed; later pilane surface absent | PWV-V0-8301/8302 and PWV-V0-004/008/011. Settlement captures identity and generation before await and discards a result cleared between validation and caller resumption. Four transition regressions pass. This BASE has no createFields, workflowRevision or PWV-V0-8100..8104; it refuses create/refine before dispatch. The README records that boundary and the existing acceptance table is contiguous. No missing 8104 rollback or reftable/unborn refusal is invented. The later pilane commit must be integrated before its other P3s can be repaired here. |
| V1-3624 | PARTIAL: live-host criterion NOT_RUN | MCPV0-8300 and MCPV0-950/115/861. The unusable-root startup repair already exists at BASE. A new regression replays the shipped plugin declaration with empty, unexpanded, missing and non-worktree roots, checks initialize/list/call and fixed stderr causes. The unexpanded case passes locally; the complete package is blocked by the sandbox marker described below. Historical build-34 stderr was not retained, so the original incident's exact cause remains NOT_OBSERVED. No additional speculative server behavior or wire change is introduced. |
| V1-4089 | MET locally | The README troubleshooting section names the macOS mcp-logs cache, file discovery/tailing, session timestamps, missing binary/Git, argument/protocol mismatch and old root-fatal builds. Other platform cache paths remain host-specific. |
| V1-4430 | MET locally | LPCV-V0-8300, LPCV-V0-965/973. Browser and Node attribution use one UTF-16 innermost-range painter, retaining outside/unexecuted/executed states and equal-span order. The Node view uses those states directly. Whole v8 and browsercov packages pass; both real-browser E2E tests ran. Existing assertions and goldens are unchanged. |
| V1-4431 | PARTIAL: anonymous collection remains unobserved | LPCV-V0-8301, LPCV-V0-979. Explicitly supplied anonymous scripts refuse attribution; browser/Node anonymous collection remains NOT_OBSERVED. An on-disk map does not legitimize untracked Node output (fixture). A lost renderer records the existing cdp-detached incomplete reason; its pending requests and a closed transport reject instead of waiting indefinitely (real-hook offline fixture). |

## Failed anonymous-collection evaluation and rollback boundary

Enabling anonymous reporting exposed host-generated evaluation as well as application code.
Filtering Playwright's isolated utility world by CDP context made its E2E fixture pass,
but Cypress also evaluates anonymous host code in application contexts. Even frame-based
classification left its frozen completeness assertion failing. Node dumps additionally carry
empty-URL scripts without source/provenance. Missing provenance cannot establish host ownership.
The attempted collection expansion and context filters were removed. The existing browser and
Node collection boundary remains explicit; unconditional anonymous collection would require a
separately accepted completeness/provenance change. This is the frozen-test blocker for V1-4431,
not a successful anonymous-coverage claim. Existing E2E assertions were never weakened.

Raw diagnostic scripts remain outside Git in `/home/russell/lanes/pibrow-evidence/`:
333 files, 582120 bytes. Sorted content-digest/name manifest SHA-256:
`70ee3d0f01e5f1c3b72e724788106a074a3c3a53526bc5f7d37d748bb0ce4769`.
These are diagnostic observations, not a qualification corpus. Rollback: restore the previous
private painters together; revert the additional attribution refusal and CDP pending-request
cleanup to restore the prior collector. The spec retains its unknowns and coverage exclusion
remains unqualified under the pre-existing LPCV promotion gates.

## Verification and limits

- `make host-adapter-node-test`: PASS. OpenCode 55 passed; Pi 142 passed and 18 named skips;
  protected Pi 19 passed and 3 named skips. Pi tests run serially to avoid host process-cleanup
  contention discovered when the newly expanded suite ran concurrently. No existing assertion
  changed. `CORVINT_PI_TASK_WRITES=0` honors the lane's native Tasks write prohibition.
- `script/heavy.sh go test -count=1 -timeout 30m ./internal/linecov/v8`: PASS (0.015 s).
- Same whole-package command for `./internal/browsercov`: PASS (84.360 s), with
  CORVINT_BROWSERCOV_DEPS naming the scratch install of @playwright/test 1.63.0 and Cypress
  15.21.1. XDG_CONFIG_HOME was redirected to writable cache for Cypress. Both existing E2Es ran.
- Same whole-package command for `./internal/mcp/cli`: FAIL (10.767 s), not equivalent to
  passing coverage. Read-only empty `/home/russell/lanes/.git` makes scratch paths appear to
  belong to that worktree: two new root cases and existing profile/unusable-root tests fail.
  A focused unexpanded-project regression passes. The host verifier must rerun the package
  outside those synthetic marker mounts; no test assertion or root validation was loosened.
- Focused v8 and browsercov vet: PASS. Staged spec generation/check and line citation repinning:
  PASS. Citation-only integration updates preserve references moved by the spec/Makefile edits.
- Repository-wide gate, Windows vet (no push), live host/CI, independent review, learning and
  selector promotion campaigns: NOT_RUN. The owner requests scoped checks and owns independent
  review externally. No provider/ranking/learning code changed; no retrieval or learning frozen
  evaluation applies. The LPCV live exclusion promotion gates remain unqualified.

Local raw test logs are retained in `/home/russell/lanes/pibrow.tmp/`; their digests are:

| Log | SHA-256 |
|---|---|
| `adapter-final.log` | `75c8ff89d90857052feefc3ead258ade7b95e3a618e84dfc7d261c44fbe57e35` |
| `v8-final.log` | `1f7ea772746fe08661187fc776c2f2d606c5f668fdacba426639a892c8f645a4` |
| `browser-final2.log` | `538ca09d59cebbc837a1d28bab2e34749c9c4095300faa96f10af1f4d9417a89` |
| `mcp-final.log` | `905062cefa8d50f5c0a667b14b04152118bd13b4ee11d3f566386e6cf7b8adcb` |
| `mcp-focused.log` | `763a71da841b5521992b00fa062c9dedf1cda7d0cdeccccb7ad8ec13dcf9c23b` |

## Self-use and friction

The start dogfood command ran before edits. Directory PATHS refused impact with
`unsupported-impact-path-suffix`; a subsequent file-based `corvint impact` succeeded but does
not retroactively produce that bound slot. Prechange query/context were retained. The first
intent path was corrected from a nonexistent spec to the owning LPCV spec. An unchanged-tree
CEM prepare refusal was expected at start. Final intents are the PWV, MCP and LPCV specs;
AHI remains an inherited constraint. Affected was used before Go tests; the recorded source
plan selected 127 units and retained 85 unknown entries. Only touched Go packages were run
whole, plus relevant native/Node tests. No broad-gate equivalence is claimed.

The configured heavy lock `/home/russell/bench/rt/.locks` is unwritable here; a private lane
lock was used and cannot claim host-wide admission. TMPDIR remained the mandated lane path.
Go 1.27.3 has no test.tmpdir flag; attempted overrides failed and were removed. All writable
scratch roots expose read-only empty .git markers in this sandbox, so relocating within those
roots would not establish an outside-repository fixture. The native Tasks templates originally
resolved relative to npm's package cwd; discovery exposed and repaired that fixture path.
An offline npm install missed cached metadata; the bounded online scratch install succeeded.
Cypress's default ~/.config path was read-only; a writable XDG_CONFIG_HOME resolved that failure.

Final CEM binding/checking follows this implementation commit with local outcome `blocked`
for the retained external criteria and failed MCP package. Independent review and a seal are
NOT_PRODUCED by this lane: the orchestrator owns review and the charter explicitly stops at
check. Ticket lifecycle writes, publication and remote qualification are not authorized here.
