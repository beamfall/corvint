# V1-0266 review round 3: retain promoted symbol support with its evidence

Base: `8f16969557041c6480feeaa27326627c876adb5f` (merged V1-0266 + V1-0315).
Intent: `GPK-V0-083` remains **proposed, pending owner acceptance (V1-0266)**.
No new requirement ID, decision, owner acceptance, promotion or ticket completion is produced.

## Finding and minimal repair

The round-3 P2 finding showed that placement retained the first candidate for a
rendered symbol identity while a separate loop overwrote its support with the
last duplicate. Same-named methods on different receivers could therefore cover
an omitted record using vocabulary absent from the emitted method's evidence.
`evalLeadWithSymbols` now assigns support inside the existing placement loop,
after its duplicate guard; placement and support use the same retained candidate.
The existing proposed clause explicitly excludes a discarded duplicate's support.
The relevance floor, substitution and active negative-claim checks still precede
placement and keep their existing support index.

`TestEvalQueryDuplicateSymbolSupportPreservesOmittedRecordAbstention` builds two
same-named receiver methods with separated source-context windows. It verifies
first-candidate support `{validate, cache, alpha, gamma}`, duplicate support
`{validate, cache, beta, delta}`, and an admitted competing `{beta, delta}` record.
At limit 1 it requires the first candidate's complete evidence, active
`omitted-competing-record` abstention and `NEEDS_WIDENING`. Without the repair,
the fixture assertions pass and the abstention assertion fails with `READY`
and reason `none` (`review3-red.log`). Earlier fixture-only failures were corrected
by separating source windows and ensuring the record reaches the competitive arm;
they are not behavioral red evidence.

The production source changed, so the conservative analyzer audit pin is recomputed
to `787774204dadc193b71545a30d7ab6cf1b5af25cd843d092b3295895b6815119`.
Schema stays `corvint-analyzer/116` as requested. No CLI dispatch, packet fields,
receipt pins or frozen goldens change. Rollback restores the placement/support loop
and its matching audit digest together, without a further schema bump or store migration.

## Verification and limits

Evidence is retained under `/private/tmp/claude-501/gotmp/q/`, using
`GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`.

- Baseline focused tests passed (`review3-baseline.log`, 0.983s).
- The new behavioral regression failed without the fix; focused placement,
  precedence, negative-claim, floor, substitution and analyzer-pin tests passed
  after it (`review3-focused.log`, 5.696s).
- The full `internal/contextindex` package test passed
  (`review3-package.log`, 600.328s).
- All nine requested document targets passed (`review3-doc-gates-final.log`).
  An evidence recheck later refused a stale scratch index: receipt test fixtures
  inherit `GIT_INDEX_FILE` and replaced it with their fixture index. The real index
  was untouched; rebuilding the scratch index from the original resolves that
  check input. The failed attempt is retained as `review3-doc-gates-evidence.log`;
  the rebuilt-index run passed all nine targets and five receipt-script fixture cases
  (`review3-doc-gates-evidence-final.log`).
  The first run found two shifted line citations in
  `docs/specs/core-compatibility-freeze-v1.md`; their line numbers were updated
  with the original content hashes preserved. Generated `REQUIREMENTS.tsv` is
  byte-identical to the base. No intent in the compatibility spec is amended.
- Standalone `sh script/repin-use-case-receipts.sh --check` passed all 18 receipts
  (`review3-receipts.log`). CLI dispatch and receipt pins are unchanged.
- Independent native read-only review found no source blockers and independently
  confirmed the analyzer audit digest, first-candidate support/evidence binding
  and the regression's behavioral red/green evidence. Frozen source-diff SHA256:
  `ae9e3e01f759970456230aa53a9e590e9e85fb8a1040e1cc036e73ca010e1fed`.
  Findings are retained in `review3-independent-review.txt`. That review excludes
  the mechanical citation repair and this evidence entry.
- Built `/private/tmp/claude-501/gotmp/q/corvint-after3`, SHA256
  `41f8e8766d3102652b55be13febe138b6a57e498dfa647d7f61f63b73ab3c8f3`;
  unchanged golden SHA256
  `d4ae96df2a3bf01d05a44a848894390c915b8ea34b93cabba9ef2a31a4ad9392`.
  All seven evaluated cases preserve supplied-base selectors/order, states and
  empty missing lists, critical misses `0`, recall `1` and must-read hits `11`.
  **Strict supplied-base parity is FAILED / environment-blocked**, not waived:
  native isolated status reads the protected original Git metadata despite the
  shim, so `trace_state` becomes `blocked-mixed-worktree` instead of `absent`
  and packet bytes become `48693` instead of `47690`. The normalized supplied-base
  SHA256 is `b1a01e5dce1ffe5b2a2ee1db2e536b40e44ea07eb0b33a76e89c7f6410aa9807`;
  candidate SHA256 is `7c5b6b673ee2f80a5db4c00a8bc307e0e323d848ad65fd91ab9568544cb134d3`.
  Normalization removes only `evaluation.metrics.latency_ms` and `evaluation.revision`.
  A supplemental existing-base-binary control in the same environment matches the
  candidate exactly under that normalization (`review3-eval-base-same-env.json`),
  independently confirmed by the reviewer. It does not replace the failed clean
  baseline comparison. Clean native checkout and strict replay remain unresolved.
- `git diff --check` passed. No commit, push, ticket mutation or full gate was run.

Corvint pre-change query and affected plans are retained as `review3-query.json`,
`review3-affected-before.json`, `review3-affected-red.json` and `review3-affected.json`.
The query's exclusions and affected-plan language-frontier unknowns remain visible.
Snapshot refresh and `make dogfood-change` cannot write protected shared Git metadata.
Final post-commit CEM/OCM binding and dogfood completion are `NOT_PRODUCED` because
commits are prohibited. No full gate or repository-wide test command is run.

The document gates use the existing local GNU `mktemp` helper so scratch paths
respect the approved `TMPDIR`; the macOS default ignored it and was denied.
The final package-result/evidence prose updates change no source or checked spec inputs;
the passing document gates are retained under that limit.
The spec is staged into a scratch index/object directory for generated requirements
and index-based document checks; the protected real index is unchanged.
The adopter checkout also uses copied scratch Git metadata because direct checkout
cannot write its protected real index. A local shim invokes real `/usr/bin/git`
with that Git directory and the actual adopter worktree; its HEAD and clean status
must prove commit `6e82abd857c77c0ca250b6c4a8f5e1c86684a535`. The first eval attempt
without the shim used the old HEAD after environment sanitization and is not valid
parity evidence. No shim fabricates Git responses. After retaining the comparison,
the adopter's files were restored to its original commit
`0d7796be23efe4e2579408a5516b8aeff1a23008` through separate scratch metadata;
its real `git status --short` is clean and its original metadata was never changed.

Installed Core is `1.0.0-rc.3 (build 407)`; Tasks is
`0.0.0-tcp01-unverified+build.407`. Official release inventory lists Core
`v1.0.0-rc.2` and standalone `tasks-dev-20260929.2`. `corvint-update check`
for both components failed `process-after-start-failed: descendant snapshot unavailable`;
freshness/compatibility remains `UNKNOWN`, with no downgrade or unverified install.
No ticket store writes are made, including filing these environment blockers.
Account usage/credit telemetry is `NOT_OBSERVED`; no provider limit was reported.
