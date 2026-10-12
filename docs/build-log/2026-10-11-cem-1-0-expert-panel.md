# 2026-10-11 CEM 1.0 expert panel for rc.3

Branch `claude/cem-1-0-rc3-expert-plan`, from `origin/main` fb11b2274. Outcome: decision 0523.

## Panel

Five lanes, each with one independent brief, read-only access to a detached worktree of
fb11b2274, and the same output shape (verdict, three moves with mechanism, building block,
MUST, falsifying fixture and rc.3 fit, risks, one cut). Lanes: proof-carrying change format
(wire/verifier contract), mechanical falsifier/program analysis, retrieval and model evaluation
methodology, supply-chain and release provenance, product strategy and discovery. Each lane was
asked to attack the orchestrator's six prior suggestions, not to confirm them. The orchestrator
checked every source citation below against fb11b2274 before synthesis.

## Confirmed findings (source-checked)

- `wire.ParseMap` refuses the stable spec by design (`CEM-V1-018`); `CEMStatementV1` and
  `cem anchor` both call it, so no `cem/1.0` map can be attested or anchored today.
- `wire/stable.go` `reference-coherence` makes an all-empty criterion/receipt/link/artifact
  graph a valid stable map. Receipt limit fields have exactly one legal value each.
- `verify/stable.go` reports coverage and discrimination witnesses as `WIRE_VALIDATED_ONLY`, as a
  per-map axis, never per hunk.
- `wire/discriminate.go` validates counts (`killed+survived <= mutants`) and bounds (positive
  integers) separately and never compares them. The witness
  `{"mutants":1000000,"killed":1000000,"survived":0,"bounds":{"maxMutants":1,...}}` is accepted.
- `gitnotes/anchor.go` attaches HEAD's committed pointer to any `--commit REV`;
  `gitnotes/provenance.go` `pointerState` checks path, blob and digest only.
- `tools/cem-trial/score.go` `scoreLane` computes miss over gold paths only; cited-not-gold is
  never counted and the recorded secondary span gold is never scored.
- `DCW-V0-019` refuses a citation plan with any non-bootstrap `unknown` hunk
  (`docs/DOGFOOD.md` §4 and the workflow spec), which is the mechanism behind the 99.8%
  `supported` rate and why the ">30% unknown" kill condition cannot fire on dogfood maps.

## Measurements

Method: all `cem/0.2` maps under `.corvint/changes` at fb11b2274 (591 maps), parsed with the
stdlib JSON reader; the falsifier lane recomputed every canonical patch for the newest 200 maps
with the `gitauth/diff.go` arguments and every `patchSha256` matched. The measurement scripts
lived in session scratch and are not retained; the numbers below are reproducible from the
committed maps and are development data under `benchmarks/README.md` (self-use partition, never
held-out).

| Measure | Value |
| --- | --- |
| Supported non-test Go code hunks citing any `_test.go` (591 maps) | 0 of 8,008 (one lane: 2) |
| Hunks citing only umbrella docs (591 maps) | 2,551 of 21,270 (31.9%) |
| Maps whose supported hunks all share one basis set (newest 200) | 100 (73 with 10+ hunks) |
| Maps citing a single span for every supported hunk (591 maps) | 169 (29%) |
| Umbrella share bimodality (591 maps) | 0 in 181 maps, 1.0 in 228 |
| Median distinct spans per map | 3 |
| Verifier-derived anchors (id/path/symbol), real pairs (newest 200) | 581 of 10,110 (5.7%) |
| Same, evidence drawn from other maps | 45 of 10,110 (0.45%); 12.9x lift |
| Any-term lexical overlap, real vs shuffled pairs | 63.7% vs 55.3% |
| Maps with zero anchored hunks / positive cyclic-shift specificity | 97 / 45 of 200 |
| Max fan-out per map (median, p90, max) | 21, 83, 291 |
| `LRF-V0-003` size rejections of current spans | 389 of 972 (40%) |
| Supported hunks citing a file the same patch edits | 5,706 of 10,110 (0 rewritten spans) |
| "Bind change evidence" commits with `gpgsig` (last 300) | 1 |
| `refs/notes/*` at fb11b2274 | none |

## Conflicts and how decision 0523 resolves them

| Conflict | Positions | Resolution |
| --- | --- | --- |
| New wire disposition `asserted` | Orchestrator and evaluation lane: yes; format, falsifier and product lanes: no, it is a producer claim | Rejected. Tiers are consumer-computed (P0). |
| Author-written anchor offsets vs verifier-derived anchors | Format lane: offsets on wire, re-proved; falsifier lane: derive from Git bytes, no wire change | Verifier-derived in rc.3; offsets revisited post-rc.3 only with human-label precision. |
| Specificity gating | Orchestrator: published thresholds; format: integer block, no thresholds; falsifier and evaluation: per-map null | Integer `citation` block plus deterministic cyclic-shift null as report axes, no gate. Seeded K-null permutation rank is post-rc.3 pending a held-out set. |
| Trial ordering | Evaluation: repair and preregister first; product: no trial before the falsifier exists | Scorer repair and registration in rc.3; run after K1 baseline, owner-commissioned, post-rc.3. |
| Provenance | Orchestrator: default-on anchoring; provenance lane: integrity is not authenticity | No default-on. Authentication slot, `cem/v2` statement, scoped anchor pointer, all before the V1-0595 freeze (P1). |
| Runner breadth | Format, provenance and product lanes independently: decouple or cut | Decoupled from the freeze; V1-0592/0594/0612/0620/0622 and non-Go V1-0593 ship as recorded limits under 0521 §2. |

## Not resolved

- P0 anchor precision against human labels is unknown; the 30-pair estimation run has not been
  run and would only report "consistent with 20% and with 0". 80% power needs about 180 pairs.
- Linux and Windows parity of the native/portable result stays `NOT_RUN`.
- Whether an owner-visible unknown share changes dogfood behaviour is unobserved until P3 lands.
- The stable proof's retained evidence lives under a host-local `/private/tmp` path; a third
  party cannot re-verify it. Decision 0523 P1 does not fix that for past evidence.

## Tickets

Filed under label `agent-memory` with `bugs`, `fixes`, `tests` or `ideas`:

- V1-1258 (bugs): `DCW-V0-019` refuses honest `unknown` hunks (P3).
- V1-1259 (bugs): discrimination counts never compared with `bounds.maxMutants` (CEM-V1-070).
- V1-1260 (bugs): anchor pointer scope and notes-ref rollback (CEM-V1-067).
- V1-1261 (bugs, tests): trial scorer recall-only; registration file (P4).
- V1-1262 (fixes): consumer-computed tiers, `citation` block, shift null (P0, CEM-V1-060..062).
- V1-1263 (fixes): provider-neutral criterion (CEM-V1-063).
- V1-1264 (fixes): verifiable receipt `authentication` slot (CEM-V1-064).
- V1-1265 (fixes): `cem/v2` statement, stable routing, verdict/environment split (CEM-V1-065/066).
- V1-1266 (fixes): `go test` hunk-revert falsifier and cite-the-guarding-test step (CEM-V1-068/069).
- V1-1267 (fixes): rc.3 release-notes record of the 0523 scope amendment.
- V1-1268 (ideas): post-rc.3 items.
