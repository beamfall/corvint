# Proof Tour V0

Owner: Russell Lewis
Date: 2026-09-30
Requirement prefix: `PT-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-approved six V1 wow additions, V1-0504;
`docs/CEM-CI.md`; `docs/specs/cem-pilot-kit.md`; `LICENSING.md`; `AGENTS.md` invariants 1–4, 7–8.

## Agent digest
- Claim: A disposable synthetic change pauses for an independent exact ACK before checksum-pinned portable structural verification.
- Status: proposed/experimental
- Exists: `script/proof-tour.sh`, `script/proof-tour-test.sh` and the retained actual independently acknowledged synthetic tour.
- Blocked on: Third-party interoperability and general usefulness remain NOT_OBSERVED; structural checks do not prove semantic support.
- Read next: Requirements; Failure modes; Acceptance evidence and rollback.

## Intent and scope

The tour makes the boundary between discovering context, citing evidence, running tests,
independent review and structural verification observable in one small disposable repository.
It creates a labelled synthetic policy whose original line 3 requires a 900-second session
lifetime, an initial Go constant of 600, and a test expecting 900. Actual installed Corvint
query/context, an observed failing baseline test, the committed 600-to-900 repair, impact/
affected, passing Go tests, CEM begin and strict status all produce retained command receipts.
The uncited map must fail strict status before the original policy line is cited. A ready CEM
and advisory Corvint review are not the independent reviewer ACK.

Fresh invocation:

```sh
script/proof-tour.sh --out /private/tmp/new-proof-tour \
  --verifier /absolute/path/to/reviewed-cem01-go \
  --verifier-sha256 LOWERCASE_SHA256
