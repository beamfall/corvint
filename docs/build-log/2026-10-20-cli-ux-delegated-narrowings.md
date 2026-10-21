## 2026-10-20 cli-ux lane: delegated narrowings (V1-0945, V1-2068, V1-2052)

Recorded under the owner's delegated 1.0 feature cut (decision 0548). The orchestrator decided
each narrowing; no new owner acceptance was requested.

- **V1-2068: CCF-V1-005 `fresh` narrowed to source-clean.** A worktree whose only dirty paths
  are recognised `.taskman/**` projection files reports `freshness.state=fresh` with a separate
  additive `task_state` member (DIRTY-CACHE-017, accepted). The CCF-V1-005 member names and types
  are unchanged; `fresh` now means source-clean. After review, projection files are also excluded
  from indexing and evidence by the index path screen (analyzer schema `corvint-analyzer/132`, renumbered `/133` in batch 1021a beside the bounds lane's `/132`).
  A modified tracked `.taskman/queue.json` is therefore never served as fresh evidence
  (`TestImpactTrackedTaskStoreProjectionIsNotEvidence`, failing before the exclusion).
- **V1-0945: 3,665-byte root help narrows the ~1.5 KB target.** CCF-V1-009 (4,096-byte bound) is
  met. The ~1.5 KB ticket target is recorded as a limit: the required glosses and support-boundary
  disclosures take most of the bytes (`2026-10-20-v1-0945-root-help-maturity.md`).
- **V1-2052: ADS-V0-001..007 accepted.** `corvint adopt --setup` as specified in
  `docs/specs/adoption-setup-report-v0.md`. Review fixes: parsed hook JSON with real enablement,
  `check-ignore` warnings reported as `unobserved`, Lstat rejection of non-regular files, and a
  `fix` on every non-ok check.
