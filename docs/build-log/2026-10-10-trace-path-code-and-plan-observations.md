# Public trace overflow code and plan-limit observations

The owner requested two review fixes against baseline
`af13c1b4ae958dbd30403b77c43de0e4501fc579`, with no commits, pushes or Tasks writes.

The 200-path trace overflow now carries a typed cause. Both record CLI envelopes expose
`trace-path-limit-exceeded`; changed-path admission and aggregate fallback retain the internal
`admitted-path-limit` classification. Schema-v2/v3 stored validation preserves the typed cause
alongside its schema failure. DCW-V0-036 states the public/internal boundary. Rollback reverts
these changes and their tests, documentation, generated requirement locations and receipt pins.

Both plan row-limit reasons are admitted as dogfood observations. Independent review found
that the OCM emitter also uses the previously unadmitted `ocm-links` step. That step is now
admitted, and the coordinator's actual overflow rows are persisted in the boundary test.
The independent reviewer reported no remaining correctness findings after repair and reviewed
the stored-path tests and strict fallback parser. Platform stubs and help.go were excluded as
instructed; no changes were made there.

Regression tests first reproduced absent/wrong CLI codes and rejected observation writes.
Final focused tests passed for internal/trace, internal/observations, internal/dogfoodflow,
cmd/corvint record and dogfood-record tests, and internal/localcompletion's strict legacy
admission tests. One concurrent dogfoodflow run failed because a 250 ms process fixture never
created child.pid; the full package passed in isolation without product or fixture changes.
The failure is retained in /private/tmp/codex-review-fixes-units-final.log.

All requested documentation checks passed after repairing five shifted citations with unchanged
content anchors, regenerating REQUIREMENTS.tsv, and repinning the one affected implementation
receipt and ledger digest. The receipt check reports all 18 receipts current. Documentation
checks used /private/tmp/codex-review-fixes.index plus a private object directory: actual git add
was refused because shared Git metadata is outside writable roots. A private mktemp shim supplies
an allowed directory for template-free calls; explicit templates use the system command.
No repository gate implementation was changed.

Corvint query, path impact and affected advice were used; original receipts are retained under
/private/tmp/codex-review-fixes-{query,impact,affected}.json. Query and impact retain result-limit
omissions, and affected advice retains language-frontier unknowns. Governing scoped policy and
the owner's named tests selected the actual checks; no full make gate was run. Native index refresh
and make dogfood-change both failed on writes to shared Git metadata. Post-commit CEM binding and
seal are NOT_PRODUCED under the explicit no-commit instruction. No Tasks filing was attempted.

Installed Core is 1.0.0-rc.3 (build 407); Tasks is
0.0.0-tcp01-unverified+build.407. The optional official update checks failed their version smoke
with descendant snapshot unavailable, leaving release freshness UNKNOWN; no update was installed.
Account usage, billed tokens and cost are NOT_OBSERVED. The existing model/effort was retained for
this bounded repair and independent wire/error review; no savings claim is made.
