# 2026-10-14 Batch 5: actionable OCM and dogfood refusal diagnostics (V1-1049, V1-1048, V1-0522, V1-1385)

Owner request (2026-10-14): batch similar tickets. The four tickets each turn a bare dogfood or OCM
refusal code into one that names its counts, bound or remedy. BASE is `origin/main`
`ee27168739ec1b65dd5a653d720cf811338c37ae`. Scope: 1.0 scope per `PRS-V1-018` (the V1-0020
dependency on the SDD 1.0 improvements).

## Requirements and acceptance

`OCM-V0-021` (V1-1049), `OCM-V0-022` (V1-1048) and `OCM-V0-023` (V1-0522) are appended to
`docs/specs/ocm-v0-dogfood.md` and are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). The assigned IDs `OCM-V0-024` and `OCM-V0-025` are unused. V1-1385 adds
no requirement: it implements the existing `CEM-V1-114` refusal's dogfood fix hint (`DCW-V0-014`).

## Decisions

- V1-1049: the three aggregate bound refusals keep their codes, fixed texts, bounds and precedence
  (bytes, then claims, then obligations, all after `duplicate-requirement`); the message appends
  `: observed T, bound B; per scope 1=N1, 2=N2, ...`. Scopes are named by their 1-based manifest
  line, not by path, so the message holds at most 34 integers (total, bound, and a line and a
  count per scope) and needs no path budget; the manifest and the finish evidence's `intents`
  already map the line to the spec. The ticket asked
  for "the same shape as too-many-obligations", but at BASE that refusal carried no counts either,
  so all three gain the same shape. Success result bytes are unchanged (only error messages change).
- V1-1048: `dogfood finish` previously reduced an aggregate refusal to `errors.New(code)`.
  `aggregateRefusal` now appends one `dogfood-ocm-status` evidence row for
  `.corvint/change.ocm-intents` with the code, and a `detail` member with the bounded message for
  the three bound codes only. The detail is dropped, not truncated, above 512 bytes. The stderr
  envelope still carries only the bare code, because `emitLocalCompletionFailure` deliberately
  admits no path or command text there; the detail reaches the operator on stdout in
  `policy.evidence`, which the command already emits on a failed finish with a lifecycle. Other
  aggregate codes (drift, duplicate, per-map verification) keep only the code: their messages can
  carry verification-cause text whose bound this lane did not establish.
- V1-0522 adjudication: the BASE diagnostics were not sufficient. A bare `too-many-obligations`
  told the operator neither how far over the bound the change was nor which declared scope held
  the obligations, so choosing what to drop meant reading every map by hand. With `OCM-V0-021`
  and `OCM-V0-022` the refusal names the total, the bound (256 at BASE; the ticket's "128" predates
  the current bound) and each scope's share, which is enough to decide which scope to keep. The
  remaining gap was guidance, now in `docs/DOGFOOD.md` section 5 "Choosing intents when the
  aggregate refuses a bound" (`OCM-V0-023`): keep the owning spec, drop an inherited-constraint
  spec from the intents and cite its lines instead, record each omitted spec and its still-applying
  constraints in the build log, split the change by owning intent when the owning spec alone is
  over the bound. No bound is widened, no obligation merged or dropped, and a smaller scope set is
  never read as a smaller gate.
- V1-1385: `internal/dogfoodflow/change.go` gains the `cem-cite:cite-self-modified-authority` fix
  hint naming the recovery (cite an unmodified governing source, or keep the hunk unknown). The hint
  table is keyed by string, so it does not need the `CEM-V1-114` code constant from PR #902. Per the
  coordinator's 2026-10-14 note, the `internal/observations` admission of that code is made inside
  PR #902 (its CI needs it), so this branch does not touch `internal/observations`.

## Failure modes

- A per-scope count is from the verified map bytes, the same values the totals sum; a drifted map
  is refused earlier as `intent-scope-drift`, so a breakdown never describes unverified bytes.
- A non-`lrfrepo.Error` coded error (a `cemcode` code) is recorded with its code and no detail.
- An uncoded aggregate error passes through unchanged and is not recorded as evidence.

## Acceptance evidence

- `internal/dogfoodocm/aggregate_test.go:TestOCMV0021BoundRefusalsNameTotalsBoundAndScopes`:
  fails on BASE (four refusal cases, bare messages) and passes after; the exactly-at-bound case
  passes on both.
- `internal/localcompletion/aggregate_refusal_test.go:TestOCMV0022FinishRetainsAggregateRefusalDetail`:
  does not build on BASE (the helper is new; BASE returned the bare code with no evidence row) and
  passes after.
- `internal/dogfoodflow/change_test.go:TestChangeDCWV0041NamesCitationRowFix` gains the
  `cite-self-modified-authority` row and passes.
- Focused packages (load average 78 to 114 on the shared host): `internal/dogfoodocm`,
  `internal/diagnostic`, `internal/specindex`, `internal/obscorpus`, `internal/unplannedread`,
  `internal/sessiontest`, `internal/witnesscollapse`, `internal/semescalate` passed;
  `internal/dogfoodflow`, `internal/localcompletion` and `internal/lrfrepo` hit the 30-minute
  per-package timeout in the combined run and passed when rerun alone; two `internal/mcp/bridge`
  repository-unavailable timeouts passed when rerun alone. `cmd/corvint -run
  'LocalCompletion|DogfoodOCM|TestDogfoodFinish'` passed. `-race` on the new and changed tests,
  `go vet` on the three touched packages, `gofmt -l` and the 12 CI doc gates passed.

## Independent review

One Codex read-only round (CONCERNS, four findings). Fixed: the requirement and this entry said a
bound message holds at most 18 integers, but a line and a count per scope make it 34; the
DOGFOOD.md split advice claimed splitting helps when one owning spec exceeds the bound, but one
spec over 256 requirements is refused earlier by `ocm prepare`, so the step now covers several
owning specs and names the earlier refusal; three unanchored `local-completion-policy-v0.md`
failure-code rows (`dogfood-coordination-failed`, `invalid-public-evidence-result`,
`public-report-path-missing`) already pointed at unrelated lines at BASE and are now pinned to
their emitting sites. Recorded, not changed: no test drives a failed `dogfood finish` to show
`policy.evidence[].detail` on stdout with the bare code on stderr (see limits).

## Limits and NOT_PRODUCED

- No end-to-end `dogfood finish` run over a real >256-obligation change was made; OCM-V0-022 is
  pinned at the helper boundary that `prepareBindings` calls.
- The `make gate` and exhaustive `go test ./...` were not run (lane policy); packages `corvint
  affected` selected through path-literal readers beyond the focused set are NOT_RUN.

## Rollback

Revert the change commit: the bound messages return to their fixed texts, `detail` disappears
from the finish evidence (it is `omitempty`), and the fix hint and DOGFOOD.md section are removed.
No wire, store or map format changes.
