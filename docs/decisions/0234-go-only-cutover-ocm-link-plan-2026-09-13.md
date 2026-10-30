# Decision 0234 — the go-only cutover OCM links are a committed replay plan, not committed map state

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

## Problem

`docs/specs/go-only-cutover-v0.md` once said the cutover OCM scope linked eight changed obligations.
In a clean clone at `573bd72b` (base `9ca27f9a`) `ocm prepare` links 0 of 9: the links had only ever
existed in private `.corvint/change.ocm.*.json` maps, which are gitignored.

## The call

1. No OCM map is committed. The map binds the full target commit (`OCM-V0-002`) and is a local
   reviewer artifact outside the mapped patch (`OCM-V0-009`, `docs/DOGFOOD.md` §5). A map committed
   inside its own target cannot name that target, and a later commit moves the target.
2. The committed input is the link plan below. Each row names the requirement-labelled `t.Run`
   case that exists at `573bd72b` (`OCM-V0-005`; decision 0029) and the single supported CEM hunk
   whose target lines contain that case. Replaying the plan onto a freshly prepared map reproduces the links.
3. `GOC-V0-007` is not linked. It keeps an unchanged executable-selection policy and has no
   requirement-labelled claim. It stays `unassessed`, which already means "not assessed by this
   change" (decision 0029), so no `ocm mark` is added.
4. A linked row is structural traceability only. It does not show adequacy or a passing test
   (`OCM-V0-008`).

## Link plan at target `573bd72b02e7205483b9b342f32e9ab8bc1ec8b1`

Each row runs `corvint --root . ocm link --map .corvint/change.ocm.001.json --obligation ID --hunk
HUNK --test-path PATH --claim SELECTOR --expected-base 9ca27f9a62a2add263ff559fd711feea5cdfd93d
--target 573bd72b02e7205483b9b342f32e9ab8bc1ec8b1`.

| ID | PATH (anchor line at target) | HUNK | SELECTOR |
|---|---|---|---|
| GOC-V0-001 | `cmd/corvint/go_only_cutover_test.go` (18) | `hunk:sha256:09bbacea9c58817c46b007f2d34a77a4485d8356df654c62363316de0c82a711` | `test:TestGoOnlySourceAndVersion/case:goc-v0-retired-engine-and-native-version` |
| GOC-V0-002 | `conformance/cli-parity-v0/runner_test.go` (806) | `hunk:sha256:d310b9fc28abe2dff38f921ead8d768ad74ce1b350721cf0bdd0ed2e93ce410c` | `test:TestGPKV0002ManifestReplay/case:goc-v0-immutable-expectation-without-a-live-oracle` |
| GOC-V0-003 | `cmd/corvint/main_test.go` (677) | `hunk:sha256:3dc59f4a09cb14dbdd6ee87294a2d52ea65c57511a9feac03a6b002ec3af653c` | `test:TestFreshProcessCLICompatibilityEdgesHaveStableNativeResults/case:goc-v0-independent-cli-boundary-expectation` |
| GOC-V0-004 | `cmd/corvint/migrate_traces_test.go` (221) | `hunk:sha256:d56181a118bce3441c22f7fa50d2592929372eb77c0a821b1bfebb7e8a51900b` | `test:TestMigrateTracesApplyMatchesPythonOracle/case:goc-v0-independent-canonical-legacy-migration` |
| GOC-V0-005 | `conformance/perf-v0/main_test.go` (12) | `hunk:sha256:5c6d9eab8491657fb4b109e9d1b29ebd3d7d8a6a9cc507b0f111ede4cf1c215c` | `test:TestRetiredPerfProtocolPreservesEvidence/case:goc-v0-cancelled-measurement-cannot-execute` |
| GOC-V0-006 | `conformance/release-artifact-v0/archive_gate_test.go` (661) | `hunk:sha256:c29d7acf24c40f4abdf5a2f1725584aae72368dcea8aada0361fb0e767f431ad` | `test:TestRawCommitExportBuildCarriesExactVCSIdentity/case:goc-v0-raw-committed-source-identity` |
| GOC-V0-008 | `cmd/corvint/host_adapter_javascript_test.go` (58) | `hunk:sha256:797d51cac3f4fb8573b3e6cdfda972fa6e29a7e30c7582d0cb7e5ff491f2eac5` | `test:TestHostAdapterJavaScriptHosts/case:goc-v0-native-core-and-java-script-host-safety` |
| GOC-V0-009 | `cmd/corvint/go_only_cutover_test.go` (35) | `hunk:sha256:09bbacea9c58817c46b007f2d34a77a4485d8356df654c62363316de0c82a711` | `test:TestGoOnlySourceAndVersion/case:goc-v0-context-abstention-remain-closed` |

## Evidence (2026-09-13, clean `git worktree` at `573bd72b`, go1.27.0)

Private coordinator inputs: `DOGFOOD_INTENTS_FILE` naming only `docs/specs/go-only-cutover-v0.md`,
an empty `DOGFOOD_CITATIONS` file, `DOGFOOD_VERIFY='make gate'` and `DOGFOOD_OUTCOME=blocked`
(the gate was not run in that worktree).

- `make dogfood-change BASE=9ca27f9a…` without the plan: OCM `ready-for-review`, linked 0 of 9.
- All eight plan rows exited 0 on the first attempt. `ocm status` then reported `ready-for-review`,
  linked 8, unknown 1 (`GOC-V0-007 unassessed`), `valid: true`.
- `make dogfood-change` then exited 0, `complete: true`, only `prechange-impact
  unsupported-impact-range` `NOT_PRODUCED`, aggregate coverage 8 of 9.
- `make dogfood-check BASE=9ca27f9a…` exited 0 with `dogfood-check: PASS` and
  `outputsAgree: true`. CEM `ready-for-ci`, 512 of 513 supported.
- Host load: the first `cem prepare` returned `git-timeout`, and the coordinator's `--replace`
  fallback rewrote the committed cited CEM. The sidecar was restored with `git checkout` and the
  runs above were repeated. This is filed in `docs/agent-memory/bugs.md`.

## Not decided

Whether OCM should accept a committed, target-independent link plan as a wire input. That is a
contract change to `OCM-V0-007`, and it waits, like decision 0029's obligation-scope question.

## Rollback

Revert this decision and the matching spec sentences. No code, wire profile or requirement changes.
