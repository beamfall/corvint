# Decision 0429: record trace producer provenance as schema 3

Date: 2026-10-04. Status: owner-selected design; requirement text proposed. The owner chose
"Schema v3 field" from the V1-0745 design options on 2026-10-04.
Ticket: V1-0745. Contract: `docs/specs/local-trace-producer-migration-v0.md`
(`LTPM-V0-015`, `LTPM-V0-016`).

## Context

Trace rows said nothing about which writer produced them. A model-reported outcome recorded
through the pi tool looked the same as an operator's `record`. The ticket asked for a closed
producer value under a schema-version bump, older rows read as `UNKNOWN`, and calibrate and eval
reports that count by producer and can exclude one without changing stored records.

The agent offered three options:
- a schema-3 field covered by the trace ID;
- a sidecar ledger keyed by trace ID;
- a producer inferred from the store path or verification text.

The sidecar adds a second mutable store that can drift from the rows. Inference invents certainty
the rows do not carry (AGENTS.md invariant 2).

## Decision

1. Every new row is schema 3 with `producer` in `cli`, `dogfood` or `pi-tool`. The producer sits
   inside the canonical basis, so it is part of the trace ID.
2. Schema-1 and schema-2 rows are not rewritten and read as `UNKNOWN`, which is never stored.
3. Every current reader learns schema 3:
   - query, impact and eval ignore it for ranking;
   - record and batch output carry it;
   - the console and skill export label it;
   - the dashboard admits it under the existing v2 registry without disclosing it;
   - the conformance verifier admits it.
4. `calibrate` and `eval` count traces by producer and accept a repeatable `--exclude-producer`
   that filters only what the read admits.
5. Once a store holds a schema-3 row, 0.8.1 and older binaries refuse it. That limitation is
   accepted. Rollback recovery moves the affected revision files aside.

## Consequences

Stored rows now distinguish operator, dogfood and model-reported outcomes for every reader that
asks, without changing schema-1 or schema-2 identities. Downgrading after a schema-3 write needs
the manual move-aside step in the contract's rollback section. Producer counts are disclosure
only: no gate, threshold or ranking reads them. Each LTPM-V0-015/016 requirement stays proposed
until the owner reviews its text. V1-0745 closes only after its acceptance evidence is retained
and the native completion write succeeds.
