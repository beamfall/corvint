# Preregistration: SWE-bench Pro paired rerun for V1-2717 (OpenCode adapter 0.7.23)

Composed 2026-10-26 (UTC) by the `claude/v2717-opencode-adoption` lane before any trial session. It is
committed on that branch in the same commit as the candidate adapter, before the first trial session;
the receipt records that commit and this file's SHA-256.

## Candidate under test

- OpenCode adapter `integrations/opencode/src` at this commit (adapter 0.7.23, `AHI-211`):
  - Only `corvint_context` is a direct tool. `corvint_expand`, `corvint_record_outcome` and
    `corvint_status` stay in the Code Mode catalog.
  - The tool and skill descriptions are one sentence each.
  - For a confident packet (`READY`, abstention inactive, at least one result with a path), the
    `user-prompt` hook injects a bounded candidate frame of at most five explained candidates. Each
    candidate has a path, line and symbol, plus the authority, confidence and reason behind it. The
    frame also carries the packet's uncertainty and degradations, inside the envelope, under a fixed
    line saying the candidates are not proof.
  - Any other packet injects nothing.
- The pinned corvint binary is built from the same tree (darwin/arm64, `1.0.0-rc.3 (build 0)`); its
  SHA-256 is recorded in the receipt.
- Baseline for comparison: the V1-2717 0.7.22 paired trial (`v2717-swebench-pro-paired-2026-10-26.json`).

  | Arm | Resolved | Cost | Cost per resolved |
  |---|---|---|---|
  | on | 9/10 | $2.8783 | $0.3198 |
  | off | 9/10 | $2.5099 | $0.2789 |

  That trial had zero Corvint tool calls, and the on arm's first-step prompt was +1314 tokens.

## Measurements taken before this file (disclosed)

- **Smoke attribution:** done while designing the change.
  - Setup: one session on the V1-0415 smoke repository, OpenCode 2.1.3, `claude-opus-5-5`, the same
    charter. The figure is the first-step prompt tokens with the plugin present minus with it absent.
  - Results on 2026-10-26: the off arm read 6926 in four runs. 0.7.22 measured 8011 (+1085) and the
    frozen 0.7.23 measured 7405 (+479).
  - Design variants measured in between are listed in the build log.
- **Frame calibration on p01:** the p01 source was indexed outside the trial checkouts. The hook was
  driven by a fake host, with no model session, to size the frame: READY, five candidates, 1776 bytes
  in total including the `AHI-016` disclosure.
- No trial session of this rerun ran before this commit.

## Design

- **Tasks:** the same ten admitted V1-0415 pairs `p01`..`p10`, with instance ids, image digests and
  source snapshots as in the V1-0415 receipt.
  - The extracted `src.tar` and `instruction.md` are reused, and no new image is pulled.
  - Each pair gets fresh `r1`/`r2` checkouts. The `on` checkout is indexed with the pinned binary
    before the session, outside the agent budget.
- **Arms:** identical to V1-0415 and the 0.7.22 rerun except for the adapter and the binary.
  - OpenCode 2.1.3, `opencode run --standalone --auto -m anthropic/claude-opus-5-5`.
  - Build agent with 50 steps and a 570 s wall clock.
  - Shell, webfetch and websearch denied.
  - Isolated HOME and an `XDG_CONFIG_HOME` per arm.
  - Both arms are rerun.
- **Deviation from the earlier runs:** the two arms of a pair run sequentially, not concurrently,
  because this lane may not run background jobs. To balance order effects, `on` runs first in odd
  pairs and `off` runs first in even pairs. Pairs run in order `p01`..`p10`.
- **Grading:** the same local docker mirror of the V2 verifier on the Windows box
  (`swebp_v2_eval.py`). The patch is replayed from the agent's `git diff --cached --binary`.
- **Metrics per arm,** read from the OpenCode session store as in `trial_receipt.py`:
  - resolved count
  - cost (USD) and cost per resolved task
  - output, cache-read and cache-write tokens
  - first-step prompt tokens
  - wall seconds
  - Corvint tool calls

  Corvint tool calls are the sum of two counts:
  - direct tool parts named `corvint_*`
  - Code Mode Corvint calls: `execute` tool parts whose input code contains `tools.corvint_`

## Hypotheses (V1-2717 acceptance criteria)

- **H1 (adoption):** Corvint tool calls > 0 in at least one `on` session. Secondary: the share of
  `on` sessions with at least one Corvint call.
- **H2a (standing overhead):** the smoke-attribution plugin overhead is at most 500 tokens.
  - It was measured at +479 before this file. It is re-measured once after the trial by the same
    method, against a same-day off run.
  - H2a holds only if both measurements are at most 500.
- **H2b (per-task overhead, descriptive bound):** the per-pair `on` minus `off` first-step prompt
  tokens, averaged over pairs, is at most 1200 tokens.
  - Rationale for the bound: the standing cost, plus a candidate frame at its bound. The frame is a
    1024-byte object plus the fixed line, envelope and `AHI-016` disclosure, about 1900 bytes.
  - The 0.7.22 rerun measured +1314.
- **H3 (cost):** `on` cost per resolved task is at most `off` cost per resolved task on this rerun.
- **V1-2717 passes** only if (H1 or H2a) and H3 hold.
- Resolve rate is reported, but no resolve-rate claim is made in either direction. At N=10 with a
  9/10 baseline in both arms, this set cannot show resolve-rate usefulness.

## Preregistered secondary analysis

- **Harder half:** the same five pairs fixed for the 0.7.22 rerun from the V1-0415 `off`-arm cost:
  `p04`, `p03`, `p10`, `p05` and `p07`. H1 and H3 are reported for this half separately.

## Stop rules and limits

- **Hard stop:** $60 total lane API spend, including the $0.43 of smoke sessions already spent.
- The run also stops early if a pair's two sessions exceed $3. The receipt then records the pairs
  completed, and H3 is reported on completed pairs only, marked incomplete.
- A session that hits the 570 s wall or 50 steps counts as unresolved for that arm and is not rerun.
- An arm whose session never starts (no session row in the store) is rerun once. Any such rerun is
  recorded.
- Pairs, metrics, hypotheses and thresholds are not changed after any result is seen. N = 10 cannot
  detect small effects, so no significance claim is made. A negative result is reported as NOT MET.
