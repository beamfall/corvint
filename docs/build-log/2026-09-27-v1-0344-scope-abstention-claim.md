## 2026-09-27 V1-0344: the README no longer promises task-level abstention

The pre-1.0 panel (finding D14) found that the README's "It says what it does not know" bullet
promised explicit abstention and that missing evidence always produces an unknown, while the
README status table says "Do not rely on ranking or abstention". A nonsense task returned `READY`
with 20 results. The two verifiers agreed on these facts but split on whether the bullet was a
defect. The finding named either an owner edit scoping the bullet or an admitted abstention metric
as its resolution. The owner has instructed that every disputed finding be treated as a defect.

Decision: scope the bullet. It keeps the claims the binary meets today: coverage, omissions,
uncertainty and freshness are receipt fields, and a CEM hunk without cited evidence is an explicit
unknown. It now says that task-level retrieval abstention is experimental and unqualified, and it
links to the status section. No abstention metric is admitted, and no code changes.

Rollback: revert the change, which restores the unscoped promise.
