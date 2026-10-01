# Supervised Codex effort and stage wall (issue 354, partial)

Date: 2026-10-01
Owner intent: GitHub #354 (follow-up to #341); native V1-0475 criteria 4 and 5.
Requirements: CAL-V0-059, CAL-V0-060 in `docs/specs/corvint-tasks-agent-leases-v0.md` (S12).

## Gap analysis at base ff3da727e95a1999cbc4a58a471cdd498693f902

| #354 requirement | Status before | This change |
| --- | --- | --- |
| Multi-repository programs | Missing in the supervisor and the dispatcher; V1-0475 depends on the V1-0463 foundation, absent from public main | Not addressed |
| Configurable effort per role/stage, policy-bounded | Dispatcher: any effort through host argv (S11). Supervisor: config fixed at `effort: "low"` | CAL-V0-059 |
| Longer, resumable runs | Dispatcher: role `wallSeconds` up to seven days. Supervisor: `wallSeconds` 1..3600; expiry gives a WAIT handoff with session resume (CAL-V0-038/039) | CAL-V0-060 raises the stage bound to a policy-owned 1..240 minutes (the lane cap ceiling). Checkpointed automatic continuation stays open |
| Host adapters beyond Codex | Dispatcher: Claude Code, Codex, OpenCode (Gemini in review). The `run` supervisor is Codex-only | Not addressed |

## Decisions

- Additive optional fields on the existing `/0` supervision policy (`efforts`,
  `stageWallMinutes`), read with the closed-object reader's optional keys. When they are absent,
  the behaviour is unchanged: low only, one-hour stages. That keeps existing policy bytes
  meaningful and avoids a policy-version or profile bump. V1-0475 records another session's
  in-progress design (a `/1` profile with policy/3). This slice does not take that path, and any
  later `/1` profile can carry the same fields.
- The config's `stageEfforts` is `omitempty`, so recorded `configSha256` values for existing
  programs keep their digests.
- Check placement:
  - A new program is refused before the runtime read and its first record.
  - An existing program is re-checked at every stage launch, not on open, because `drain` and
    `cancel` open the workflow. A policy narrowing must never trap a program that is already
    running.
- New and resumed (`exec resume`) Codex invocations share one argv builder. Both carry the
  stage's own effort; previously both carried the single config `effort`.
- Effort vocabulary is limited to `low|medium|high`. Codex `minimal`/`xhigh` are excluded until
  they are qualified.

- Repair r1 (independent review FAIL):
  - `stageWallMinutes` was first accepted up to 1440, but the stage deadline is the minimum of
    `wallSeconds`, the lane `wallClockMinutes` (bounded by `wire.MaxLaneWallMinutes` = 240) and
    the remaining program time. Values 241..1440 therefore had no effect. The bound is now
    `MaxStageWallMinutes = wire.MaxLaneWallMinutes`, and larger values are refused with
    `LIMIT_EXCEEDED` through the same reader as the lane cap. Raising the lane ceiling is a wider
    contract change and is out of scope; 24-hour stages stay open under #354.
  - A mixed allowlist (`{"implement":["low"],"repair":["high"]}`) now witnesses the closed-stage
    check: with that check removed, the known stage satisfied "names a stage" and the unknown
    key was silently ignored. A scratch mutation deleting the check fails
    `TestCALV0059_PolicyEffortAllowlist`.
- Known ordering (not changed): for `integrate`, the GRANT record is written in the integrate
  path before `stage()` re-checks the current policy, so a narrowing that refuses `integrate`
  leaves that GRANT recorded without a launched stage. The refusal still precedes any host
  process.
- Compatibility: binaries older than this change read `supervision` as a closed object and refuse
  a policy that carries `efforts` or `stageWallMinutes`. That fails safe (no dispatch under an
  unread bound), but an owner must upgrade every reader before adding the keys.

## Evidence

- Focused tests:
  - In `internal/tasks/intent`: `TestCALV0059_PolicyEffortAllowlist` and
    `TestCALV0060_PolicyStageWallBound`.
  - In `internal/tasks/store`: `TestCALV0059_StageEffortSelection`,
    `TestCALV0059_CheckProgramConfigEffort`, `TestCALV0059_StageRechecksCurrentPolicy`,
    `TestCALV0059_OpenWorkflowRefusesBeforeMutation` and
    `TestCALV0060_CheckProgramConfigStageWall`.
  - The full `internal/tasks/...`, `cmd/corvint-tasks`, `internal/specindex` and
    `internal/taskman` packages, plus `go vet` on the tasks packages.
- `corvint affected` selected 153 units. The doc edits make most of them reachable through
  documentation readers. Those beyond the packages above are `NOT_RUN`, by the owner's
  focused-test preference; the affected-plan scope stays `UNKNOWN` (language frontiers).
- Live Codex qualification at a non-low effort, and a stage longer than one hour: `NOT_RUN`. No
  cheap fixture exists, and the task forbade paid runs. The provider's honouring of the effort is
  `NOT_OBSERVED` beyond the argv the supervisor passes.

## Remaining under #354 / V1-0475

- Multi-repository programs.
- Non-Codex supervisor adapters.
- Checkpointed automatic continuation beyond WAIT/resume.
- Per-role models.
- Live qualification of CAL-V0-059/060.
