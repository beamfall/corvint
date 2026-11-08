# V1-5094: one qualified CI and local Go selection contract

BASE = `9761ea1f459c0d3b3c963953e24a76eaeda16370`.
Lane: selcon. Requirements: AFP-V0-9500 and AFP-V0-9501, experimental under the
owner's standing approval. The owner requested shared selection, fail-closed proof,
a corpus parity gate, and no `.github` edits in this lane.

## Decision and limits

Both execution consumers now call `gateselect.SelectQualified` (the local caller can
reuse its structural index). Conservative attribution remains the starting point.
The existing Go planner re-observes the tested tree and recomputes additive and
import-set evidence against the immutable base. A graph-bound exclusion or an observed
unit with no tests can remove an attribution; receipt-selected packages and fresh
positive witnesses are retained as a union. This replaces the local runner's unconditional
projection back to the received package list. It does not intersect disagreeing sets.
The legacy `Select` and `SelectStructural` APIs remain for their existing consumers and
assertions; the two executable selection paths use the qualified contract.

The extra observation costs work; no planning-latency improvement is claimed. Reusing the
full caller graph would need a separately bound cross-process representation, so this
slice chooses recomputation over a new receipt shape. A partial Go observation retains
Go test holders of declared runtime targets it did not observe, naming the unresolved
edge. Supplied additive exclusions cannot certify themselves. Unsupported scopes,
malformed paths, unknown frontiers, capped scans and unreadable observations widen with
an existing FALLBACK verdict. No frozen wire, error code, exit code, stderr, golden or
core-freeze output was edited.

`selection-contract-check` is registered in `doc-gates`. Its script builds both actual
consumers and replays every range in `benchmarks/selection-baseline/changes.json` with
the baseline result cache disabled. It compares scope as well as exact root-Go sets;
FULL is not treated as an empty successful selection. The explicit profile differences
are slicing, placement/order, non-Go adapters, and nested-module suites outside the root
shards. The workflow's independent `.github` guard is not a package-selection rule.
The script does not waive divergence because one set contains the other.

## Evidence

The existing audit `2026-11-06-selection-audit.md` reports driver-minus-shadow differences
of 47–133 packages and 17,256 package-seconds across nine narrowed CI runs. Those are
historical audit numbers, not a measurement at this lane's BASE and not a predicted saving.

| Measurement | Before at lane BASE | After this change |
|---|---|---|
| Selection-baseline driver-minus-shadow package counts | NOT_RUN | NOT_RUN |
| Selection-baseline driver-minus-shadow package-seconds | NOT_RUN | NOT_RUN |
| CI job/package elapsed savings | NOT_PRODUCED | NOT_PRODUCED |

The round explicitly prohibits selection-baseline-check, corvint test, doc-gates and
whole-repository selection. Consequently corpus qualification belongs to the host verifier;
zero divergence or elapsed savings is not claimed from unit fixtures. The parity checker
reports zero package-set difference only after comparing an actual successful replay.
Changing trusted CI pins and the forward-row equality guard is a separate control-plane
PR, and remains required before deployment. The handback supplies the proposed diff.

Focused evidence exercises fresh selection, union of positive obligations, refusal of
forged additive claims, immutable-base additive narrowing and its invalidation, malformed
and capped inputs, and retention of an unobserved runtime reader. Replay comparison tests
accept explicit execution differences and reject extra/omitted packages, scope drift,
unknown verdicts, invalid units, and a NOTHING manifest carrying Go work. Existing assertions
and goldens are unchanged. The first whole gateselect test run failed because the new
fixture retained the newline from `git rev-parse`; trimming that fixture input repaired
its immutable-base proof without changing the assertion.

Verification results and evidence digests are recorded below. Raw command
logs and the affected receipt stay in `/home/russell/lanes/selcon-evidence/`.

## Self-use, rollout and rollback

The start `make dogfood-change BASE=...` collected query and impact context. With no diff
yet, CEM preparation/citation, OCM aggregation, CEM status and local outcome were
NOT_PRODUCED (`no-diff-yet`); query packet coverage was NOT_PRODUCED
(`packet-coverage-unreadable`). The affected receipt selected all changed Go packages.
No task-store write, independent model review, push, PR, full gate or seal is performed
in this lane; independent review and host qualification remain orchestration work.

