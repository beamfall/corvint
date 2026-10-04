# Pool replay fixture clock consistency — 2026-10-04

Native ticket V1-0703 addresses the shared replay fixture failure observed independently in PR496 and PR505 hosted shard 2. A real-clock policy update could advance the receipt head beyond the synthetic release timestamp. The transaction correctly refused the earlier timestamp before writing, masking the strict-policy or ordinary-compatible-handoff assertions.

The fixture now uses its initialized synthetic timestamp for PolicyUpdate. Product clock behavior and CAL-V0-012 remain unchanged. No spec or catalog amendment is required. Rollback is the one-line fixture reversal.

The unmodified focused baseline passed on this host. A temporary negative fixture deliberately advanced the policy timestamp by one minute, proved canonical initialization and successful PolicyUpdate, asserted the earlier-head refusal and unchanged durable state, and reached CLOCK_BOUNDARY_REACHED before the original strict-policy assertion failed. The original source hash was then restored exactly before applying the one-line fix. The injection is absent from delivered source.

Supporting CODE270 evidence: strict-policy case PASS (1.193s), complete TestPoolLaneUntouched_Replay PASS (3.729s), and the same replay under race PASS (6.721s). Both branches retain their original policy and durable-state assertions. The independent source review passed with no findings and verified all 122 code artifacts and the final source hash. These dirty-source checks are supporting evidence, not final frozen receipts.

The initial zero-diff dogfood attempt failed. Retained NOT_PRODUCED reasons: cem-prepare git-diff-failed; cem-cite cem-map-not-produced; ocm-prepare-001 exit-2; ocm-status-001 exit-2; ocm-aggregate intent-scope-drift; cem-status map-unavailable; local-outcome outcome-input-not-provided. Prechange query/impact remain NOT_OBSERVED agent-receipt-absent. Affected retains UNKNOWN build-constraint variants and nested-module frontier. All CODE controller jobs joined; independently observed owned groups were absent without signals.

Terminal committed-target checks and CEM seal require their actual receipts. Publication, protected hosted checks, current-main integration into PR496/505, and native focused-docs completion remain separate root-owned obligations. This entry does not claim those outcomes.
