# pk-affected: Verifier repair 1

Owner intent: repair only the two failures reported by the 2026-11-05 host
verifier at `014bdd157ed8679a2518cb8569ef27119b7cdb5a`, retaining the lane's
immutable base `6c33df33bc5670131482f5263b410cb980580657` and frozen contracts.

The executable Go shim in `TestExecutableBuildHelpersHonorSharedGoCache` used
`os.WriteFile` with executable permission, contrary to ASD-V0-929. Use the existing
`execfile.WriteFile` helper, which holds the fork lock across the write. Preserve
the script, permissions, cache precedence checks and every existing assertion
(TSP-V0-7315).

The UC-CHANGE-CONSEQUENCE implementation receipt still pinned the earlier bytes
of `cmd/corvint/affected.go`. Run the existing receipt repinner to refresh its
subject digest and repository revision, then the ledger's receipt digest
(UCV0-004/005). No receipt schema, product output, golden or assertion changes.

`make -s use-case-receipts-check test-exec-writes-check` passes; all 18 receipts
are current. Its log SHA-256 is
`567c0eece45ede5c5c20e5b206b460acd67ee4e8f2f104765a8f1d49e12a3788`.
`script/heavy.sh make windows-vet` passes; log SHA-256 is
`f8bff6f70ed5f66e8e3171b28a9a8db6d7d423b6f1410222c5fbcc18cea6f676`.

The mandated heavy-wrapped whole `cmd/corvint` package run fails after 307.955 s.
The existing negative-repository fixtures discover the enclosing
`/home/russell/lanes` repository; work fixtures reject scratch ancestors owned by
another user; adapter fixtures also fail. These sandbox qualification limits
remain explicit, without changing assertions or product refusals. Whole-run log
SHA-256: `6468fc28a8f57653cfc72cbba9606d30ce107e9c32bc26eab9c5f7239be75566`.
The unchanged cache test passes in a separate heavy-wrapped focused run (0.038 s).

Raw logs and affected selection remain outside the repository under
`/home/russell/lanes/pk-affected-evidence/`, with prefix `verifier1-`. The external
handback records whole-package qualification, final CEM binding/check outcomes
and retained platform or sandbox limits. The prior pre-change capture is reused;
independent review and host doc-gates remain orchestrator-owned. No seal,
publication or Tasks write is authorized in this repair round.

Rollback: revert these fixture and pin repairs together; doing so restores both
reported verifier failures. No design fork or product contract amendment is needed.
