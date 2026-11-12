# Decision 0598: remove Greptile; Codex CLI reviews replace it

- Date: 2026-11-12
- Status: accepted as owner direction (owner chat, 2026-11-12, relayed by the orchestrator). No
  owner acceptance row is written by the change that carries this file.
- Authority: owner direction, 2026-11-12: "remove greptile from the stack. we can use codex cli for
  reviews instead".
- Amends: `docs/specs/release-process-v1.md` (REL-V1-012, the `greptile-gate` merge-check
  paragraph and its waiver-count limit), `docs/specs/PRODUCT-PLAN-1.0.md` (the REL-V1-012 note),
  `docs/specs/ci-narrowing-v0.md` (CIN-V0-5019, the preserved required contexts).
- Retires: `greptile-gate-v0` (GRG-V0-001..002).
- Satisfies: the condition in `docs/specs/source-removal-v1.md` (RMV-V1-035) under which
  `.github/workflows/greptile-gate.yml` is deleted: the owner removed the required status.
- Read with: decisions 0594 and 0596; `docs/build-log/2026-11-12-remove-greptile.md`.

## Context

Greptile was made a required status on `main` on 2026-11-06 (GRG-V0). On 2026-11-12 the
organization's Greptile trial ended: every review request was answered with a reactivation notice,
so `greptile-gate` stayed pending and blocked an otherwise green pull request (beamfall/corvint#198).

## Decision

1. `greptile-gate` is removed from the `main` ruleset's required status checks. The required
   contexts are `go-product`, `ci-control-plane` and `doc-gates`. Done 2026-11-12 by the
   orchestrator on the owner's direction; the prior ruleset JSON is retained for rollback.
2. `.github/workflows/greptile-gate.yml` is deleted in its own `.github` pull request, after this
   change removes its test, so `main` never carries a test for a missing workflow.
3. The independent pre-merge review is a Codex CLI review (`codex exec -s read-only`, on the CLI's
   own login, never an API key) of the exact pull request head, with its findings fixed or
   recorded before auto-merge is armed. It is a process step, not a required status.
4. Release: there is no merge-check waiver to count, so the REL-V1-012 "greptile waiver during the
   release" scenario is removed.

## Rollback

Restore the ruleset's `greptile-gate` required context from the retained JSON, revert this change
and the `.github` deletion, and reactivate the Greptile organization.
