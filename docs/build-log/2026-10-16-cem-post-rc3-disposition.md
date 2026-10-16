# 2026-10-16 V1-1268: disposition of the CEM 1.0 items decision 0523 defers past rc.3

V1-1268 is a spike. It asks that each item decision 0523 defers past rc.3 either gets its own
ticket or a recorded decision not to pursue it, with the measurement that decided it, and that no
rc.3 artifact advertises any of them as delivered. This entry records both. It amends no decision,
requirement, spec, packet or wire contract. Decision 0523 already filed these items as post-rc.3 and
committed to none of them, so a decision not to pursue an item now leaves 0523 unchanged.

Base: `origin/main` `a0b4c4108`. Measurements were made over the committed tree at that base with
`jq` and `git`. No scratch script was kept. Load averages were 59 to 63 on the shared 12-core host;
none of the counts depends on load.

## Disposition

| # | Item (decision 0523) | Disposition | Deciding measurement |
| --- | --- | --- | --- |
| 1 | Per-basis rank against K null spans seeded by `patchSha256`, with a BH-corrected per-hunk fraction | Ticket, gated on the V1-1337 prospective outcome and a labelled held-out set from another repository | Authored excess over the seeded derangement null is 1.94% [1.53%, 2.34%], against 67.1% for the grep bot |
| 2 | Optional author anchor offsets on the wire | Not pursued. Reopen only on human-label evidence from item 1's labelled set | Grep bot anchors at least the authored count in 629 of 629 scored maps; no human-label precision exists |
| 3 | Frozen mutant-operator table, so discrimination counts can be recomputed | Not pursued. Reopen when a committed map carries a `discriminates` witness | 0 of 27,039 committed hunks carry one; the table is 4 operators |
| 4 | Coverprofile pinned as a Git blob by OID plus sha256 | Not pursued. Reopen when a committed map carries a `coverage` witness | 0 of 27,039 committed hunks carry one; 0 of 755 maps carry any artifact |
| 5 | Preregistered reviewer trial, owner-commissioned, after K1 has a baseline | Ticket: existing V1-0836, with the 0523 P4 ordering added as dependencies (orchestrator update request) | K1 has no baseline: 0 `test-claim` citations on 243 non-mechanical Go code hunks across 58 maps sealed since P2 landed |

## Measurements

### Committed corpus at `a0b4c4108`

The corpus is 755 maps under `.corvint/changes/`, all `cem/1.0`, with 27,039 hunks: 26,996
`supported` and 43 `unknown`.

| Measure | Value |
| --- | --- |
| Hunks carrying a `coverage` witness | 0 |
| Hunks carrying a `discriminates` witness | 0 |
| Maps with any `artifacts[]`, `runnerReceipts[]`, `criterionBindings[]` or `criterionLinks[]` entry | 0 of 755 for each |
| Basis relations | `specification` 27,250; `implementation` 1,189; `test-claim` 731; `dependency` 224; `decision` 149; `incident` 21; `call-site` 2 |
| Stable packet tamper-matrix cases with a `discriminates` witness | 3 of 22 (`discrimination-at-bound`, `discrimination-mutants-over-bound`, `discrimination-survivors-inconsistent`) |
| Same, with a `coverage` witness | 0 of 22 |

The coverage and discrimination witnesses are wire-valid in `cem/1.0`
(`internal/cem/wire/stable_encode.go`) and remain `WIRE_VALIDATED_ONLY` in the verifier
(`internal/cem/verify/stable.go`). The coverage witness pins `profileSha256` only. Producers
exist: `cem cover` and `cem discriminate` write the witnesses into a `cem/1.0` map
(`internal/cem/workflow/cover.go`, `internal/cem/workflow/discriminate.go`). But the dogfood loop
runs neither, and no committed map carries their output.

### Mutant-operator table

The Go mutation plan is `internal/liveverify/mutate/plan.go`, at about 240 lines. It defines 4
operators: `negate-condition`, `swap-binary`, `replace-literal` and `delete-statement`. Their tables
are:

