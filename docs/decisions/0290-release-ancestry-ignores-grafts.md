# 0290 — Release ancestry ignores Git grafts

Date: 2026-09-13

Owner-authorized release correctness work repairs two known reachable ancestry readers:
`corvint witness` and the publication receipt checker. A real graft can hide a true parent or
invent an unrelated ancestor even when replacement objects are disabled. Pin the existing bounded
Git runners to the null graft file and disable deprecation advice, without changing repository
files, wire formats, authority or output budgets. AGW-V0-002, ARTIFACT-RDY-V0-007 and PUB-V0-019
own the behavior. The production dashboard runner already has this protection.

Focused fixtures reproduce the failure before the fix and check local plus ambient graft paths.
Other recorded trace/migration and historical-oracle sites remain explicitly open; this release
repair does not claim a repository-wide audit. Rollback reverts these two runner changes.
