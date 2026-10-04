# Pool safe reuse successor seed: issue 498

Human-owned intent: GitHub issue498, native V1-0695. On 2026-10-04 the owner approved the
proposal whose text has SHA-256 3b2393e7414a2020224557ecf9dbf61c323b88a6ca964a82c152072bae044783:
preserve the original #498 enrollment as current NON_SUCCESS and qualify a seeded successor. This
entry records that decision once. It is not a successor qualification.

The original enrollment stays as it was recorded: base 211916336638c42e950fa5ec3785cf8eed5a182b,
session key a2284577ab53634942447be30b904d4d348bca03efb61e5608cba3ed43d7aa11, plan digest
fe209579e0f0649cadfe4e52b998eab980bfe45cfb2098a653690950dc9a10cb, intent pointer at
671d1acc2625fb87cca4fa75c7ec727db535f50e, zero observations and a null report set. Its early
daily report kept cem-cite citation-plan-not-provided, cem-status not-ready and local-outcome
record-index-failed, and its untracked CEM (SHA-256 6b09aa9a0f78ff8d8921c73796c4c9dd9eb09a99bcb1725a4891c6d0627610d6)
kept 123 UNKNOWN hunks. Neither selected check ran through that key. Those results are not
relabelled. The reviewed source commit c2471020fc88f6279230a3fbea7d3baaf75ca1fc stays retained.

The original seed 671d1acc is not an ancestor of public main 4b10a02144faed4a77b36eba4b0d1cbc315706d3,
which lacks the PSR intent. A seed present only at HEAD cannot supply citations while the base
stays 211916, so this seed descends from main 4b10 and its checked seal becomes the successor
base. This explicitly changes base and key under owner decision; it is not a continuation of the
original frozen enrollment.

The seed adds the PSR spec, one INDEX entry with delivery not-started and no implementation
paths, one README row, and ten REQUIREMENTS locators. Every other catalog record is byte-preserved.
The PSR `## Requirements` clauses are exactly those of the reviewed c247 contract. The owner
explicitly adopted four requirement amendments relative to 671d1acc for the successor:
PSR-V0-004 (one absolute reconciliation deadline and one 30-second terminal finalization
allowance), PSR-V0-008 (an already-owned exact terminal observation may finalize under an
audited ALL barrier), PSR-V0-009 (canonical optional allocation selector, first-admission
validation, original-request replay and a shared deadline under contention) and PSR-V0-010
(the optional asynchronous dispatcher sweep). Adoption is not a claim that these amendments are
qualified, nor that they are byte-identical to 671d1acc.

At this seed CAL-V0-030 is unchanged, and PSR delegated confirmation has no implemented effect.
No code, journal or CAL contract changes here. Source and runtime qualification of the seed is
NOT_PRODUCED: no source or test execution is claimed for PSR behavior. The seed needs only
focused documentation checks. The successor plan keeps the original single PSR intent and both
original selected checks, psr-focused and psr-vet, with their argv and timeouts unchanged under a
distinct key. The 58 mandatory integrated observations are not waived. Physical and Linux
qualification remain unclaimed.

Rollback removes the PSR spec, its INDEX entry, README row and REQUIREMENTS locators, and this
entry. No stored schema, journal or runtime behavior is involved.
