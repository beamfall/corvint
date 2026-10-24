# 2026-10-24 V1-2030: falsification server identity carries its source

Owner-held decision (docs/build-log/2026-10-24-owner-held-decisions.md, PR #1721; decision 0552)
for V1-2030. Requirement BBF-V0-216 in `docs/specs/browser-behavior-falsification-v0.md`.

## Wire change

Under the opt-in shared-server isolation profile only, each recorded `before`/`after`
`ServerIdentity` gains `source`. The runner never contacts the server (BBF-V0-020), so every
identity is `hook-reported`; `observed` is reserved for an independent probe, which is optional and
not 1.0 work. A report whose isolation rests on any non-observed identity appends the coverage gap
`isolation/hook-reported-identity`, so a forged hook identity can never give a complete verdict
(the test-acceptance worker already requires an empty gap list for `killed`). `verify-receipt`
recomputes the gap and re-derives the source from retained bytes. The isolation limitation now
says the identity is hook-reported, not independently observed.

Plans, reports and receipts without isolation are byte-identical (BBF-V0-023,
`TestBBFV0023LegacyWireIsByteIdentical`). An isolation receipt exported before this change has
identities without `source` and is refused by `verify-receipt`; re-run `execute-receipt`. The
profile is experimental, so no migration is provided.

Rollback: drop `source` and the gap from `isolationIdentity`, `hookReportedIsolation`, `Execute`
and `recountReport`.
