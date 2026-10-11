# 2026-10-10: Tasks idle dispatcher safety net (V1-0913)

## Intent

V1-0913: idle CPU of a dispatcher at 10,000 tickets was reported as 1.44 CPU-s/min. Owner approved
(relayed by the coordinator) lengthening the CAL-V0-139 full-read safety net to 15 minutes.

## Change

- Proposed CAL-V0-217 (pending owner acceptance): `idleFullEvery` 60 s -> 15 min.
- `internal/tasks/dispatch/idle.go` constant and `TestCALV0217_SafetyNetIsFifteenMinutes`.

## Correction to the first report

The first report estimated 0.46 CPU-s/min for a per-entry `lstat` witness at 10,000 files. That was
wrong: `cli/dispatch_witness.go` stats only the top-level `.taskman` entries (the tickets directory
mtime already is the directory-level witness), so the proposal to replace it is already how it
behaves and nothing was changed there. The remaining idle cost is the full read every safety-net
interval (2.4 to 3.4 CPU-s per read at 10,000 tickets, build log 2026-10-06), about 3 per minute at
60 s against about 0.2 per minute at 15 min.

## Not run

The 10-minute 10,000-ticket idle profile: `tasks-scale gen --tickets 10000` reached only 1,869
tickets in about 1.5 hours (it slows as it grows), so no real before or after number was taken.
The figures above are derived from recorded read costs. A real before and after is a follow-up.
