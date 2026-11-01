# CEM Reviewer Trial V0

Owner: Russell Lewis
Date: 2026-09-03
Requirement prefix: `CRT-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: the V4 status record ("Run 30 Beamfall CEM lanes versus 30 controls. Require at
least 20% fewer missed-evidence findings, at least 70% citable material hunks, and under 5% incorrect
verifier hard failures"), `benchmarks/README.md` (partitions and the first-observation rule),
`docs/specs/confidently-wrong-trial-v0.md` (arms, dispatch, canonical output, reply grammar),
`docs/CHANGE-EVIDENCE-MAP.md` and `internal/cem` (the cem/1.0 workflow under test since 2026-10-24; cem/0.1 before), `AGENTS.md`
invariants 2 and 4, and decision 0523 P4
(`docs/decisions/0523-cem-1-0-rc3-verifier-computed-support-2026-10-11.md`: precision scoring, the
registration file, the shipped-profile treatment arm, and no run before decision 0523 P2 has a
baseline).

## Agent digest
- Claim: `tools/cem-trial` runs one agent over frozen Beamfall changes under control, treatment and seeded arms and scores base-reachable withheld test-or-spec evidence.
- Status: proposed/experimental; the owner accepted the 2026-10-01 amendments to CRT-V0-004, CRT-V0-005 (`seeded` arm), CRT-V0-007, CRT-V0-008 and CRT-V0-012 on 2026-10-25 (V1-0836 closeout).
- Exists: `tools/cem-trial` (`select`, `run`, `score`), the five-change pilot manifest under `tools/cem-trial/testdata/pilot`, and `tools/cem-trial/testdata/fake-agent.sh`; added 2026-10-11 (V1-1261, accepted under the owner's standing auto-accept (2026-10-11)): the cited-not-gold precision rate (CRT-V0-013) and the registration file and its order check (CRT-V0-014); added 2026-10-28 (V1-2948): every summary pairs each arm with the mechanical stem baseline and says plainly when it is not beaten (CRT-V0-017, optional `stem` worklist arm), and scores evidence discovery apart from adjudicated defect detection, which is `NOT_OBSERVED` (CRT-V0-018); the compact CEM worklist arm is not built.
- Blocked on: an arm builder for the shipped profile's artifact (CRT-V0-015), which needs decision 0523 P2 to land and have a baseline; and the owner-commissioned held-out run, run on 2026-10-24 (N 57, valid; neither hypothesis supported; `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-trial-heldout-run.md`). Since 2026-10-24 (V1-0836) the `treatment` and `seeded` arms build an all-`unknown` cem/1.0 worklist for development and pilot runs (CRT-V0-005), and the ten-pair development run froze the held-out pair count at 57 (CRT-V0-008). Since the 2026-10-24 owner decision a registered run may carry that map (CRT-V0-015). The real-agent pilot is run and valid (2026-09-04) but its treatment carried no evidence and three of its five changes had gold no arm could cite; no run before decision 0523 P2 has a baseline is outcome evidence.
- Read next: Requirements (selection, withheld patch, arms, metrics, CRT-V0-012..018); Power; Non-goals; Stopping and publication.

## Intent and scope

The V4 status record names a numeric gate over 30 Beamfall CEM lanes against 30 controls, and nothing
in the tree runs it: `tools/cw-trial` measures confidently-wrong claims over retrieval and change
tasks, not evidence citation over a change under review, and `benchmarks/dogfood_measure.py` leaves
outcome metrics `NOT_OBSERVED`. This slice is that dispatcher plus its scorer.

Affected user: the repository owner reading V4's CEM gate. Measurable job: for one frozen set of
Beamfall changes, produce the mean fraction of the change's own withheld test-or-spec evidence the
agent failed to cite, per arm, with a paired 95% interval; the fraction of material hunks the
treatment arm's citations left supported; and the rate at which `cem verify` hard-failed a citation
an independent replay says was valid.

Definition encoded: a **missed-evidence finding** is a gold path the agent did not cite. Gold is the
set of test-or-spec files the change itself touched, withheld from the presented patch. The agent
sees the change's source hunks and the repository at the base revision; the evidence it is asked to
name is exactly what the change's author also wrote and the harness removed.

## Requirements

- `CRT-V0-001`: Unit and selection. One change is one non-merge Beamfall commit `C`, and its lane
  runs at `C^`. The selection rule is frozen verbatim in the manifest (`selection_rule`):
  non-merge commits since `--since`, oldest first, touching between 3 and 12 text files, with at
  least one modified non-test source file (`.go|.ts|.tsx|.swift|.py`) and at least one
  test-or-spec file (`_test.go`, `*.test.ts`, `*.test.tsx`, `*Tests.swift`, `_test.py`,
  `docs/specs/**`); the presented patch is the source-file hunks alone in the canonical CEM diff
  form (myers, no indent heuristic, three context lines, no inter-hunk context; the options
  `cem prepare` maps with, `internal/cem/gitauth`; amended 2026-10-31 by V1-2619), at most 64 KiB, over
  which `cem begin` parses between 3 and 20 hunks; a change is rejected when any source hunk
  contains a gold path literal; candidates are ordered oldest-first and selected at index
  `floor(i*N/n)`. The manifest records the population `N`, the rule string, every commit SHA, and
  each change's patch sha256; `run` recomputes those digests and refuses the manifest on mismatch,
  and refuses a manifest that lists one change id twice.
- `CRT-V0-002`: Pairing. Every change runs in both arms; the estimate is within-change. The arm order
  per change is a seeded shuffle over `--seed`, recorded on every lane as `arm_order`. Each arm runs
  in its own disjoint clone and its own agent session; no state crosses arms.
- `CRT-V0-003`: Withheld-patch construction. The presented patch `P(C)` contains only the
  source-file hunks of `C`, in the canonical CEM diff form of CRT-V0-001, so every presented hunk
  starts on the line `cem prepare` maps it to; git's default indent heuristic slid one hunk by a
  line and errored all six lanes of a held-out pair (V1-2619). The test-or-spec hunks are withheld
  and become gold. Both arms receive byte-identical `P(C)`, its sha256, and a harness-generated
  numbered hunk list in the order `cem begin` reads them.
- `CRT-V0-004`: Ground truth. Primary gold for `C` is the set of test-or-spec paths `C` itself
  touched **and that exist at `C^`** (git status `M` or `D`; amended 2026-10-01, accepted by the owner 2026-10-25);
  secondary gold is the one-based line spans of those files' added lines, read from `C`'s own
  diff. A test-or-spec path `C` adds is recorded on the task as `gold_unreachable` and is never
  scored: the lane runs at `C^` and the reply grammar holds every cited path to that revision,
  so such an item is missed by construction in every arm. The 2026-09-04 pilot scored three of
  its five changes at miss 1.0 in both arms on exactly such files (c03, c04, c05, whose gold
  spans begin at line 1); its manifest keeps that gold as history and is not rescored.
  Non-circularity: a change is rejected at selection when any source hunk contains a gold path
  literal, reachable or not. The known gameability is stem-convention guessing; the mitigation is
  a mechanical stem-baseline recorded per change at selection time, whose miss rate the report
  carries beside every arm so an arm is read as lift over that floor, never over zero.
- `CRT-V0-005`: (Amended 2026-10-11: V1-1261, accepted under the owner's standing auto-accept (2026-10-11)) Arms. `control` receives the prompt skeleton, `P(C)`, the numbered hunk list, and a
  read-only clone at `C^` with its history. `treatment` receives an identical prompt, model, effort,
  timeout, tool access, and clone, plus exactly three artefacts: the cem/0.1 map from
  `corvint cem begin --patch P(C)`, the `cem status` worklist, and `cem report`. Leak control: the
  map MUST be built with `cem begin` over the withheld bytes and never with
  `cem prepare --base C^ --target C`; the clone is scrubbed of `.corvint/` and holds no commit at or
  after `C` (a fetch of exactly `C^`, detached, never a shared clone of the source); the harness
  asserts `git cat-file -e C` fails in the clone and errors the change's lanes if it does not. The
  control prompt never names Corvint, CEM, or a map. A third arm, `seeded` (amended 2026-10-01,
  accepted by the owner 2026-10-25), receives the treatment prologue followed by a `Suggested evidence` block: for each
  presented source file, the rows `corvint context --task "evidence for a change to PATH"
  --subject PATH --limit 8` returns in the same prep clone at `C^` (path, relation, reason), all
  but the subject's own row, labelled as suggestions that are never cited; the `cem begin` map
  stays all-`unknown`. The clone is the leak control: it holds no object at or after `C`, so a
  suggestion can only be evidence the agent could itself have found at the base revision. The
  2026-09-04 pilot's treatment carried a map in which every hunk was `unknown` with no evidence,
  so it measured the wording of a worklist and not Corvint's evidence; `seeded` is the arm that
  carries the evidence, `treatment` becomes the format control between `control` and `seeded`.
  Each seeded lane records `suggested`, the distinct suggested paths. Amended 2026-10-11
  (V1-1261): for a registered run the treatment arm is redefined by CRT-V0-015; the
  `cem begin` worklist arms above remain only for pilot and development runs. Amended 2026-10-12
  (V1-1278, decision 0525 §1, CEM-V1-215): cem/0.x is retired, so the `treatment` and `seeded` arms, which carry
  a cem/0.1 map, now refuse with `errRetiredMapArms` before writing into the prep clone; the `control` arm
  is unchanged. Retiring this spec is S3's. Amended 2026-10-24 (V1-0836, accepted under the owner's
  standing lane-requirement auto-accept): the `treatment` and `seeded` arms carry a cem/1.0 map and
  `errRetiredMapArms` is removed. Per change the harness clones the prep clone into a private target
  clone under its symlink-resolved path, applies `P(C)` with `git apply --index`, commits it as a
  synthetic target `T` with fixed author and committer dates, and runs
  `cem prepare --base C^ --target T`. `T` is not `C`: it holds only the presented source hunks, exists
  only in the target clone, which is removed when the change ends, and the prep clone and every lane
  clone still cannot resolve it. The map is refused unless its hunks match the presented patch's in
  count, order, path and start lines, so the map's hunk `N` is the prompt's hunk `N`. The three
  artefacts are the prepared map (every hunk `unknown`), `cem status` and `cem report`, with the
  target clone's path replaced by `<target-clone>` so artefacts and prompts are byte-identical across
  builds. A lane's citations are replayed with `cem cite` into a pristine copy of the map restored
  before each lane, and `cem status` and `cem verify` read the result (CRT-V0-007); a Corvint
  command that refuses is a measurement, one that produces no verdict (cancellation, timeout, launch
  failure) errors the lane, so it is neither scored as a verifier failure nor reused (CRT-V0-010). The seeded arm's
  suggestions are unchanged. Since 2026-10-24 a registered run may name both arms (CRT-V0-015).
- `CRT-V0-006`: Reply grammar. Every reply MUST end with one fenced JSON block
  `{"citations":[{"hunk":"N","path":"...","lines":"S:E","relation":"specification|decision|test-claim|implementation|call-site|dependency|incident","confidence":"certain|likely|unsure"}],"unknown":["N"]}`.
  The scorer takes the last fenced block naming `"citations"`, or a reply that is itself one JSON
  object naming it. No such block is `ABSENT`; a block that does not parse into a citations array is
  `MALFORMED`. Both score as "cited nothing" and are counted, never dropped. A lane whose agent
  process exited non-zero produced no reply at all and is not one of these states: it is an errored
  lane under CRT-V0-010, dropped with its change, and the harness records the exit code and the
  bounded stderr tail as the lane's error. A reply cut at the reply bound (`reply_truncated`) with
  no citations block left is errored the same way, never `ABSENT`: the cut lost the observation.
  Amended 2026-10-04 (V1-0742, proposed): a reply holding only whitespace is `EMPTY`, distinct
  from `ABSENT` and `MALFORMED` and scored the same way as "cited nothing". A codex reply file is
  read only up to one byte past the reply bound, so an oversized reply is never allocated whole
  and is recorded `reply_truncated`.
- `CRT-V0-007`: (Amended 2026-10-11: V1-1261, accepted under the owner's standing auto-accept (2026-10-11)) Metrics. `miss(c,a) = |gold(c) \ citedPaths(c,a)| / |gold(c)|`. The primary estimate
  is `Δ = mean_c[miss(c,control) − miss(c,treatment)]`, the relative reduction is
  `R = Δ / mean_c[miss(c,control)]`, and the interval is a 95% BCa bootstrap over changes (10,000
  resamples, from a stream derived from the recorded seed). Secondary: the paired binary "missed
  anything" with an exact McNemar test. `citable` is `supported / (total − mechanical)` from the
  `cem status` counts the treatment lane's own citations produce, treatment only. The verifier
  hard-failure rate is the treatment lanes with at least one citation where `cem verify` reports
  `ok:false`; a hard failure is **incorrect** when an independent replay — resolve `base:path`, check
  the one-based range lies inside the file, extract and re-find the cited span, re-check the relation
  vocabulary — says the citation was valid. The replay is implemented against Git alone and never
  calls `internal/cem/verify`. Both `citable` and the verifier rate count only treatment lanes that
  did not error: a lane with an `error` or an observed non-zero `exit_code` (CRT-V0-006) is excluded
  whether or not its `cem` counts were recorded. Amended 2026-10-01 (accepted by the owner 2026-10-25): every compared
  arm is estimated against `control` with the same `miss`, `Δ`, `R`, BCa interval, McNemar and
  verdict wording; the report's top level keeps the `treatment` contrast under the pilot's keys
  and `summary.arms[arm]` carries each arm's block, with `citable` and the verifier rate computed
  over that arm's own lanes. McNemar is additionally reported over lanes (`mcnemar_lanes`): the
  control lane and the compared lane of the same change and repeat form one pair, because
  averaging the repeats first turns every change both arms sometimes missed into a tie, which is
  how the pilot reported zero discordant pairs over fifty lanes. Amended 2026-10-11 (V1-1261): `miss` is reported beside the cited-not-gold rate of CRT-V0-013 and is never read
  without it.
- `CRT-V0-008`: Power, pre-declared. 30 pairs is roughly 120 gold items per arm; with clustering
  (ICC 0.3, m=4, design effect 1.9) the effective n is about 63. At a control miss of 0.50 a 20%
  relative reduction is `Δ = 0.10` with SE ≈ 0.088 and a 95% CI of about (−0.07, 0.27); 80% power at
  α = 0.05 would need about 180 pairs. 30 pairs is therefore an **estimation run, not a test**: the
  gate "at least 20% fewer missed-evidence findings" is declared met only when the point estimate
  reaches 0.20 relative **and** the interval's lower bound is greater than 0, reporting
  "met: at least 20% fewer missed-evidence findings". Otherwise, an upper bound below 0 reports
  "not met: treatment missed more evidence than control"; a lower bound above 0 reports
  "below target but interval excludes 0". An interval covering both 0 and the target reports
  "consistent with 20% and with 0"; one covering 0 but excluding the target reports
  "consistent with 0 but not with 20%". Bounds are absolute miss-rate deltas: compare the target
  as `0.20 × mean control miss`, using the unrounded mean, with inclusive interval endpoints.
  No win is claimed for any non-met reading. Fewer than two scored pairs have no resampling
  variability, so `delta_ci95` is `NOT_OBSERVED` and the reading is
  "not estimable: fewer than two scored pairs", never met. The pair count is a manifest parameter
  (`--limit`) so the owner can widen it. Amendment (2026-10-01, accepted by the owner 2026-10-25; the
  scorer prints the amended reading and kill readings since V1-2673; the 2026-10-24 held-out report
  is not rescored, and neither of its arms is met under either reading because both lower bounds
  are below 0): the decision-grade run is powered for the
  `seeded` contrast at the gold-reachability-corrected control miss. With two repeats per arm,
  about four gold items per change and between-change heterogeneity τ² ≈ 0.02, 80% power at
  α = 0.05 two-sided needs about 55 pairs for a 20% relative reduction from a control miss of
  0.50 (`Δ = 0.10`) and about 26 pairs from 0.70 (`Δ = 0.14`); a 35% reduction needs about 18
  and 9 pairs (floor 20); with one repeat each figure roughly doubles. The gate then reads: met
  when the interval's lower bound exceeds 0 **and** the interval does not exclude the 20%
  target; requiring the point estimate to reach the target as well has only 50% pass
  probability at a true 20% effect. Kill readings: an upper bound below `0.10` absolute rules
  out a 20% gain, and a `seeded_vs_treatment` upper bound at or below `0.05` reads as "the
  map's evidence adds nothing beyond its worklist". Under the amended gate "met: at least 20% fewer
  missed-evidence findings" requires `low > 0` and `high >= 0.20 × mean control miss` (inclusive),
  whatever the point estimate; a lower bound above 0 with an upper bound below the target reads
  "below target but interval excludes 0", and the other readings are unchanged. Each arm's
  `verdict.kill_readings` lists "upper bound below 0.10 absolute: rules out a 20% gain" when its
  upper bound is below `0.10`, and `seeded_vs_treatment.kill_reading` is "the map's evidence adds
  nothing beyond its worklist" when that contrast's upper bound is at or below `0.05` (empty list or
  no key otherwise, and none without an interval). The ten-pair run that estimates the
  between-change variance before the pair count is frozen is drawn from the development
  partition and is development forever. Recorded 2026-10-24 (V1-0836): the development run
  (`benchmarks/results/cem-reviewer-trial-development-2026-10-24.json`, manifest
  `tools/cem-trial/testdata/development-2026-10-24`, ten pairs, two repeats, 60 lanes, none
  errored) observed a control miss of 0.54 over reachable gold and a between-change standard
  deviation of the per-change `seeded` miss delta (control minus seeded, repeats averaged) of
  0.290 (variance 0.0842). With `n = (z₀.₉₇₅ + z₀.₈)² · σ_d² / Δ²`, `z₀.₉₇₅ + z₀.₈ = 2.8016` and
  `Δ = 0.20 × 0.54 = 0.108`, `n = 56.6`, so the held-out pair count is **frozen at 57**, two
  repeats per arm, which is consistent with the planned 55. The variance comes from ten pairs:
  its χ² interval (9 df) puts `n` between about 27 and 190, and the per-change deltas are not
  normal (mostly 0 or ±0.5), so 57 is a planning figure, not a guarantee. The `treatment`
  contrast alone would need 39.
- `CRT-V0-009`: Pilot. The pilot uses its own seed and partition; its changes are excluded from the
  main population. Pilot results may tune prompts and timeouts, and are then development forever
  under `benchmarks/README.md`'s first-observation rule.
- `CRT-V0-010`: (Amended 2026-10-11: V1-1261, accepted under the owner's standing auto-accept (2026-10-11)) Stopping and publication. There is no interim analysis: the scorer runs once, after
  every lane has completed. The report is labelled first-run at
  `benchmarks/results/cem-reviewer-trial-first-run.json` with the partition `heldout`, the manifest
  sha256, the prompt-skeleton sha256, both prologues' text and sha256, the model id, the effort, the
  `corvint` version and sha256, and the seed. A run is **invalid** — discarded and re-declared, never
  re-scored — when the prompt digests differ between arms beyond the declared prologue, any lane's
  clone resolved `C`, more than three lanes errored (the scorer sets the report's `invalid` field
  from that count, reading each lane's recorded exit code so a report written by an earlier harness
  reaches the same verdict), the selection rule, seed, gold, or scorer changed after the first
  invocation, or any lane was re-run with a changed prompt. `--resume` and `--reuse`
  may only re-run lanes that never finished; a lane is reused only when its prompt sha256 is
  byte-identical, its wall time was observed, and it did not error under CRT-V0-006 (read from the
  recorded exit code too, so a prior report's failed lane without an `error` is re-run).
  Amended 2026-10-11 (V1-1261): a run is also invalid when CRT-V0-014's registration
  check fails — a `heldout` report with no registration, a registration whose fixed fields, scorer
  digest or arms disagree with the report, or a registration commit newer than the run's first
  lane. Each lane records its launch time as `started_at` (a reused lane keeps the original), and
  the scorer reaches the same reading on `score --report` from the recorded fields alone; several
  reasons are joined with `; ` after the errored-lane reason.

- `CRT-V0-011`: Invocation lifecycle (proposed). `runCommand` MUST share the
  `internal/procgroup` pre-reap supervisor with the other trial dispatcher. On Darwin and
  Linux, cancellation, deadline, normal leader exit and the runner's real INT/TERM path
  terminate the owned process group and bound cleanup and pipe draining to five seconds.
  Incomplete cleanup or exit observation is an invocation error. Normal and nonzero exit
  codes retain their existing shapes; timeout and cancellation remain errors. Empty/relative
  roots and PATH/root-relative executables normalize before dispatch; stdin is EOF and the
  environment is inherited.
  Capture keeps the first 8 MiB stdout and 64 KiB stderr while draining excess, with no
  overflow-induced short write, cancellation or exit-code change. This corrects the old
  CEM bounded writer's short-count defect when a write crossed its remaining capacity.
  The shared supervisor's default remains fail-on-overflow; only these invocations opt
  into truncation. Qualification covers the owned group only: changed process groups or
  sessions, other platforms, and other trial subprocess helpers remain outside this slice.
  It adds no hostile-execution containment claim and changes no gold, prompt, scorer,
  stopping condition or outcome qualification.
  Amended 2026-10-04 (V1-0742, proposed): `runCommand` reports the stdout overflow, and a lane
  whose agent stdout reached the capture bound records `stdout_truncated: true`. A cut codex event
  stream records `tokens` and `tool_calls` as `NOT_OBSERVED`, and a cut `script` reply, or a codex
  reply taken from a cut stream because no reply file was written, is `reply_truncated`. A `turn.completed` usage event whose counts are not all finite, non-negative
  numbers is not a token observation and records `NOT_OBSERVED`.
- `CRT-V0-012`: Seeded-arm decomposition (2026-10-01, accepted by the owner 2026-10-25). A seeded run reports
  `summary.seed_coverage` over the seeded lanes that did not error: `gold_items`,
  `suggested_items` (gold the suggestions carried, recorded per lane as `seed_hits`), `fraction`
  = `suggested_items / gold_items`, `miss_given_suggested` and `miss_given_unsuggested`, each
  `NOT_OBSERVED` when its denominator is zero. Beside a `treatment` arm it also reports
  `summary.contrasts.seeded_vs_treatment`: the paired mean of `miss(treatment) − miss(seeded)`
  over the changes both arms scored, its BCa interval over those changes (seeded from the run
  seed and the arm names) and its lane-level McNemar. A seeded result is read as retrieval
  (`fraction`) times adoption (`1 − miss_given_suggested`) and neither is reported as the other:
  a seeded arm that misses what it was shown is an adoption result about the agent, one that
  misses only what it was not shown is a retrieval result about Corvint, and a `fraction` below
  0.5 is reported as retrieval-limited and supports no claim about the map. The suggestions are
  computed once per change in the prep clone and shared by that change's seeded lanes, so a
  seeded lane's prompt digest covers them.
- `CRT-V0-013`: (V1-1261, decision 0523 P4; accepted under the owner's standing auto-accept (2026-10-11)) Cited-not-gold precision. Every
  scored lane records `cited_not_gold` beside `miss`, scored against the same reachable gold:
  `paths` = the distinct normalized cited paths not in `gold(c)` over the distinct cited paths;
  `spans` = the distinct cited lines (each path's cited ranges merged, a line cited twice counted
  once) outside the recorded secondary gold spans of the same gold path (CRT-V0-004), over the
  distinct cited lines. A citation whose range does not parse or starts before line 1 counts as one
  cited line outside the gold, so a malformed range never escapes the rate. Each rate is
  `NOT_OBSERVED` for a lane that cited nothing, and `spans` is `NOT_OBSERVED` for a report whose
  changes carry no `gold_spans` (a report written before this requirement). The report carries each
  change's `gold_spans`. Every arm block and the `seeded_vs_treatment` contrast report
  `cited_not_gold[arm]` = `{paths, spans, lanes, span_lanes}`, the mean lane rate over the
  non-errored lanes of the scored pairs that cited something, for control and the compared arm.
  An arm that buys a lower miss by citing every suggestion shows that cost here: a scripted agent
  citing all eight suggestions per file reaches miss 0 and loses on both rates to an arm that
  cites only the gold it can justify. Limits: gold is test-or-spec evidence only, so a valid
  implementation or call-site citation counts as not gold in every arm alike; gold spans are line
  ranges at `C` of the lines `C` added while citations are ranges at `C^`, so `spans` over-counts
  where a hunk is shifted or appended past the base file's end. Both rates are therefore read as a
  paired comparison between arms over the same changes, never as an absolute error rate, and
  neither changes the CRT-V0-008 verdict wording.
- `CRT-V0-014`: (V1-1261, decision 0523 P4, extends CRT-V0-010; accepted under the owner's standing auto-accept (2026-10-11)) Registration.
  An outcome run is preregistered by one JSON file, committed before `select` runs, holding exactly
  the keys `profile` (`corvint-cem-trial-registration/0`), `hypotheses` (a non-empty array of
  non-blank strings), `primary_metric` (`miss`, the CRT-V0-007 contrast and the only primary metric
  this scorer estimates), `alpha` (`0.05`, the level of the fixed 95% interval), `n` (the pair
  count, equal to the manifest's change count), `scorer_sha256` (the sha256 of
  `tools/cem-trial/{gold,main,register,score,select}.go` concatenated in that order, which the
  binary computes from the sources it embeds), `model` and `effort` (the strings the report
  records, `NOT_OBSERVED` for the script agent or an unset effort) and `partition`. An unknown key,
  trailing data or any other value for the fixed fields is refused. `run --registration FILE`
  reads the file, requires it to be committed byte for byte at `HEAD` of the Git repository holding
  it, and records its repository-relative path, sha256, the commit that last wrote it and that
  commit's committer time (UTC RFC 3339) as `registration`. Before any lane runs, `run` refuses a
  `heldout` manifest with no registration and a registration whose partition, model, effort, `n`
  or scorer digest differs from the run, or whose run names an arm CRT-V0-015 bars, and a
  registered run refuses a `--reuse` report or `--resume` checkpoint whose model or effort differs
  from the run. `score --report` decodes the carried registration as strictly as the file and
  refuses a report whose registration holds a key the object does not define, and `--resume`
  refuses a checkpoint that exists but does not decode rather than rerunning the lanes it holds.
  The report is
  invalid (CRT-V0-010) when the registration commit is newer than the earliest recorded lane
  `started_at`, when a lane that did not error recorded no start (such as a lane reused from an
  older report, which may have run first), or when any of those checks fails on rescore, including
  a carried registration whose fields are malformed or not the fixed values and a rescore under a
  scorer whose digest differs from the registered one. Limits: the
  committer time is self-asserted by whoever wrote the commit, so the order check catches a
  registration written late by mistake, not a forged date; and "before selection" is a procedural
  obligation the tool does not check, because a manifest records no selection time.
- `CRT-V0-015`: (V1-1261, decision 0523 P4; accepted under the owner's standing auto-accept (2026-10-11)) Treatment arm and pre-baseline runs.
  In a registered run the treatment arm is the artifact the shipped CEM profile produces for
  the change — the map with verifier-computed support of decision 0523 P0 and P2 — never the
  all-`unknown` cem/0.1 worklist that `cem begin` builds. The harness builds no such artifact yet,
  so `run` refuses a registered run that names the `treatment` or `seeded` arm, both of which still
  carry that worklist, and a registered report naming either is invalid on rescore. Any run made
  before decision 0523 P2 has a baseline (kill criterion K1 of decision 0523) is invalid as outcome
  evidence, whatever its partition or reading: it may tune prompts and the harness and is
  development forever under `benchmarks/README.md`'s first-observation rule. The 2026-09-04 pilot
  is such a run. Replacing the arm builder is a follow-up that amends this requirement and
  CRT-V0-005 together. Amended 2026-10-24 (V1-0836): CRT-V0-005's cem/1.0 builder now runs the
  `treatment` and `seeded` arms of pilot and development runs. It still carries an all-`unknown`
  worklist, not the shipped profile's artifact with verifier-computed support, so the
  registered-run refusal and the rescore invalidity above are unchanged. Amended 2026-10-24 (V1-0836,
  owner decision): the owner counts the all-`unknown` cem/1.0 map that `cem prepare` builds for the
  `treatment` and `seeded` arms as the shipped profile's artifact for this trial. `run` no longer
  refuses a registered run naming either arm, and such a report is no longer invalid on rescore. The
  pre-baseline clause above (kill criterion K1 of decision 0523) is not changed by this amendment.
- `CRT-V0-016`: (V1-0836, V1-2619; 2026-10-24, owner decision; extends CRT-V0-010 and CRT-V0-014) Reuse across a re-registration.
  A registered run may `--reuse` lanes from a report written under an earlier registration only when
  the two registration documents are identical except for `scorer_sha256`, and that digest differs.
  `run` refuses any other earlier registration, an earlier registration committed after that
  report's first lane, and an earlier report with a completed lane that recorded no start. Lane
  reuse is otherwise unchanged (CRT-V0-010): a byte-identical prompt, no error, an observed wall time.
  The earlier registration is the same registration only when it has the same commit and an
  identical document; two registration files committed together are still compared (V1-2624). A
  never-started pending lane (no start, no observed wall time) of the earlier report does not count
  as a completed lane without a start. When `--reuse` and a `--resume` checkpoint both hold a lane,
  the first eligible one is reused, so an ineligible `--reuse` lane never shadows an eligible
  checkpoint lane. The report carries the earlier registration and that report's manifest digest as
  `re_registered`; `--resume` restores a checkpoint's `re_registered` record and refuses a
  checkpoint carrying a different one. Each reused lane records `reused_from` (the manifest digest)
  and `reused_registration` (the digest of the registration it first ran under), so a lane reused
  across a scorer-only re-registration of an unchanged manifest is told apart from a resumed one. A
  lane reused from the earlier registration is checked against that registration's commit time,
  and every other lane is checked against this run's (CRT-V0-014). Lanes resumed from a checkpoint
  of the same manifest and registration count as this run's own. On rescore the report is invalid
  when the carried registration differs beyond the scorer digest, when it is newer than a lane
  reused from it, when a lane was reused from any other report, or when a report carries
  `re_registered` with no registration of its own, partition aside. `select` presents each patch
  byte for byte as `internal/cem/gitauth` `CanonicalDiff` derives it for those paths: the same
  pinned diff configuration (`diff.noprefix=false` and the other pins), `--attr-source` set to the
  empty tree, `--full-index`, `a/` and `b/` prefixes and `--end-of-options`, run under gitauth's scrubbed
  allowlist environment, so neither `GIT_DIFF_OPTS` nor `GIT_CONFIG_PARAMETERS` changes the bytes. These V1-2624 fixes
  change the scorer digest; the 2026-10-24 held-out report stays rescored at the registered tree
  `bc580295f`, where it was registered and run. The run's build-log entry must name the scorer change, the reused
  lane count, and that the reused pairs were already observed in the earlier run. Limit: the scorer
  change itself is not checked to leave the reused lanes' prompts and scoring unchanged. Prompt
  identity covers the prompts. Scoring is recomputed from raw fields on every `finalize`.
- `CRT-V0-017`: (V1-2948; accepted 2026-10-28 under the owner's standing lane-requirement
  auto-accept) Stem-worklist baseline. The 2026-10-24 held-out run's mechanical stem guesses
  (CRT-V0-004) missed less withheld evidence than every arm, so every summary carries
  `stem_baseline`: for each arm the report ran, control included, the paired contrast with the
  mechanical stem worklist over the same scored changes (a change with an errored lane is dropped
  whole, CRT-V0-007), with `mean_miss`, `delta` (stem miss minus arm miss), `relative_reduction`
  (`NOT_OBSERVED` when the stem baseline misses nothing), the BCa `delta_ci95` seeded from the run
  seed and the arm, and `verdict`. An arm beats the baseline only when it misses at least 20%
  less and the interval excludes 0; every other reading, an unestimable one included, begins
  `not beaten`. `beaten_by` lists the arms that beat it and `statement` says it in one sentence:
  `the stem baseline is not beaten by any arm`, or, when one is beaten, that adjudicated false
  findings are `NOT_OBSERVED`, so the reviewer bar (20% fewer missed findings with no increase in
  adjudicated false findings) is not established. A fourth arm, `stem`, is the control prompt
  plus one declared `Stem worklist` block listing the recorded guesses, with no Corvint map,
  retrieval or verifier and no `--corvint` requirement; when it runs, `stem_contrasts` carries each
  other arm's `contrast` against it. CRT-V0-008's verdict against control is unchanged. Limits: the
  compact CEM worklist arm with snippets, verification status and unresolved obligations on the
  shipped map profile is not built (`NOT_RUN`), and no run of the new arm or of the held-out set
  is part of this change. A scorer edit changes `scorer_sha256`, so an earlier registered report
  rescored under this scorer is invalid as CRT-V0-014 already requires. Rollback: drop
  `stemBaselineBlock`, `stemPrologue`, `stemWorklist` and the `stem` arm name.
- `CRT-V0-018`: (V1-2948; accepted 2026-10-28 under the owner's standing lane-requirement
  auto-accept) Separate outcomes. Evidence discovery and adjudicated defect detection are scored
  apart. Every summary carries `outcomes`: per arm, `evidence_discovery` (the withheld-gold `miss`
  every block scores, with the non-errored lane count, the lanes that cited something and
  `citations_used`, the evidence the lanes used), `evidence_opened` = `NOT_OBSERVED` (no agent
  adapter records the files an agent read without citing), and `defect_detection` with
  `adjudicated_defects` and `adjudicated_false_findings` both `NOT_OBSERVED`; the top-level
  `defect_detection` states that no adjudication records exist and that it is never inferred from
  discovery. Adjudication needs a human adjudicator and is outside this tool (Non-goals). Rollback:
  drop `separateOutcomes`.

## Non-goals and simpler baseline

Human reviewers; a judge model; any agent CLI beyond `codex` and the `script` adapter; multi-turn
dialogue; measuring defect detection in general; declaring CEM adopted or product-validated; any write
to the Beamfall repository, which is read-only history. The question the README asks, "does CEM
help a reviewer?", is therefore answered by this trial only for an agent reviewer; the human
within-subject leg (decision 0427) is a separate study with its own sealed task counting.

Two ground-truth designs were considered and rejected. **Agent-authored gold spans** would let the
measured system define its own answer key, so a treatment win could not be separated from a treatment
gloss on the question. **Seeded evidence-gap mutants** would make the gold exact but would measure a
synthetic distribution of gaps rather than the one real changes produce; they may run as a
pre-registered secondary stratum on 5 changes, reported separately and never pooled with the primary.

The simpler baseline is the mechanical stem-convention guess, which needs no agent at all; CRT-V0-004
records it per change so the report shows what the agent added to it.

## Trust boundary, resource limits, failure modes

The tool trusts the manifest, the local Git history it is given, and the `corvint` and agent
executables named on the command line; it downloads nothing. Manifest 16 MiB; patch 64 KiB; artefact
256 KiB each; reply 1 MiB; agent stdout 8 MiB; `corvint` call 2 minutes; agent call `--timeout`
(default 5 minutes).

- `cem begin` rejects a patch: the change is excluded at selection, before the seed is fixed.
- Agent timeout or launch failure: the lane records `error`, and its **pair is dropped whole**, so the
  arms always cover the same changes.
- A gold path leaked into a source hunk: the change is excluded at selection.
- Stem-convention guessing inflates both arms: the report carries the stem baseline's own miss rate
  beside them, and treatment is read as lift over that floor.
- The treatment arm ignores the map: recorded, not corrected. The trial measures what the agent did
  with the artefacts, not whether it read them.
- Zero supported hunks — the observed prior, `benchmarks/results/cem-first-run-beamfall-pass-v0.1.json`
  records 0 supported against 4 unknown — leaves `citable` at 0 and the trial still publishes.
- `corvint context --subject` fails or returns an unreadable packet for a presented file: the
  change's lanes error in every arm (the seeded prompt cannot be built), so the pairing stays
  whole; the failure is recorded as the lane error.
- The suggestions carry no gold (`fraction` 0): the seeded arm reads as retrieval-limited; no
  product claim follows and the run still publishes.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CRT-V0-001 | `selectionRule`, `candidateCommits`, `inspectCommit`, `validated`, `beginHunks`, `evenlySpaced`, `readManifest` (`tools/cem-trial/select.go`, `main.go`) | `TestSelectionIsDeterministicAndRefusesGoldLeak`, `TestSelectionPresentsTheCanonicalCEMDiff`; pilot manifest `tools/cem-trial/testdata/pilot/tasks.json` (population 620, five changes) |
| CRT-V0-002 | `plan`, `shuffledArms`, `seededStream`, `laneClone` | `TestCloneCannotResolveTheChangeCommit`; pilot dry run (three of five changes ran treatment first) |
| CRT-V0-003 | `inspectCommit` (source-only diff), `hunkList`, `readPatch` digest check | `TestWithheldPatchExcludesEveryGoldPath`, `TestSelectionPresentsTheCanonicalCEMDiff` |
| CRT-V0-004 | `goldSpans`, `hunkRanges`, `stemBaseline`, `stemGuesses`, `leaksGold`, `hunkBody`, `stemMiss` (`gold.go`, `score.go`) | `TestSelectionIsDeterministicAndRefusesGoldLeak`; `TestWithheldPatchExcludesEveryGoldPath` (spans present) |
| CRT-V0-005 | `skeleton`, `treatmentPrologue`, `buildPrompt`, `buildArtefacts`, `prepareTarget`, `commitTarget`, `renderArtefacts`, `alignedHunks`, `patchHunks`, `mapTarget`, `cloneAtBase`, `resolvable`, `laneClone` | `TestBothArmsShareTheSamePromptDigestApartFromThePrologue`; `TestMapArmsCarryAPreparedCEM10Map` (2026-10-24 amendment: prepared map, leak control, pristine restore, symlinked `TMPDIR`); `TestMisalignedMapIsRefused`; `TestCloneCannotResolveTheChangeCommit` |
| CRT-V0-006 | `extractCitations`, `citationsBlock`, `extractUnknown`, `scoreLane` | `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `TestScoreRebuildsTheReportByteIdentically`; `TestExtractCitationsKeepsEmptyAbsentAndMalformedDistinct`, `TestCodexRunBoundsTheReplyAndRecordsStdoutTruncation` (2026-10-04 amendment) |
| CRT-V0-007 | `scoreLane`, `summarize`, `bca`, `mcnemar`, `exactBinomial`, `citable`, `verifierRates`, `applyCitations`, `replayCitations`, `replayCitation` | `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `TestReplayCheckerDisagreesWithVerifyOnAKnownGoodCitation`; `TestCitableAndVerifierSkipNonZeroExitLanes`; 2026-09-04 pilot (`citable` 0.815 over 200 material hunks, verifier incorrect-hard-failure rate 0 over 25 citing lanes) |
| CRT-V0-008 | `verdicts`, `killReadings`, `seededKillReading`, `verdictAtLeast`, `verdictBelow`, `relativeGate`, `citableGate`, `hardFailureGate`; absolute interval target is `relativeGate × controlMiss` | `TestKillReadings`; `TestMissedEvidenceVerdictIntervalCompatibility` (`CRT-V0-008` subtests cover all five readings, absolute target scaling, and inclusive endpoints); `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json`'s stored `missed_evidence` wording predates the 2026-09-04 amendment (control miss 0.90, interval [0, 0.06] now reads "consistent with 0 but not with 20%"); the stored report is history, not rescored; 2026-10-24 development run `benchmarks/results/cem-reviewer-trial-development-2026-10-24.json` and `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-trial-dev-run.md` (frozen held-out N 57) |
| CRT-V0-009 | `--partition`/`--seed`/`--exclude` on `select`, `excludedCommits`, `objectName`, `withoutExcluded`, `inertExclusion`, `excludeError`, `poolError` (`tools/cem-trial/select.go`), `report.Pilot`; the manifest's `population` and the summary's `population N, excluded M` are the pool after the M drops | `tools/cem-trial/testdata/pilot/tasks.json` carries `partition: pilot`; `benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json` (seed `pilot-2026-09-03`, 5 pairs, 50 lanes, `gpt-5.6-sol` at medium); `TestExcludedPilotChangesAreDroppedAndCounted` (three qualifying changes, one-commit pilot, non-empty disjoint held-out set of population qualifying − excluded), `TestExcludeFileWithoutIdentifiersIsRefused`, `TestExcludeIdentifiersMustBeFullLowercaseHex`, `TestExcludeThatDropsNothingIsRefused`, `TestEmptyPoolIsRefusedAndAShortPoolWarns`, `TestExplicitlyEmptyExcludeListIsRefused` |
| CRT-V0-010 | `finalize`, `rescore`, `write`, `checkpointer`, `loadReuse`, `reuseSource.apply`, `laneFailed`, `report.Invalid` | `TestScoreRebuildsTheReportByteIdentically`; `TestErroredLaneDropsItsPair`; `TestNonZeroAgentExitErrorsTheLaneAndInvalidatesTheRun`; 2026-09-03 pilot rescored to `invalid` (31 errored lanes); 2026-09-04 re-run valid (`invalid: null`, 0 errored lanes) |
| CRT-V0-011 | `runCommand`; `internal/procgroup.Run`, `OverflowPolicy`, `Spec.StderrLimit` | `TestRunProcessLifecycleMatrix`; `TestRunProcessTruncateOverflowPolicy`; `TestRunProcessDefaultOverflowPolicyStillTerminates`; `TestRunProcessRejectsInvalidOverflowPolicyBeforeSpawn`; `TestTruncateCaptureConsumesCrossingWrite`; `TestRunCommandPreservesExitShapes`; `TestRunCommandNormalizesPathsAndEnvironment`; `TestRunCommandRefusesBeforeDispatch`; `TestRunCommandTruncatesOutput`; `TestRunCommandCancellationKillsDescendant`; `TestTrialMainSignalsCleanOwnedAgentGroup`; 2026-10-04 amendment: `TestCodexRunBoundsTheReplyAndRecordsStdoutTruncation`, `TestScriptRunRecordsStdoutTruncation`, `TestNegativeTokenCountsAreNotObservations` |
| CRT-V0-004 (2026-10-01 amendment) | `inspectCommit` (status `M`/`D` gold, `unreachable`), `task.GoldUnreachable`, `printSelection` | `TestSelectionRecordsUnreachableGoldSeparately` |
| CRT-V0-013 | `scoreCitedNotGold`, `citedNotGold`, `mergeIntervals`, `overlap`, `citedNotGoldArms`, `changeMeta.GoldSpans`, `goldSpansOf` (`tools/cem-trial/score.go`, `main.go`) | `TestCiteEverySuggestionAgentLosesOnCitedNotGold` (the cite-every-suggestion agent wins on miss, Δ 0.5, and loses on both rates); `TestCitedNotGoldCountsDistinctPathsAndLines`; `TestRunChecksItsRegistration` (a run carries `gold_spans` and scores the rate) |
| CRT-V0-014, CRT-V0-010 (2026-10-11 amendment) | `registration`, `parseRegistration`, `loadRegistration`, `registrationRecord`, `registrationProblems`, `registrationMismatches`, `registrationInvalid`, `firstLaneStart`, `unstampedLanes`, `registeredPrior`, `registrationRecord.UnmarshalJSON`, `checkRegistration`, `scorerDigest`, `scorerFiles`, `lane.StartedAt`, `finalize` (`tools/cem-trial/register.go`, `score.go`, `main.go`) | `TestScorerDigestCoversEveryScorerSource`; `TestRegistrationRefusesAnIncompletePlan`; `TestRegistrationMustBeCommitted`; `TestReportRefusesARegistrationNewerThanItsFirstLane`; `TestRunChecksItsRegistration`; no registered run yet: the run is post-rc.3 and owner-commissioned (`NOT_RUN`) |
| CRT-V0-015 | `registrationMismatches` (no arm refusal since the 2026-10-24 owner decision), `armsNeedCorvint` | `TestReportRefusesARegistrationNewerThanItsFirstLane` (`CRT-V0-015` subtest: a registered cem/1.0 map arm is valid); the held-out run is recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-trial-heldout-run.md` |
| CRT-V0-016 | `reRegistration`, `reRegister`, `adoptReRegistration`, `resumeReRegistration`, `reRegistrationProblems`, `reRegistrationInvalid`, `firstFreshLaneStart`, `ownLane`, `reuseSource`, `reuseCandidate`, `lane.ReusedRegistration`, `report.ReRegistered`, `canonicalPatch` (`tools/cem-trial/register.go`, `main.go`, `score.go`, `select.go`) | `TestReuseAcrossAReRegistration`, `TestReuseCodexRoundFindings`, `TestSelectPatchMatchesCanonicalDiffUnderHostileConfig`; the held-out run is recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-trial-heldout-run.md` |
| CRT-V0-017 | `stemBaselineBlock`, `stemMiss`, `stemPrologue`, `stemWorklist`, `buildPrompt`, `prologueIdentity`, `parseRunOptions`, `invokeLane` (no replay for `stem`), `summarize` (`stem_contrasts`) (`tools/cem-trial/score.go`, `main.go`) | `TestCRTV0017StemWorklistArmCarriesOnlyTheStemGuesses`; `TestCRTV0017ReportSaysPlainlyWhenTheStemBaselineIsNotBeaten`; compact CEM worklist arm and any run `NOT_RUN` |
| CRT-V0-018 | `separateOutcomes` (`tools/cem-trial/score.go`) | `TestCRTV0018DiscoveryAndDefectDetectionAreSeparateOutcomes`; adjudicated defect detection `NOT_OBSERVED` |
| CRT-V0-005 (seeded arm), CRT-V0-012 | `seededPrologue`, `suggestEvidence`, `suggestedPaths`, `buildArtefacts`, `buildPrompt`, `prologueIdentity`, `armsNeedCorvint`, `lane.Suggested`, `lane.SeedHits`, `seedCoverage`, `contrast`, `mcnemarLanes`, `armSummary`, `citableArm`, `verifierRatesArm` (`tools/cem-trial/main.go`, `score.go`) | `TestSeededArmSharesThePromptApartFromItsPrologue`; `TestRunRefusesSeededArmWithoutCorvint`; `TestThreeArmSummaryKeepsPilotKeysAndDecomposesTheSeededArm`; `TestTwoArmSummaryIsUnchanged`; `TestLaneLevelMcNemarKeepsDiscordance`; 2026-10-24 three-arm development run (`benchmarks/results/cem-reviewer-trial-development-2026-10-24.json`, 60 lanes); the held-out run is owner-commissioned (decision 0427) and `NOT_RUN` |

## Rollout, rollback, compatibility

For the proposed lifecycle slice, rollback disables trial dispatch while reverting the
adapter/supervisor option change and its clauses together. Do not resume a frozen run under
a changed harness; retain its existing artifacts and first-observation restrictions.

The tool ships under `tools/`; no binary, adapter, or benchmark manifest calls it. Rollback is
deleting `tools/cem-trial` and this spec plus its README and index rows; nothing in `cmd/corvint`,
`internal/cem`, or `benchmarks/` changes. A discarded run leaves its report in place with
`"invalid": <reason>` rather than being re-scored.

Compatibility and drift: the harness copies `tools/cw-trial`'s dispatch, checkpoint, and canonical-
output shapes on purpose rather than importing them, because both are `package main`; if the codex
event names or the reuse rules change, both change in one commit. The `cem` command surface it drives
(`prepare`, `cite`, `status`, `verify` and `report` at the cem/1.0 wire, CRT-V0-005; amended
2026-10-25, V1-2648, proposed: this sentence named the retired `begin` and the cem/0.1 `--patch`
rule) is the one in `internal/cem/workflow`; a lane clone must not carry `objects/info/alternates`, which `corvint`
refuses, so lanes are plain local clones.

## Unresolved decisions and promotion or kill

Which model and reasoning effort the judged run uses; whether the secondary mutant stratum runs at
all; whether `unknown` hunks the agent declares should score beside the citations rather than only
being recorded; how the shipped-profile treatment arm is built once decision 0523 P2 has a
baseline, and whether the all-`unknown` cem/1.0 worklist of the 2026-10-24 builder can stand as
that artifact (CRT-V0-015, needs the owner). Promote to
`implemented` when a `heldout` set selected with the amended rule and frozen before any agent
output was read, sized by the amended CRT-V0-008 (frozen at 57 pairs on 2026-10-24 from the development run's
corrected control miss of 0.54), has run once in all three arms and its first report is preserved unrepaired, with a
CRT-V0-014 registration committed before its selection and before its first lane, and a treatment
arm that is the shipped profile's artifact (CRT-V0-015). V4's gate is
then read from that report by the owner under CRT-V0-008's wording, never from the pilot, and
the README row changes only to what that report supports: an agent-reviewer result with its
interval and its retrieval/adoption decomposition, never "helps a reviewer" without the human
leg.
