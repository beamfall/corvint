# LSP snapshot independent-review repair

Independent review of `add1fc8b7a926c02bc000e1f980f4cfe4398a76e` found that the cloned overlay URI
was paired with a map key retaining the caller's original string backing allocation. Session ID and
root assignments had the same issue. Small admitted substring lengths could therefore retain large
caller allocations despite the live content budget. The reviewer supplied a deterministic pointer
comparison reproducer rather than relying on noisy heap-size measurements.

`TestCallerBackingOwnership` reproduced all affected paths before repair, including the map key on
both open and change. Session construction/reset now clone retained IDs/roots and map insertion uses
the owned overlay URI. The test checks admitted byte equality and distinct backing storage for a
short substring of a one-MiB allocation. It also covers the already-owned overlay text and URI.
`TestEqualSessionCoordinates` separately verifies cross-session rejection with equal session ID,
root, URI, epoch, generation, version and digest; the previous test had differing generations.

Post-repair Go 1.27.1 race tests for `internal/lspsnapshot` and `internal/specindex`, snapshot vet,
and spec-requirements, requirement-definitions, traceability-tests, decision-numbers and line-citations
checks passed. Raw red/green/check logs and affected receipts are retained in
`/tmp/lsp-snapshot-evidence/repair-*.log` and `affected-repair*.json`. Source/test edits invalidate
the prior candidate's check binding; these new observations cover the repaired content. Weekly usage
was 90% at repair admission. Independent re-review and final CEM binding/seal remain pending; there
is no support, integration or ticket-completion claim. No full gate or publication was performed.
