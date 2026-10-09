# 2026-10-09 — owner acceptances recorded by decision 0472

The owner approved three items in chat on 2026-10-09; decision 0472 records them.

- `CAL-V0-198..204` (V1-1042, issue 696): markers, section status, authoritative inputs, table row and
  delivery status flipped to accepted in `docs/specs/corvint-tasks-agent-leases-v0.md`, its
  `docs/specs/README.md` row and `docs/specs/INDEX.json`.
- `LCP-V0-018` (V1-1043): marker and digest flipped in `docs/specs/local-completion-policy-v0.md`.
- V1-0500 outcome: the delivered outcome is the portable task bundle, so `SESSION-V0-020..022` are
  marked accepted and the session-context-dividend intent status, README row and INDEX entry name
  them. `SESSION-V0-001..016` stay proposed and deferred; `SESSION-V0-017..019` are unchanged.

No requirement text changed. `docs/specs/REQUIREMENTS.tsv` was regenerated with
`make spec-requirements`.

Evidence: `go test ./internal/specindex` and the doc gates (`spec-requirements-check`,
`requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
`line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
`use-case-receipts-check`, `diagnostic-coverage-check`) passed before commit.

Limits kept open: CAL-V0-198..204 live qualification `NOT_RUN`; SESSION-V0-020..022 delivery stays
experimental with receiving-host and cross-provider continuity `NOT_OBSERVED`; the V1-0500
`COVERAGE_UNKNOWN` blocker, V1-0523 closeout adjudication and CEM rebinding stay open. No ticket was
mutated or completed.
