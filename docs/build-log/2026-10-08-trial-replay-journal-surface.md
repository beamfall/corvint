# 2026-10-08: trial replay journal, first surface

## Intent

The owner asked for ideas from the npm package `@andrewjacop/pi-herdr` 0.6.0. One idea was its
workflow journal: a content-keyed, append-only log that lets a rerun replay unchanged steps. The
owner then asked for a spec. The result is the proposed
`docs/specs/trial-replay-journal-v0.md` (TRJ-V0-001..012). No code has changed.

## Surface choice

Chosen: the two agent trial dispatchers, `tools/cw-trial` and `tools/cem-trial`. They are the only
Corvint runners that pay for many independent non-deterministic invocations and already rerun them
often. Across the 26 committed `corvint-cw-trial/0` and `corvint-cem-trial/0` reports in
`benchmarks/results/`, 1,397 of 3,333 recorded invocations are reused, all of them in cw-trial,
and development reruns reuse 65–100% of theirs. Each tool already has its own `--resume` and
`--reuse`, which have drifted apart. Neither compares the recorded `agent` identity, which holds the codex version, argv
template and effort, or the script digest. Neither keys on the agent's ambient codex configuration.
None of the 14 distinct cw-trial `reused_from` source digests matches a JSON file tracked in the
current tree. cem-trial's `reused_from` is a manifest digest, not a source file digest.

Rejected:

- **`make gate` steps and dogfood stages.** These are deterministic, so they belong to the gate
  ledger (`docs/specs/gate-ledger-v0.md`), which already reuses a step only for byte-identical
  inputs. Replaying an observation would be weaker than rerunning a deterministic step.
- **Tasks attempt-runner retries (ATR-V0).** A retry is a new claim on new work. Replaying an
  earlier attempt's output would hide exactly what the retry exists to observe.
- **Post-merge workflow stages.** Post-merge Replay V0 records that no historical replay set exists
  yet, so there is nothing to measure.

## Decisions

- **Matching is by key; replay by position is not used.** pi-herdr replays by position because its
  workflow scripts share hidden state between steps. Trial steps are independent, and any future
  dependent step must declare its upstream keys (TRJ-V0-011). That gives a correct rule with no
  ordering concern.
- **The `agent` identity is part of the key.** This closes the silent reuse across codex versions,
  argv templates, effort levels and scripts that today's checks allow.
- **Every committed replayed observation must name a tracked source (TRJ-V0-009).** Today's reports
  are exempt and keep their gap visible.
- **Legacy reports replay nothing by default.** Whether to keep a labelled legacy path is left to
  the owner.

## Evidence

| Item | Result |
|---|---|
| Reuse share in committed reports | 1,397 / 3,333 invocations, from a script over `benchmarks/results/*.json` at `3989698e` |
| Source digests tracked in the current tree | 0 of 14 |
| Wall-clock or token cost of an edit-and-rerun loop | `NOT_OBSERVED`: no agent run in this session |
| Checkpoint write volume | Estimated at ~165 MB for a 252-invocation, 1.3 MB report; not measured |
| Independent review | One reviewer subagent checked every current-state claim against the code: 11 findings, all applied |
| Dogfood loop (`make dogfood-change`) | `NOT_RUN`: proposal-only documentation change; no implementation exists to bind |
