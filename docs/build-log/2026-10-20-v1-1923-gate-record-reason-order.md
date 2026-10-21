# V1-1923: `gate record --reason` composes a sorted set instead of refusing input order

Date: 2026-10-21. Ticket: V1-1923 (residual from GitHub #1189). Requirement: `ERG-V0-023` (new).
Base: origin/main 90ab9f33e.

## Finding

The external review request's `reasons` and `evidence` arrays are non-semantic sets, so the locked
writer's decode (`wire.Reader.Array(16, false)`) requires canonical-byte order without duplicates.
The CLI composed them in flag order, so `gate record --reason TESTS:x --reason DOCS:b` refused
MALFORMED and callers had to pre-sort by element bytes, which differs from flag-string order
(`TESTS:x` < `TEST:x` as strings, but `{"code":"TEST",...}` < `{"code":"TESTS",...}`).

## Decision

Canonicalize at the CLI (the ticket's first option): sort `--reason` and `--evidence` flags by the
canonical bytes of the element each composes and drop exact repeats, before both the retained-request
match and composition, so a reordered retry replays. The writer-side decode is unchanged and still
refuses an unsorted array from any other producer. Rollback: revert `canonicalReviewList` and its
two call lines in `internal/tasks/cli/external_review.go`; no wire or state format changes.

## Evidence

`TestERGV0023_ReasonsCanonicalizedNotRefused` (native store, `gate record` + `gate history` + replay)
and `TestERGV0023_EvidenceFlagsComposeSortedSet`. Without the fix the first fails with MALFORMED on
the out-of-order flags.
