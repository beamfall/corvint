# V1-0977: adopter product name removed from public tracked paths

Date: 2026-10-07. Ticket V1-0977 (found during issue 665).

## Decision

A private adopter's product name appeared in tracked paths (one test file name, Go test identifiers
and fixture strings, a JSON wire-member tag, specs and `docs/BUILD-LOG.md`). It was replaced with the
neutral synthetic name `example-app` / `ExampleApp`. The JSON wire-member tag of the frozen `/1`
behavior provider (`internal/doccorpus/behavior.go` `BehaviorRevisions.E2E`, plus its mentions in
`docs/specs/application-flow-understanding-v1.md`, `docs/specs/documentation-corpus-v1.md` and
`docs/BUILD-LOG.md`) was NOT renamed: AFU-V1-006 pins the `/1` provider bytes
(`TestAFUV1BehaviorProviderV1BytesUnchanged`), so a rename changes the digest and breaks existing
producers. Those four paths still contain the name pending an owner decision (accept a `/1` wire break
with a repinned digest, or add a neutral alias on decode while keeping the legacy encoding).
One NATO-alphabet fixture word that matched the same
text (`conformance/frontier-v0/runner_impl.go`) was changed to `gamma`; it only labels generated hunks.

## Redaction exception

`docs/BUILD-LOG.md` is closed (decision 0423). The coordinator authorized a name-only replacement there,
with no other edit. This is a one-time redaction exception, not a reopening of the log.

## Limits

Git history still contains the name; no history rewrite or force-push was done. Behavior and test
coverage are unchanged (test file renamed with `git mv`; test and function identifiers renamed).
