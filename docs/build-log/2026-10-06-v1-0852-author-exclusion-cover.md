## 2026-10-06 V1-0852: explicit exclusions cover unrecorded author generations

Human-owned intent: owner request [issue 623](https://github.com/beamfall/corvint/issues/623),
ticket V1-0852. `claim --exclude-authors` (CAL-V0-098) refused `INDEPENDENCE_UNVERIFIED` for a
ticket whose implement generation predates V1-0788 even when the caller named the author with
`--exclude-member`, and refused a review or integrate claim on a ticket with no implement generation
at all. The V1-0789 entry recorded both refusals as an implementer extension raised as owner
questions; the issue answers them.

Requirement: new `CAL-V0-107` (V1-0852 subsection of `## Requirements` in
`docs/specs/corvint-tasks-agent-leases-v0.md`); `CAL-V0-098` amended to defer to it.

### Change

- `transaction.DeriveAuthors` covers, when at least one explicit member is supplied, a reached
  generation that is `NOT_OBSERVED`, stage-less without a member, or an implement generation without
  a member. The covered generation contributes no member and is named in `AuthorExclusion.Covered`;
  the walk continues. A stage-less generation that recorded a member still refuses. Without explicit
  members every refusal is unchanged.
- A ticket with no recorded implement author is no longer refused; its exclusion set is the explicit
  members, and a `RESOURCE_COLLISION` detail says `excluded implement authors: none recorded`.
- The admitted claim's derivation now reaches the result (`leaseOutcome.authors` to
  `Result.AuthorExclusion` to `store.Report`), and `leaseResult` adds `AuthorExclusion.Notes()` as
  warnings: covered generations are caller-asserted, not recorded; no implement author is recorded.
- POOL_PREPARE carries the claim's explicit members and covers only when they are present
  (corrected after review; see `2026-10-06-gh623-626-review-fixes.md`).

### Decisions

- A never-implemented ticket is admitted at every allowed stage (review and integrate), with or
  without explicit members: there is no author to exclude. The issue asked this for integrate (and
  review); applying it without explicit members too is the same rule.
- Covered generations do not stop `LATEST`; it still stops at the first recorded author, so a legacy
  generation newer than a recorded one is covered and the recorded author is excluded.
- Plan preview keeps its CAL-V0-098 shape; the caveat is on the claim result only.
- Replay derives nothing, so an exact replay carries no warning; preimages and CAL-V0-065 binding
  are unchanged (`TestCALV0107_CLIClaimReportsCoveredGenerations` replays).

### Evidence

`GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m -run 'CALV0098|CALV0107'
./internal/tasks/transaction ./internal/tasks/store ./internal/tasks/cli`: all `ok`. New
`TestCALV0107_ExplicitMembersCoverUnrecordedGenerations` (mixed legacy and recorded generations,
with and without explicit members, LATEST and ALL, no-implement tickets, prepare cover) and
`TestCALV0107_CLIClaimReportsCoveredGenerations`; the CAL-V0-098 store and CLI tests were updated
where they pinned the old never-implemented refusal, and gained a stage-less pooled case that still
refuses and a health-backed covered claim. `go vet` (darwin, `GOOS=linux`, `GOOS=windows`) on the
three packages, `gofmt -l`, `make spec-requirements-check requirement-definitions-check
line-citations-check traceability-tests-check` and `go test -run TestAgentLeasesSpec
./internal/lrfrepo`: clean. One spec line citation (`model.go:613`) moved to 615 with an unchanged
anchor.

NOT_RUN: full package suites, `make gate`, live multi-agent qualification.

Rollback: revert the commit. No stored field or request preimage changes; claims admitted under the
cover keep their allocations.
