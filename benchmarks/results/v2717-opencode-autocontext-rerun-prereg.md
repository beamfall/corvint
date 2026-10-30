# Preregistration: V1-2717 auto-context rerun (OpenCode adapter 0.7.26)

Composed 2026-10-26 (UTC) by the `claude/opencode-auto-context-2717` lane before any trial session.
It is committed on that branch in the same commit as the candidate adapter, before the first trial
session; the receipt records that commit and this file's SHA-256.

## Candidate under test

- OpenCode adapter `integrations/opencode/src` at this commit (adapter 0.7.26, `AHI-212`):
  - Auto-injection is unchanged from 0.7.23-0.7.25 (`AHI-211`). On every user prompt the
    `session.prompt` hook runs the same bounded `user-prompt` query the Claude Code hook uses. For a
    confident packet (`READY`, abstention inactive, at least one result with a path) the
    `session.context` hook adds the explained candidate frame (at most five candidates, a
    1024-byte object inside the untrusted-data envelope, with receipt id and uncertainty). Any
    other packet injects nothing and records `prompt-context-withheld`.
  - New in 0.7.26: setup registers only `corvint_context` (direct, a one-sentence description).
    `corvint_expand`, `corvint_record_outcome`, `corvint_status` (Code Mode) and the bundled skill
    are added on the first `corvint_context` call, which OpenCode delivers as appended notices.
- The pinned corvint binary is built from the same tree (darwin/arm64, `1.0.0-rc.3 (build 0)`,
  SHA-256 `56203d619966cf541307e7b3c5b232f5f437f5cd343354dcfbea44215d4c2c38`). The plugin runs from
  a pinned copy of `integrations/` identical to this commit's `integrations/opencode/src`.
- Baseline for comparison: the 0.7.23 paired rerun
  (`benchmarks/results/v2717-opencode-adoption-paired-2026-10-26.json`).

  | Arm | Resolved | Cost | Cost per resolved | Mean first step |
  |---|---|---|---|---|
  | on | 9/10 | $2.9445 | $0.3272 | 9731.2 |
  | off | 9/10 | $2.8896 | $0.3211 | 8681.7 |

  That rerun's per-pair first-step deltas were 479 (no frame) in p03 and p09 and 1164-1223
  (frame injected) in the other eight pairs.

## Measurements taken before this file (disclosed)

- **Idle smoke attribution:** the `AHI-211` method. One session on the V1-0415 smoke repository,
  OpenCode 2.1.3, `claude-opus-5-5`, the same smoke charter; plugin present minus plugin absent, same
  day. On 2026-10-26: off 6926, 0.7.26 on 7023 first-step prompt tokens (+97), on a clean smoke
  checkout. An earlier pair on a dirty smoke checkout read the same 6926 / 7023.
- **Unlock smoke:** one session whose charter asked the agent to call `corvint_context`, then
  `tools.corvint_status` through `execute`. The call succeeded and reported adapter 0.7.26, and the
  agent saw the corvint skill listed after the first call.
- No trial session of this rerun ran before this commit.

## Design

- **Tasks:** the same ten admitted V1-0415 pairs `p01`..`p10`, with the instance ids, image digests
  and source snapshots of the V1-0415 receipt.
  - The extracted `src.tar`, `instruction.md` and `charter.md` are reused.
  - Only grading images already present on the Windows box are used; no image is pulled. All ten
    were present when this file was written (`docker.exe image inspect`). A pair whose image is
    missing at grading time is stopped and reported, and the rest continue.
  - Each pair gets fresh `r1` (on) / `r2` (off) checkouts. The `on` checkout is indexed with the
    pinned binary before the session, outside the agent budget.
- **Arms:** identical to the 0.7.23 rerun except for the adapter.
  - OpenCode 2.1.3, `opencode run --standalone --auto -m anthropic/claude-opus-5-5`, the charter
    attached, prompt `Follow the attached charter.`
  - Build agent with 50 steps and a 570 s wall clock.
  - Shell, webfetch and websearch denied.
  - Isolated HOME and an `XDG_CONFIG_HOME` per arm; the off arm has no plugin.
  - Both arms are rerun. Arms of a pair run sequentially in the foreground: `on` first in odd pairs,
    `off` first in even pairs. Pairs run in order `p01`..`p10`. Session titles are
    `v2717c-pNN-{on,off}`.
- **Grading:** the same local docker mirror of the V2 verifier on the Windows box
  (`swebp_v2_eval.py`), run in the foreground. The patch is the agent's `git diff --cached --binary`.

## Metrics

Read from the OpenCode session store as in the 0.7.23 receipt (`trial_receipt.py`), per arm and per
pair:

- **Sessions with injected context.** An `on` session counts as injected when its first-step prompt
  tokens exceed the same pair's `off` first-step prompt tokens by more than the idle overhead plus
  50 tokens (that is, by more than 147 at the 97-token idle figure). Cross-check, reported but not
  decisive: the `frame_probe.mjs` fake host replays the same prompt hook against the indexed `on`
  checkout after the session and reports whether a frame is produced and its bytes.
- **Injected tokens** per injected session: the per-pair first-step delta minus the idle overhead.
- **First-step delta:** mean per-pair `on` minus `off` first-step prompt tokens.
- **Resolved** count per arm.
- **Cost** (USD) per arm and **cost per resolved** task.
- **Cache read and cache write** tokens per arm, plus input, output, wall seconds, tool calls and
  Corvint tool calls (direct `corvint_*` parts plus `execute` parts whose code contains
  `tools.corvint_`).
- **Idle overhead,** re-measured once after the trial by the smoke method against a same-day off run.

## Hypotheses

- **H1 (context received):** at least 6 of the 10 `on` sessions have injected context.
- **H2 (idle overhead):** the idle overhead is at most 150 tokens both before (+97) and after the
  trial.
- **H3 (cost):** `on` cost per resolved task is at most `off` cost per resolved task on this rerun.
- **Verdict:** PASS only if H1, H2 and H3 all hold. Otherwise NOT MET, naming the failed hypotheses.
- Resolve rate is reported, but no resolve-rate claim is made in either direction. At N=10 with a
  9/10 baseline in both arms, this set cannot show resolve-rate usefulness, and the 60-pair trial
  stays held.

## Stop rules and limits

- **Hard stop:** $12 total lane API spend, including the smoke sessions already spent (about $0.14).
  If spend passes $12, no further session starts; the receipt records the pairs completed and the
  hypotheses are reported on completed pairs only, marked incomplete.
- A session that hits the 570 s wall or 50 steps counts as unresolved for that arm and is not rerun.
- An arm whose session never starts (no session row in the store) is rerun once; any such rerun is
  recorded.
- Pairs, metrics, hypotheses and thresholds are not changed after any result is seen. N = 10 cannot
  detect small effects, so no significance claim is made. Anything not measured is reported as
  `NOT_OBSERVED`.
