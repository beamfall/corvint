# V1-2049: a dirty Tasks store projection is task state, not an unknown

Owner input: ticket V1-2049 (P1 bug; no GitHub issue). `corvint affected --base` in a repository
with a Corvint Tasks store reported every dirty `.taskman/` projection path as
`UNOWNED_DIRTY_PATH`, so the plan was always `UNKNOWN` (Beamfall core-history: 820 of 866
unknown entries). AFP-V0-088 was added to `docs/specs/affected-plan-v0.md`; it is accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). No CAL-V0 ID was added.

## Decision

A closed projection layout (`policy.json`, `queue.json`, `tickets/<name>.json`,
`releases/<name>.json` directly under the root `.taskman/`) is a typed non-source class,
`TASK_STORE_PROJECTION`, with one stated reason per class in the new plan member `nonSource`.
It is not silently dropped: the paths stay visible on the wire, in dirty-path order.

The classification runs after the reader rules and only removes `UNOWNED_DIRTY_PATH` entries.
A projection path that a unit names by a resolving path literal (AFP-V0-021, including the
AFP-V0-072 file-name narrowing, so V1-1766's basename `policy.json` still counts) or holds in a
declared read scope (AFP-V0-023) keeps its unknown entry, and the reader stays selected, since
selection only adds. An unbounded reader is selected on every dirty path, so it does not count as
naming one; otherwise every repository with one root-locating test would never classify anything.
A classified path also takes no part in `participants`, because an unowned path otherwise makes
every plugin take part and brings every plugin's frontier with it (13 frontier entries in
core-history).

The reason is per class, not per path, because core-history's projection carries 809 paths in one
diff and a per-path reason would add over 100 KiB to the default receipt.

## Measurements

Binary built from this branch; plans over `HEAD^..HEAD` of scratch clones, `--full`.

| Repository (commit) | Projection paths | Before | After |
|---|---|---|---|
| core-history `52704702f` | 809 | `UNKNOWN`; 809 `UNOWNED_DIRTY_PATH`, 13 `LANGUAGE_FRONTIER`; 58 selected (`UNBOUNDED_READER`) | `UNKNOWN`; 0 `UNOWNED_DIRTY_PATH`, 3 `LANGUAGE_FRONTIER`; `nonSource` 809; 58 selected, unchanged |
| Corvint `99dac5234` | 13 | `UNKNOWN`; 13 `UNOWNED_DIRTY_PATH`, 25 `LANGUAGE_FRONTIER`; 48 selected (40 `UNBOUNDED_READER`, 8 `PATH_LITERAL_READER`) | identical; no `nonSource` |

Core-history's remaining frontiers (`go:build-constraint-variants`,
`go:workspace-module-outside-root`, `shell:foreign-invocation-unmodelled`) belong to units the
unbounded-reader rule reached; they are genuine and keep that plan `UNKNOWN`.

In Corvint's own repository no reader is hidden: `.taskman/queue.json` is named by the
`internal/tasks/*` packages, `companionrelease` and `opencodequalification` (the 8
`PATH_LITERAL_READER` units), and every ticket record is named by the store-relative `/tickets/`
literal of `internal/tasks/transaction`, so all 13 paths keep their unknown entries. That literal
names the store layout rather than this repository's projection, so Corvint's own ticket-only
diffs still widen; this over-widens and is left as a residual rather than special-cased.

## Not produced

Impact freshness (`mixed` in `internal/gokernel` `repositoryFromObservation`,
`mixed-worktree` in `internal/worktreeimpact`) is `NOT_PRODUCED`: both are shared worktree states
governed by other specs and consumed by every context and hook packet, so classifying task state
there is not the same small change and needs its own requirement. The fast-tier mirror
`tools/gate-affected-select` is not changed; it still falls back to the full run on an `UNKNOWN`
plan.
