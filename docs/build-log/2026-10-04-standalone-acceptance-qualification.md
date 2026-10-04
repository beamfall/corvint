# Standalone acceptance: retained local qualification for issue #394

Issue #394 / V1-0541 requested rejection of nonasserting and flaky new tests with a
rendered change-request body. The experimental standalone/source-direct implementation
was qualified at `ceaaddaa9a323760cd605235789218282ddc8fe7`, then sealed by the pure CEM
rename commit `1a5d734a5076193810c103b7d4bf16e82f363213` (PR #495).
This record preserves that historical binding; it does not qualify later source trees.

Eight frozen checks qualified: focused Go tests, focused vet, reporter tests, docs,
specindex, complete-attempt regressions, live N2, and the single live N32 campaign.
Independent integration review passed before live execution. Stable N2 was accepted
with measured Strength KILLED; nonasserting N2 was rejected with SURVIVED. Flaky N32
was rejected with 18 passed, 14 failed and no other outcomes. Its four additional
order/isolation probes produced original failed, reversed failed, isolation passed
and failed (36 actual runs). The one-file order was not varied. All three rendered
bodies matched their report digests. The campaign marker remains consumed.

Runtime companion SHA-256 was
`5131313d1fefc72d45ffca3841cd51444033dc6c794244debbf661911b7bf5d3`.
The CEM digest was `0a13c2e30f023f3a125decaf6854aa25d8025976e2c62cca2dda6209d843921d`:
21 supported hunks, 40 basis links. Strict finish passed at the binding; the archived
CEM separately verified after the rename. Post-seal keyed status reported stale
checks/report set/final check, retained explicitly rather than reset or rerun.
OCM was structural evidence only: 0 of 19 obligations linked, all unassessed.

Cleanup observed 29 outer groups and 6,942 retained inner PID/start identities absent;
port 4394 was available. This is bounded observation, not universal containment.
Authenticated operator/hook semantics, full descendant containment, general browser/host
qualification, compiled-source provenance, and Core/MCP/flows consumers remain unqualified.
Full `make gate` was NOT_RUN under the scoped issue policy. Failed earlier evaluations
and repaired-recorder evidence remain in the private inventory.

The adjacent `2026-10-04-394-qualification-evidence.json` identifies the exact final packet,
private inventory and raw witness hashes. Durable owner-controlled evidence is retained in
`.corvint/test-evidence/394-post493-01/` in the primary repository, ignored by Git.
On 2026-10-04 at 02:10:06 UTC, all 161 files / 19,457,371 bytes were copied without
rewriting their internal paths and every destination hash was verified. The private
inventory includes all 132 original final-packet artifacts, the final packet itself,
all 28 keyed enrollment files, selected check logs, raw requests/reports/bodies/process
records and the consumed marker. The retained `RETENTION-RECEIPT.json` has SHA-256
`454b96cb5e91f95a4a055dc960df0da640ecc31133c23b5c1a8c6fe8e6caf774`.
Native retention attempt254 was returned at receipt2425; ticket evidence was recorded
at receipt2426. This records evidence retention, not ticket completion.

This follow-up changes only documentation and evidence retention. It leaves the original
source, plan, enrollment key, binding, seal, archived CEM and live marker unchanged.
Integration, CI and native ticket completion are separate closeout facts; this entry
makes no completed-ticket claim. Rollback is a documentation revert; retained raw evidence
and the consumed marker must not be erased or used to authorize another live run.
