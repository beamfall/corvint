# 2026-10-08: Proposed obligation ledger spec (V1-1022, GitHub #680)

## Intent

The owner asked (GitHub #680) for a native per-ticket obligation ledger with step-level Playwright
witnessing, replacing the external `fp-ledger.py` script, so that progress and no-progress detection
count proven obligations instead of sessions. This change adds only the proposed executable spec
[`corvint-tasks-obligation-ledger-v0.md`](../specs/corvint-tasks-obligation-ledger-v0.md)
(TOL-V0-001..020), registered in the spec README and INDEX. Intent is `proposed` and delivery
`not-started`; acceptance is human-owned, and no code, store member or command was written.

## Decisions (proposed, not accepted)

- Storage by reference: the record carries `obligations {prefix, revision, head, counts, highWater}`;
  the ledger is an immutable canonical evidence-store snapshot chained by `previous`, as ON-V0-002
  does for notes. Inline storage would exceed the 128 KiB record bound.
- Writes are revision-only (TEA-V0-001 precedent): `acceptanceRevision` and gate results stay bound.
- Crediting uses only retry-0 results. A step credits from its own `error` member, so independent
  `expect.soft` steps credit independently. A title-path id credits only for a passed retry-0
  result. Every match of an id must pass.
- Commit binding uses a source presence check at the declared commit, because the default json
  report has no commit. Reports are never stored, because they carry stdout and stderr. Only a
  re-derivable, secret-screened extract is stored.
- The CAL-V0-057 fingerprint changes only for tickets that have a ledger. Those tickets swap
  revision for acceptance revision plus a monotone witnessed high-water mark. CAL-V0-185 stall
  restarts on a raise. CAL-V0-102 gains an optional `witnessedHighWater`. A queue status key is
  added only when a ledger exists.
- Closed result codes are reused with stable detail prefixes, so no new code is added.

## Evidence

- Pre-design observation (OBSERVED, non-qualifying): a scratch run under
  `/private/tmp/claude-501/v1022-td/pw` with Playwright 1.61.1 (not the PWP-V0-008 qualified
  1.63), Node v22 and the built-in json reporter with `retries: 1`. It showed the following:
  - `test.step` entries carry their own `error`.
  - A passing sibling of a soft-failed step has no error.
  - A parent of a soft-failed nested step carries an error.
  - A hard failure omits later steps.
  - A soft failure outside steps appears only in `result.errors`.
  - A retry-1 pass yields test status `flaky`.
  - `config.metadata` holds only `actualWorkers`.
  - `spec.file` is relative to `config.rootDir`.
- Doc gates passed: spec-requirements, requirement-definitions, traceability-tests (29 planned
  tests), decision-numbers, line-citations, error-code-ownership, unbounded-readers,
  use-case-receipts and diagnostic-coverage. `go test ./internal/specindex` passed.
- Failing-before/passing-after: not applicable. This is a document-only proposal and changes no
  behaviour.

## Non-goals, failure modes and rollback

The non-goals and failure modes are listed in the spec:
- Non-goals include completion gating, running tests, qualified-receipt crediting and the `splits`
  heuristic.
- Failure modes include forged titles, duplicate ids, a report from a different commit, flaky
  retries and a missing report.

Rollback of this change is reverting the spec, its README row, its INDEX entry and the regenerated
REQUIREMENTS.tsv rows.

## NOT_RUN

Every TOL-V0 test, the live Playwright 1.63 fixture, owner acceptance, `go test ./...` and
`make gate` are `NOT_RUN`. No code was touched, so no Go test beyond specindex applies.
