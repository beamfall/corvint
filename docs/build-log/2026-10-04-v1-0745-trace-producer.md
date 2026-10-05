## 2026-10-04 V1-0745: trace producer provenance as schema 3

Human-owned intent: the owner asked to start V1-0745. Trace rows carried no record of which writer
produced them. The owner chose the "Schema v3 field" design (decision 0429): new rows are schema 3,
`producer` is part of the trace ID, schema-1/2 rows read as `UNKNOWN`, every reader learns
schema 3, and 0.8.1 and older binaries refuse a schema-3 store, recovered on rollback by moving
those rows aside.

### Change

- **`internal/trace`.**
  - `SchemaVersionV3` and the closed producer set `cli`, `dogfood` and `pi-tool`.
  - `UNKNOWN` is reported for schema-1/2 rows only and is never stored.
  - Schema 3 is decoded by the strict typed decoder and refuses each of the following:
    - a missing, unknown, non-string or `UNKNOWN` producer;
    - a producer on a schema-1/2 row;
    - a producer changed without re-sealing the ID.
  - Migration re-identifies a schema-3 row and keeps its producer.
  - Amended as LTPM-V0-015.
- **Writers.** `record` writes `cli`, the pi tool writes `pi-tool`, and dogfood recording writes
  `dogfood`.
- **Readers.**
  - query, impact, eval and batch read schema 3.
  - The console and skill export label it.
  - The dashboard adapter admits it under the existing v2 registry. It never discloses the
    producer and adds no descriptor (amendment under LOD-V0-035).
  - The conformance dashboard verifier admits it.
- **`calibrate` and `eval`.**
  - Both report counts by producer and take a repeatable `--exclude-producer`.
  - Excluding a producer filters only what the read admits and writes nothing.
  - `eval` on an empty store without an exclusion keeps its existing baseline bytes (LTA-V0-001).
  - `eval --learn-slot-weights` refuses the flag.
  - Amended as LTPM-V0-016, with amendment lines under OCL-V0-001 and LTA-V0-001.
- **Compatibility.**
  - record, eval and calibrate are not CCF Core verbs, so no frozen Core shape changes.
  - The CCF spec records the extension and the older-binary refusal.

### Evidence

- **Tests:**
  - `TestLTPMV0015ProducerGoldens` (independently computed bases and IDs)
  - `TestLTPMV0015ProducerRefusals`
  - `TestLTPMV0015LegacyRowsReadAsUnknown`
  - `TestLTPMV0015MigrationKeepsProducer`
  - `TestCalibrateCountsAndExcludesProducers_LTPM016`
  - `TestEvalCountsAndExcludesProducers_LTPM016`
  - the dashboard adapter's mixed-store test
  - conformance `TestProducerTraceConformance`, which includes the built snapshot binary's
    black-box check
- **Live rollback check (scratch repository):**
  - A schema-3 `cli` row was written with the new build.
  - The installed 1.0.0-rc.1 binary refused `calibrate` and `query` with rc 2,
    `unsupported-query-trace-state`.
  - After the revision file was moved aside, the old binary succeeded.
  - After it was moved back, the new build read it again.

### Limits

- The LTPM-V0-015/016 requirement text is proposed until the owner accepts it.
- Producer counts are disclosure only: no gate, threshold or ranking reads them.
- A downgrade after a schema-3 write needs the manual move-aside in the contract's rollback section.
- Full `make gate`: `NOT_RUN` (focused tests per owner preference).
