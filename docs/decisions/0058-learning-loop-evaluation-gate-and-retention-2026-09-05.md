# Decision 0058 — Learning loop: the evaluation gate, retention, and the secret screen

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-3-learning.md` after independent review of its cited evidence.

## Context

Invariant 5 requires learning to be evaluation-gated. `docs/reviews/r11-learning.md` confirms no
requirement ID, code, or measurement implements that clause: `internal/evalrepo/evaluate.go`
refuses to evaluate when any passed trace exists, so the learned path has never been scored. The
store fills at 1,000 rows and never recovers, and the shared secret screen is shape-only.

## Decision

1. "Evaluation-gated" means the mechanism gate already accepted in decision 0037 item 4 (development
   byte-weighted precision at or above 0.80 under both engines; held-out recall non-decreasing),
   applied to the learned path. Per-store and per-row evaluation admission is rejected: it would need
   a golden set for the user's own repository, unobtainable inside invariant 7.
2. A new spec `docs/specs/learned-trace-admission-v0.md` carries `LTA-V0-001` (a second scored arm in
   `corvint eval` over the same pinned corpus with a frozen trace fixture; deltas reported in gate
   order: critical misses must not rise, abstention and epistemic-state accuracy must not fall,
   development precision against the 0.80 floor, recall informational; a delta under two cases is
   reported as not distinguished), `LTA-V0-002` (fixture and case disjointness is checked, and a
   contaminated fixture fails the run), and `LTA-V0-003` (retention). The BT-3 A/B replay mechanism
   is accepted as this measurement; its admission semantics are not.
3. Retention is whole-file eviction inside `record` only, ordered by Git ancestry distance from HEAD
   (unreachable-commit files first, then files holding no passed row farthest-first, then the rest
   farthest-first). Wall-clock age is rejected because it breaks byte-identical repeat queries. Caps
   are unchanged, so the frozen dashboard bound contract is untouched.
4. The screen's assignment alternation gains `credential|credentials|passphrase|passwd`. `auth` is
   rejected (it would match `authority` and `author`, Corvint's own vocabulary) and an entropy
   heuristic is rejected (a 48-hex string is shape-identical to a digest). The four copies of the
   pattern move together behind a parity test; the reader copies that validate stored rows widen only
   after row-level rejection exists, so a widened screen never bricks a store holding valid rows.
5. Blind-v4 stays sealed under its preregistered run condition; nothing here may be validated
   against it. `GPK-V0-044` keeps describing shipped Python-parity semantics; any Go change to
   learned-path semantics moves `src/context_corvint_trace.py` in the same change or is registered in
   the divergence register. The LTPM non-goal line cross-references LTA-V0.

## Consequences

`corvint eval` gains a second arm and a delta block. Learned-path score constants change only on a
measured non-negative delta. Existing stores keep working; first eviction occurs only at the cap.

## Rollback

LTA-V0 is additive: delete the spec, revert the second arm (restoring the present refusal), revert
the eviction branch (restoring the hard refusal at 1,000 rows), revert the four screen copies
together. No stored row, digest, or frozen partition is mutated.
