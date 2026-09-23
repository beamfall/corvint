# Decision 0373: Corvint 1.0 Core and companion scope ratified

Date: 2026-09-23. Status: accepted (owner answers of 2026-09-23; ticket V1-0001). Authority: the
repository owner answered the eleven questions in `docs/specs/corvint-1.0-product-and-release-v1.md`
("Decisions the owner must make") with "yes to all", delegated the five open 0.7 dispositions below
to the coordinator with an expert check, answered the 0.7.0 upgrade question (item 17) and
authorized one full-gate run at a candidate head. Independent review of the first commit
(FIX-FIRST, twelve findings) is applied in the same change and recorded in `docs/BUILD-LOG.md`.

## Owner answers (all yes)

| # | Question (`PRS-V1`) | Answer | Note |
|---|---|---|---|
| 1 | Core definition and companion list (`PRS-V1-001`) | yes | |
| 2 | Classification table is the 1.0 disposition of every shipped surface (`PRS-V1-010`) | yes | Surfaces missing from the table default to experimental. |
| 3 | darwin/arm64 Core; linux/arm64 and darwin/amd64 FALLBACK (`PRS-V1-004`) | yes | |
| 4 | linux/amd64 second Core platform (`PRS-V1-004`) | yes | No native linux/amd64 host is named yet. Without native evidence at candidate freeze the platform is reported FALLBACK, as the spec's failure mode says. GitHub Actions `ubuntu` runners are native linux/amd64 and are the default host unless the owner names another. |
| 5 | Installed Codex CLI and Claude Code stay FALLBACK Core host rows on exact versions (`PRS-V1-006`) | yes | The classification rows conditioned on this question are now Core. |
| 6 | Formal host FULL and protected authority leave the Core path | yes | Amendment recorded in `public-release-v0.md`. |
| 7 | V1-0014 independent interoperability is post-1.0; no interoperability claim in 1.0 (`PRS-V1-007`) | yes | V1-0014 leaves v0-7; V1-0015 is already COMPLETED, so its recorded V1-0014 dependency stays (the store refuses `set-dependencies` on completed tickets) and is moot. |
| 8 | V1-0019 stays a Core blocker in owner-closable form (`PRS-V1-008`) | yes | The untouched public repository is not yet named; the owner selects it before the cases are frozen. V1-0019 becomes MANUAL. |
| 9 | `PUB-V0-020` and V1-0004 move to the companion profile (`PRS-V1-009`) | yes | |
| 10 | Amend `public-release-v0.md` prospectively (`PRS-V1-011`) | yes | |
| 11 | Record acceptance in a numbered decision and reconcile the task store | yes | This decision; store reconciliation listed below. |

Acceptance of scope is not delivery, qualification or promotion. Every gate, ledger claim and
release readiness rule keeps its meaning; a Core-only candidate path does not exist yet (the
combined manifest reader and installer do not admit a Core-only packet) and remains a V1-0007 or
V1-0018 follow-up.

## Delegated 0.7 dispositions

