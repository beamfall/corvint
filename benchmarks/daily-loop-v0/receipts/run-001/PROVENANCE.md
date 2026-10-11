# Run-001 provenance (V1-0163)

This retrospective reference is not part of the sealed run or a new attestation. The three
receipts retain candidate `1894b9e5c992d69a7cbefa4b305485494e5622c4` and their original results.

The governing scope is [decision 0332](../../../../docs/decisions/0332-verified-local-workflow-scope-2026-09-22.md)
(verified local-workflow scope for 0.6), with the verb/job contract in
[Daily change-evidence workflow V0](../../../../docs/specs/daily-change-evidence-workflow-v0.md),
particularly DCW-V0-001 through DCW-V0-009. Decision 0332 covers twelve Core surfaces:
init, adopt, index, query, context, impact, affected, prove, CEM, OCM, frontier, and dogfood's
retained local outcome. It accepts scope, not qualification of every verb by this run.

| Job / receipt | Measured treatment | Recorded outcome |
|---|---|---|
| UC-TASK-ORIENTATION | `context --task` | FAIL |
| UC-CHANGE-CONSEQUENCE | `affected --base`; `impact --base` recorded but unscored | FAIL |
| UC-EVIDENCE-CARRYING-COMPLETION | `cem status` and the dogfood change/check loop with missing/stale-evidence controls | PASS |

The [run result](../../runs/run-001.json) pins the candidate and the original preregistration
seal `73f178f8b1bcaaa57115c2ee9389f3a57fa3d889ee6e8a50035edeec3ae04b9d`.
The current preregistration includes a later amendment: it explicitly preserves run-001 as scored
under superseded digests. This sidecar does not substitute current preregistration bytes for that
historical seal, rerun the benchmark, or promote a failed job. Token/cost savings and broader
Core qualification are not established here.

## Why the receipts are unchanged

`harness.py`'s `write_receipts` emits SHA-256 identities for the exact receipt bytes. The
`corvint-use-case-evidence/0` consumer in `conformance/use-cases-v0/main.go` checks a ledger's
receipt digest, admits a closed receipt field set, and requires each sealed-benchmark attestation
digest to bind a subject. Adding a provenance field would violate that closed format; adding a
subject would change receipt bytes and their identity. The current ledger points to run-002,
not run-001; no current ledger pin for run-001 was found. Preserving historical bytes avoids
rewriting emitted identities or implying a newly sealed run. Neither the harness, corpus,
preregistration, results nor receipt JSON files are edited.

Original receipt SHA-256 values (exact bytes):

| Receipt | SHA-256 |
|---|---|
| [UC-CHANGE-CONSEQUENCE.json](UC-CHANGE-CONSEQUENCE.json) | `66513302c06063447419232d7bff8053bba87534b66c26301d4b2babd230a06f` |
| [UC-EVIDENCE-CARRYING-COMPLETION.json](UC-EVIDENCE-CARRYING-COMPLETION.json) | `64c81ecc5f258420df3d2330d723b07eac0a836f13c27ea253231f8ac8fa136f` |
| [UC-TASK-ORIENTATION.json](UC-TASK-ORIENTATION.json) | `4bd2a48c3a9d421e0f9e40d9ba47d5de9fe198ee84e9c2e7de25551e12325f98` |
