## 2026-10-05 V1-0789: review and integrate claims can exclude the implement author

Human-owned intent: owner request [issue 586](https://github.com/beamfall/corvint/issues/586) part 2,
ticket V1-0789. A reviewer or integrator could avoid the member that implemented a ticket only by
naming it with `--exclude-member`, which meant reading ticket history by hand. The owner decided:
by default exclude only the most recent implement generation's member (D5); offer
`--exclude-authors=all` for every recorded implement generation, under which any `NOT_OBSERVED`
generation in scope also refuses; no new codes; no backfill of legacy member facts.

Requirement: `CAL-V0-098` in `docs/specs/corvint-tasks-agent-leases-v0.md`, defined inside
`## Requirements` (V1-0789 subsection) so the OCM reader enumerates it. It was first drafted as
CAL-V0-085, which collided with V1-0790, and was renumbered on the coordinator's allocation:
CAL-V0-074..075 V1-0755, 078 V1-0780, 079..081 V1-0793, 082..085 V1-0790, 095 V1-0781, 096 V1-0788
(prior-generation history, merged here from its repair commit 16d3a7d9), 097 V1-0786, 098 V1-0789.

### Change

- Request: `CLAIM` and `CLAIM_NEXT` lease requests gain optional `excludeAuthors` (`LATEST` or
  `ALL`), omitted from the canonical preimage when absent. It requires an explicit pool and stage
  `review` or `integrate`. The mode, not the derived set, is bound, so replay precedes derivation and
  a changed mode under the same request ID is `REQUEST_ID_CONFLICT`.
- Derivation (`transaction.DeriveAuthors`): every generation of the ticket's attempts, newest first;
  prior generations read the V1-0788 history and current generations read what that history would
  record. Review and integrate generations are skipped; any other generation reached must be an
  implement generation with a recorded pool member. `NOT_OBSERVED`, stage-less, member-less
  implement and never-implemented cases refuse `INDEPENDENCE_UNVERIFIED` before member selection or
  health preparation.
- Allocation: the union of explicit and author members feeds the existing CAL-V0-065 predicate in
  allocation, health preparation (via the model's refused-result derivation) and final prepared
  admission. Exhaustion refuses `RESOURCE_COLLISION` naming each author's member, pool and generation.
- Plan: the same derivation is a per-ticket blocker in `PriorityFirst`, so `claim --next`, its
  outside-lock scope deriver and `plan preview` agree. With the flag, preview entries add `detail`
  and `excludedAuthors`; without it the output is unchanged.
- CLI: `--exclude-authors` and `--exclude-authors=all` on `claim` and `plan preview`; repeats and
  other values are malformed. Help usage and `docs/TASKS-EXTERNAL-AGENTS.md` updated.

### Decisions and limits

- Stage-less generations, implement generations outside any pool and tickets with no implement
  generation refuse rather than pass unfiltered; these extend the owner's NOT_OBSERVED rule and are
  raised as owner questions.
- Authors are matched by (pool, member); an author in another pool excludes nothing in the
  requested pool.
- Scope is every attempt of the ticket, across acceptance revisions.
- Multi-selection preview counts global slots with explicit exclusions only; a ticket whose own
  derived set leaves no slot is still reported as a blocker. `claim --next` admits one ticket, so it
  is exact.
- Not actor authentication, not an independence proof, not physical separation.

### Evidence

- Focused tests: `TestCALV0098_*` in `internal/tasks/{transaction,store,cli}`, with the
  transaction and store packages and the cli lease/plan/pool subset run in full (see the handoff for
  results); gofmt, `go vet` of touched packages, `go build ./...`, `GOOS=windows` cross-build, the
  CI doc gates and use-case receipt checks.
- OCM enumeration: `TestAgentLeasesSpecEnumeratesCALV0098` (`internal/lrfrepo`) asserts `requirementsFromBlob` lists `CAL-V0-098`.
- NOT_RUN: `make gate`, the repository-wide suite, interop, live multi-agent qualification and the
  dogfood bind/seal (owned by the coordinator after review).