12. Decision 0368 is ratified. AGENTS.md invariant 4, `SOL-V0-003` and the `URE-V0` non-goals now
    carry the carve-out for the operator-invoked `corvint eval --learn-slot-weights` step, whose
    output reaches ranking only after the frozen held-out gate admits it (`LTA-V0-009` to
    `LTA-V0-012`). The step stays experimental. Zero admitted proposals is evidence that the gate
    refuses, not against the carve-out. Ticket V1-0088 was completed and its code merged (PR #92)
    before this ratification; until this decision the merged code conflicted with the invariant
    text.
13. V1-0083 (identifier graph, personalized PageRank) is not delivered: the frozen evaluation
    regresses recall@20 on every subset, so its closing rule is unmet. Decision 0367 is accepted
    as experimental, off by default, and the ticket is archived and removed from v0-7. The
    off-by-default cost stays on the default path: the graph section is built on every index and
    the schema bump rebuilds every snapshot. One follow-up ticket carries the three open
    follow-ups from 0367 and the `NOT_RUN` full-sample rerun.
14. V1-0023 (compact evidence summaries) moves to v0-8 together with V1-0154, the preregistered
    matched complete-task trial. The trial's registration forbids running it before owner
    acceptance and its fixed conditions are still placeholders; the summary and expand views stay
    experimental and opt-in. The v0-7 criterion for summaries says so.
15. V1-0100 (application flows) is complete as delivered-experimental on the merged slice
    (57765fb, 70f531b): `AFU-V0` is traced, the browser gate ran on base a98d770, the dogfood
    binding is sealed (3f60009, 2a14291), and `make gate` against the merged commit is `NOT_RUN`
    under the owner's focused-verification policy. Its first ticket criterion asked for the full
    gate, browser gate and dogfood binding recorded against the merged commit; it is amended to
    record the browser gate on base a98d770, the sealed binding, and the full gate as `NOT_RUN`
    until the candidate-head run, so the COMPLETED status no longer contradicts the criterion.
    Profile acceptance is `NOT_PRODUCED`; the capability stays experimental under its last ticket
    criterion (acceptance only by explicit owner decision), the v0-7 application-flow criterion
    and the 1.0 classification table.
16. V1-0011 (the three Core ledger rows) cannot reach `VERIFIED`. The sealed run-001 receipts
    record `FAIL` for `UC-TASK-ORIENTATION` (one treatment-only critical miss) and
    `UC-CHANGE-CONSEQUENCE` (five treatment-only critical misses, all changed packages without
    tests), no Beamfall dogfood receipt exists, and no hostile-tests receipt exists. The three rows
    move to `experimental` with contract and implementation receipts; every claim stays
    `UNPROVEN`. The ticket stays open on those three gaps, which block v0-6 until a new sealed run
    passes and Beamfall dogfood is retained.
17. Owner answer 19 (the 0.7.0 N-1 upgrade question asked with the scope questions): the 0.6.0 to
    0.7.0 `upgrade-b` failure under the decision 0341 byte-identity rule is fixed in 0.8, not
    documented as a 0.7 limitation. Ticket V1-0190 and a v0-8 acceptance criterion carry the 0.7.0
    to 0.8.0 qualification. Ticket V1-0189 was filed alongside: `script/release-checklist` exits
    non-zero before promotion for every candidate, so readiness attestation must record that
    structural exit.

## Effects

- `docs/specs/corvint-1.0-product-and-release-v1.md` status accepted; `README.md` and
  `INDEX.json` rows updated; `docs/PRODUCT.md` pointer updated.
- `docs/specs/public-release-v0.md` gains the "1.0 Core scope amendment" section replacing the
  "Proposed v1 amendment" paragraph. No `PUB-V0` requirement text changes.
- `AGENTS.md` invariant 4, `docs/specs/self-observation-ledger-v0.md` (`SOL-V0-003`) and
  `docs/specs/unplanned-read-events-v0.md` (non-goals) carry the 0368 amendment; decisions 0367
  and 0368 record their accepted status.
- `conformance/use-cases-v0/ledger.json` rows for the three Core jobs become `experimental` with
  receipts under `conformance/use-cases-v0/receipts/`.
- Task store (`.taskman/`): V1-0001 completed; V1-0019 kind MANUAL with the owner-closable
  criterion; V1-0021 and the v1-0 criterion drop "independent interoperability passes"; V1-0015
  keeps its moot V1-0014 dependency (COMPLETED tickets refuse `set-dependencies`); V1-0014 leaves
  v0-7 and takes the milestone label `post-1-0` (the store refuses an empty milestone); v0-7 drops
  V1-0014, V1-0083 and V1-0023 and rewords three criteria: the proof-wire criterion ("versioned and
  usable from a digest-pinned CI verifier through its canonical vectors and the in-repo second
  consumer; no independent interoperability claim"), the summaries criterion ("experimental and
  opt-in ... trial deferred to 0.8") and the state-of-the-art criterion ("items that fail their
  frozen evaluation ... are archived as not delivered"); v0-8 gains V1-0023, V1-0154 and V1-0190
  and two criteria (the matched trial before default promotion; the 0.7.0 to 0.8.0 `upgrade-b`
  pass); V1-0083 archived; V1-0100 completed with its first criterion amended (item 15); follow-up
  tickets V1-0184 to V1-0190 filed for the V1-0011 gaps (V1-0184, V1-0186, V1-0187, V1-0188), the
  0367 follow-ups (V1-0185), the release-checklist exit (V1-0189) and the upgrade qualification
  (V1-0190).

## Rollback

Revert this decision's commit: the spec returns to DRAFT, the amendment section returns to the
proposed paragraph, invariant 4 returns to its prior clause (then the 0368 code must be reverted
too), the ledger rows return to `specified` with their receipts deleted, and the task-store
mutations are reversed by new mutations (reopen, restore, release update) since the journal is
append-only. No tag, publication or promotion depends on this decision.
