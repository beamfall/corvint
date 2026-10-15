# 2026-10-14 V1-1359: the cem-verdict predicate domain is separate from the claim scope

Lane V1-1359, Corvint 1.0 scope `PRS-V1-018` (accepted, owner instruction 2026-10-13). Requirements
`TCB-V0-035..039` (`docs/specs/tasks-criterion-binding-v0.md`, section "Verdict predicate domain"),
all accepted under the owner's standing auto-accept (2026-10-11). BASE
adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f.

## Intent

Policy v9 ran `touched-anchored` over `{claim.touchPaths}`, the claim-era ticket's
`effects.touchPaths`. That field is also the Tasks reservation (the claim scope). An ordinary cited
hunk never reaches tier `anchored`; only generated spec projections do. A ticket that declared its
real write set therefore could not pass the gate, and V1-1348 declared only `docs/specs/README.md`
and `docs/specs/REQUIREMENTS.tsv`, then widened its scope after the claim. One field had two
consumers and the narrower one won. A ticket now declares its full write set as `touchPaths` and the
governed subset as `effects.verdictPaths`.

## Decisions

- Field: optional `effects.verdictPaths`, the `touchPaths` shape (sorted, duplicate-free, at most 256
  repository Paths). The `effects` key lists move to `internal/tasks/wire`
  (`TicketEffectsKeys`, `TicketEffectsOptionalKeys`), so the native codec, the mutation payload
  reader and Core's planner admit the same keys. A record without the key keeps its bytes.
- Subset rule: each verdict path must be covered by a touch key under CAL-V0-021, the rule
  `ticket.PathCovers` applied at BASE, now shared as `wire.PathKeyCovers`. CREATE and SET_EFFECTS refuse
  `OUT_OF_SCOPE` with `VERDICT_PATH_OUTSIDE_TOUCH_PATHS`. REFINE carries no effects, so "create/refine
  time" is CREATE and SET_EFFECTS. Both readers refuse a stored record that breaks the rule.
- Empty list: fail-closed, `MALFORMED` with `VERDICT_PATHS_EMPTY`. An empty domain is invalid gate
  input (TCB-V0-007), and reading it as "absent" would give one meaning two encodings. Omit the key to
  get the fallback.
- Placeholder: a new `{claim.verdictPaths}`, falling back to `touchPaths` when the key is absent.
  `{claim.touchPaths}` keeps its meaning, because a placeholder that silently preferred another field
  would no longer name what it carries. The charter allowed that alternative; it is rejected here.
- Policy: `.taskman/policy.json` moves from version 9 to version 10. The only change is the
  `cem-verdict` argv value `--touch-paths {claim.verdictPaths}`. This lane runs no `policy update`.
- Rejected:
  - a per-gate path list in the policy (it is per gate, not per ticket);
  - deriving the domain from the CEM's anchored set (circular);
  - changing `touchPaths` in place (it would break the reservation and collision detection).

## Failure modes and limits

- Deployment order: install a `corvint-tasks` containing TCB-V0-037 before the store adopts policy
  v10. An older runner passes the literal `{claim.verdictPaths}`, and `corvint cem gate-verdict`
  refuses that argv with exit 2 (`cem-gate-verdict-invalid-arguments`, the existing
  `TestTCBV0016GateVerdictArgvIsClosed` "touch not json" case). The gate fails closed. It never
  governs another domain.
- Composition: Tasks imports no Core package. One test therefore asserts the exact arrays the runner
  substitutes (`TestTCBV0037_VerdictPathsPlaceholder`: `["src/"]` and `["lib/","src/"]`). Another feeds
  those exact arrays to the real Core gate over the shared sealed-map
  gate fixture (`newVerdictFixture`, used by TCB-V0-007..014)
  (`TestTCBV0038_VerdictDomainNarrowsTheGovernedSet`). No single in-process test spans both. A live
  store run is NOT_RUN: it needs the installed binary and policy v10 that the orchestrator applies
  after merge.
