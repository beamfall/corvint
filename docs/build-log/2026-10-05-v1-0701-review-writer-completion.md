# Issue 504 native review writer completion (V1-0701)

## Intent

Finish the native ERG-V0 writer so that issue 504 can close once promotion completes: REVIEWER and
WORKER recorders, EVIDENCE candidates and evidence references, the issue 394 acceptance-verdict
connection, the full `gates[G]` field set, and the two-writer, crash/redo and descriptor fixtures.
The contract is the fifth-delivery text and traceability in
`docs/specs/corvint-tasks-external-reviews-v0.md`.

Already on main before this change, and not redone here:
- `gate record|resubmit|history`;
- typed `Ticket.Gates` routing with misroute regressions;
- external trust kept distinct from executable gates (ERG-V0-006/007);
- the ERG-V0-011 `complete-manual` offer (V1-0792).

## What landed

- **Actors.** REVIEWER records only LEASE_BOUND on its own holder-matched review-stage lease and
  never resubmits. WORKER never records, and resubmits only on its own author-stage lease. Neither
  role is admitted to any other mutation.
- **Artifact link.** `transaction.ExternalArtifactPosts` derives links from receipts. A link is an
  evidence entry of an executable GateResult for the subject attempt, generation and candidate tree
  whose process exited (EXIT) in a clean worktree at that tree. It must be posted fresh in the same
  receipt as the attempt record that lists it. The writer
  (`externalArtifactStates`) and the receipt fold (`ExternalReviewReceiptAudit`) both require a link
  for an EVIDENCE candidate and for every evidence reference.
- **Issue 394 connection.** When an EVIDENCE candidate artifact parses as a
  `corvint-new-e2e-assessment/0|1` report, the verdicts map as follows:
  - accepted admits only PASS;
  - rejected admits only RETURN;
  - blocked admits no verdict (MISSING_EVIDENCE);
  - any other value is MALFORMED.

  The rule is enforced at write time and at replay. `--from-acceptance REPORT_PATH` takes the verdict
  from the report. Tasks restates the two schema names rather than importing the browser-executing
  producer.
- **CLI.**
  - `gate record --candidate-evidence SHA:BYTES`, `--from-acceptance REPORT_PATH` and repeatable
    `--evidence LABEL=SHA:BYTES`.
  - `gate state TICKET` exposes the full `gates[G]` field set. The dispatcher `GateView` also gains
    the candidate, subject, trust and evidenceSha256 fields.

## Fail-closed choices (owner questions)

- A REVIEWER cannot resubmit.
- Subject currency follows submission history only. A re-claim or a review-stage submission does
  not stale a verdict.
- A link does not require a PASSED GateResult, only a clean exit at the candidate tree. A rejected
  acceptance report is produced by a failing gate and must still support RETURN.
- The acceptance mapping is detected from content and has no policy switch.
- The read bound for EVIDENCE artifacts is the 16 MiB gate-output bound.
- The writer admits links only from receipts after the subject. The fold accepts any linked receipt,
  so the writer is the stricter of the two.

## Evidence

- Code and fold mutations were each verified to fail their regression:
  - writer link check removed: `TestERGV0002_EvidenceCandidatesNeedAnArtifactLink` fails;
  - writer acceptance check removed: the same test fails;
  - fold evidence, acceptance or subject-link check removed:
    `TestERGV0002_ForgedArtifactEventsRefuseAtRecovery` fails in the matching case.
- `TestERGV0005_TwoWritersOneWinner` passes with `-race -count=3`.
- `TestERGV0009_ReviewDescriptorMeasured` measures a maximal review record descriptor at 5 of 6
  artifacts and 1342 of 1670 bytes. A blob-backed ticket post adds one EVIDENCE artifact and stays
  under the generic 1669-byte maximum (`TestONV0006`).
- Focused suite: `go test ./internal/tasks/... ./cmd/corvint-tasks/...`; gofmt and `go vet
  ./internal/tasks/...`.

## Review

- Codex round 1 (gpt-6-astra, read-only, `d524530f..93ca49f1`):
  - P2: links ignored the executed tree and worktree cleanliness, so a gate that moved its tree or
    dirtied its worktree mid-run could link an accepted report as a PASS on the wrong candidate.
    Fixed in `externalProducedAtCandidate`. Regression: the dirty and moved gates in
    `TestERGV0002_EvidenceCandidatesNeedAnArtifactLink`, each verified to fail without its check.
  - P3: the docs gave `--from-acceptance SHA:BYTES`, but the flag takes a report path. Docs fixed.
- Codex round 2 confirmed both fixes and found one more issue:
  - P2: the settled-forgery check rejected only an observed RETURN. A forged PASS exposed by the
    dispatcher, an observation error or a missing ticket would have passed. Both forgery tests now
    require a successful observation that holds the ticket with its gates unobserved. Verified by
    forcing the dispatcher to ignore a refused fold: all three settled cases fail.

## NOT_RUN

- `make gate` and the full repository suite. Lane rule: `corvint affected` advises 171 packages.
- Live qualification of `corvint tests accept` feeding a real queue. The mapping is tested with
  fixture reports only.
- Promotion: independent review beyond the Codex rounds, current-main integration, and native
  completion of V1-0701. The orchestrator owns Tasks state.
