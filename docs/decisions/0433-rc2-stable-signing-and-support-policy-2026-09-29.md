# Decision 0433: rc.2 and stable signing selection and support window

Date: 2026-09-29. Status: accepted (owner accepted both prepared policies in the delivery chat
and the matching combined question in the integration coordinator chat).
Tickets: V1-0020, V1-0021.

Numbering: prepared on 2026-09-29 as 0428 and left unpublished while `main` accepted a different
decision 0428 on 2026-10-04; published as 0433 on 2026-10-05 with its content unchanged.

## Decision

1. For **Corvint 1.0.0-rc.2** and **Corvint 1.0.0**, the owner selects **No signing** under
   `release-artifact-integrity-v0.md`. Each release carries its exact verified `SHA256SUMS`,
   retained reproducible-build and candidate qualification evidence, and **publisher identity
   NOT_VERIFIED** in its release notes and publication receipt. GitHub publisher metadata is
   attribution only, not a signed artifact or provenance claim. No agent may create or use a
   signing identity under this decision. This selection does not automatically cover later
   versions; decision 0420 remains the historical rc.1 selection.
2. From 1.0.0, security fixes cover the latest published, non-withdrawn stable Corvint release
   artifact set until its next stable successor is published. Prereleases do not end the current
   stable release's support. Withdrawn releases are unsupported immediately. Release candidates
   are evaluation builds with no stable support guarantee. The existing private reporting channel
   and seven-day acknowledgment target remain; no fix-time SLA or older-version backport promise
   is added. Exact independently versioned component support is listed with the release artifacts.
   This does not promote experimental or FALLBACK surfaces to stable Core.
3. Before each tag, the owner reviews the frozen candidate toolchain against current official Go
   security releases. A newer patch with a standard-library security fix requires the pin to move
   and the candidate to be rebuilt and requalified, as decision 0420 requires. Its require-free,
   pinned-toolchain check remains Core-only, not a vulnerability scan or companion security
   certification.

## Evidence and authority boundary

These are policy selections, not completed release actions. Under `SRR-V1-009`, record
`policy/signing` as `NOT_RUN`, decision `0433`, with the reason
`No signing: SHA256SUMS only; publisher identity NOT_VERIFIED`. Do not attach an evidence-file
digest to that NOT_RUN row or report signing PASS. Retain the exact artifact identities and
actual checksum/reproducibility results separately in the candidate packet. The readiness
record's owner toolchain/tag/publication/promotion rows remain fixed NOT_RUN; actual owner
reviews and subsequent actions are retained separately.

Exact candidate gates, independent review, rollback, two unchanged-candidate qualification runs
and explicit owner approval of RC publication and stable publication/promotion remain required.
This decision authorizes no tag, upload, publication, promotion or production authority switch,
creates no candidate, and completes no ticket. The urfave preregistration and prior failed or
aborted qualification evidence are unchanged.

## Rollback

A later owner decision may replace these prospective policies. Preserve this accepted record and
all historical release evidence; do not move published tags or rewrite assets. A withdrawn release
is unsupported immediately. A different signing mechanism requires its own accepted identity and
qualified implementation before use.
