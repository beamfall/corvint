# Issue 378: explicit owner recovery of exhausted tickets

Owner intent: [issue 378](https://github.com/beamfall/corvint/issues/378), native V1-0496,
and the existing CAL-V0-013 promise of OWNER reopen. CAL-V0-043 specifies the recovery boundary;
CAL-V0-042 is reserved in the owner's existing private lineage and is intentionally not reused.
Base: `6dc8ed1bceaa563c4e2cddb505b5891741bbce5a` (`origin/main`).

The real-store regression confirmed the mismatch: four cancelled generations exhausted the
retry budget, while `ticket reopen` returned `BLOCKED/TICKET_STATE` because the ticket was OPEN.
Cancellation accounting remains unchanged. An explicit OWNER reopen now advances acceptance
revision after complete journal-backed attempt validation, leaving old attempts and gate history
intact. The next claim starts fresh and still requires ordinary admission, approval and gates.
Local role bindings remain operator claims, not authentication.

Independent Gate A passed the recovery design with required ticket/revision-bound facts,
full attempt/reservation validation and ambiguity refusal. During reason-retention verification,
we found that generic mutation journaling retained only a request digest, not its reason bytes.
The initial plan assumption was wrong. A limited independent addendum required and accepted
one bounded canonical request-envelope evidence POST, pinned in the successful recovery receipt,
plus stage descriptor bounds and interrupted-publication tests. Evidence-first publication may
leave an orphan before receipt commit; that blob alone confers no recovery.

Self-use: clean governance query, tracked-path impact, affected planning and the keyed dogfood
workflow were used. Query ranking returned governance rather than behavioral proof; omitted
ranked/test candidates remain unknown. Initial pre-change dogfood coordination was incomplete
(no change/intent/outcome yet), and its original failure is retained. The private measurement
receipt was started after the initial query, so pre-query measurement chronology is not claimed.
Billed tokens, paired baseline savings and native host adoption remain `NOT_OBSERVED`.
Provider, mutation, retrieval-learning and hosted/runtime qualification routes were not applicable:
this slice changes local owner recovery and does not launch any agent or external provider.

Verification: focused real-store before/after, mutation and transaction safety, actual CLI
reopen/preview/audit, exact retained request and replay conflict, old approval invalidation,
required failed/fresh gates, schema-valid attempt tampering, and pre/post receipt fault recovery.
The initial complete independent source review found two defects: blank recovery reasons were
accepted, and a shared maximal-descriptor fixture broke an existing inner-binding negative test.
Repair cycle 1 added an OPEN-only nonblank reason guard and separated the new maximal descriptor
from the legacy fixture. Independent repair readback passed with no remaining findings against
candidate manifest `2ee72207dcbf85613091d4d00e529fe8e7902831048833a97d1f9236a861eb0d`.
The initial selected run passed mutation (0.345s), transaction (4.125s), store (118.162s), and CLI
(34.809s), while snapshot failed that retained fixture test. Targeted repair checks passed
mutation (0.250s), CLI (1.491s), and snapshot (0.461s), including the unchanged inner-binding
negative. The actual built CLI also completed a disposable recovery/preview/replay/audit journey
with reason lookup; this precommit exercise is not final artifact qualification. Focused vet and
spec index freshness passed. Final clean-bound results belong to the dogfood selected-check
receipts and supplemental snapshot logs; earlier dirty-tree observations are not reused as
final-target evidence.
Repository-wide `make gate` is `NOT_RUN` under the owner's scoped issue preference; selected
packages and supplemental snapshot checks do not imply exhaustive repository coverage.

Rollback disables the new OPEN transition but preserves issued receipt parsing and immutable
history. Native task integration and completion remain separate from source/CEM delivery.
