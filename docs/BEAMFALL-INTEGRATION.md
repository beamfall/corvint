# Beamfall integration contract

Beamfall is Corvint's first demanding consumer, not part of Corvint Core.

## Corvint owns

- deterministic indexing, evidence, ranking, receipts, budgets, traces, and evaluation;
- the Beamfall record adapter as a generic compatibility plugin;
- stable CLI and JSON contracts;
- package releases and checksums.

## Beamfall owns

- feature and scenario ledgers, accepted-decision policy, and verification commands;
- the Beamfall retrieval corpus and local successful-task traces;
- a pinned Corvint version and checksum;
- compatibility tests for `script/bf context-*`.

## Cutover sequence

1. Corvint publishes an explicitly licensed V4 alpha release.
2. Beamfall pins the exact release and checksum in its agent-tool dependency manifest.
3. Beamfall's existing `script/context_corvint*.py` files become thin compatibility entry points that
   call the package; their behavior tests remain in Beamfall.
4. Byte-equivalence tests compare the package and embedded implementation on Beamfall's frozen
   corpus before the embedded implementation is deleted.
5. Beamfall records an integration ticket as complete only after its full gate passes at the pinned
   revision.

No submodule, copied long-lived implementation, Beamfall-only Corvint branch, or mutable source path
is permitted after cutover.

