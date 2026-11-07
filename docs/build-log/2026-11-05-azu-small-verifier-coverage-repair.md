# AZU small Verifier repair 1: runtime coverage uncertainty

Date: 2026-11-05
Starting HEAD: `4a23d0a263d24566517d87b472cdd20ade0bb6e0`.
Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Scope: V1-4689 / AZU-V0-006 only. V1-4688 / AZU-V0-005 and V1-4690 / AZU-V0-007
are unchanged. The host verifier found existing Ruby and TypeScript package failures after
shared graph coverage gaps moved from unknown to uncovered.

Decision: preserve the distinction between absence of a static dependency path and proof about
runtime input. A Ruby template without a reachable route-dispatched test keeps the existing
RUBY-AFFECTED-V0-761 NO_SELECTABLE_TEST abstention. Its renderer is unresolved; it is not a
proven coverage gap. Playwright reconciles both shared-graph lists through its existing
TJAA-V0-106/107/121 out-of-suite and unreached-path declaration rules. A static uncovered entry
does not establish that an end-to-end application cannot load the changed file. This repairs
the consumers of AZU-V0-006 without changing their wire contracts, reason codes, errors,
goldens or existing assertions. The owning AZU clause now names both boundaries explicitly.

Acceptance evidence uses the existing whole Ruby, TypeScript and shared affected packages.
The existing Ruby view test checks the exact abstention as well as successful route-dispatch
selection. Existing TypeScript tests check out-of-suite widening, any-language input,
discovery membership, and accepted/refused unreached-path declarations. No assertion or
golden was edited. Rollback: revert this repair's source/spec commit and rebind; that restores
the verifier failures and is not successful qualification.

Self-development routes: reused the original pre-change evidence; ran installed affected
selection before tests, selecting all three packages; used native CEM rebinding and reviewer
reports for structural checking. New pre-change capture, broad selected-test execution,
mutation and browser execution are deferred under the repair scope and lane restrictions.
Independent review remains with the orchestrator. Token usage, cost savings and external
interoperability remain NOT_OBSERVED.

Verification results and raw-evidence digests follow below. All produced logs remain outside
the repository under `/home/russell/lanes/azu-small-evidence/verifier1-current/`. Tests use
Go 1.27.3, the required TMPDIR with mode 700, and script/heavy.sh with its configured shared
lock directory; no lock override was used.

The whole Ruby package passed in 0.745s and the whole TypeScript package passed in 7.939s.
The first shared-package run failed only TestSelectionOnTheLiveDirtyWorktree: median
135.425939ms, calibration probe 20.243608ms, ratio 6.69 against the unchanged 6.67 bound
(100ms absolute budget). All results are retained; host load was high during the round.
This is an observed timing failure, not evidence of its cause. One unchanged-input
whole-package retry was requested; no performance assertion or selector optimization was added.

Friction: the requirement generator and citation checker read the index. The first generator
invocation refused unstaged-spec; staging the three changed files and rerunning regenerated
no metadata and passed the requirements and traceability checks. Citation repinning after
staging found no moved content; line-citations-check passed (180 existing skipped citations).
The build waited its configured 120s host-load admission bound, with no override. Native
shared-graph Darwin files remain unexecuted on this Linux host. Full selected execution is
NOT_RUN in this repair: the supplied host aggregate already failed and the lane permits Go
tests only through heavy.sh one package at a time; the failing packages were rerun whole.
Publication, native Tasks writes, model-backed review and sealing are NOT_RUN by charter.
Guarding-test discovery is NOT_PRODUCED because its runner would bypass heavy.sh. Existing
pre-change chronology, optional OCM links and external qualification limits remain visible in
the prior handback; structural CEM success cannot replace execution or independent review.

The whole shared-package retry failed only TestIncrementalSelectionMeetsTheLiveBudget
(249.073s package runtime). Its retained worst round has p95=256.659599ms and probe
p95=47.269198ms, ratio 5.43 against the unchanged 5.00 bound. The earlier live-worktree
check passed on this retry. No functional failure was reported in either shared-package
run. These two failed evaluations remain visible; no additional retry or out-of-scope
performance change is made. Shared-package timing qualification remains PARTIAL pending
host-side rerun. Both host-reported language-package regressions are repaired. The broader
selected aggregate has not been rerun, so the explicit local outcome remains failed.
The optional real Ruby repository and native Mocha qualifications remain skipped: their
explicit repository/tool/evidence environment inputs are absent.

Retained package and selection evidence (SHA-256):

| File | Bytes | SHA-256 |
|---|---:|---|
| `test-ruby-whole.log` | 857 | `e9ca7c1a78753d84719af610dd0cca8f3edfa88bfead915496fbb92902218930` |
| `test-typescript-whole.log` | 864 | `3482fab5bcfda3e2576aa84c74acd5da96c869979e182c650fc6aa6df6123bff` |
| `test-affected-whole.log` | 1592 | `e5dffaef0ea0c833a03427daa37a4ef83ca642e0314556a8f4602686273a9896` |
| `test-affected-whole-retry.log` | 2161 | `6d6439961b9f8e6007820d6127aab842caa9b214975a8b7dbf689f25ba6d2c98` |
| `affected-installed.json` | 143829 | `4e52a20ef00b91242f6ceb1d8314de11c93d360927812aedbc834cde3ec0d989` |
| `affected-current.json` | 92414 | `b1d8132e3ccc52323291a1f0c3d2498fb38e7d5e0735938354f8b73f962d39a6` |
| `scoped-docs-staged.log` | 148 | `7d6bb5e3b394557bd55cfa8f2082e943eec85216abc2522e6fb937d490273049` |

Full make doc-gates passed, including use-case conformance (valid=true, 22 use cases;
1 VERIFIED and 21 UNPROVEN claims). The frozen selection baseline passed with 33/43 narrow
merged cases and zero misses over 26 failing packages from 15 failed CI runs. Its gate took
319s on this contended host; the final conformance step queued 183s for a slot. These are raw
run observations, not performance claims. doc-gates.log: 23819 bytes, SHA-256
`ad5b1c009e0d4bd26d43845cebcd363581b142ee235557e910994ec8803ff7f2`.
