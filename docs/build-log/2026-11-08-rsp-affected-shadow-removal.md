# Runner selection: shadow removed from `affected`, two P1 repairs

Decision (orchestrator-made under the owner's 2026-11-02 delegation; owner may
override). Independent review of the RSP integration branch found that the
`advice.runner_selection_shadow` member, added by the T2 slice
(`2026-11-08-runner-selection-shadow-t2.md`), breaks AFP-V0-009: that
requirement closes the `affected` advice member set. Six existing `cmd/corvint`
tests failed, and the member added roughly 25-30 KB of near-identical FULL
reasons to every default and compact receipt and to the guidance review
projection. The options were to amend AFP-V0-009 together with RGV-V0-013/015
and the goldens, or to keep the shadow out of the receipt. We chose the second.
`cmd/corvint/affected.go` and `affected_compact.go` are back to their base
bytes, `cmd/corvint/affected_runner_shadow.go` is deleted, and AFP-V0-009 is
unchanged. The shadow selector stays reachable only through the opt-in
RSP-V0-051 reader: `corvint test-evidence --explain TEST --base COMMIT` and the
`test-evidence` MCP tool profile. `TestRSPV0040AffectedReceiptCarriesNoShadow`
pins the absence in the default, `--full` and compact receipts. The
`UC-CHANGE-CONSEQUENCE` use-case receipt subject digest for `affected.go` is
repinned to the restored bytes.

Consequence: `internal/runnerpolicy` (RSP-V0-038, 039, 046, 057) no longer has a
production consumer. Its trace row is marked PARTIAL. RSP-V0-046's
shadow-only rule is unaffected, because the governing plan never read the shadow.

Repairs in the same change:

- RSP-V0-003 (`internal/jslinecov/runner_results.go`): the Node reporter
  dropped `test:fail` events and relied on the per-file summary. When that
  summary was missing (a truncated report), a failure could vanish. A validated
  `test:fail` now records its file as pending. Any pending file without a
  summary becomes one failed result under the file identity, and a later
  summary does not double count it. `TestRSPV0003NodeTruncatedFailureKept`
  checks that the inventoried run stays failed after a passing FULL retry.
- RSP-V0-057 (`internal/runnerpolicy/miss.go`): requalification used the
  highest sequence among all admitted records with the runner's name, so a
  record for another configuration could clear a miss.
  `runnerqualification.Plan` now carries the accepted record's own `Sequence`,
  and only that sequence can clear it.
  `TestRSPV0057OtherConfigurationCannotClearMiss` covers the case: A passes at
  9, A misses at 10, B is admitted at 11, and A's miss stays.

Also: the Agent digest now records the Ruby runners' present Inventory and
TestEvidence pieces. Delivery status is `experimental`. The RSP-V0-033..037 row
is PARTIAL until the pinned declaration file (RSP-V0-028) is read.
`internal/mcp/cli` uses a `toolProfileTestEvidence` constant.

Rollback: revert this change. That restores the advisory member, together with
the AFP-V0-009 conflict and its failing tests.
