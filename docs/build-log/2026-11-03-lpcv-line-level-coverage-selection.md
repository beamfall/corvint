# LPCV amendment: line-level coverage test selection

Base: `ea09e3db5` (`origin/main`). Decision 0590. Requirements `LPCV-V0-950..963`; amended
`LPCV-V0-008` and `LPCV-V0-027`.

The owner asked that a changed line run only the tests it can affect, E2E and every runner included,
and fast. The existing LPCV contract was the right home but forbade coverage-based exclusion. The
amendment adds per-test blob-OID-keyed records, a static fallback list, cross-process collection,
a per-family collector inventory, a miss detector and a latency budget. The Go prototype
measurements are in decision 0590. Status: accepted, not-started; nothing is delivered.
