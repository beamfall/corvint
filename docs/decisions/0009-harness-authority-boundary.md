# Harness authority boundary

Status: accepted — option 2 (2026-09-06)
Date: 2026-08-31
Owner: Russell Lewis

## Context

Under Corvint's current local-only, single-UID boundary, no acceptable authority root exists for
`harness-execution-attested-v0`. A same-UID caller can fabricate command, target, report, result,
and a local process output. Existing live-test and harness implementations are execution surfaces,
not authority roots; therefore they cannot close Change Frontier obligations.

## Accepted decision

On 2026-09-06 the repository owner selected "Local workflow gate (recommended)" in the Codex task
repairing Corvint self-dogfood. This accepts option 2 below: retain the local-only boundary and add an
explicit, bounded, non-authority completion policy. The selected policy requires current evidence,
reviewed reports and passing selected verification without a tamper-proof attestation claim.
`docs/specs/local-completion-policy-v0.md` specifies the experimental implementation slice.

Option 1 was not selected. The original alternatives are retained for provenance:

1. Change the trust boundary to introduce an owner-accepted authority root that a same-UID caller
   cannot impersonate, with an explicit identity, key/capability lifecycle, portability, failure,
   and rollback contract.
2. Retain the local-only boundary and keep execution observations caller-reported; move any future
   completion treatment to an explicitly non-authority policy decision that cannot mint
   `HARNESS_ATTESTED` or close the Frontier by itself.

## Consequences

`harness-execution-attested-v0` still has no accepted root set and must close nothing. The accepted
option authorizes only a separate, explicitly enrolled local workflow policy, including bounded
native Stop remediation under the new policy contract. It does not authorize a daemon, network
service, signature scheme, execution attestation, automatic commits or any redefinition of the
legacy harness/Frontier profiles. A local policy pass must never be promoted into Frontier closure.

## Evidence

- `docs/specs/harness-authority-relation-v0.md` documents the empty root set.
- `docs/specs/change-frontier-v0.md` keeps caller-reported TCQ non-closing.
- `docs/specs/test-claim-qualification-v0.md` excludes same-UID execution attestation.
