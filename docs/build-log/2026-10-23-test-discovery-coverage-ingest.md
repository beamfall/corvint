# Static test discovery and per-test coverage ingest

Date: 2026-10-23. Lane: claude/test-evidence-gh1582-1587 (base 0b7d59ba6). Issues: GitHub #1582
and #1587. Requirements `DCP-V1-370..376` (`docs/specs/documentation-corpus-v1.md`) and
`AFU-V1-112..119` (`docs/specs/application-flow-understanding-v1.md`) are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

An agent that cites test evidence had two gaps. It could join a test's run receipt to its source
example (`tests example-join`), but could not read from a committed test file what that example
asserts, visits, reaches and inherits from hooks. It could also not use per-test coverage, which
many suites already collect, as observed `tested_by` evidence, or see where a declared link is
contradicted by what the test executed.

## Decisions

- `corvint docs corpus discover-tests` lives in `internal/testsource` beside the TSA-V0 readers it
  reuses: the RSpec structural reader, scoped example IDs, setup-select reasons and the JS title
  reader. Every fact is anchored through a new exported `doccorpus.LineIndex`. It applies the
  compiler's own blob digest, span digest and `maxSpanBytes` excerpt rule, so the anchors validate
  exactly as a corpus provider's would. The identity field is the key `tests example-join` gives the
  same example. This makes discovery and run receipts join without a second identity scheme.
- The coverage ingest is a standalone `corvint-coverage-ingest/1` report from `flows
  ingest-coverage`, not a corpus provider record. Coverage relations need source and test subjects,
  and the corpus gap path exists only for the adoption schema, so a provider record would have
  needed a schema change outside this slice's ranges. The report reuses `doccorpus.Evidence`
  (derivation `observed`, trust `generated`) so a later provider can carry it unchanged.
- A covered test joins only the one `test-run-evidence/0` record whose test key equals it, and only
  when that record ran clean at the requested commit and tree. Anything else is reported with its
  reason and not joined. Coverage from another revision therefore cannot produce a relation.
- V8 byte offsets are applied to the committed blob only when the entry's source text equals it,
  or the file is plain JavaScript without a source. Without source-map resolution, any other mapping
  would be a guess, so a transpiled or bundled script is an unknown. V1-1182 (Playwright V8 through
  pinned source maps, `AFU-V1-021`) therefore stays open.
- A link is `conflicted` only when the subject file was instrumented for that test and none of its
  lines ran. A subject the coverage did not instrument is `unknown`, because absence of
  instrumentation is not evidence that the test missed it.

## Independent review fixes (PR #1595)

One Codex round found 1 P1, 6 P2 and 1 P3; each fix has a test.

- P1: a clean record at the right commit did not prove that the coverage file came from that run.
  `test-run-evidence/0` now carries optional `coverage_artifacts` (SHA-256 pins, set by `flows
  ingest --coverage-artifact ID=FILE`), and a record that does not pin the coverage input stays
  unjoined with `coverage-not-bound-to-run` (AFU-V1-117, AFU-V1-120). The member is omitted when
  empty, so existing records keep their canonical bytes. A record produced without the flag can no
  longer back a coverage relation.
- P2: a multi-test LCOV trace (a second distinct `TN:`) is refused instead of merged under one key
  (AFU-V1-114).
- P2: identical V8 entries are applied once, and the scanning and painting work across all scripts
  shares one aggregate bound (AFU-V1-113).
- P2: an inline RSpec inclusion counts its shared body's runtime slots through nested inclusions.
  A loop, or an unresolved or cyclic inclusion, makes later scoped IDs unknown (DCP-V1-371).
- P2: covered files or link subjects with no anchorable lines are the unknown
  `coverage-unanchorable`, with no relation, corroboration or conflict (AFU-V1-116).
- P2: discovery facts whose anchors fail are dropped with `fact-span-unanchored`, never emitted
  with zero-valued anchors (DCP-V1-370, DCP-V1-376).
- P2: a multi-line RSpec block expectation reads its matcher from the line that closes its own
  block, counting nested blocks, or abstains with `matcher-not-read` (DCP-V1-372).
- P3: an input over 5 MiB is `coverage-bound-exceeded`, not `coverage-refused` (AFU-V1-118).

## Evidence

Focused tests: `TestDCPV1370..1376` (internal/testsource), `TestDCPV1370DocsCorpusDiscoverTestsCLI`,
`TestAFUV1112..1118` and `TestAFUV1120RunEvidenceCoverageArtifacts` (internal/appflows) and `TestAFUV1112FlowsIngestCoverageCLI`. They cover
byte-identical reruns, no repository mutation, anchor validity, V8 nesting and refusals, Istanbul,
LCOV and RSpec parsing, corroborated and conflicted links, stale, dirty and skipped runs, and the
bounds.

## Residual limits

- No source-map resolution; V1-1182 is not satisfied.
- The coverage report is not compiled into a corpus artifact. Joining it into `docs corpus build`
  needs a provider schema change.
- Coverage is line-granular; each input is bounded at 5 MiB.
- Coverage joins only through a run record that pins the coverage file's SHA-256. The producer
  must pass `--coverage-artifact` to `flows ingest` when it writes the record.
- A Playwright example's identity needs the project from a run receipt before it forms the
  `js-title-path` key. RSpec example IDs must equal run-record test keys exactly.