- 10 binary swaps: `==`/`!=`, `<`/`>=`, `>`/`<=`, `+`/`-`, `&&`/`||`, each pair in both
  directions.
- 4 literal rules: `0` to `1`, any other integer to `0`, `""` to `"mutant"`, any other string
  to `""`.
- Deletion rules: only expression statements and non-`:=` assignments are deleted.

Mutants are ordered by source position, kept only when they still parse, and capped at
`maxMutants`. The table is not stated in `protocol/cem-1.0/stable/ALGORITHMS.md`, which states only
the `maxMutants` bound (`CEM-V1-090`). Freezing the table would be roughly 20 lines of packet text.
Once frozen, though, `CEM-CB-025` makes any later revision a new profile. A consumer also could not
recompute the counts without re-running `go test` per mutant.

### K1 baseline

K1 is decision 0523 §5. The tally is defined in
`docs/build-log/2026-10-15-v1-1266-guarding-tests-k1.md`. It is extended here to `a0b4c4108`:

| Window | Maps | Maps with Go code hunks | Non-mechanical non-test Go hunks | Cite a `test-claim` basis | `held` (upper bound) |
| --- | --- | --- | --- | --- | --- |
| Sealed after #893 (`3fcc1d11c`) | 58 | 30 | 243 | 0 | 0 |
| Sealed after #944 (`f81114713`, the `--guarding-tests` step) | 9 | 3 | 44 | 0 | 0 |

`internal/dogfoodflow` does not run the guarding-test step (V1-1449). Until it does, the held share
stays 0, and the 30 post-P2 changes that K1 needs carry no signal. The reviewer trial therefore
cannot be ordered after a K1 baseline yet.

### Citation specificity

These numbers are retained, not re-run. The seeded null that item 1 builds on already exists as an
evaluation measure: `tools/cem-citation-baseline` uses N = 200 derangements per map, seeded by
`patchSha256` (`DCW-V0-053`). On the post-S3 corpus (634 maps scored, 618 with a null, 23,830 supported hunks)
(`docs/build-log/2026-10-13-v1-1278-post-s3-citation-baseline.md`) it reports:

| Arm | Anchored hunks | Excess over the null |
| --- | --- | --- |
| Authored | 1,250 (anchored tier) | 1.94% [1.53%, 2.34%] |
| Grep bot | 17,737 of 23,830 | 67.1% [64.6%, 69.7%] |
| Preimage | 295 (1.2%) | -0.38% [-0.66%, -0.09%] |

`held` is 0. The registered prospective run (V1-1337,
`docs/build-log/2026-10-13-cem-citation-prospective-registration.md`) treats E ≤ 2.1% as a kill,
and the committed corpus already reads as one.

## Decisions

- **Item 1 gets a ticket, not a decision.** The condition comes from the 2026-10-11 panel entry
  ("Seeded K-null permutation rank is post-rc.3 pending a held-out set") and from the V1-1268
  ticket body ("needs a labelled held-out set from another repository"). Decision 0523 itself
  does not state the condition. This repository cannot supply external labels.
  - A per-hunk rank against K nulls keeps per-observation information that the pooled excess
    discards: two anchored hunks with null-hit counts (0, 100) and (50, 50) of 200 give the same
    pooled excess but different ranks and BH discoveries. The axis can therefore add
    information, but whether that information tracks real support needs labels.
  - The ticket is gated on V1-1337 reading "improve". If V1-1337 reads "kill", the consumer axis
    has nothing to rank.
  - The orchestrator payload is `item1-seeded-null-rank.json`.
- **Item 2 is not pursued.**
  - Decision 0523 makes it conditional on P0 precision against human labels, and there are none.
  - The measured gap is in support, not in where the anchor sits: the grep bot anchors at least
    the authored count in 629 of 629 scored maps (V1-1336 corpus, as recorded in the V1-1337
    registration entry).
  - An author-written offset would reopen the producer-written strength channel that 0523 §4
    rejected. After the V1-0595 freeze it would also need a new profile ID (`CEM-CB-025`).
  - Reopen only if item 1's labelled set shows verifier-derived anchors missing human-labelled
    support in a way an offset would fix.
