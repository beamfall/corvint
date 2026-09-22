# Decision 0005 — comparing `eval`'s `latency_ms` in the parity corpus

Date: 2026-08-28. Status: accepted. Authority: `GPK-V0-002`, `GPK-V0-033` (`spec-gap` resolution).

## Problem

`eval` was carried as "blocked on determinism." Measurement shows that description was too broad.

`latency_ms` (`src/context_corvint.py`, from `time.perf_counter()` calls at two other sites) is the
**only** non-deterministic value in the entire `eval` envelope. The other ~40 fields — accuracy
ratios, hit and case counts, byte totals, trace-replay figures, promotion thresholds, corpus digest —
are all derived from corpus content and repository state. `time`, `uuid`, `random`, and `datetime`
appear nowhere else in the eval path.

So `eval` is not non-deterministic. It is deterministic with one measured field.

## Spec status

No accepted spec states whether `eval`'s `latency_ms` must be byte-identical across runs. Under
`GPK-V0-033` that makes this a **`spec-gap`**, which must be resolved by amending the owning spec —
not by preferring whichever runtime is convenient. This decision is that amendment.

## Decision

Compare `latency_ms` **structurally, not by value**: the field MUST be present, MUST be a number,
and MUST be non-negative. Its value is not compared between runtimes or against a stored expectation.
Every other field in the envelope remains byte-exact with no exception.

This is deliberately stronger than excluding the field:

- a missing `latency_ms` still fails;
- a `null`, string, or malformed `latency_ms` still fails;
- a negative value still fails;
- adding, removing, renaming, or reordering any field still fails, because the rest of the envelope
  is unchanged byte-exact canonical JSON.

The case must declare the structural field in the manifest and name it in replay output, the same way
the accepted lock divergence and the location-dependent normalization already announce themselves. A
structurally-compared field must never read as an unqualified byte-exact pass.

## Rejected alternatives

**Exclude the field from comparison.** Strictly weaker: it also stops detecting a missing or
malformed field, which is the failure a port is most likely to introduce.

**Inject a clock (`CORVINT_FAKE_CLOCK` or equivalent).** Puts a test-only hook into shipped production
code and creates a second code path that only the corpus exercises. The corpus would then certify a
configuration no user runs. Rejected on the same grounds the migration rejects any `go` wire variant.

**Round or bucket the value.** Still environment-dependent; only moves the flake threshold, and
invites the corpus to be re-tuned whenever hardware changes.

## Consequences

- `eval` is unblocked for a port lane. Its remaining work is ordinary porting, not a determinism
  problem.
- This resolution covers `latency_ms` specifically and any future field that is a pure measurement of
  the run. It does NOT cover location-dependent output, which decision 0004 governs separately, and
  it does NOT license structural comparison as a general escape from byte-exactness. A field earns it
  only by being a measurement of the execution rather than a result of the computation, and only by
  being declared.
