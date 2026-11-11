# Tasks: run-evidence

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/run-evidence-v1.md` with RVD-V1-001 to RVD-V1-012 from the spec delta as
   proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`.
   Requirements: RVD-V1-001 to RVD-V1-012. Proof: the spec toolchain check passes.
   Done: V1-5448 filed; RVD-V1 reserved; `docs/specs/run-evidence-v1.md` created and accepted under G14
   (acceptance-log row `run-evidence`).
- [x] 2 Add the store package: hash-named records, locked append to `log.jsonl`, reader that
   normalises receipts and ingested documents to per-test rows. Requirements: RVD-V1-001,
   RVD-V1-002, RVD-V1-003, RVD-V1-006. Proof: unit tests for naming, concurrent append and
   normalisation of both profiles.
- [x] 3 Add retention, the size cap and corruption handling.
   Requirements: RVD-V1-009, RVD-V1-010. Proof: unit tests at 21 runs, at the cap, with a bad record
   and an unreadable log.
- [ ] 4 Append from `corvint test` and `corvint watch`; print `run-evidence-write-failed` on
   failure. Requirements: RVD-V1-004, RVD-V1-010. Proof: CLI test that a run adds exactly one
   record, and a test with a read-only store directory.
   Open: the `corvint test` half and its proof are delivered (V1-5448). `corvint watch` runs
   `go test` itself and writes no receipt on this tree, so it has no run document to append; its
   append is not yet built, and this task stays unticked until it is.
- [ ] 5 Add `flows ingest --record`.
   Requirements: RVD-V1-005. Proof: CLI test that ingest with `--record` adds one record and without
   it prints as before.
- [ ] 6 Make `flows status`, map, gaps, impact, `corvint test-evidence` and the MCP tools read the
   store by default, with `--no-store`. Requirements: RVD-V1-007, RVD-V1-008, RVD-V1-012. Proof: the
   PROD-TEST-020 end-to-end test; a read-only test showing the store unchanged after each reader.
- [ ] 7 Check that no selection path reads the store.
   Requirements: RVD-V1-011. Proof: a test that a failing stored result does not change the plan.
- [ ] 8 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every RVD-V1 ID to a test.