```

Fresh runs exit 3 awaiting independent review. The operator gives the exact original fixture,
patch, CEM, source evidence and report to a fresh reviewer. Only that reviewer/authorized
operator supplies the inert seven-line LF-terminated ACK:

```text
proof-tour-ack/0
base=FULL_BASE_COMMIT
head=FULL_HEAD_COMMIT
map_sha256=LOWERCASE_SHA256
patch_sha256=LOWERCASE_SHA256
verdict=ACCEPT
reviewer=INDEPENDENT_REVIEWER_IDENTIFIER
```

Resume with `script/proof-tour.sh --resume /private/tmp/new-proof-tour --ack /absolute/review.ack`.
The script does not generate an ACK, dispatch an agent, read chat histories or authenticate a
reviewer's identity cryptographically. Independence and authorship are operator-established
facts; the file binds the supplied verdict to exact bytes. Missing, rejected or stale ACKs do
not complete the tour. Developer tests use only rejected/stale MOCK_TEST_ONLY ACKs, never an
accepted fabricated review. Resume keeps the original fixture and every prior refusal.

## Requirements

- `PT-V0-001`: The fresh tour MUST create a labelled disposable synthetic Git fixture in a new
  or empty normalized absolute output directory (no trailing slash or dot components), rejecting symlinks in existing path components and
  refusing existing receipts. It MUST preserve unrelated work and never publish or merge.
  The scripts and this spec remain AGPL-3.0-or-later under `LICENSING.md`. Their dependencies
  are developer-only Bash >=3.2, Git, local Go, installed corvint, sha256sum/shasum and ordinary
  POSIX tools; Core gains no runtime dependency.
- `PT-V0-002`: The tour MUST execute actual installed Corvint query/context/impact/affected and
  Go tests on the fixture, retaining each argv, stdout, stderr and exit. The observed baseline
  test MUST fail specifically at 600 versus 900; the repaired test MUST pass. Unsupported
  commands, timeouts or unexpected results MUST remain explicit failures/incompleteness.
- `PT-V0-003`: The producer MUST derive the exact documented canonical diff profile, use
  `cem begin` for `cem/0.1`, observe `max-unknown=0` missing-witness refusal with exit 1, cite
  original policy lines 3:3 as a decision, commit the sidecar, and retain strict ready/report
  and advisory-review receipts. It MUST never relabel readiness or advisory review as semantic
  adequacy or independent reviewer approval.
- `PT-V0-004`: Fresh completion MUST pause with NOT_RUN independent review and exact base,
  head, original map digest and patch digest. Resume MUST revalidate the clean original Git
  fixture (Git status command failure MUST reject, never mean clean, and its exit status and
  original Git diagnostic MUST be retained as that round's status receipt), committed map,
  original patch bytes and re-derived patch before admitting the
  separately supplied seven-line ACK. Absent, rejected, invalid or stale ACKs MUST stay
  incomplete. Every planned resume receipt and auxiliary ACK/tool-identity output MUST be
  preflighted before the first write; existing files or symlinks MUST refuse without overwriting
  their destinations or partially creating the round. Neither fixture regeneration nor old
  receipt replacement may erase a refusal.
- `PT-V0-005`: Only after the exact supplied accepted ACK may resume rerun strict readiness
  and Go tests, then execute the copied digest-checked absolute verifier in local CI mode.
  A copy's checksum MUST be checked before execution and again on resume. Structural
  acceptance MUST be labelled a second Corvint-authored implementation, with third-party
  interoperability NOT_OBSERVED and semantic support a reviewer assessment. No network
  installation or account/service is part of the tour; no network-denied sandbox claim is made.
- `PT-V0-006`: Every owned command MUST run in its own ordinary process group with a bounded
  wall-clock timeout. EXIT/HUP/INT/TERM and timeout MUST retire ordinary descendants, including
  TERM-ignoring descendants and descendants whose leader already exited, preserving outputs.
  A deliberately escaped session/process group is outside this developer-script guarantee.
  Tests MUST verify timeout, leader exit, TERM and INT cleanup rather than infer it from a trap.
- `PT-V0-007`: Focused tests MUST cover syntax/help, populated and symlink output refusal,
  invalid timeout, digest mismatch before execution, lifecycle cleanup, and absent/rejected/
  stale ACK, failed Git status with its retained diagnostic across a retry, output collisions or
  changed original fixture without regeneration. The actual positive tour MUST
  use a fresh independently supplied ACK; fabricated test ACKs cannot qualify semantic review.
- `PT-V0-008`: Before writing any resume receipt (proposed, V1-0519), resume MUST admit every
  planned destination of the round, including the retained ACK copy and tool-identity receipt,
  and refuse with operational `resume-output-collision` when any already exists as a regular
  file, directory or symlink (dangling or not). The ACK and identity writes MUST additionally
  refuse to clobber (`retain-ack-failed`/`retain-identity-failed`). A refusal MUST leave existing
  receipts and symlink targets byte-identical; a collision-free resume MUST proceed unchanged.

## Qualification limits and failure modes

This is a synthetic example, not a real authorization system or a claim that Corvint repaired
an arbitrary application. CEM `cem/0.1` uses explicit out-of-band patch bytes; the script's
independent committed revisions and canonical patch profile bind those bytes, but the map's
own verifier does not establish semantic adequacy. Go tests prove the constant-only selected behavior; token-expiration runtime and timing
boundaries are NOT_PROVEN, and external adoption is NOT_OBSERVED.
The independent reviewer must read the original policy, changed source, test, patch and map.
An operator-written ACK is an assertion of that review, not a digital signature.

A new/empty directory precondition avoids mixed runs. Resume retains numbered new receipts and
never rebuilds an existing fixture. Changed head, map, patch, source or missing verifier forces
incompleteness. Refusals are retained even when a later valid ACK completes the tour. No output
directory or user repository is recursively deleted by the tour. The fixed synthetic fixture
uses disabled Git hooks, no GPG signing and no external diff/text conversion; it is not a general
hostile-repository execution sandbox. A tool that returns an unsupported read does not authorize
silently dropping that step. Bash job-control process groups cover ordinary descendants, not
hostile daemonization/session escape. Tool binary identity and verifier pin stay visible.

## Acceptance evidence and rollback

| Requirements | Witness |
|---|---|
| PT-V0-001,007 | focused script output/refusal tests and syntax check |
| PT-V0-002,003 | actual synthetic tour command receipts, failing baseline, repaired test and missing-witness exit 1 |
| PT-V0-004,007 | original review-request bindings; negative resume tests; independent operator-supplied ACK |
| PT-V0-005 | checksum before execution and resumed structural CI receipt after actual review |
| PT-V0-006 | mock ordinary-process lifecycle tests, explicitly not semantic/reviewer evidence |
| PT-V0-008 | focused symlink and regular-file collision tests preserving bytes, clean-resume admission test; scratch mock-ACK completion is mechanical only |

Root integration records the actual tour directory, independent findings, ACK author and
bindings, final receipts, uncertainty and source review in the common build log. No unit test
is itself an independent reviewer. Scoped checks do not imply a repository-wide gate.

Rollback: remove the two optional developer scripts. Preserve already produced fixture,
receipt and review directories as experimental evidence; their cleanup is an operator decision.
No Core wire, index, store, host configuration or production policy requires migration.
