# Ticket priority

The Corvint task store treats `priority` as an ordered label (P0 highest). This page is the rubric
for choosing it. Pick the highest level whose test the ticket meets, then apply the adjustments.
Priority orders work; it does not decide release scope (that is the release's own cut).

| Level | Test |
|---|---|
| P0 | Blocks the release now, or harms the host or data: store corruption or loss, a broken security or authority boundary, host-breaking resource use, a release step. |
| P1 | Needed for the release with no workaround: a wrong answer presented as certain or a silent fail-open (product invariant 2), a release-gate gap, a regression of a frozen quality baseline, a committed release feature, or a defect that makes a capability unusable for a real user repository. |
| P2 | A confirmed defect or missed target that has a workaround or fails closed: a typed refusal where success is expected, latency over its budget, a confirmed flake, CI or lane waste, a missing diagnostic that costs a retry. |
| P3 | Polish and measurement: wording and hints, small speedups without a budget, cleanup, consolidation, a measurement to take, a suspected defect not yet reproduced. |

Adjustments:

- **Suspected starts low.** An unreproduced "suspected" defect is P3 until a reproduction is
  recorded; then re-rate it on the table.
- **Fail-open outranks fail-closed.** The same symptom is one level higher when it yields wrong
  evidence than when it yields a refusal or UNKNOWN.
- **External reports.** A defect reported from a real user repository is at least P2.
- **Residuals inherit nothing.** A residual filed from a delivered ticket is rated on its own test,
  not its parent's level.
