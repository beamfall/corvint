# V1-0266: wider symbols lead a standing query packet

Base: `71b2eb8fc3e56a9df368b8be8227bf5ad125f8c9` (current `origin/main`).
Intent: `GPK-V0-083` is **proposed, pending owner acceptance (V1-0266)**.
Implementation remains experimental; no decision file or owner acceptance is produced.
The maximum generated GPK requirement was 082; open PRs 749–759 were checked on
2026-10-10 and contained no later GPK reservation. Historical non-main commit
`c985828933d71d49c60dba2dd8b07f6e89ab55c0` supplies the idea, not accepted authority.

## Contract and implementation

Confident symbols with support strictly wider than every competitive record and both
admitted document slots lead a packet that independently passes the relevance floor.
Compare against the full record universe, not the result-limit prefix. Support uses query
words as written, so expanded stems are not independent matches. The existing class order
survives behind the placed symbols, which are not duplicated if already carried there.
Without a strictly wider symbol the original packet is returned unchanged.

The original packet and original support index decide the floor and GPK-V0-066 substitution
before placement. Unlike the historical draft, wider symbols outside the narrowed confident
selection add support only after that judgment, preserving the original floor semantics.
Record ties behind placed symbols no longer independently request widening; omitted competing
readings and nearest negative claims keep their existing checks. Learning and scores are unchanged.

The analyzer audit pins all production contextindex sources, including consumer ranking;
its conservative convention requires schema `corvint-analyzer/113` -> `corvint-analyzer/114`
and a recomputed digest. No current doc/golden pins `/113`; its two mentions in the historical
2026-10-08 batch-L build log retain their historical meaning. Line citations in
`core-compatibility-freeze-v1.md` and `snapshot-batch-v0.md` are repinned to their intended
unchanged statements. No packet golden or fixture file is intentionally regenerated.
Rollback removes placement and restores schema/audit pins together. Snapshots are derived state;
no task/trace store or wire migration is needed.

## Evidence and limits

- Baseline focused floor, substitution, omission and schema tests passed.
- The new historical-shaped precedence test failed before implementation: one-word plugin
  feature records preceded `PluginTrustRoots`, with `NEEDS_WIDENING`. The original failure is
  retained in `/private/tmp/v1-0266-red.log`.
- Focused tests passed after the port, including the four-phrasings fixture, precedence,
  floor, substitution, omissions, helper deduplication and analyzer-schema audit
  (`/private/tmp/v1-0266-focused.log`, 25.528s).
- A temporary comparison against the original `evalQuery` body proved nine complete JSON
  packets byte-identical for no-wider supported/unsupported queries at limits 1, 3 and 10
  (`/private/tmp/v1-0266-unchanged-bytes.log`, 20.225s). The temporary test is retained outside
  the repository at `/private/tmp/v1-0266-packet-compat-test.go`.
- Independent read-only review found no blocking correctness issue. It confirmed full-universe
  dominance, original-floor isolation, order/deduplication and schema conventions. Recommended
  follow-up coverage: an omitted wider competitive record blocking placement, and a placed
  symbol retaining an uncovered omitted-reading or negative-claim abstention. Existing logic
  remains unchanged for those checks; they are not separately demonstrated by this regression.
  The reviewer-requested new test names were added to the proposed traceability row.
- All requested doc gates passed against the staged temporary index:
  `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, and `use-case-receipts-test`.
  The last gate passed five regression cases; the original combined log is
  `/private/tmp/v1-0266-doc-gates.log`. No receipt, packet golden or fixture changed.
- Full `go test -count=1 -timeout 30m ./internal/contextindex` passed:
  `ok  	github.com/Beamfall/corvint/internal/contextindex	1262.594s` (`/private/tmp/v1-0266-package.log`).
- Standalone `sh script/repin-use-case-receipts.sh --check` passed (all 18 receipts current).
  `cmd/corvint/main.go` is unchanged, so no receipt regeneration was needed.
- The adopter-shaped limit-10 regression includes five one-word feature records, a decision
  that clears the floor, both `PluginTrustRoots` and `NewModelLoader`, and four task phrasings.
  Local fixture scores differ from the adopter's 705; score tuning is not part of the fix.
- Adopter before/after evaluation numbers are orchestrator-owned and `NOT_RUN` here.
  Repository-wide `make gate` and `./...` are `NOT_RUN`, explicitly excluded by the task.
- No commits, pushes, publication, decision files or `.taskman` writes are authorized here.

## Corvint self-use and environment

Used installed Core `1.0.0-rc.3` build 407 for the pre-change query and affected plans;
query named the owning GPK spec and retained limit/test-symbol omissions. Originals are in
`/private/tmp/v1-0266-query.json`, `v1-0266-affected-baseline.json`, and `v1-0266-affected.json`.
Affected advice remains advisory: mixed-language/build-constraint frontiers and unowned docs
are unresolved, and its broad gate suggestions do not override the explicit scoped commands.
The independent reviewer uses the inherited model/effort for ranking/evidence semantics;
live setting controls and account usage telemetry were unavailable (`NOT_OBSERVED`).

Snapshot refresh is unavailable: common Git metadata lives outside the writable sandbox.
`corvint index --if-stale` refused writing its snapshot temp file. Initial
`make dogfood-change` also refused its Git evidence directory. Post-commit CEM/OCM/check/seal
is `NOT_PRODUCED` under the explicit no-commit scope; no immutable final-binding claim is made.
The real Git index cannot be staged here. Documentation generation/checks use a copied index
and temporary object directory under `/private/tmp/v1-0266-git`; no common Git state is changed.
The native macOS no-argument `mktemp` chooses an unwritable system temp directory even with
TMPDIR set; a scratch PATH shim supplies an explicit template in the requested writable TMPDIR.

Installed Tasks reports `0.0.0-tcp01-unverified+build.407`. Official release listing was read
with `gh api`: latest Core is `v1.0.0-rc.2` (2026-10-06), latest standalone Tasks developer
channel is `tasks-dev-20260929.2`. No local downgrade or update was attempted:
`corvint-update check --allow-network --component core|tasks` failed its version smoke with
`process-after-start-failed: descendant snapshot unavailable`, so freshness/qualification
remains `UNKNOWN`. Tooling blocker filing is deferred to the orchestrator because the user
explicitly forbids ticket-store writes in this task.
