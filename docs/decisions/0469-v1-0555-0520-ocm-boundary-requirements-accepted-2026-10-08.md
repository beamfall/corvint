# Decision 0469: accept OCM-V0-017 and OCM-V0-018

Date: 2026-10-08. Status: accepted (owner approval in chat, 2026-10-08: "accept OCM-V0-018 and
OCM-V0-017").
Tickets: V1-0555, V1-0520.

## Context

Two lanes proposed requirements in `docs/specs/ocm-v0-dogfood.md` for OCM refusals that read as
missing capabilities:

- V1-0555 found `ocm link` refusing `claim-obligation-mismatch` when an anchor adjoins the
  obligation ID to further token bytes (`CVI-V0-002-strict-grammar`), with no explanation of the
  exact-token rule.
- V1-0520 found that a Go `t.Run` case anchor on the same line as its parent `func Test...(` header
  yields no case claim, because the read-path verifier takes the parent from the last header that
  ends before the anchor's line, and the `/case:` miss hint wrongly listed that layout as supported.

## Decision

1. `OCM-V0-017` is accepted: the `claim-obligation-mismatch` refusal keeps its text and appends one
   bounded hint naming the adjoined ID, the adjoining byte and side, the token rule and a
   space-separated example. The `OCM-V0-005` exact-token matcher, refusal code, exit status and
   `ocm verify`/`status` messages are unchanged.
2. `OCM-V0-018` is accepted: a case anchor on its parent's header line is intentionally unsupported
   syntax. It yields no case claim, its test claim remains, a `/case:` selector for it is refused
   `claim-selector-out-of-range`, and the miss hint names the boundary. Admitting the layout needs a
   verifier contract change with its own oracle and divergence adjudication (V1-1033).
3. The spec's overall intent status stays proposed; only these two clauses are accepted.

## Limits

Neither clause widens matching, extraction, selection or any verdict; both change refusal text
only. The V1-0520 boundary was not compared against the frozen Python oracle, which is not in this
snapshot.

## Rollback

Return both clauses to proposed by restoring their "(proposed ...)" markers and the spec status lines, and removing this file and its index row (they land in the V1-0520 merge commit of batch J). Reverting the V1-0555 or V1-0520
lane merges removes the hint text and the requirement.
