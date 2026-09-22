# Decision 0004 — normalizing location-dependent output in the parity corpus

Date: 2026-08-28. Status: accepted. Authority: `GPK-V0-002`, `GPK-V0-033`.

## Problem

`record` emits an absolute repository path in its `store` field. Go
(`internal/tracerecordrepo/adapter.go:237@4858d617`, via `trace.StorePath`) and Python
(`src/context_corvint_trace.py`, `str(store)` derived from `corvint.root`) **both** do this, and they
agree. An earlier note treating this as a Go defect blocking the `record` positive case was wrong.

The actual blocker is that a captured expectation embeds the capture machine's root, so replay on any
other root or machine fails on bytes that carry no behavioral meaning. `eval`'s `latency_ms` is the
same class: real output, no stable expectation.

## Decision

Separate the two comparisons, which were previously conflated:

1. **Cross-runtime parity** — Go against Python, same root, same environment, same process
   conditions. Remains **byte-exact with no normalization whatsoever**. `GPK-V0-002` is untouched.
   This is the comparison that certifies the port.
2. **Cross-environment replay** — a stored expectation against a later run at a different root.
   Applies a declared normalization to fields marked location-dependent.

Normalization obeys the same discipline the operator set for the lock-artifact carve-out
(2026-08-28), because the failure mode is identical — a blanket allowance silently hides real
defects:

- **Narrow.** Only the repository root prefix, and only in fields explicitly declared
  location-dependent in the manifest. Never a general "ignore paths" rule.
- **Directional where a direction exists.** Non-determinism that is not location-derived is not
  covered. `latency_ms` is not a path and does not qualify; it needs its own treatment.
- **Visible.** A case relying on normalization is marked in the manifest and in replay output, so it
  never reads as an unqualified byte-exact pass.

## Consequences

- `record`'s positive case becomes replayable and its manifest row can advance once its remaining
  classes replay.
- `eval` stays blocked. `latency_ms` is genuinely non-deterministic rather than location-dependent,
  so this decision does not unblock it; it needs either an injected clock or exclusion from the
  compared surface, and that is a separate spec question.
- Because both runtimes agree here, this is **not** a divergence and gets no register entry. The
  register is only for Go/Python disagreement (`GPK-V0-033`).

## Rejected alternative

Capturing expectations at a fixed well-known root. It makes the corpus depend on a writable absolute
location, which fails in sandboxes and parallel worktrees — the same fleet conditions the lanes
already run under.
