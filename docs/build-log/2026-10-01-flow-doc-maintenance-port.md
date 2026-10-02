# Flow-document maintenance ported to current main — 2026-10-01

V1-0465 ports the bounded flow-document maintenance profile (FDM-V0) from the unmerged branch
`origin/codex/flow-doc-maintenance` (sealed at `44cf8ac58f761327859ae98652c2f9119cd0f934`, base
`2e281305c49badc68d1a46035ca4ff966cbaff7c`, 593 commits behind) onto `origin/main`
`25ba271a1f34cc7de5990ef1f2662135f6ab1b0d`. The codex branch's 2026-09-28 build-log narrative and
CEM are not carried over: that evidence is bound to the old base and stays on the codex branch as
historical record, not as proof for this port.

The port is additive. `internal/appflows/docs_maintain.go`, `internal/doccorpus/apply_pair.go`,
their tests, `cmd/corvint/flows_docs_maintain_test.go` and the spec were copied from a local
checkout of the codex branch, and the `cmd/corvint/flows_docs.go` hunk applied cleanly. Neither
that branch nor `44cf8ac5` is published on origin, so fidelity to the codex source cannot be
verified from the public repository; what is verifiable is the content of port commit `4c73e037`
and the tests below. Drift checked against today's code: main widened
`doccorpus` for `/2` corpora (`MaxCorpusBytes`, `encodingLimit`), but `Encode` keeps the 4 MiB
`MaxBytes` bound for every value other than a `/2` manifest or artifact, so the maintenance
proposal and receipt bound in FDM-V0-002/005 is unchanged. Main's separate `docs maintain`
(SDD-V0) is a different profile and is not touched. One addition beyond the codex branch:
`flows --help` now names the experimental `flows docs maintain` usage, so the CLI label reflects
the delivered profile. The spec, INDEX.json entry and README row are added; REQUIREMENTS.tsv is
regenerated.

FDM remains proposed/experimental. Owner acceptance of the technical contract, Beamfall and
release-boundary qualification against this port, separate docs-MCP parity for maintained output,
and concurrent-process qualification are NOT_RUN here. Repository-wide `make gate` is NOT_RUN by
the owner's scoped-work instruction; focused tests come from `corvint affected`. Rollback reverts
this commit; the pre-existing immediate `flows docs` generation and `--check` are unchanged.

The ticket's touchPaths name `2026-09-28` build-log paths from the codex branch; this port records
its evidence here instead, so the delivered path differs from the ticket's touchPaths.

Repair r1 (review rev-465-r1 FAIL, tests did not exercise the checks they named). The refusal cases
in `TestFlowDocMaintenanceRefusals` previously applied the pre-tamper digest, so every case was
caught by the digest mismatch. Each case now re-previews after tampering and asserts its specific
refusal message on both preview and stale apply, and checks that the outputs are unchanged. New
cases cover untracked and status-hidden inputs and outputs, a valid receipt whose outputs are
stale, and a stale digest against a no-op state. Two package-private seams in `docs_maintain.go`
(`publishMaintenancePair`, `readRunEvidence`) let `TestFlowDocMaintenanceDerivationRaces` race
the staging, evidence-reread and HEAD rechecks. `TestMaintenancePairPostPublishRecheck` edits the
first output while the second publishes, and `TestFlowDocMaintenanceCLIGuards` covers both CLI flag
guards and the receipt-delivery report. Each named check was deleted in turn and its named test
failed (15 mutations, all killed). The pre-mutation digest check is kept, not removed: it is the
only refusal before the no-op early return, so without it a stale digest against a no-op state
returns the previous receipt (`noop-stale-digest` kills it). The receipt-integrity condition is
mutation-checked as a whole, not per disjunct, and the post-publication parent recheck in
`apply_pair.go` was outside the review list and stays untested.
