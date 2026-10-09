# Decision 0476 — read facets, `--count` and release readiness counts (CAL-V0-206..209) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("CAL-V0-206..209 accepted").

## Context

The owner asked in chat on 2026-10-09 that corvint-tasks answer questions like "how many tickets to
1.0" instantly, for every kind of search. The read-facets lane proposed `CAL-V0-206..209` and
amendment A26 in `docs/specs/corvint-tasks-agent-leases-v0.md`: a bounded facet-count summary
(`--facets`) over the whole match set of `ticket search`, `ticket list` and `roadmap`; `--count`,
which returns only that summary; and `release readiness` member counts and milestone drift, with
the source observation skipped for a release without a candidate. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `CAL-V0-206..209` as written, with amendment A26 (the absent-only optional
`facets` member of `taskman-command-result/0`). The delivery status, the authoritative-inputs line,
the read-next line, the requirement table row, the requirement section status and each requirement,
the matching entries in `docs/specs/README.md` and `docs/specs/INDEX.json`, and the lane's build log
record the acceptance.

## Limits

This decision settles intent only. The evidence is focused tests and a read-only real-store
measurement (`docs/build-log/2026-10-09-tasks-read-facets.md`). Live qualification is `NOT_RUN`.
Native tickets V1-1051, V1-1052 and V1-1057 are not completed by this decision.

## Rollback

Revert this decision and return the CAL-V0-206..209 status text to proposed. To withdraw the
behavior, also revert the read-facets change: the `--facets` and `--count` flags, the `facets`
result member and its decoder, the readiness `memberCounts` and `milestoneDrift` members and the
candidate-less source-observation skip, then regenerate `docs/specs/REQUIREMENTS.tsv`. Stores are
unchanged: every affected verb is a read.
