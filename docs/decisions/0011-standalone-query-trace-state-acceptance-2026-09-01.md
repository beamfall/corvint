# Decision 0011 — accept standalone query trace-state profile

Date: 2026-09-01. Status: accepted. Authority: repository owner, explicit decision this session:
"GPK-V0-044 is ACCEPTED — design (a) trace-consuming, repository-intent-only scope, conformance
plan as written in the proposal at docs/specs/go-production-kernel-migration-v0.md (commit
3103027). Nothing else is newly accepted."

## Scope

The instruction accepts exactly `GPK-V0-044`, the standalone `corvint query` `repository`-intent
trace-state profile proposed at commit `3103027`. The accepted design consumes the bounded local
trace store through the existing native Go read contracts and retains the proposal's conformance
plan unchanged except for its acceptance status.

## Explicit exclusions

This decision does not widen `GPK-V0-028` or the `project-operations` authority-start profile,
change its `unsupported-query-trace-state` refusal, change the existing
`query-present-trace-store-refusal` conformance case, accept any other proposal or open decision,
or authorize promotion, cutover, merging, tagging, signing, publication, or release action.

## Consequences

The repository-intent implementation and its named conformance matrix are authorized. Delivery
status and compatibility remain experimental until the accepted evidence and existing promotion
gates are satisfied.