- **Items 3 and 4 are not pursued.**
  - Decision 0523 tied both to cases in the 0.3 packet. That packet has since been deleted
    (`CEM-V1-220..225`), and no committed map carries either witness.
  - Freezing the operator table or adding a blob OID to the coverage witness would fix wire or
    packet text for an input that no committed map carries and that the dogfood loop never
    produces. Under `CEM-CB-025` that text could not be revised
    without a new profile.
  - Reopen either item when the first committed map carries the matching witness, or when a
    consumer re-executes discrimination or replays coverage instead of reporting
    `WIRE_VALIDATED_ONLY`.
- **Item 5 is the existing V1-0836.**
  - The "about 180 pairs" figure in the V1-1268 ticket body is the original `CRT-V0-008`
    single-repeat power estimate. V1-0836 already carries the run at about 55 pairs with two
    repeats. That figure comes from the 2026-10-01 `CRT-V0-008` amendment, which is proposed and
    not accepted, so the pair count stays an open design choice for that ticket.
  - Filing a second trial ticket would compete with it. Instead, the update request
    `item5-V1-0836-update-request.json` adds what 0523 P4 contributes:
    - V1-0836 depends on V1-1266 and V1-1449, so the run waits for a K1 baseline.
    - The treatment arm is the shipped `cem/1.0` artifact, never the all-`unknown` `cem/0.1`
      worklist.
    - The run stays owner-commissioned.

## No rc.3 artifact advertises these items

At `a0b4c4108`, `README.md`, `docs/RELEASE-NOTES.md`, `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/`
(excluding the generated `REQUIREMENTS.tsv`) and `protocol/cem-1.0/` were searched for null-span
rank, BH correction, permutation specificity, author anchor offsets or hints, mutant-operator
table or enumeration, coverprofile blob pin, reviewer trial, "180 pairs", "helps reviewers" and
"groundbreaking". No match claims delivery of any of the five items.

- **rc.3 release notes.** The `1.0.0-rc.3 (unreleased): recorded limits` section mentions only
  `held` through `--hunk-revert` and states that "K1 is open".
- **Reviewer-trial matches.** These are the 2026-09 preregistration note, the 2026-09-03 pilot
  published as invalid, the valid 2026-09-04 five-pair pilot re-run (`delta: 0.04`, explicitly
  not an outcome run), and specs that name the trial as a future gate.
- **Discrimination and coverage matches.** These describe the existing witnesses, not a frozen
  table or a blob pin.

## Review (one Codex round)

One read-only Codex round reviewed `a0b4c4108..124fb8772`. It confirmed every corpus, witness,
relation, tamper-case, operator-table, K1 and cited baseline count. It also found no README,
release-notes, spec or packet text that claims delivery. Its five findings were all fixed in this
entry:

| # | Finding | Fix |
| --- | --- | --- |
| 1 | `cem cover` and `cem discriminate` do produce `cem/1.0` witnesses, so "no producer emits" was false | Reworded: no committed map carries a witness and the dogfood loop runs neither command |
| 2 | The 55-pair figure is a proposed, not accepted, `CRT-V0-008` amendment | Stated as proposed |
| 3 | A per-hunk K-null rank does not reduce to the pooled excess, because per-observation null rates differ | Inference removed; the payload's criteria no longer require a graded statistic |
| 4 | Decision 0523 does not state item 1's held-out-set condition | Attributed to the panel entry and the ticket body |
| 5 | The search summary omitted the valid 2026-09-04 pilot re-run | Added |

## Residual (not this ticket)

`docs/CHANGE-EVIDENCE-MAP.md` lines 122-133 and 184-197 still describe the `coverage` and
`discriminates` witnesses as "`cem/0.3` only" and say they are written only to an `--output` path
beside a `cem/0.2` input. After the 0.x retirement, `cem/1.0` encodes both witnesses. This is
suspected to be stale; whether `cem cover` and `cem discriminate` now emit `cem/1.0` was not checked
here.

## Rollback

Revert this file. The ticket payloads live only in the lane's scratch directory until the
orchestrator files them. Nothing here changes the queue, a decision or a requirement.
