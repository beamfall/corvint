# Playwright run evidence is bounded before decode (V1-1504, V1-1503)

Date: 2026-10-16. Tickets: V1-1504, V1-1503. Requirements: `AFU-V1-037` and `AFU-V1-036` in
`docs/specs/application-flow-understanding-v1.md`, unchanged; no requirement was added or amended.

## Problem

`AFU-V1-037` says byte, record, attempt, link and traversal bounds apply before decode and walk.
For Playwright JSON (`parsePlaywrightRun`) only the byte and traversal bounds did: the whole
document went through `json.Unmarshal` and the record, attempt and link bounds were counted
afterwards. The `playwright-attempts/3` receipt was decoded twice (identity check, then
`parsePlaywrightReceipt`) before any count, and had no traversal bound at all. A count-over-bound
document that the decode refused was reported as an invalid document, not as incomplete.

`AFU-V1-036` had matrix tests but no Traceability implementation row.

## Contract

- Both Playwright adapters run one streaming `json.Decoder` token pass (`runprebound.go`) after the
  byte bound and before any decode. It refuses nesting deeper than `maxRunDepth` as it descends and
  counts tests, attempts and links, returning the existing `run-evidence-*-bound` code with zero
  records. For the receipt the pass runs before the identity decode.
- The pass counts what the decode keeps, so in-bound reports decode and encode exactly as before:
  member names match case-insensitively and a repeated member replaces the earlier one (as
  `encoding/json` decodes them); a Playwright result's links are its attachments plus its located
  `errors` entries, or its located `error` when `errors` is empty; a receipt detail's links are its
  `anchor` and `artifacts`. Not mirrored: `encoding/json` merging a repeated object member into the
  earlier value; such a document is counted on its last occurrence.
- When several bounds are exceeded the pass names the one the decode hits first: suites in
  `collectSpecs` order (a suite's specs, then its nested suites); Playwright JSON appends a test before
  keeping its attempts, the receipt keeps a test's attempts before appending it.
- The post-decode checks stay as they were; the pass only moves the refusal ahead of the decode.
- `AFU-V1-036` gains a Traceability row naming `input.go`, `docs.go` and `intent.go` and the callers.
  The `AFU-V1-037` row lands with PR #981 (open at this change); moving it from partial to
  implemented is left to the change after #981 merges.

## Evidence

- `TestAFUV1RunEvidenceBoundsIncomplete` gains 15 cases and 4 in-bound checks. On BASE `ff96980fd`
  (load averages about 47) 7 of the new cases fail: three Playwright JSON count cases whose report the
  decode refuses (`invalid Playwright JSON report` instead of the bound) and the four receipt cases
  (`external-attempt-runner-identity-mismatch` instead of record, attempt, link and traversal). The
  Playwright byte-bound and receipt byte-bound cases, the three JUnit count cases and the `go test`
  traversal case pass on BASE: those bounds already applied before decode or during the walk.
- Mutations, each failing exactly one assertion: dropping the receipt append-order rule; counting
  unlocated errors as links; exact instead of case-folded member names; counting each spec's tests
  twice (caught by an at-bound report of 8192 distinct tests).
