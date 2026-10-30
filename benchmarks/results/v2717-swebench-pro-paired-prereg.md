# Preregistration: SWE-bench Pro paired rerun for V1-2717 (OpenCode adapter 0.7.22)

Composed 2026-10-26 by the V1-2716/V1-2717 lane before the first agent session. Committed on the
lane branch `claude/v2716-v2717-retrieval-adoption` before any session ran; the receipt records the
commit and this file's SHA-256.

## Candidate under test

- OpenCode adapter `integrations/opencode/src` from the lane change (adapter 0.7.22, `AHI-210`): the
  `user-prompt` hook injects a receipt-linked orientation frame (state, up to eight evidence paths,
  degradations; no packet rows, no handles) instead of the full repository-data packet, and the
  `corvint_context` tool description states when to call it. The pinned corvint binary is built from
  the same lane tree (darwin/arm64, sha256 recorded in the receipt) and includes `GPK-V0-090/091`.
- Baseline for comparison: V1-0415's paired trial (`v0415-swebench-pro-paired-2026-10-25.json`,
  adapter 0.7.19, corvint 1.0.0-rc.3 build 0): on 9/10 at $3.1017, off 9/10 at $2.6304, zero Corvint
  tool calls, on-arm first-step prompt tokens about 3.4k above off.

## Design

- Tasks: the same ten admitted V1-0415 pairs `p01`..`p10` (instance ids, image digests and source
  snapshots as in the V1-0415 receipt), reusing the extracted `src.tar` and `instruction.md`. No new
  image is pulled. Fresh `r1`/`r2` checkouts per pair; the `on` checkout is indexed with the pinned
  binary before the session, outside the agent budget.
- Arms: identical to V1-0415 except the adapter and binary. OpenCode 2.1.3, `opencode run --standalone
  --auto -m anthropic/claude-opus-5-5`, build agent 50 steps, 570 s wall clock, shell/webfetch/websearch
  denied, isolated HOME, `XDG_CONFIG_HOME` per arm. Both arms of a pair run concurrently; pairs run
  sequentially. Both arms are rerun (the `off` arm is not reused from V1-0415) so day-to-day model
  variance affects both arms alike.
- Grading: the same local docker mirror of the V2 verifier on the Windows box (`swebp_v2_eval.py`),
  patch replay from the agent's `git diff --cached --binary`.
- Metrics per arm (from the OpenCode session store, as in `trial_receipt.py`): resolved count, cost
  (USD), cost per resolved task, output/cache-read/cache-write tokens, first-step prompt tokens,
  Corvint tool calls by name, wall seconds.

## Hypotheses (acceptance criteria V1-2717)

- H1 (adoption): Corvint tool calls > 0 in at least one `on` session. Secondary: the share of `on`
  sessions with at least one `corvint_context` call.
- H2 (overhead): `on` minus `off` first-step prompt tokens, averaged over pairs, is <= 500 tokens
  (V1-0415: about 3,400).
- H3 (cost): `on` cost per resolved task <= `off` cost per resolved task on the same rerun.
- Resolve rate is reported but, at N=10 and a 9/10 baseline, no resolve-rate claim is made in either
  direction.

## Preregistered secondary analysis

- "Harder half": the five pairs with the highest V1-0415 `off`-arm cost (`p04` 0.450, `p03` 0.385,
  `p10` 0.364, `p05` 0.314 (unresolved by both arms), `p07` 0.245), fixed here from the baseline before
  this rerun. H1-H3 are reported for this half separately; it is where a context packet could plausibly
  change behaviour.

## Stop rules and limits

- Hard stop: $60 total lane API spend (ContextBench runs are offline and cost nothing). The run stops
  early and the receipt records the pairs completed if a pair's two sessions exceed $3.
- A session that hits the 570 s wall or 50 steps is counted as unresolved for that arm, not rerun.
- N = 10 cannot detect small effects; no significance claim. A remaining negative result (no tool
  calls, or higher cost per resolved task) is reported as NOT MET.

## Amendment (2026-10-26, after the trial)

The adapter version label above read 0.7.20 when this file was committed
(d94999635a65416112c13d5d4c28361e17f0fd27, sha256
505dc9fd2aa3ec1df80e0520b2583f4903806691854e8eacd9126a679680d96f). Versions 0.7.20 and 0.7.21
shipped in beamfall/corvint#1845 while the lane was open, so the lane adapter is published as 0.7.22.
This amendment changes only that label and adds this section; no hypothesis, subset, limit or stop
rule changed. The receipt records both digests.
