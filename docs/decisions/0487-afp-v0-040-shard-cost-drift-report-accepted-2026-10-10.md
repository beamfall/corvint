# Decision 0487 — advisory CI shard cost drift report (AFP-V0-040) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept AFP-V0-040"), relayed by the coordinating agent.

## Context

The AFP-V0-022 shard cost table went stale within six days of its 2026-10-04 refresh. In hosted run
38055182050 `internal/tasks/store` ran about 1,000s longer than recorded and seven executed packages
had no entry, and nobody noticed until the shards visibly unbalanced. The shard-drift lane proposed
`AFP-V0-040` in `docs/specs/affected-plan-v0.md`:

- Each full `go-product-shard` keeps its `go test -json` stream as a short-retention artifact. Capture
  is best-effort, and only `go test`'s own exit status decides the required shard.
- The non-required job `ci-shard-cost-drift` runs `tools/ci-shard-costs check --advisory` only when
  every shard succeeded and exactly N well-formed, finished streams are present. Otherwise it abstains
  with its reason in the job summary. It reports material findings (10% of the ideal shard by default)
  as at most ten warnings, and the job never ends red.

Independent review of 22159dea raised three P2 findings, which were fixed in a following commit. AGENTS.md
invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AFP-V0-040` as written. The requirement status, the spec header and digest, the
`docs/specs/README.md` row, `docs/specs/INDEX.json` and the lane's build log
(`docs/build-log/2026-10-10-ci-shard-drift-detection.md`) record the acceptance. This decision
accepts no other requirement of the spec.

## Limits

This decision settles intent only. The evidence is the `TestAFPV0040*` tests, `actionlint`,
`make ci-least-privilege-check`, the doc gates and a local dry run of the job's steps against run
38055182050's shard logs.

- Hosted behaviour is `NOT_OBSERVED`. This covers artifact upload and download, annotation rendering,
  the job summary, re-run attempts and the extra job time. Until the change's own CI runs, nothing
  shows that the workflow behaves on GitHub as it did in the dry run.
- The change edits `.github/workflows/ci.yml`, so its pull request needs the admin-posted
  `ci-control-plane` status (AFP-V0-016) before it can merge. Nothing in this decision posts it.
- The report is advisory. It never posts or changes a commit status, never refreshes or commits the
  table, and never changes shard membership or placement. Refreshing stays the operator's
  AFP-V0-022 `refresh` step.

## Rollback

Revert this decision and return the AFP-V0-040 status text to proposed, pending owner acceptance, in
the requirement, the spec header and digest, the README row, `INDEX.json` and the build log. Then
regenerate `docs/specs/REQUIREMENTS.tsv`. To withdraw the behaviour, also revert the lane's changes:
delete the `ci-shard-cost-drift` job and the retention step, restore the plain `go test` invocation in
`go-product-shard`, and remove `--advisory` from `tools/ci-shard-costs`. No stored state, table or
required check depends on it.
