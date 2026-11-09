# TSP-V0-7330: the selection baseline uses CI's `.github` guard

Experimental, under the owner's standing lane auto-accept (2026-10-11); base
`fc47f142487e1506b863131ec301e9c10445dd89`, branch `claude/selbase-guard`.

`tools/selection-baseline` counted a change as `FULL` whenever any path started with `.github/`.
CI does not: its narrowing verdict runs `.github/forward-rows/github-guard.sh`, which forces `FULL`
only for the guarded control-plane inputs (forward rows, `cishards`, `testconfine`, `actions`, the
two control-plane workflows and the guarded `ci.yml` sections). The baseline therefore
under-reported narrowing for changes that touched an unguarded workflow or release gate, and the
floor measured a stricter rule than the one CI enforces.

The baseline now reads the guard once from the replaying (trusted) checkout and runs it over each
change's base and head in the clone. `0` records no `.github` change; `1` records one with the
inputs the guard names in `githubInputs`; any other output, or a guard that cannot run, fails closed
as a change with an `unreadable:` input. The guard's bytes join the result-cache key (cache schema
`/2` rejects `/1` entries), and the guard is an explicit input of the baseline's own gate decision.

Evidence:

- `TestTSPV07330GuardDecidesTheGithubScope_TSPV07330` covers stub guards printing `0`, `1` with
  inputs, unreadable output, a lone `1`, empty output, exit 3 and a missing guard.
- `TestTSPV07330MissingGuardKeepsTheReport_TSPV07330` replays without a guard: the report survives
  and the change records an `unreadable:` input.
- `TestTSPV07330RealGuardNarrowsAnUnguardedWorkflow_TSPV07330` runs the repository's real guard
  over a temporary repository: another workflow narrows; a `forward-rows` script change returns
  true and names that input.
- The whole `tools/selection-baseline` package passed under `script/heavy.sh` with `-count=1`.
- Replay: `narrow=35/43 merged (rate 0.814) bounded=0 misses=0 over 26 failing packages of 15
  failed CI runs; median cost 0.818`, `OK floor narrow>=30 misses<=0` (from 33/43 before the guard).
- `changes.json` refreshes the nine entries the replay moved from `FULL`/`FALLBACK` to
  `NARROW`/`run` (pr-4, pr-6, pr-41, pr-43, pr-46, ci-4a7bf04c8, ci-8db625151, ci-cbc790640,
  ci-0df97c8c7), per TSP-V0-710. The floor stays at
  30 here so this change does not conflict with the open floor raise in PR 163; raising it to the
  measured count is a follow-up once both land.

Non-goals: the guard itself is unchanged. Whether a forward-rows script change needs the full Go
suite (rather than `tools/ci-forward-row`'s tests) is a separate control-plane question.

Rollback: revert this commit; the cache schema bump discards `/2` entries on the old reader.
