# Generator and spec documentation repairs — V1-0524, V1-1109, V1-0178, V1-0163

Base: `29e1f653486fca13216606683d3a5c2b05cacd36`. Builder-only delivery: no commit,
push, new decision file or native ticket-store write. Each ticket owns separate files, except this
shared evidence record and the generated requirement index.

## Contracts and implementation

- V1-0524: SRG-V0-002 and SRG-V0-003 are **accepted, decision 0506**.
  Corpus inspection found 3,317 bullet definitions, 32 table winners, and one unmarked winner:
  the wrapped prose `SHA-256`. Unmarked definitions now require a paragraph boundary; marked
  definitions, table fallback, token syntax, ranking, staged reads and deterministic ordering
  remain supported. Fixture case 5 failed on `SHA-256` before implementation and passes afterward.
  No string-specific exclusion was added. Decision 0105's existing content pin retains its hash;
  only its shifted source line number changes, with exact old/new line equality checked.
- V1-1109: corrected the existing failure row, without changing requirements. Barrier invokes
  `redoPendingFor`, whose first operation removes orphan stages; reconcile instead reaches the
  locked request lookup and immediately refuses orphan slots. No existing test directly asserted
  barrier slot removal, so `TestCALV0019_BarrierRemovesOrphanStage` now checks it after proving a
  working initialized barrier cycle and the injected orphan/absent-descriptor preconditions.
  `TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting` supplies the contrasting witness.
- V1-0178: DCG-V0-001 is **accepted, decision 0506**. The checker names skipped
  basename citations on stderr; it preserves exit status, findings, continuation anchors and the
  unpinned ceiling. Case 22 checks passing, failing and root-basename behavior. The integration
  assessment now cites the full OCM section/prefix/ID grammar, distinguishes implemented
  experimental frontier from missing host closing authority, and pins both formerly bare spec
  citations to full paths and relevant spans. Installed `corvint frontier --help` succeeds.
- V1-0163: run-001 gets `benchmarks/daily-loop-v0/receipts/run-001/PROVENANCE.md`, referenced from
  PCCO acceptance evidence with the same decision 0332 and daily-change-evidence verb contract.
  The receipt consumer admits a closed field set and verifies exact receipt digests; the harness
  emits those digests. The current ledger uses run-002, not run-001. The sidecar preserves run-001
  bytes, historical seals and FAIL/FAIL/PASS outcomes, and distinguishes three measured treatments
  from qualification of all twelve Core surfaces. No requirement was changed for this ticket.

## Verification and limitations

Focused checks and independent review are recorded below as they finish. Full `make gate` and
repository-wide Go tests are NOT_RUN by explicit task scope. CEM binding, post-commit dogfood seal,
publication and native ticket completion are NOT_PRODUCED because this builder must not commit,
push or write `.taskman`. These edits do not accept the proposed amendments or close the tickets.

Corvint context query succeeded with three ranked results omitted by its requested limit and two
test-path candidates withheld. `affected` selected the store unit and retained UNKNOWN scope,
unowned shell/document paths and language-frontier uncertainties. The requested focused checks
supply evidence for this patch without claiming exhaustive impact closure.

Installed Core: `1.0.0-rc.3 (build 407)`; Tasks: `0.0.0-tcp01-unverified+build.407`.
Both `corvint-update check --allow-network --component core|tasks` calls failed with
`process-after-start-failed: descendant snapshot unavailable`; available release versions and
freshness remain UNKNOWN. No update was attempted after failed verification. Account usage and
credit telemetry were NOT_OBSERVED (no available usage tool). No cost or savings claim is made.

The real Git index and Corvint snapshot live under the shared repository outside the writable
sandbox. `git add` and `corvint index --if-stale` were refused. Checks use a copied scratch index
and separate scratch object directory, with the original object store read-only. The real index
is unchanged; the integrator must stage the patch. The host's no-argument `mktemp` selected a
non-writable Darwin cache despite TMPDIR; a temporary PATH wrapper supplies the requested TMPDIR
explicitly. Temporary cp/rm wrappers use `/bin/cp -f` and `/bin/rm -f`. No repository build recipe
was changed for these environment restrictions.

## Final observed results

Environment: `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3`.

- PASS: `sh script/gen-spec-requirements_test.sh` (including the before-fix failure above).
- PASS: `sh script/check-line-citations_test.sh` (including case 22).
- PASS: `go test -count=1 -timeout 30m ./internal/tasks/store -run 'TestCALV0019_BarrierRemovesOrphanStage|TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting'`.
- PASS: `go test -count=1 -timeout 30m ./conformance/use-cases-v0` (the sealed-benchmark receipt consumer; no daily-loop-specific Go test package was found).
- PASS: `make -s spec-requirements-check requirement-definitions-check traceability-tests-check decision-numbers-check` in the requested combined invocation, including `internal/specindex`.
- PASS after repair: `make -s line-citations-check error-code-ownership-check unbounded-readers-check use-case-receipts-check use-case-receipts-test`.
  The first combined invocation stopped at two citation failures: a blank first line in the new
  frontier span and decision 0105's shifted source locator. The new span was narrowed, and the
  unchanged old/new source bytes verified before moving the historical locator with its same hash.
  The repaired invocation exited zero; skipped-basename notes are diagnostics, not failures.
- PASS: `sh script/repin-use-case-receipts.sh --check`: all 18 receipts current; the receipt fixture
  suite reports five cases passed. No product receipts were repinned.
- PASS: generated TSV identity comparison: only `SHA-256` removed, zero IDs added, all 3,349 genuine
  existing IDs retained. The other changed rows are SRG/DCG amended titles and shifted locators.
  All three run-001 receipt JSON files compare byte-for-byte with HEAD.
- PASS: independent native reviewer found no substantive issues; its trailing-whitespace finding
  was fixed. Follow-up review of the citation-only repairs found no new issues. `git diff --check`
  passes. No model override was used for this bounded source/doc review; runtime effort telemetry
  remains NOT_OBSERVED.

Original context, affected-plan outputs, the TSV diff and check logs are retained outside source
under `/private/tmp/batch-gen-*`. Real-index staging remains the integration handoff; all process
runs owned by this builder have completed. Proposed requirements remain pending owner acceptance.
