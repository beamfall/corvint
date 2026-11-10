# Design: `ticket-taxonomy`

Evidence marks: VERIFIED means the cited line was read in corvint-tasks build
`1.0.0-rc.3.dev+build.144` (paths under `internal/tasks/`). NOT_VERIFIED means it was not read.

## Decisions

- Facets are labels in fixed namespaces plus existing fields. `component:<v>`, `product:<v>`,
  `type:<v>`, `runner:<v>`, `agent:<v>` are labels. Anchor is `requirementRefs`. Milestone is the
  existing field. No other label is allowed except `suspected`. Labels fit: at most 64 bytes each
  (VERIFIED `wire/primitives.go:194-197`).
- Vocabulary. One file, `docs/specs/ticket-vocabulary.json` (proposed path), holds the 27
  components (28 with `rig`, C21) with their product, the six types with their kind, the runner
  and agent lists, the bug
  classes, the delivery slices, the chore actions, the root paths of each component, and the
  component of each change folder. The store policy carries the same lists in a new `facets` block,
  modelled on `MilestonePolicy` (VERIFIED `intent/policy.go:277-289`).
- Components. testing: affected, test, watch, inventory, line-evidence, runner, run-evidence,
  measurement. docs: docs, flows, how, mcp. tasks: store, claims, completion, receipts, reads, cli,
  policy. integration: action, agents, catalog, release, specs, gates, shared, preview.
- Rig (decided 2026-11-09 under R42, C21). Friction about the build rig (lanes, hosts, CI
  plumbing) is filed in the Corvint store under component `rig`. Its milestone is fixed to
  `post-1-0`, `UNPLANNED` is allowed, and it is left out of the `v1-0` release object and of the
  default roadmap view. Defects in shipped CI surfaces (the action, required checks, the release
  workflow) are not rig; they file under `action`, `release` or `gates`. There is no list outside
  the store. When the lane tooling moves to the private operator repository (G27, B8b), open
  `rig` tickets move with it and the component is retired. C21 does not name the product of
  `rig`; that is NOT_DECIDED (TTX-V1-002).
- Product is computed, never chosen. In the sample, product was arguable on 5 tickets and component
  was not.
- Type is chosen by six ordered questions (decision, bug, optimization, test-gap, delivery, chore).
  Kind follows type: bug BUG, delivery FEATURE, decision SPIKE, others CHORE. A delivery without an
  existing requirement is refused.
- Anchor. Every delivery and every `v1-0` ticket names a `PROD-*` requirement. A delivery covered by
  a change folder also names that folder's requirement ID. `UNPLANNED` is allowed only on a
  `post-1-0` ticket of type bug, optimization, test-gap, chore or decision, with a one-line
  reason; such a ticket can never enter the 1.0 release object (decided 2026-11-09 under R42,
  C20). IDs are checked against a configured requirements file
  by a create guard, modelled on the path guard (VERIFIED `cli/create_source_ref.go:13-30`).
- Identity key. Lower-case, dot-joined, at most 60 bytes; longer keys keep 53 bytes plus `-` plus 6
  hex digits of the SHA-256 of the full key.
  - delivery: `<requirement-id>.<slice>[.<runner>]`
  - bug: `<component>.<class>.<locus>`
  - test-gap: `<component>.gap.<locus>`
  - optimization: `<component>.slow.<locus>` or `.large.`
  - chore: `<component>.chore.<action>.<f-object>`
  - decision: `<component>.decide.<topic>`
  - Locus precedence: `t-` test or step name, `c-` printed code, `o-` output field, `r-` violated
    requirement, `v-` CLI verb, `w-` workflow or `m-` make target, `p-` package, `f-` file.
- The key is a new field `identityKey` with an index from key to ticket IDs. The collision check
  runs inside the create transaction, next to the ID check (VERIFIED `ticket/graph.go:204-214`,
  `mutation/apply.go:793-797`). Using the key as the ticket ID was rejected: it loses the V1 IDs that
  plans and pull requests cite, and it cannot record a second ticket under one key.
- A key is a bucket. The refusal `DUPLICATE_KEY` lists every holder. A second ticket is accepted only
  with `distinctFrom` naming every open holder, each with one line of difference. The sample forced
  this: V1-4973, V1-5013 and V1-5034 are three different defects with one key. Confirmed, with
  one addition: queue health counts the keys with more than one open holder (decided 2026-11-09
  under R42, C19; TTX-V1-015).
- Same problem again: the filer adds evidence to the holder with the existing `ticket recur` or
  `ticket attach-evidence`. `recur` refuses a finished ticket and says to reopen it first (VERIFIED
  `cli/similar.go:395-396`).
- Near-duplicates. After the exact key: a sibling list (same component and locus under another
  class; same component and anchor), then the lexical check, run for every create and limited to
  the component. Today the lexical check runs only for label `agent-memory` (VERIFIED
  `cli/similar.go:426-428`).
