# Decision 0524 — Session Test Runner V0 requirements accepted

Date: 2026-10-11. Status: accepted (owner instruction in chat, "auto accept it once codex passes",
2026-10-11). Authority: repository owner.

## Context

V1-0040 was widened on 2026-10-11 into one `corvint test` command. It plans from the diff, runs
only the selected tests through runner adapters, shards deterministically, and falls back alike
locally and in CI. `docs/specs/session-test-runner-v0.md` proposed `STR-V0-001` through
`STR-V0-018`. A Codex read-only review ran four rounds. Rounds 1 to 3 requested changes and each
was addressed in the spec. Round 4 returned PASS with no findings. The owner directed acceptance
once that review passed. AGENTS.md invariant 8 keeps acceptance human-owned, and this instruction
is that acceptance.

## Decision

`STR-V0-001` through `STR-V0-018` in `docs/specs/session-test-runner-v0.md` are accepted as
written. The spec's intent status changes from proposed to accepted, citing this decision. The
open decision on the command name is closed: `STR-V0-001` fixes it as `corvint test`. The other
open decisions in the spec stay open.

## Limits

Delivery stays not-started. Acceptance relaxes none of `AFP-V0-028` to `AFP-V0-030`. CI narrowing
still needs the `AFP-V0-029` forward shadow bar. CI executes only universe placement until an
accepted `AFP-V0-028` amendment says otherwise. Neither call site moves before the `STR-V0-013`
parity report. Moving `.github/` call sites still needs `AFP-V0-016` admin consent.

## Rollback

To withdraw acceptance, revert this decision and the status text. The requirements return to
proposed. No implementation depends on this acceptance yet.
