# Decision 0036 — release-hardening wave: honest packets, reproducible gates, the snapshot on the query wire

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "work to get
it to release. use sub agents as much as possible" (2026-09-02), given after the release-readiness
answer that named five blockers: the V4 gates on a fresh partition, `make gate` off the authoring
machine, query p95 on a Beamfall-scale repository, three confidence-reporting bugs, and the CEM
interoperability check.

## What is decided

1. A packet that abstains says why. `compactBudgetEnvelope` keeps `abstention.reason` on every
   branch (GPK-V0-045), and the Python oracle takes the identical repair, so no parity divergence is
   registered. The `cli-parity-v0` manifest is re-captured at the committed oracle revision.
2. A `query` packet whose every cited non-advisory result rests on `authority: syntax` alone counts
   zero `authoritative_results` and carries one deterministic uncertainty line (GPK-V0-046).
   Ranking is unchanged. `falsifiable-packet-v0.md`'s premise that the `query` wire cannot change
   is superseded by a dated note; its requirements are unchanged.
3. `query` and the harness `user-prompt` event read the `corvint index` snapshot when the tree
   matches (IDX-SNAP-V0-008) and treat a missing `.corvint/index/` as a miss without a Git
   observation (IDX-SNAP-V0-009). The packet is byte-identical to the built path.
4. `freshness.scope=git+working-tree` is defined for the `working-tree-untracked` impact profile
   (DIRTY-CACHE-012) and forbidden on dirty-cache-reuse receipts; the emitted value is unchanged.
5. `impact` and `harness event` keep their spec-authorised ledger appends (SOL-V0-001, SOL-V0-007);
   the top-level help discloses them instead of claiming the verbs read without mutating.
6. Three kernel paths that described a failure as an answer are repaired: a non-missing
   `git cat-file` error is `INVALID`, refused imports file one `Unparsed` row per source, and the
   CEM map serialisation matches the oracle's bytes for U+2028/U+2029.
7. `benchmarks/run.py` can score the native binary (`--engine corvint`) and a repository subset
   (`--repos`); a subset run can never report `v4_release.ready`.
8. `make gate` is reproducible from a fresh clone; see the BUILD-LOG entry for the composition.

Wave 3 (2026-09-02), owner directive verbatim: "skip the inert diversity patch, merge the rest of
wave 3".

9. `EvalQuery`'s confident-symbol selection carries the oracle's index-wide symbol term frequency
   and dependency-link promotion (GPK-V0-047); the Go engine has no critical miss on the
   development corpus.
10. Web (JavaScript/TypeScript) symbols are members of the index, not a per-query enrichment.
11. A verification plan prescribes a profile's command only when that profile was detected, and
    the fallback profile's closing gate is a Makefile target the repository declares. The oracle
    takes the identical repair, so no parity divergence is registered; the budget sweep and the
    `cli-parity-v0` manifest are re-captured from the repaired oracle.
12. Term specificity, the answerability gate and the type-diversity reservation are not merged:
    measured inert or net-negative (BUILD-LOG). Reference chains are not merged pending the
    precision-allowance call below.

## What was measured

- Beamfall clone (3,233 files), interleaved fresh-process n=20, load ~10, snapshot present:
  `query --limit 1` p50/p95 337.7/361.2 → 181.0/193.1 ms; `harness user-prompt` 421.3/451.0 →
  219.2/231.0 ms. Snapshot absent: unchanged within noise. 160 runs, one stdout digest per verb.
  GPK-V0-017(b) is met only where `index` has run; `conformance/perf-v0` never runs `index`, so the
  preregistered criterion still measures the miss path and is `NOT_RUN` for this change.
- Blind-v3 public repositories under both engines: contract metrics identical (recall 0.1, 8
  critical misses, top-five 0.5, abstention 0.0); non-critical selectors differ (byte-weighted
  precision 0.067107 Python, 0.073438 Go). Recorded as a bug, not repaired here.
- Wave 3, development corpus under both engines: Go 0/31 critical misses, recall 1.0, precision
  0.780332; Python 0/31, recall 1.0, precision 0.804952. Blind-v3 public: contract metrics identical
  under both engines; selector drift reduced to one cobra impact case (DR-0017 rows).
- Wave 3, reference chains (not merged): blind-v3 recall 0.10 → 0.20 (misses 8 → 7), development
  precision −0.069 against the 0.02 allowance.
- CEM 0.1 kit run standalone from outside the repository: `doctor` PASS, 50/50 fixture digests,
  the same-author Go probe 31/31. Not an independence claim; the gate still needs an outside author.

## What is not decided

The V4 generalization gate still fails on blind-v3 and can only be passed by a fresh untouched
blind-v4 partition authored independently; the four general mechanisms the diagnosis named are in
`docs/agent-memory/ideas.md`. Whether the preregistered performance protocol may run `index` first
is an owner call recorded in `docs/agent-memory/questions.md`, as are whether reference chains may
trade development precision for held-out recall and whether blind-v4 may be run once now.
