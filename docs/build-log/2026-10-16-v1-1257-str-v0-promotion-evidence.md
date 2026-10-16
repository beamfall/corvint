# 2026-10-16 STR-V0 promotion evidence: measurements met, status stays experimental (V1-1257)

Human-owned intent: `docs/specs/session-test-runner-v0.md`, "Acceptance evidence: promotion and
kill criteria". Promotion from experimental needs:

- the STR-V0-013 parity report;
- 20 merged PRs through the queue on the new CI call site, with complete forward rows and no trip;
- median and p90 PR wall time and runner-minutes per merged PR, before and after.

Balanced placement is promoted separately, against its own bars.

This entry continues `2026-10-14-v1-1257-str-promotion-evidence.md`, which stood at 3/20 PRs and
left a shell declaration gap; V1-1373 (merged `c77b51c6`, 2026-10-14T19:14:09Z) closed that gap.

| Item | Value |
| --- | --- |
| BASE | `08195a48314d9cf0a8bdeeeb92ba8ab932b52c9a` |
| Snapshot | forward rows to 2026-10-16T14:13:40Z; merged PRs to #960 (2026-10-16T14:14:17Z) |
| Access | read-only (`gh api`, `gh run download`); nothing was triggered and no setting was changed |
| Evidence | `evidence/v1-1257-2026-10-16/` |
| Reused scripts | `evidence/v1-1257/scripts/` (rows, timing, statistics), unchanged |
| Lane scripts | `evidence/v1-1257-2026-10-16/scripts/` |

## Verdict per acceptance criterion

| AC | Verdict |
| --- | --- |
| AC1 parity | Met. 50 local pairs and 142 hosted shadow runs. 0 `UNEXPLAINED`, 0 `MISS`, 0 `TARGET_MISMATCH`. |
| AC2 20-PR bar | Met, 20 of 20: #914-#938 after the trip cleared, all with complete `CLEAN` PR and queue rows, no shadow limits and no AFP-V0-030 trip during their span. One owner reading is flagged below. |
| AC3 timing | Met as observed: three windows with their sample sizes. Hosted billing and queue time are `NOT_OBSERVED`. |
| AC4 promotion | NOT done. AC1 to AC3 are met, but `implemented` needs executable evidence for every in-scope requirement, and STR-V0-004's versioned capability manifest and STR-V0-011 have none. The status stays `experimental`, with the evidence cited. |
| AC5 balanced placement | Unpromoted. The local bar is `NOT_RUN`; there is no accepted AFP-V0-028 amendment. No `.github/` edit. |
| Kill criteria | Not fired: no unexplained parity difference, no runner-caused queue `MISS`, and no balanced-placement measurement. |

## AC1: STR-V0-013 parity

### Local replay

The tool and corpus are the ones decision 0526 retained:

- tool: `tools/str-parity`;
- corpus: `tools/str-parity/testdata/pairs-645304a2.txt`, 50 first-parent pairs ending at `645304a2`;
- binaries: `corvint`, `gate-affected-select` and `corvint-pr-tests`, all built from BASE;
- declaration: `.github/cishards/test-commands.json`, sha256
  `fd182c12dec41bba13fe869f1714f51f97c29467b885f96ab0018c7f53adf292`.

The corpus ran in six chunks so that no command ran in the background:

| Chunk | End | Pairs | Seconds | rc |
| --- | --- | --- | --- | --- |
| 1 | `645304a2` | 10 | 530 | 0 |
| 2 | `686fb1ac` | 8 | 360 | 0 |
| 3 | `5e664ce9` | 8 | 263 | 0 |
| 4 | `a3607ec5` | 8 | 339 | 0 |
| 5 | `284c4312` | 8 | 339 | 0 |
| 6 | `e5298bc5` | 8 | 351 | 0 |

Load averages were 43 to 61 on 12 CPUs. Outputs are in `parity/c1..c6.json` and `.err`.

Result: 50 pairs and 307 differences, 0 of them `UNEXPLAINED`.

| Category | Count |
| --- | --- |
| `ci-admission-no-qualification` | 95 |
| `ci-closed-environment` | 50 |
| `ci-exec-confine` | 50 |
| `local-json-flag` | 50 |
| `str-v0-003-unknown-widening` | 12 |
| `str-v0-004-declared-language` | 50 |

