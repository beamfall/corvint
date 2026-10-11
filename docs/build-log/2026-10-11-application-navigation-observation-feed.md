# V1-1159, V1-1160, V1-1161, V1-1165 navigation observation and edge-ledger feed

Date: 2026-10-11. Base: `2ed4caa77a9f0a817279ce0179d6c6917d5222ac` (`codex/w5-appmap-b`).
Scope: four experimental navigation-ledger slices in `docs/specs/application-navigation-ledger-v0.md`,
each a proposed requirement pending owner acceptance: `ANL-V0-021` (V1-1161), `ANL-V0-022`
(V1-1159), `ANL-V0-023` (V1-1160) and `ANL-V0-024` (V1-1165). IDs `ANL-V0-017` to `ANL-V0-020`
were left unused for the parallel appmap-impact lane; renumber on integration if they collide.
No decisions, ticket-store writes, PRs or adopter content.

- Typed triggers (021): `goto`, `click`, `submit`, `press`, `goBack`, `goForward`, `reload` and
  redirect map to a closed trigger vocabulary; every other action stays `UNKNOWN`. An event
  annotation can only declare a typed trigger, recorded as `declared_trigger`, never as proof.
- Trace zips (022): read in memory (no extraction) from one root context trace with archive,
  entry, line, entry-count, URL and event bounds; navigations come from main-frame snapshot URLs
  and are attributed to an action only inside its before/after snapshots. Multi-page and
  multi-context traces refuse; input values are never read. Intermediate redirect hops are not
  visible and are not claimed. The caller supplies `--test-key`.
- Ledger feed (023): only the explicit `edges --observation FILE` flag applies observations; a
  sealed, untrimmed observation from the same map binds its test key like a declared `proves`,
  and proof still needs a qualified receipt at the observation revision.
- Self-proof (024): an assumed edge whose only prover is the assuming test is unsatisfied.

Evidence: focused `TestANLV00*` in `internal/appmap` and `cmd/corvint`. A local structural smoke
of `LoadTraceZip` on nine adopter trace zips read eight (24 navigations; counts only, no content
copied) and refused one holding two context traces. Live adopter qualification of the full
observe-to-ledger path is NOT_RUN. The test-plan `proven_by` path is not changed by 024.
