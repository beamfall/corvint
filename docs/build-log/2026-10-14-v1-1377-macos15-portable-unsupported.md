# 2026-10-14 V1-1377: hosted macos-15 portable consumer `UNSUPPORTED` in the cem-verdict gate

Lane V1-1377 (BUG, P1). Requirements `TCB-V0-040..044` (`docs/specs/tasks-criterion-binding-v0.md`)
and `CEM-V1-185..189` (`docs/specs/cem-stable-v1.md`), accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). Base `4c1ca0d0f94ec36e2ac141c5515742a1fb23a758`
(origin/main after PR #878).

## Intent

Release-gates nightly run 38559681509, job 115837380242 (`macos-15`, full-gate, head 8803b7ddc,
Go 1.27.2) failed internal/taskman `TestTCBV0010..0013/0017/0018` and cmd/corvint
`TestTCBV0016GateVerdictRunsBothConsumers` with `CEM_VERDICT_VERIFIER_DISAGREEMENT`: native
`ACCEPT`, portable `UNSUPPORTED`. The retained job log (`agent-1377/job.log`) carried only the gate
document, which discarded the consumer's `stage` and `code`, so the cause could not be read from
it. The ticket's AC1 requires the cause from retained diagnostic output, AC2 a passing hosted
macos-15 run, and AC3 a truthful refusal that names an unsupported portable route.

## Decisions

- Pin the consumer's answer. `cemPortablePin` gains `stage` and `code` (members of the
  `stage` and `code` enumerations of `protocol/cem-1.0/stable/RESULT.schema.json`, held as two
  sets a test pins to the schema; `code` `null` on `ACCEPT`); anything else is an unreadable
  result (`portable-consumer-failed`, no pin). No host path or free text enters the document
  (TCB-V0-040, CEM-V1-187).
- Refuse an unsupported route by name. Portable `UNSUPPORTED` against a native non-`UNSUPPORTED`
  refuses `CEM_VERDICT_UNVERIFIABLE` / `portable-route-unsupported` with both legs pinned. This
  supersedes the TCB-V0-031 clause that kept `CEM_VERDICT_VERIFIER_DISAGREEMENT` for that case;
  it is still a refusal, never a grant, and `verifier-disagreement` remains for every other
  mismatch (TCB-V0-041). The consumer's closed code set lives in `protocol/cem-1.0` and is
  untouched; the consumer already reports `git-diff-failed` at `repository` (its canonical diff
  child exited non-zero), which is the reason, so `interop/cem01-go` is unchanged. The lane's
  first draft expected `git-read-failed` by inference; a git shim that rejects `--attr-source`
  showed both the real consumer and the native stand-in answer `git-diff-failed`, and the tests
  now reproduce the host condition through that shim instead of a `sed`-patched result.
- Diagnose the host, not the hypothesis. The charter's hypothesis (process containment after
  V1-1358) was wrong. A bounded probe (`bareHostGitDiagnostic`: `command -v git`,
  `git --version`, one `git --attr-source=HEAD diff HEAD HEAD` under `PATH=/usr/bin:/bin`) is
  retained in the TCB-V0-016 failure message, and `TestTCBV0042PortableConsumerUnderBareHostPath`
  runs the real consumer under that fixed PATH and accepts only the answer the probe explains
  (TCB-V0-042, CEM-V1-189). Because the CI test run is non-verbose, a passing run retains no
  log, so the lane dispatched release-gates twice: once at the diagnostic commit (expected to
  fail with the cause retained) and once at the fix.
- Fix the cause in the gate. Go's `exec.Command("git")` resolves through the gate process's
  PATH, so the native leg and every gate read used the runner's newer Git while the consumer,
  given `PATH=/usr/bin:/bin`, resolved Apple Git 2.39.5, which lacks `--attr-source` (Git 2.41).
  `portablePath()` now puts the directory of the gate-resolved `git` ahead of `/usr/bin:/bin`
  (fixed path alone when unresolved, relative or already under it), so both legs run one Git.
  The directory never enters the document; `runRead` keeps the fixed PATH (TCB-V0-043,
  CEM-V1-186).
- Rejected: probing `git --version` and refusing below 2.41 (a version string is not a
  capability proof); adding a new consumer code (needs a packet change); making the consumer
  search Homebrew or Xcode paths (an unbounded search for an executable it then trusts).

## Verification

Local host darwin/arm64, go1.27.2 (`GOTOOLCHAIN=local`), Apple Git 2.54.0 at `/usr/bin/git`
(`--attr-source` accepted), shared host at load about 20.

- `GOTOOLCHAIN=local go test -count=1 -race ./internal/taskman -run 'TestTCBV00'`: ok (104s).
- `GOTOOLCHAIN=local go test -count=1 -race ./cmd/corvint -run 'TestTCBV0016|TestTCBV0042'`:
  ok (9s); native and portable `verdictSha256`
  `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476`, TCB-V0-042 `GRANT`.
- `go vet ./internal/taskman ./cmd/corvint`: clean. `gofmt -l cmd internal tools
  .github/cishards .github/testconfine`: empty.
- `corvint affected --base 4c1ca0d0f` at lane start: `PLAN_ONLY` / `EMPTY_SELECTION` (no dirty
  paths yet); `corvint context` named the gate files and tests that were then edited.
- Doc gates (12, after staging; `spec-requirements-check requirement-definitions-check
  traceability-tests-check decision-numbers-check line-citations-check eol-policy-check
  license-headers-check ci-least-privilege-check unbounded-readers-check test-commands-check
  error-code-ownership-check use-case-receipts-check`): all rc 0, rerun after every docs edit.