Scopes, as `corvint test --profile local` against `script/gate-affected.sh`:

| `corvint test` | Driver | Pairs |
| --- | --- | --- |
| NARROWED | NARROWED | 46 |
| FULL | NARROWED | 3 (the STR-V0-003 widenings) |
| FULL | FULL | 1 |

No pair is `INCOMPLETE`.

Against the 2026-10-12 report (283 differences, 26 of them declared-language), the change is the
declared-language row, which is now 50.

- Every pair's local plan declares `make shell-test`: 37 alone, and 13 with `make host-adapter-test`.
  The shell command is the V1-1373 shell declaration.
- The CI profile declares no command. This is because slice 3 (STR-V0-012) routes ci-profile
  declared commands to `Checks` instead of running them.

Both differences are explained by accepted requirements, and none needs a fix.

### Hosted shadow against the driver

The hosted comparison is `shadow-parity.tsv`, produced by `scripts/parity.sh` over every
`pull_request` run that retained `corvint-test-shadow-<k>` and `ci-outcomes-pull_request-<k>`.
This `parity.sh` copy differs from the earlier lane's in one way: a missing document produces a
`NOT_OBSERVED` row instead of a stop.

Coverage: 142 runs and 1136 shard rows (142 x 8). 1115 rows are `subset`. 21 are `NOT_OBSERVED`:

- runs 38753717156 and 38891438019 retained no outcomes;
- runs 38677974865 and 38796527286 each lack 2 outcomes;
- run 38684092732 retained 7 of 8 shadow and 7 of 8 outcome documents.

Results:

- 0 `TARGET_MISMATCH`, 0 `UNEXPLAINED`, 0 `MISS`;
- selected-not-executed is 0;
- the argv prefix is identical in every observed comparison;
- all 16 driver package failures were selected (failed-not-selected 0);
- executed-not-selected totals 14,265, because the driver still runs every shard in full.

Among observed shards, shadow exit 3 occurs only for the pre-V1-1373 shell limit: 184 shards in 23
runs. Every other observed shard exits 0. Slice identity was not rechecked; the 2026-10-14 entry covered it.

## AC2: switch, epoch and the 20-PR count

### Switch and epoch

- Epoch 1 was advanced by slice 3 (`8b4ef6fa3`), merged through #815 (`1f75ce55277d`,
  2026-10-12T17:16:45Z).
- The shadow call site went live with #868 (`5163d1ed4b61`, 2026-10-13T23:57:00Z, AFP-V0-047).
- `CORVINT_PR_FORWARD_EPOCH` is still `"1"` at BASE, and `CORVINT_TEST_SHADOW_SOURCE` is still
  `5163d1ed`.

### Forward rows

`forward-rows.tsv` records every retained `ci-forward-*` row with its `created_at`. There are 226
rows, all in epoch 1:

- 219 `CLEAN`, 7 `INCOMPLETE`, 0 `MISS`, 0 `FLAKY`;
- outcome sources: 133 `pull_request`, 75 `merge_group`, 18 `reuse`.

All 7 `INCOMPLETE` rows fall between 05:32 and 06:08Z on 2026-10-15:

- #905 pr, #907 queue, #908 queue twice (#908 closed unmerged), #909 pr, #910 pr twice.

Each lacks one shard whose job failed at the hosted "Set up job" step: runner infrastructure, not
`corvint test`. Run 38684092732 for #910 also had a real Go test failure in shard 5. The PR rows
were later replaced by `CLEAN` rows from reruns of the same PR and sha.

### AFP-V0-030 replay

AFP-V0-030 has no runtime implementation yet; `.github/forward-rows/bar.jq` computes it as an
advisory count. `bar-replay.tsv` replays `bar.jq` over the newest-first row names at every row's
creation time.

- It reads `TRIPPED` (3 to 7 `INCOMPLETE`, 0 `MISS`) at 22 replay points, from 2026-10-15T05:46:08Z to
  13:23:10Z.
- It reads clear at every point since.
- Latest state: 20 complete, 1 failing, clear.

### Counted set

The 20 PRs are the first 20 merged after the trip cleared, through 2026-10-15T23:11:12Z
(`merged-rows-post-clear.tsv`):

