# Consume delivery evidence before dispatch; reconcile by default (V1-3496)

Owner charter: r1cleanup, Corvint half only, base
df315e35056d15c7933b5ff97c605cedf5ef4919. The tree already runs a blocking local
subject/charter screen; the triage's hint-only description is stale for that path.
The missing integration is consuming the extracted Tasks delivery-screen result.

Decision (ASD-V0-2201): run delivery-screen with the pinned full base and lane refs,
retain its JSON, and refuse delivered/on-lane/completed/held/in-flight/truncated,
unknown, malformed, duplicate or missing rows before publishing refs or writing
lane.env. Snapshot hosts use the runtime's unaudited route. CLEAR, MENTIONED and
NOT_OBSERVED may proceed: absent PR evidence stays in the retained result, never
becomes a claim that unseen PRs are clear. Local-screen overrides cannot bypass
this screen. Historical classification remains the Tasks lane's responsibility.
No new wire shape is introduced.

Decision (ASD-V0-1922): without LANDING_RECONCILE, run the installed Tasks
closeout-debt command from the checkout whose origin/main was fetched and verified.
The default is read-only, leaving umbrella/ambiguous citations as proposals. Preserve
explicit --no-reconcile, custom reconciliation, merge/fetch checks, once-only completed
scans, retryable refusals, and alerts. Never mark a missing runtime as reconciled.

Evidence: `LANE_SCRIPTS_GROUPS=tools bash script/lane-scripts_test.sh` PASS,
15 groups including existing dispatch/watcher checks and new classification/default
reconcile fixtures. The complete shell suite was also run and FAILED with 60
heavy/offload/host-environment and pre-existing workflow fixture diagnostics; the
raw log is retained, not relabeled as a pass. No heavy lock-dir override was used
for lane commands. The shell read-scope digest was rebound after auditing the new
fixtures: they still read only scratch build-log entries, not repository records.

| Log (under /home/russell/lanes/r1cleanup-evidence/) | Bytes | SHA-256 |
|---|---:|---|
| lane-tools-final.log | 36 | 37f5fbf2225257afc642c3f0dcb03cfdb90087187232cd82495583d76920e990 |
| lane-scripts.log | 80199 | 84a76d4ee0bd74fa991ff70bcb3befa1ae82d5ed0aa6cfa387ac3ff062ff62f4 |

Overall ticket PARTIAL: old-line/snapshot/prior-build-log classification, closeout
per-ticket deduplication, seal/umbrella filtering and scan-window implementation
belong to the other repository/lane. Real post-merge integration and independent
review belong to the orchestrator. No Tasks write or live landing action ran here.
Rollback: revert these script/spec changes together, retaining the explicit gaps.
