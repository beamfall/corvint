# V1-1613, V1-1584, V1-1583: bounded plan read, retained tables and sliced shard plans

Date: 2026-10-17. Tickets V1-1613, V1-1584 and V1-1583 follow up V1-1579 (GitHub
beamfall/corvint#1022). Spec `docs/specs/session-test-runner-v0.md`, `STR-V0-055..057`, all
experimental. The requirements are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). They are not an owner review of the design.

## Contract

- `STR-V0-055` (V1-1613): `corvint test --shard-plan PATH` reads the plan through
  `readBoundedFile`.
  - That read is no-follow, non-blocking and regular-file-only, within `MaxShardPlanBytes`.
  - It happens before any Git work. A FIFO, device, directory, symbolic link, missing or oversized
    plan exits 2 with `test-shard-plan-unreadable`. A FIFO with no writer no longer blocks the run.
  - Decode failures keep `invalid-arguments`. `sessiontest.ReadShardPlan` became
    `DecodeShardPlan(name, raw)`, so the package itself opens no plan path.
- `STR-V0-056` (V1-1584): `tools/ci-shard-costs refresh --history DIR [--keep N] [--at TIME]`
  writes the same bytes it is about to install to `DIR/<UTC stamp>-<revision>.json`, before the
  table is replaced.
  - It then removes retained names beyond the newest N, using the strict name pattern, default 5
    and at most 64. Retention is bounded by count, not by age, and other files are never touched.
  - A refused refresh writes nothing.
  - `corvint test shards --durations` reads the retained files directly. A table that is not
    admitted yields the STR-V0-053 fallback, and its reason begins `test-shards-history-invalid`.
- `STR-V0-057` (V1-1583): `corvint test shards --test-slices PATH --split-allow PATH` (both or
  neither) carries AFP-V0-041 slices when history decides.
  - Each allow-listed split package becomes its `cishards.SplitPackages` slices. Each slice
    weighs `max(1, w*m_i/sum(m))` of the package's median.
  - Slices are placed heaviest first onto the lightest shard that holds no sibling.
  - Each shard's `slices` member is omitted when empty, so a plan with no slice is byte-identical
    to before. When any slice exists, the partition digest also binds every shard's slices. Every
    listed entry is placed exactly once, whole or as all its slices.
  - The plan reader refuses a malformed slice even under a recomputed digest.
  - Measured placement runs each slice on its planned shard with its `-run`/`-skip` pattern.

## Choices

- Slice weights come from the slice file's relative milliseconds scaled to the history median.
  This keeps one source of package weight. Rounding is downward with a floor of 1 ms.
- Unusable slice inputs split nothing rather than refusing. This matches AFP-V0-041's own
  posture. Over 256 slices, or over 4 MiB of encoded slices, also splits nothing.
- The digest key `slices` appears only when a slice exists, so existing unsliced plans and their
  digests are unchanged.
- CI wiring of `--history` would edit `.github/`. That is outside this change, so CI still keeps
  only the git history of `package-costs.json`.

## Live qualification

Inputs:

- the 340 packages of `go list ./...` at `ea03c96d0`
- the last six git versions of `.github/cishards/package-costs.json`
- the committed `test-slices.json` and `test-split-allow.json`
- K=5 and 8 shards

Results:

- Whole placement printed `6 unknown`, with a heaviest planned shard of 895576 ms.
- Sliced placement printed `6 unknown, 1 sliced` (`internal/tasks/store`, 2 slices on shards 1
  and 2), with a heaviest planned shard of 834217 ms, 6.9% lower.
- Both reruns were byte-identical.
- A `--placement measured --dry-run` against the sliced plan was NARROWED with 271 units and no
  incompleteness. It placed `internal/tasks/store` slices 0/2 (`-run`) and 1/2 (`-skip`) on
  shards 1 and 2.

These are planned weights, not measured wall clock.

## NOT_RUN

- The STR-V0-057 20% wall-clock comparison against whole-package measured placement. It needs
  full sharded runs, and the full suite was out of scope for this lane.
- The CI workflow change that would pass `--history` to the refresh job (`.github/`).
- The exhaustive repository gate (`make gate`), per the owner's scoped-issue preference.
