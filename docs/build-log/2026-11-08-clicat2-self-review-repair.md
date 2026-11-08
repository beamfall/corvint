# V1-4846 self-review repair: offline MCP completeness

Requirement: CAP-V0-8800, AC2. The self-review found that a lexical inventory
missed all six constant-backed `tasks-work` descriptors, and that the catalog's
introductory guidance still denied offline discovery.

Decision: use the existing offline descriptor constructors for the core profiles,
corpus, docs, test validity, Tasks reads and every Tasks work role. Their concrete
names join the supplemental source inventory before exact catalog membership is
checked. This removes dependence on name-expression syntax for these providers:
constants, table projections and computed names are ordinary descriptor values.
The admitted membership is the exact set of concrete names; an unfamiliar name
has no namespace or expression exemption and a missing entry fails. Source
scanning remains supplementary for standalone server declarations; no claim of
a general Go program analysis or live configured availability is made.

The six Tasks work entries retain AGENT/OPERATOR/OWNER availability for lane verbs
and findings, and OPERATOR/OWNER-only availability for store-host verbs. The root
catalog now contains 50 entries with unchanged member shapes. The introductory
route is `corvint help --json` / `mcpTools`; live `tools/list` still supplies schemas
and configured availability. No protocol implementation, golden or native refusal
changed. Rollback removes this checker augmentation and its six catalog entries,
restoring the documented incomplete discovery boundary.

Evidence: whole-package `tools/capabilities-check` passed (10.077 s), including
`TestCAPV08800OfflineDescriptorOmissionFails` (each of the six constant-backed
names omitted through the actual machine check) and
`TestCAPV08800UnknownOfflineDescriptorFailsClosed` (unfamiliar computed descriptor
name remains missing). Whole-package `internal/capcatalog` passed (0.066 s),
including `TestCAPV08800TasksWorkRoleConditions` and existing embedded/generated
catalog parity. The first checker run failed because its synthetic repository
omitted the newly required concrete descriptors; the fixture now supplies them.
Every existing assertion remains intact. Raw logs remain outside the repository
in `/home/russell/lanes/clicat2-evidence/`.

The orchestrator owns independent review, broader host gates and ticket completion.
No live server or Tasks mutation was used. Final CEM outcome and exact command
receipts are retained in the lane handback; this entry is not independent review.
