# Decision 0346 — One trust class per cited row, and a tainted row satisfies no basis

Date: 2026-09-22. Status: proposed (experimental delivery; ticket V1-0090). Adds TCP-V0-023 and
FPK-V0-032; changes no accepted wire.

## Context

A `context` evidence row and a `prove` proof row each carry an `authority` label that says where
the citation came from, but nothing says which labels a reader may lean on. The governance receipt
(TCP-V0-009) and the `critical` selectors (TCP-V0-011) were computed over every reserved row, and
the `prove` falsifier tables key on the label with no floor beneath them: a row labelled by a
learned ledger, an unverified analyzer contract, a provider fetch, or a label no generator emits
was set apart only by whichever table happened to name it. Prompt-injection work on agent
pipelines (CaMeL, arXiv 2503.18813) separates data by provenance so that fetched or tool-produced
content can never authorise an action; Corvint's invariant 3 says the same of authority, and
invariant 2 says an unknown must read as uncertainty, never as trust.

## Decision

Every evidence row of a `context` packet and every `prove` proof row carries exactly one `trust`
member from a closed set of five: `project-authority`, `repository-content`, `repository-history`,
`external-provider`, `tool-output`. The class is a function of the row's `authority` label alone,
through one table (`trustByAuthority`, `internal/contextindex/trust.go`) that both packets reuse; no
new input is read and the label is not verified. A label the table does not name is `tool-output`,
so an unlisted label is the least trusted class by omission. `external-provider` and `tool-output`
are tainted: in `context` a tainted reserved row satisfies neither `governance` nor `critical` and is
named in the always-present `coverage.governance_refused`; in `prove` a tainted row keeps falsifier
`none`, is never `PASS` and carries a `refusal` naming the row. Both changes are additive; the
`query` and `impact` wires, the `external` section, and the CEM ledger readers are untouched, and a
consumer of the previous row shapes decodes today's wire unchanged (tests in
`internal/contextindex/trust_test.go` and `cmd/corvint/prove_trust_test.go`). Stamping the class on
the `query`/`impact` wire and on `internal/extevidence` rows is left as follow-up work under their
own contracts.
