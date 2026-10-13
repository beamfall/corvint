# 2026-10-13 local-completion sidecar-only reuse for a verified cem/1.0 sidecar (V1-1303)

Branch `claude/v1-1303-stable-sidecar-reuse`, from `origin/main` `94a7d3dc`. Decision 0525 (RC3
ships `cem/1.0` only). Requirements `CEM-V1-205..209` (new, proposed) in
`docs/specs/cem-stable-v1.md`, under the owner's standing auto-accept of lane requirements
(2026-10-11); the `CEM-V1-150` sentence now says it is superseded for the grant path by
`CEM-V1-205`, without renumbering.

## Intent

V1-1276 cut the dogfood producer over to `cem/1.0`, and V1-1278 removed the `cem/0.x` reader that
local completion's sidecar-only reuse (`AllowCemSidecarOnlyReuse`) used. Since then a check observed
before the CEM commit always ran again (`CEM-V1-150`, fail closed). This change restores the grant
for a `cem/1.0` sidecar, and only on a stable verifier `ACCEPT` over the exact committed bytes.

## Decisions

- In-process verifier. `stableSidecarReuse` (`internal/localcompletion/sidecar_reuse.go`) calls
  `verify.Stable` with the bytes read from `<root>/.corvint/change.cem.json` (bounded by
  `wire.MaxMapBytes`). `Repository` and `ArtifactRoot` are both the absolute repository root,
  `ExpectedBase` is the plan's base and `Target` is the current target. It does not run a
  subprocess and does not re-encode, which is the same contract the dogfood loop gets from
  `cem status --artifacts <root>` (`CEM-V1-146`, `CEM-V1-148`).
- Full object IDs first. A plan base or target that is not 40-hex refuses
  `sidecar-ids-not-full` before any verifier run. A SHA-256 object-format repository therefore
  never reuses (limit below).
- Content digest first. The observation's digest of the tree excluding the sidecar must still
  match, so the sidecar is the only difference the verifier is asked to account for.
- Belt and braces on the accepted result. An `ACCEPT` grants only when `sidecar` is `EXACT`,
  `mapSha256` equals the SHA-256 of the bytes read, and `expectedBase` and `targetRevision` equal
  the plan's base and the current target. Any other accepted shape refuses
  `sidecar-binding-mismatch`.
- Closed reasons on the wire. `CheckObservation` gains an additive, omit-empty `reuseRefused`
  member that carries the `CEM-V1-207` reason. The check stays `selected-check-unverified`, with
  the existing `dogfood verify --check` rerun action, so callers need no new handling.
- Fail-closed default kept. Without the plan flag, no verifier runs and nothing changes
  (`CEM-V1-208`). `TestCEMV1150StableSidecarGrantsNoReuse` keeps its name (cited by `LCP-V0-004`
  and `CEM-V1-210`) and now asserts this default without the flag.
- Context and budget. `checkQualifies` takes the caller's context again. As before V1-1278, the
  aggregate prerequisite path clears the aggregate operation budget for qualification
  (`gitrun.WithOperationBudget(ctx, nil)`, `ALO-V0-017`).
- Line-neutral edits. `docs/specs/local-completion-policy-v0.md` cites unpinned lines in
  `types.go` and `lifecycle.go`, and that file is outside this lane's scope. The edits therefore
  keep every line number in `types.go`, `lifecycle.go` and `finish.go` the same: the
  `checkQualifies` wrapper lives in `sidecar_reuse.go`, and the two regexp variables in
  `types.go` share one declaration line.
- Dead code. The 0.x reuse path was already removed by V1-1278 (`744e8fc4`), so no 0.x code
  remained in `internal/localcompletion` to delete.

## Failure modes

| Case | Result |
| --- | --- |
| Source changed after the observation | `sidecar-content-changed`; no verifier run |
| Abbreviated or non-40-hex base or target | `sidecar-ids-not-full`; no verifier run |
| No sidecar at the root, or unreadable or oversized | `sidecar-unavailable` |
| Map from the earlier target (stale) | `sidecar-rejected` (`patch-digest-mismatch`) |
| Committed map with an edited digest | `sidecar-rejected` (`patch-digest-mismatch`) |
| Working-tree sidecar differs from the committed one | `sidecar-rejected` (`excluded-artifact-mismatch`) |
| Wrong base | `sidecar-rejected` (`base-revision-mismatch`) |
| Runner artifact missing under the root, verifier refusal or `UNSUPPORTED` | `sidecar-unverifiable` (for example `artifact-unavailable`) |
| Map accepted but not committed at the target (`sidecar` not `EXACT`) | `sidecar-binding-mismatch` |

None of these cases reports an internal error, and none grants. Each one costs one rerun.

## Acceptance evidence

Focused run, darwin, go1.27.2:
`GOTOOLCHAIN=local go test -count=1 -v -run 'TestCEMV1150|TestCEMV120[5-8]' ./internal/localcompletion/`