- Directory keys: V1-1356 (PR #874, TCB-V0-020..023) makes the gate refuse an exact touch key naming a
  directory. Verdict paths reach the gate as its `--touch-paths`, so that refusal covers them. At
  create time the store has no tree and only warns, through CAL-V0-211, which now also inspects verdict
  keys.
- SET_EFFECTS replaces the whole `effects` object. Re-setting effects without `verdictPaths` drops the
  key and restores the fallback, which the test asserts. SET_EFFECTS stays blocked while an attempt is
  live.

## V1-1348 against the new contract

The following reads were made read-only:

- `ticket show V1-1348`: COMPLETED. Effects `touchPaths ["docs/specs/README.md",
  "docs/specs/REQUIREMENTS.tsv"]`, with no `verdictPaths`. `requiredGates ["cem-verdict"]`. Completion
  VERIFIED, evidence 34b109619d38… and 97e2a6554b42…, manifestSha256 8c02f72f98a4…, recorded
  2026-10-14T00:11:15Z.
- Build-log entry: `2026-10-14-v1-1283-native-completion-through-cem-verdict.md` (gate receipt
  000000005368, GRANT under policy v9).
- `receipt show`: NOT_RUN. The installed verb is not implemented and reports "arrives with a later
  TCP-02 slice".
- `receipt audit`: OK, head sequence 5453, structural consistency CONSISTENT, projection AGREES.

The record declares no `verdictPaths`, so `{claim.verdictPaths}` substitutes exactly the
`{claim.touchPaths}` array v9 passed. The gate argv, inputs digest and GRANT are unchanged under v10.
No re-run is needed, and no completion was created. The lane's post-claim `widen` stays as recorded.
Future tickets declare the write set in `touchPaths` and the governed projections in `verdictPaths`.

## Acceptance evidence

Toolchain go1.27.1 (`GOTOOLCHAIN=local`). `corvint affected --base adf587fdb` selected 124 units
(scope UNKNOWN, with language-frontier unknowns). The units run are listed below. The other selected
units reach these packages only transitively and are NOT_RUN under the lane's shared-host budget.

- `go test -race` passed on `internal/tasks/wire`, `internal/tasks/ticket` and
  `internal/tasks/mutation`. It also passed on `internal/taskman` in full, before the Codex fix, and on
  that fix with `-run 'TCBV003|Issue502|Decode|Ticket'` afterwards. Results for `internal/tasks/store`
  and `internal/tasks/cli` are in the lane report.
- `go test` passed:
  - `internal/tasks/cli`, with `-run 'Template|Help'` after the template fix;
  - `cmd/corvint`, with `-run 'TCBV00|GateVerdict'` (two tests, including the "touch not json" exit-2
    case);
  - `internal/tasks`, `archive`, `authority`, `criterionbinding`, `dispatch`, `importer`, `intent`,
    `journal`, `obligation`, `release`, `scale`, `scopes`, `service`, `snapshot`, `supervisor` and
    `transaction`;
  - `conformance/work-queue-v0`, `internal/dashboard/roadmap` and `internal/specindex`.
- `go vet` on the touched packages passed, and `gofmt -l` is empty.
- The 12 CI doc gates pass after staging.
- `make gate` and `go test ./...`: NOT_RUN, by the lane frame.

One Codex read-only round produced three findings, all confirmed and fixed:

- P2: Core's reader did not refuse an unsorted, duplicate or over-256 `verdictPaths`, which the Tasks
  codec refuses. Core now applies `CheckSortedUnique` and `MaxTouchPaths`, with tests.
- P3: the TCB-V0-036 traceability row overstated which lists each reader and mutation refuses. The row
  now names the tested cases, and CREATE gained unsorted and duplicate cases.
- P3: the CREATE template described `effects` as four members only. It now names the optional
  `verdictPaths`.

## Rollback

Revert this change. A store that adopted policy v10 then takes a version 11 with `{claim.touchPaths}`.
Before that, any ticket carrying `verdictPaths` must drop it by SET_EFFECTS, because a reader without
the key refuses the record.
