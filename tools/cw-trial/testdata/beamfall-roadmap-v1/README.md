# beamfall-roadmap-v1: frozen `context --task` paired-trial protocol (V1-0319)

Status: protocol frozen 2026-09-25, not run. The run waits on the owner questions below.

## Gate

`docs/specs/task-context-packet-v0.md` Agent digest, "Blocked on": "a paired trial reading against
`grep` on the held-out set". The trial is `tools/cw-trial` (`docs/specs/confidently-wrong-trial-v0.md`),
whose judged configuration is fixed by decision 0022. The owner ruled on 2026-09-25 (panel dispute
D13, decision 0398, not yet recorded in `docs/decisions/`) that this gate runs on Beamfall
roadmap-ticket-to-diff tasks. Skills and `docs/DOGFOOD.md` §1 may route orientation to
`context --task` only after the gate passes.

## Task set

`tasks.json` (sha256 `7fbe8124e55d247bc98ef0a06f3a0d820f6824aff680853b9c69a353fe85db85`) holds 50
`retrieval` tasks with 181 gold paths. It was drawn from 247 candidates in Beamfall/core's
first-parent history at `02835b1be8a2c6989ea2f4b74dbf6ec0163c2b35`. `build_roadmap_set.py`
(sha256 `412917690587e29161da3a6f2ecb254750ea45c2917293eb2b032d4fdf06c41c`) reproduces the set from
a Beamfall/core clone using only Git: `python3 build_roadmap_set.py REPO 02835b1be tasks.json`.
The manifest's `sampling_rule` states the rule in full:

- A candidate is a `roadmap: check off ID` commit whose checked-off entry names Primary Repo
  `beamfall`.
- The resolving diff is every non-merge commit whose subject starts with `ID `.
- The base is the first parent of the earliest resolving commit.
- Gold is the non-binary paths those commits touched that exist at the base, excluding
  `docs/plans/roadmap/`. A task has 1 to 40 gold paths.
- The task text is the entry's one checkbox line. Its sub-bullets (Allowed Paths, Do, Acceptance)
  are withheld because Allowed Paths names the gold.
- LCRES-15 and COREAPI-AUDIT-0822-3 were observed by the panel and are excluded.
- Tasks are selected at index floor(i*N/n).

The set was authored without running Corvint on any task. Under `benchmarks/README.md` it becomes
development as soon as any result from it changes Corvint.

## Arms, metric, run

- Arms: `grep` (the control and the paired comparator), `corvint` (`context --task TEXT --limit 20`,
  CWT-V0-003), and `none` (the memorisation check that decision 0022 keeps).
- Metric (CWT-V0-007, CWT-V0-013): the paired reading of `corvint` against `grep` on
  `confidently_wrong_tasks`, `success` and `tokens`, per the kill criteria in CWT "Promotion or kill".
  The retrieval-native `packet_top_1/3/5/10`, `gold_as_result` and `mean_f1` are reported beside it
  as the orientation reading.
- Threshold: `corvint` has at least a third fewer confidently-wrong tasks than `grep`, at non-inferior
  success and at most 25% more tokens. Under CWT-V0-009 the owner reads this; the report carries no
  verdict field.
- Runs and model: one first-observation run, preserved unrepaired. Model and effort are those of
  decision 0022: `codex exec`, `gpt-5.6-sol`, effort `medium`, `--access none`, `--limit 20`,
  5-minute timeout, sequential. That is 150 invocations. The Corvint binary is built from `main` at
  `489701ca`, and the report records its version and sha256.

```sh
cw-trial run --tasks tools/cw-trial/testdata/beamfall-roadmap-v1/tasks.json \
  --history BEAMFALL_CORE_CLONE --corvint CORVINT_BIN --arms none,grep,corvint \
  --agent codex --model gpt-5.6-sol --effort medium --access none --limit 20 --timeout 5m \
  --output benchmarks/results/cw-trial-beamfall-roadmap-v1-first-run.json
```

## Owner questions (block the run)

1. Non-inferior success has no margin in any spec or decision. The bet plan the CWT spec cites,
   `docs/plans/BREAKTHROUGH-BET-2026-09-01.md`, is not in the tree.
2. It is unresolved whether `likely` wrong claims join the confidently-wrong criterion (CWT
   "Unresolved decisions").
3. Does the orientation reading (for example `packet_top_10` or `gold_as_result`, `corvint` against
   `grep`) carry its own pass threshold, or is it descriptive only? D13 concerns orientation recall,
   and the bet criterion measures agent claims.
4. Confirm that `gpt-5.6-sol` at `medium` (decision 0022) is still the model, and approve the
   150-invocation spend.