Rollback restores the executable callers' legacy selector/projection paths and removes
the new doc-gate target, with both trusted CI pins rolled back together after deployment.
There is no data migration. Corpus replay, independent review and the separate CI
control-plane change remain promotion requirements; the ticket is not reported completed
in the task store.

## Focused verification result

The existing whole packages `internal/gateselect`, `internal/liveverify/affected`,
`tools/gate-affected-select` and `tools/selection-baseline` passed through `script/heavy.sh`
with `GOTOOLCHAIN=local go test -count=1 -timeout 30m`, one package per command.
Focused `go vet` passed for all five changed packages. `gofmt -l` returned no paths,
`sh -n script/selection-contract-check.sh` passed, and
`make -s spec-requirements-check line-citations-check` passed. The toolchain was go1.27.3.
Repinning after staging found no unchanged citation content that needed moving.

The existing whole `cmd/corvint` suite failed (rc 1); the final whole run took 306.012
package-test seconds and reported 45 top-level failing tests. This is test-run
elapsed evidence, not CI package-cost or selection-savings evidence. Representative
failures are `TestAffectedRejectsNonRepositoryAndExtraArguments` (scratch is discovered
inside the surrounding lanes repository) and `TestWorkRunParentOverride` (the sandbox
exposes `/home` with UID 65534, so the private-parent ownership check refuses it).
Both controls fail identically on a clean detached clone at BASE, rc 1. A final run with
`GIT_CEILING_DIRECTORIES` at the required scratch root did not make the suite pass.
Other failures and their diagnostics remain in the raw log; the controls do not prove
that every failing test is unrelated. No failed assertion, stderr contract or golden was
changed. The final empty-input scope validation was additionally covered by the whole
`gateselect` and selector executable suites; host requalification remains required.

Every test invocation used the heavy wrapper without lock-directory overrides and without
exported commit identity. Full repository gates, corpus replay, Windows-wide vet and
independent review were not run under the round restrictions. The local dogfood outcome
is `failed`, preserving the failed package suite rather than treating structural binding
as a passing behavioral gate.

| External evidence file | rc | SHA-256 |
|---|---:|---|
| `test-gateselect-final.log` | 0 | `91186780dfd9813a16b691697712e6a3bfacfa7ffc603d49e0cd635ae87144b2` |
| `test-affected.log` | 0 | `2ca77d097afea7e74d91af9ea5b48211ff6c7223200ae07f91d56951534cecb6` |
| `test-selector-final.log` | 0 | `00de8b5be38c46cef16afaf6d42bb93a1c3576460e614bd5c2750da02a06f498` |
| `test-baseline-final.log` | 0 | `8b2637f1d8a6ea2888eeb0642f3c0ac2284475b0e930b4642efd3c90f39446f8` |
| `test-corvint-final.log` | 1 | `42907b0e410c1177f0a11686f387629c9212f02c13533f3656ed19aab31e18ea` |
| `test-environment-base.log` | 1 | `ab1931c26d12a63d4d8efb7c7bdb961fe566b71823a914c203c8330cc4295309` |
| `vet-gateselect-final.log` | 0 | `64d6194e89ef3c8d68e91c32970dec7a870669c45c696404629e40f1855b028a` |
| `vet-affected.log` | 0 | `60fed58ae14bfe57b5b8c3e28a66b1cfa3fdd0ce80b105e041cad86c7da3d2fc` |
| `vet-selector.log` | 0 | `444934092e9f0fd6368400ca4b60318582c07474deea04aed5adf1264e28d17d` |
| `vet-baseline-final.log` | 0 | `6bad30c17417fd94e2adb740dbb8ecb1890d2c83e5517f8b51ad551ad3ac5f6c` |
| `vet-corvint.log` | 0 | `d210d6de6946fc2bffe834a5ad7218f9001aaa4bb77a7ef86d634799593e1241` |
| `affected.json` | 0 | `34a24e917de328b1c8293da8c7b0d14e71882518538a23b52112f9cf74b913f2` |
| `spec-line-check.log` | 0 | `1975b118d037c6998136e882b3319173ad94faae3fda1cb0fb6abc50b96e115b` |