- Effects. Create requires coverage QUALIFIED and at least one touch path, and refuses a path of one
  directory level. The store never reports ELIGIBLE (VERIFIED `ticket/view.go:321-329`), so the
  guarantee is that `COVERAGE_UNKNOWN` and next action `set-effects` never appear.
- Templates. Each type has required body lines, checked by prefix. Body at most 1,200 bytes. The
  recurrence-line parser is the model (VERIFIED `similar/similar.go:285-293`).
- Search. New filters `--key`, `--requirement`, `--path`, and `--label` more than once. Today there
  is one `--label` and none of the others (VERIFIED `cli/cli.go:964`, `:1430-1458`).
- Interim. Until the store enforces the rules, the key is label `key:<k>` and one gate script in
  this repository checks every rule before a create. It is also the seed gate.
- Order. The field, the policy block and the filters are part of schema v1 and the
  supported-command list, so they land before `tasks-conformance` freezes them.
- Dates (decided 2026-11-09 under R42, C18). The key is built into Tasks before the freeze, not
  left to the label and gate script. Build items B1 to B3 (`identityKey` field and index,
  `DUPLICATE_KEY` inside the create transaction, `distinctFrom`; the `facets` policy block;
  search by key, requirement, path and several labels) land on corvint-tasks main by 2026-11-27.
  Schema v1 freezes on 2026-12-01. The cutover seeds now with the interim `key:` label and gate
  script, and a script backfills the field. B4 to B6 are best effort; if not in by 2026-11-27
  they stay in the gate script for 1.0 and arrive in 1.0.x as policy options that need no format
  change. Reason: after the freeze any format change restarts the 21-day trial (G9).

## Open decisions, all decided 2026-11-09 under R42

1. C18: build the identity key into Tasks before schema v1 freezes (TTX-V1-007, TTX-V1-014,
   TTX-V1-016, TTX-V1-018).
2. C19: the bucket rule as designed, plus the health count (TTX-V1-007, TTX-V1-008, TTX-V1-015).
3. C20: a `delivery` ticket and every `v1-0` ticket must name an existing PROD requirement
   (TTX-V1-004).
4. C20: `UNPLANNED` only on `post-1-0` bug, optimization, test-gap, chore or decision tickets,
   with a one-line reason (TTX-V1-004).
5. C21: rig friction files in the Corvint store under component `rig`, milestone fixed
   `post-1-0` (TTX-V1-001, TTX-V1-002).

## Failure modes

- Two creates with one key race. With the field: the second is refused in the transaction. In the
  interim: both can pass the script. The seed runs one writer, so the seed is safe; live filing in
  the interim is not.
- Filers pick different loci for one problem. The exact key misses it. The sibling list or the
  lexical check has to catch it. In the sample this happened for 3 pairs; one pair (V1-5141,
  V1-5259) is caught only lexically.
- A filer writes a `distinctFrom` reason that says nothing. The store cannot judge prose. Review of
  same-key pairs is one search, and queue health counts the keys with more than one open holder
  (C19).
- The key field slips past 2026-11-27. It adds a schema change three weeks before the Tasks RC,
  on top of `tasks-hardening`; a slip moves the trial start and removes the one week of slack in
  the C22 dates (C18).
- Rig tickets fill the store again (49 lane-host tickets were not seeded under C13). The fixed
  `post-1-0` milestone keeps them out of every 1.0 count (C21).
- Coarse delivery keys. A delivery that names only a change folder shares a bucket with every other
  ticket of that change (45 of 145 sampled). The anchor rule requires the requirement ID.
- A ticket holds several problems (12 of 145 sampled). It cannot get one key. It is split before
  create.
- The vocabulary file and the store policy drift apart. The gate script compares them and refuses.
- A requirement ID is retired after a ticket cites it. The ticket keeps the ID; the anchor check
  runs at create only. A sweep is not part of this change.
- Decision keys use a free topic. They are the weakest key and rely on the sibling list.

## Acceptance evidence

- The sample: `sample.tsv` with 158 real tickets, 145 in scope, 125 clean on every facet, and the 20
  shared keys listed in the design note beside it.
- Store tests for each refusal: unknown label, missing facet, unknown requirement, key collision,
  incomplete `distinctFrom`, missing touch path, missing template line.
- A two-process test that creates the same key at once and observes exactly one success.
- Timed reads for each search in TTX-V1-014 on a store of at least 5,000 tickets, recorded with the
  build and host.
- The seed gate run on the survivor set: exit 0, a `held` list with a reason per ticket, and no
  seeded ticket with next action `set-effects`.

## Rollback

- The `facets` policy block is opt-in. Removing it from the policy returns create to today's rules.
  Tickets keep their labels and key.
- `identityKey` and `distinctFrom` are additive members. A build that does not know them is not
  supported after schema v1; before the freeze, removing the field leaves the `key:` label in use.
- The gate script is a file in this repository. Removing it removes the interim enforcement only.
- Nothing in this change deletes or rewrites an existing ticket.