#915, 914, 916, 917, 918, 919, 920, 921, 923, 922, 924, 925, 926, 928, 931, 929, 933, 934, 935, 938.

Every one of them:

- merged through the queue;
- has complete `CLEAN` PR and queue rows;
- carries no shadow limit;
- has 0 of 49 replay points `TRIPPED` during its span.

**N = 20 of 20.**

Two further sets were checked:

- **First 20 epoch-1 PRs with complete rows** (#861 to #897, `merged-rows-first20.tsv`). All rows
  are `CLEAN`, and 0 of 63 replay points trip through 2026-10-15T01:22:26Z. It is not counted,
  because all 20 carried the pre-V1-1373 shell limit (`shell-limited-prs.txt`): their shadow plans
  exited 3, and `row.jq` classes a limit-only row `CLEAN`.
- **All merged PRs with complete rows.** 83 PRs (`merged-rows.tsv`), 63 of them without limits.

Merged PRs without rows:

- #862 to #870 predate the rows or the shadow;
- #880, 885, 894, 932, 943 and 954 touch `.github/`, which `collect.sh` excludes by design. The log
  of run 38600258028 shows this for #880.

## AC3: timing

The method is the 2026-10-14 entry's, unchanged:

- `timing.py` attributes runs to merged PRs by head and merge sha;
- `stats.py` takes the median as the mean of the two middle values, and p90 as nearest rank;
- wall time is the final PR `ci.yml` run's first-attempt duration;
- runner-minutes sum the job durations over all attempts.

Runs were listed for 2026-10-11 to 2026-10-16: 1554 runs, 1293 attributed to 137 merged PRs
from #818. Jobs were fetched with `scripts/fetch-jobs.sh`. The before window reproduces the
2026-10-14 numbers exactly.

Values are median / p90, in minutes. n=20 for the before window, 25 for queue pre-shadow and 91 for the shadow window.

| Metric | Before (#818-839, n=20) | Queue, pre-shadow (#840-868, n=25) | Shadow (#869-960, n=91) |
| --- | --- | --- | --- |
| PR `ci.yml` wall | 22.9 / 24.8 | 24.1 / 37.8 | 28.8 / 53.6 |
| Queue `ci.yml` wall | none | 21.1 / 26.4 (n=23) | 22.9 / 42.2 |
| Runner-min, all workflows | 251.7 / 362.1 | 403.5 / 627.9 | 341.2 / 610.1 |
| Runner-min, `ci.yml` | 250.7 / 360.8 | 401.9 / 625.5 | 339.6 / 607.2 |
| Final PR run runner-min | 153.5 / 158.2 | 153.0 / 158.3 | 162.4 / 175.2 |
| Queue run runner-min | 0 / 0 | 139.1 / 146.2 | 149.1 / 160.3 |
| Open to merge | 23.8 / 53.7 | 53.6 / 122.7 | 78.1 / 493.4 |

The whole after window (#840-960, n=116) reads:

- PR wall 27.5 / 48.1;
- runner-minutes, all workflows, 344.2 / 616.1.

One job in the shadow window was not completed: run 38580551537 (#880) left a merge-group job
`in_progress` but with a `completed_at`, a hosted anomaly.

Rows are in `timing-before.tsv`, `timing-queue-preshadow.tsv`, `timing-shadow.tsv`,
`timing-after.tsv` and `timing-runs.tsv`.

### Direct shadow step cost

`shadow-step-cost.tsv` covers 91 final PR runs. The shadow steps are capture, build, plan, retain
and post; the Go tests step is excluded.

- mean 256.2 s, median 261 s and p90 275 s of runner time per run;
- about 4.3 runner-minutes per PR run;
- most of it is the pinned planner build, about 24 s per shard.

### Confounders

- The merge queue arrives at #840.
- AFP-V0-024 reuse arrives at #869.
- Nothing narrows: the after window measures the cost of the shadow, not narrowing.

`NOT_OBSERVED`:

- billed minutes;
- runner queue time;
- runs before 2026-10-11;
- wall time of pushes before the final head.

## AC4: promotion not made

AC1 to AC3 are met, but the status cannot honestly move. `docs/SPEC-DRIVEN-DEVELOPMENT.md`
defines `implemented` as all in-scope requirements having executable evidence, and two
requirements lack it:

- **STR-V0-004** requires a versioned capability manifest per adapter, the LPCV-V0-021 subset. Its
  traceability row says there is "no versioned LPCV-V0-021 capability manifest yet (partial)".
- **STR-V0-011** (no reader other than `verify-shards`) holds "by construction"; no test asserts it.

The independent review found this (finding 1 below). Building the manifest is adapter code outside
this evidence lane. Narrowing STR-V0-004 would amend an accepted requirement, which needs the owner.

What changed instead:

- **Status:** `Delivery status` stays `experimental`. Its qualifier now cites this entry and names
  the gap; the digest status matches it, and `README.md` and `INDEX.json` are regenerated.
- **Digest "Blocked on" line:** it names the STR-V0-004 and STR-V0-011 evidence, the AFP-V0-029 bar
  for CI narrowing, and the balanced-placement bars. The bar was met at 2026-10-14T17:59:52Z and
  reads NOT MET again at 1/5 failing rows.
- **Rollout step 3:** it records the promotion measurements.
- **Traceability:** the STR-V0-013 and STR-V0-016 cells cite this evidence; hosted shadow runs are
  now observed.

No requirement text changed.

Rollback: revert this entry's commit. It changes no code, CI configuration or selection.

## AC5: balanced placement

- **Locally:** the 20% p90 slowest-shard bar is `NOT_RUN`. The host was contended, with load
  averages of 30 to 61 on 12 CPUs, and any executed comparison would be dominated by noise.
- **In CI:** it stays computed-only. There is no accepted AFP-V0-028 amendment, so there is no CI
  bar to measure against.
- **Status:** unpromoted. It is not removed, because neither bar has been measured to fail.

## Owner decisions flagged

1. **AFP-V0-030 reading.** Should an advisory trip that has since cleared bar promotion only while
   it is tripped, or should it latch?
   - The non-latching reading is the one taken here. It counts 20 PRs after the clear.
   - A latching reading would need a new epoch (an `.github/` change) and 20 more PRs.
   - The trip came from hosted "Set up job" failures, with 0 `MISS`.
2. **Wording.** The ticket and the charter say "delivered". `docs/SPEC-DRIVEN-DEVELOPMENT.md` has no
   such status; the closest is `implemented`.
3. **STR-V0-004 and STR-V0-011.** Choose one:
   - fund the versioned Go adapter capability manifest and a test for STR-V0-011, then promote; or
   - accept an amendment that scopes the manifest out of V0, then promote on this evidence.

## Residuals

- **Hosted job failures make rows `INCOMPLETE`.** A job that fails at hosted "Set up job" makes the
  attempt-1 row `INCOMPLETE` and counts toward the advisory trip.
  - Expected: infrastructure failures are separable from selector or runner incompleteness.
  - Actual: they read the same.
  - Reproduction: run 38686348902 (#908 queue).
- **AFP-V0-029 bar.** It is NOT MET again at 1/5 failing rows, which gates CI narrowing.
- **Shadow pin.** `CORVINT_TEST_SHADOW_SOURCE` is still `5163d1ed`, which predates V1-1373. Hosted
  shadows still plan with the declaration read from the tested checkout, so the shell limit cleared
  without moving the pin.

## Review

One Codex round (`codex exec -s read-only`, diff `08195a48..13ab87aa5`) reported three findings.

1. **High.** `implemented` contradicted STR-V0-004's partial traceability row and the vocabulary's
   "all in-scope requirements" definition.
   - Fixed: the status was reverted to a qualified `experimental` (AC4 above). STR-V0-011 was
     found in the same pass.
2. **Low.** The hosted row count included the TSV header, and the claims of identical argv and
   exit 0 were not limited to observed shards.
   - Fixed: 1136 rows, and the claims are now limited to observed comparisons.
3. **Low.** The tripped replay points carry 3 to 7 `INCOMPLETE` rows, not 3.
   - Fixed.

Codex reproduced these:

- 219 `CLEAN` and 7 `INCOMPLETE` forward rows;
- the counted 20 PRs' rows;
- the 307 parity differences over 50 pairs;
- the timing medians and p90s;
- the shadow-step cost.

It confirmed that no accepted requirement text changed.