Hosted macos-15 evidence (`release-gates.yml` `workflow_dispatch`, inputs `sha` and the
label-only `release`; the workflow pins, commits and publishes nothing):

- Run 1, 38650914390 at 22fca01e63472971d367ce25519d5c8c5b592e2f (diagnostics and the named
  refusal, no PATH fix): macos-15 full-gate job 116149186818 `failure`, as intended, with the
  cause retained (AC1). cmd/corvint `TestTCBV0016GateVerdictRunsBothConsumers` and
  `TestTCBV0042PortableConsumerUnderBareHostPath` failed at 23:31Z with the probe output
  `/usr/bin/git`, `git version 2.39.5 (Apple Git-154)`, `attr-source: rejected exit 129`,
  `unknown option: --attr-source=HEAD`, and the gate document
  `"portable":{"citationSha256":null,"code":"git-diff-failed","outcome":"UNSUPPORTED","route":"verify-stable-canonical","stage":"repository",...}`
  under `CEM_VERDICT_UNVERIFIABLE` (that commit still asserted `git-read-failed`, which the
  retained document corrected to `git-diff-failed`). internal/taskman `TestTCBV0010..0013`,
  `0017` (both), `0018`, `0020`, `0023` and `0040` failed at 00:10Z the same way (every
  fixture `CEM_VERDICT_UNVERIFIABLE`, portable `repository` / `git-diff-failed`). The
  ubuntu-24.04 full-gate job 116149186944 failed only `error-code-ownership-check`
  (`portable-route-unsupported` was not yet named by a spec at that commit; the lane's spec
  sections name it). linux-core, interop-gate, companion-release and focused-docs succeeded.
  Retained log: `agent-1377/run1.log` (scratchpad).
- Run 2, 38654510601 at 52ad86a368ae06520617b536b12f22d38102b362 (the PATH fix; the gate code is
  identical to the PR head, later commits touch docs and `.corvint/` only): every job `success`
  (AC2). macos-15 full-gate job 116160979035: `ok github.com/Beamfall/corvint/cmd/corvint
  1015.682s`, `ok github.com/Beamfall/corvint/internal/taskman 28.989s`, no `FAIL`; the runner's
  PATH git is `/opt/homebrew/bin/git` 2.55.0 (from the checkout step), its `/usr/bin/git` is
  2.39.5, so the fix ran the consumer on the gate's 2.55.0 and both legs agreed. ubuntu-24.04
  full-gate, interop-gate, companion-release, focused-docs and linux-core `success`. Retained
  log: `agent-1377/run2.log` (scratchpad). Both permitted dispatches are used.

## Codex review

One read-only round (`codex exec -s read-only`, `agent-1377/codex.log`), verdict `REJECT`
with four findings, all addressed before the hosted run 2:

1. Blocking: the unsupported-route branch pinned a consumer document whose `verdictSha256`
   claim was never compared with the gate's recomputed digest, so a consumer could name
   `UNSUPPORTED` against a foreign verdict. Fixed: the named refusal requires the claim to
   equal the recomputed digest; a false claim falls through to the comparison and refuses
   `verifier-disagreement` (test case in `TestTCBV0040PortableUnsupportedRefusesNamingTheRoute`).
2. Should-fix: `closedToken` accepted any lowercase word (for example `private-hostname`) as a
   stage or code. Fixed: `portableStages` and `portableCodes` are the `RESULT.schema.json`
   enumerations, pinned by `TestTCBV0040PortablePinSetsMatchTheResultSchema` (read scope added).
3. Should-fix: TCB-V0-041 said `UNSUPPORTED` exits 2 while the gate accepts 1 or 2. Fixed in the
   spec text: a non-zero exit under TCB-V0-017.
4. Blocking: the hosted run 2 placeholders were unfilled at review time. Resolved by the run 2
   evidence recorded above.

## Failure modes

A host whose fixed-path Git lacks `--attr-source` on both legs refuses through the native leg's
`git-diff-failed`; a host where only the portable leg loses it refuses
`portable-route-unsupported`; a consumer that mis-states `stage` or `code` is unreadable. No path
grants. A caller that puts an untrusted directory first in the consumer's PATH runs that Git; the
gate only ever puts first the directory of the Git it already trusted for its native reads.

## Rollback

Revert the V1-1377 commits. The document loses `stage` and `code`, an unsupported portable
route refuses `verifier-disagreement` again, the consumer runs under `/usr/bin:/bin` and the
hosted macos-15 gate tests fail again. No packet, committed map, seal, Tasks schema, policy entry
or gate digest changes.

## NOT_PRODUCED and limits

- `make gate` and the exhaustive `go test ./...`: `NOT_RUN` (charter; focused `-race` tests, vet,
  gofmt and the doc gates ran).
- `interop/cem01-go` tests: `NOT_RUN` locally (module unchanged by this lane).
- The hosted runner's Git versions are read from run 1's retained failure output only; no claim
  is made about other GitHub images.
- A later nightly on a main containing d3be74b06 was not re-run by this lane; run 1 at commit
  22fca01e6 (which contains it) reproduced the failure with the cause retained.
- CEM bind/seal (cem/1.0, `make dogfood-change` / `dogfood-check` / `dogfood-seal` against
  base 4c1ca0d0f): bound in the commit after this one and sealed in the next;
  `dogfood-check: PASS`, `cem=accepted hunks=27 supported=27 unknown=0 mechanical=0`,
  `ocm=ready-for-review requirements=2 linked=2 unlinked=0`. `prechange-query` and
  `prechange-impact` `NOT_OBSERVED` (`agent-receipt-absent`: no agent receipt was recorded for
  the pre-change reads).
