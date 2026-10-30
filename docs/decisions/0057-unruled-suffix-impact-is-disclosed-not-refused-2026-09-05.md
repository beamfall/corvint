# Decision 0057 — An unruled-suffix impact path is disclosed, not refused

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-2-surface.md` after independent review of its cited evidence.

## Context

`GPK-V0-027` required a typed refusal (`unsupported-impact-path-suffix`) for a path whose suffix has
no named reverse-import rule, and `GPK-V0-038` bound the harness to that profile. Since DR-0006 the
kernel instead computes the packet and names the uncompiled dimension in `coverage.uncertainty`
(`internal/contextindex/receipt.go`, `reverseImportProfileGap`). The clauses and the surfaces
disagree: `impact` and `prove PATH` exit 2 on `.rs` or `.kt` while `harness event --event
file-change` answers the same path (`docs/reviews/r12-cold-start.md` F3).

## Decision

1. A path whose suffix the index admits receives the disclosed packet on every surface, byte-identical
   between the CLI and the harness. A path whose suffix the index does not admit keeps the typed
   refusal. `GPK-V0-027` is amended in place; `GPK-V0-038` loses its now-false refusal sentence. No
   reverse-import rule is granted for any new language; widening stays under `GPK-V0-027` and
   `GPK-V0-033`. The Python oracle refuses where the candidate discloses: one
   `conformance/divergence-register.md` row, outcome `python-defect`, expectation authored from the
   amended clause.
2. Untracked-path impact distinguishes a non-`.go` path (`unsupported-working-tree-impact-path`,
   checked before the repository condition) from a repository that lacks a slash-qualified Go module;
   `GPK-V0-029` gains the precedence sentence. The refusal stays: a non-Go untracked file has no
   parsed binding behind a packet.
3. The `affected` walk gives each language-opted directory (`build`, `dist`, `target` for Go) its own
   entry sub-bound; exhaustion skips that subtree and reports `go:included-directory-walk-bounded`
   through the plan frontier (scope `UNKNOWN`) instead of denying the verb. The global
   `MaxWalkEntries` denial is unchanged. `AFP-V0-008` names the admission, the sub-bound and the reason.
4. The hook adapters' `corvint-invocation-timeout` line names the deadline it exceeded and states that
   a deadline is a bound, not a diagnosed fault. The wire code is unchanged.

## Consequences

Indexed-but-unruled repositories get an impact packet whose coverage block states what was not
computed. Two tests that used `impact README.md` to provoke the refusal move to an unadmitted suffix.
No new wire member, degradation code, or language rule; `.go` path bytes are unchanged.

## Rollback

Restore the `ImpactRuleNamed` guard, the two spec sentences, the register row, the precedence check,
the sub-bound, and the adapter sentence; each item is independently revertible.
