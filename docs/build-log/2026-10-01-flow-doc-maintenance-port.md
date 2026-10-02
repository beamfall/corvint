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