| Requirement | Test | Result |
| --- | --- | --- |
| CEM-V1-205 | `TestCEMV1205StableSidecarAcceptGrantsReuse` | PASS: a committed `cem prepare` stable map qualifies the check with no rerun |
| CEM-V1-205 | `TestCEMV1205ArtifactRootIsRepositoryRoot` | PASS: a map whose receipt cites runner artifacts under the root grants; with one artifact removed, `sidecar-unverifiable` / `artifact-unavailable` |
| CEM-V1-206 | `TestCEMV1206EditedSidecarRefusesReuse` | PASS: committed digest edit gives `sidecar-rejected` / `patch-digest-mismatch`; uncommitted re-spacing gives `sidecar-rejected` / `excluded-artifact-mismatch` |
| CEM-V1-207 | `TestCEMV1207StaleSidecarRefusesReuse` | PASS: earlier-target map gives `sidecar-rejected` / `patch-digest-mismatch`; later source drift gives `sidecar-content-changed` |
| CEM-V1-207 | `TestCEMV1207WrongBaseSidecarRefusesReuse` | PASS: `sidecar-rejected` / `base-revision-mismatch` |
| CEM-V1-207 | `TestCEMV1207UnboundSidecarRefusesReuse` | PASS: no sidecar gives `sidecar-unavailable`; uncommitted gives `sidecar-binding-mismatch`; once committed it grants; abbreviated base gives `sidecar-ids-not-full` |
| CEM-V1-208 | `TestCEMV1208SidecarReuseFailsClosedByDefault`, `TestCEMV1150StableSidecarGrantsNoReuse` | PASS: no flag means `selected-check-unverified` with no `reuseRefused` |

Every refusal test also asserts the `corvint dogfood verify --session-key ... --check test` rerun
action. Full package: `go test -count=1 ./internal/localcompletion/` ok (141.4s, after the line-neutral refactor).
`go build ./...` and `go vet ./...` are clean, and `gofmt -l` is empty. The affected plan
(`corvint affected --base 94a7d3dc...`) selected 47 units, with scope `UNKNOWN` because of two
`LANGUAGE_FRONTIER` unknowns (`go:build-constraint-variants`, `go:nested-module-frontier`). Its
other units are whole-repository readers and do not import `internal/localcompletion`, even through
test dependencies, except `cmd/corvint`. The first run of the 46 non-`cmd/corvint` units happened
while other lanes were loading the host. It passed 34 units and failed 12 with process-observation
errors: `descendant snapshot unavailable`, `adapter-process-failed` and `repository-unavailable`.
Ten of those units were rerun with less load and passed: `lspstdio`, `mcp/bridge`,
`postmergeworkflow`, `postmergehost`, `testrunner`, `opencodequalification`, `upgradechanges`,
`benchmarks/selfuse-batch`, `conformance/mcp-2026-07-28` and `conformance/release-artifact-v0`.
`internal/tasks/cli` and `cmd/corvint-go-test-provider` were not rerun. The `lspstdio` failures
also passed at `BASE`, in a detached worktree of the same clone. For `cmd/corvint`, the
local-completion, qualified-lifecycle, transport-recovery, portable dogfood-flow and
stop-git-calls tests ran, together with `TestDogfoodFinishRunsFromBinaryInForeignRepository`,
`TestTraceGitignoreFinishThenChangeCheckSeal` and `TestAggregateFinishNativeForeignRepository`:
ok (273.6s). The full `cmd/corvint` package was not run.

## Limits

- The suite skips outside darwin and linux, because its fixtures need the stable verifier's host
  tuple. Windows is `NOT_RUN`.
- Repositories using the SHA-256 object format never reuse (`sidecar-ids-not-full`).
- Each reusing check runs the verifier once per evaluation, and the result is not cached.
- `LCP-V0-004` (`docs/specs/local-completion-policy-v0.md`) still describes reuse in its older
  words, because that file is outside this lane's scope. Its trace test name is unchanged.
- `reuseRefused` is a new optional member of the dogfood status check objects. Strict consumers
  that pin the old shape see one extra member, and only when reuse was allowed and refused.
- A committed sidecar re-encoded with whitespace only, which still verifies, is accepted as its
  exact bytes. The stable wire identifies a map by its raw digest, not by a canonical form.

- Affected-plan units `internal/tasks/cli` and `cmd/corvint-go-test-provider` failed only
  under host load and were not rerun. Neither imports `internal/localcompletion`. The full
  `cmd/corvint` package was not run, only its local-completion subset.

## Rollback

Revert the V1-1303 change. Sidecar-only reuse then grants nothing again (`CEM-V1-150`),
`reuseRefused` disappears, and checks observed before the CEM commit rerun. No wire, schema,
packet, plan or committed map changes.
