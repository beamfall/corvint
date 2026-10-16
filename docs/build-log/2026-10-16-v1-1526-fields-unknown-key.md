# V1-1526: unknown `--fields` key diagnostic

Date: 2026-10-16. Base: `38d4fef8a9be69b993e04c9dce55e803508b04de` (origin/main).
Ticket: V1-1526, from GitHub issue 996. Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`,
CAL-V0-310, accepted under the owner's standing auto-accept of lane requirements (2026-10-11).
No accepted requirement is amended: CAL-V0-166 already requires the `MALFORMED` refusal, and
CAL-V0-310 only fixes what its warning says.

## What the issue reported and what already existed

The issue showed `{"codes":["MALFORMED"],"outcome":"ERROR",...}` for `ticket show T --fields body`
and called the refusal detail-free. At base the envelope already carried one warning,
`MALFORMED: fields: field "body" is not in this read's items`. The excerpt in the issue left it
out. The gap was the way forward: the warning listed no valid keys, gave no hint that `body`
lives under `record.`, and the `ticket show` usage said only `--fields KEY[.SUB],...`.

## Decisions

- Wire slot. The detail goes in the existing `warnings` member of `taskman-command-result/0`.
  No envelope member, result code or profile changes. The code, outcome, empty items and
  read snapshot are as before (CAL-V0-166).
- No aliases. The issue offered top-level aliases (`body` for `record.body`). They were declined:
  a projection that answers a key the item does not carry would make `--fields` inexact and give
  one value two names. The refusal suggests the path instead.
- One place. Every `--fields` read (`ticket list`, `ticket search`, `ticket show`, `queue status`,
  `roadmap`, `attempt show`, `plan preview`) goes through the shared `checkCompactFields` in
  `internal/tasks/cli/compact.go`. The diagnostic is added there, so every command gets the same
  text.
- Suggestion rule. The suggestion is offered for a one-segment key only, because `PARENT.KEY.SUB`
  would exceed the one-level projection. `PARENT` is any top-level key whose object value, or
  any of whose array elements, carries `KEY`. This also covers
  `queue status --fields holderStatus` pointing to `liveAttempts.holderStatus`. An absent sub-key
  (`record.nope`) lists that key's sub-keys.
- Help. Only the `ticket show` usage names the `record.` prefix. List items omit `record`
  (CAL-V0-173), so the prefix does not apply to `ticket list` or `ticket search`. The usage line
  changes in both `commandUsage` and the top-level usage list. `helpFlags` derives the same flags
  from both.

## Evidence

- `TestCALV0310_FieldsUnknownKeyDiagnostic` (`internal/tasks/cli`) checks the following:
  - `ticket show --fields body` returns the item's sorted keys and `did you mean "record.body"?`.
  - A key absent at every level gets no suggestion.
  - `record.nope` lists the `record` sub-keys.
  - `record.body,record.acceptanceCriteria` still projects.
  - `queue status --fields headseq` lists the queue status keys.
  - `holderStatus` suggests `liveAttempts.holderStatus`.
  - `ticket show --help` contains `record.body`.
- `TestCALV0310_UnknownFieldMessageAcrossItems` (package-internal) pins the exact text over two
  heterogeneous items: the sorted key union, two sorted suggestions (one through an array), the
  sorted sub-key union, no suggestion for a two-segment key and the `retries` hint.
- Base reproduction: with the base `compact.go` the test fails at its first assertion, on the
  bare warning above.
- Mutation: renaming the suggestion text fails the test. Both runs restored the fixed file.
- Projection and refusal regressions: `TestCALV0165`, `TestCALV0166` and the rest of the
  `internal/tasks/cli` package (see the PR for counts and gates).

## Independent review

One Codex round (read-only, `codex exec -s read-only`) over `38d4fef8..e47c21a3` found two Low
items and no correctness, determinism or wire-contract defect:

- The `retries` refusal returned early without the key list that CAL-V0-310 requires. Fixed: it now
  uses the common diagnostic and appends the `--retries` hint.
- The test covered one item and one suggestion only. Fixed by the package-internal
  heterogeneous-items test above.

Residual: the keys listed are those of the returned items, so an empty page still accepts any
well-formed name (unchanged CAL-V0-166 failure mode).
