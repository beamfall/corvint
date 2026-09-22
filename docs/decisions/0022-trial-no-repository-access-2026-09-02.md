# Decision 0022 — The judged confidently-wrong trial runs with no repository access, on the pilot's model

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "go, no repo
access, same model as pilot" (2026-09-02), in reply to the recommendation that the judged run
"give the agent no repository access at all: claims rest only on the supplied context, the `none`
arm becomes a pure abstention test, and confidently-wrong becomes observable", and that the model
and effort be "the pilot's pair for comparability".

## What is decided

1. The judged run over `tools/cw-trial/testdata/heldout-v1` uses `cw-trial run --access none`
   (`CWT-V0-010`): every arm's context is produced from the materialised copy, the copy is removed,
   and the agent is invoked from an empty directory under a skeleton that states the repository is
   unavailable and forbids commands. The three arms differ only in supplied context.
2. The agent is `codex exec` with model `gpt-5.6-sol` and `model_reasoning_effort=medium`, the
   pair the pilot recorded (`benchmarks/results/cw-trial-pilot-first-run.json`), `--limit 20`,
   per-invocation timeout five minutes, sequential.
3. Compliance is observed, not enforced: codex's shell stays available inside its read-only
   sandbox, so the report counts tool calls per invocation and the reading must state how many
   invocations ran a command. A run whose `explored_tasks` is not zero in some arm is still the
   first observation; it is read with that caveat, never rerun to a cleaner number.
4. The frozen set is imported through `cw-trial import-heldout` (`CWT-V0-011`), which verifies the
   set's digest and every chunk file's digest and refuses any row it cannot map. The frozen files
   are not edited; the produced manifest is scratch, identified in the report by the set's name
   and `tasks_sha256`.

## Why

The pilot (read-only access, five tasks) scored 5/5 in every arm with zero confidently-wrong
claims because the agent found gold by exploring the copy; the arms measured nothing. The bet's
kill criteria compare confidently-wrong rates between context arms, which requires that context
be the only thing an arm supplies. Removing access is the smallest change that makes the
comparison meaningful; keeping the pilot's model keeps the pilot as a shape reference.

## Consequences and first-observation rule

The run's report is preserved under `benchmarks/results/` and `first_observation` in the frozen
manifest is set to it; from that moment heldout-v1 is development (`benchmarks/README.md`). The
reading in `docs/BUILD-LOG.md` states success, confidently-wrong, abstention, tokens, and
`explored_tasks` per arm and reads them against the bet's criteria without a pass/fail field in
the report (`CWT-V0-009`). No repair of Corvint may use this run's per-task rows without recording
that the set is no longer held-out.
