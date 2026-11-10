# Spec delta: watch-flows-hardening

The lane copies each statement below into `watch-flows-hardening-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: WFF-V1-001 A planning error on save is an answer

Source: PROD-TEST-007, PROD-TEST-002. When planning one save fails (diff, parse, package list or
runner plan error), `corvint watch` MUST write a `plan` event with `scope: UNKNOWN`, the reason
`plan-error`, the error message, and the file and line when the error names them. That event MUST
list the widest test set watch can name: every package of the Go module, and the full-suite command
of each other affected runner. Watch MUST keep running and MUST answer the next save normally.
Watch SHALL exit 2 only when it can no longer read events, read the index or write output, and in
that case it MUST first write an `error` event on stdout.

#### Scenario: Half-typed test file

- **GIVEN** a running watch on a Go module
- **WHEN** the user saves a `_test.go` file with a syntax error
- **THEN** watch writes a `plan` event with `scope: UNKNOWN`, reason `plan-error`, the file and
  line, and every package of the module
- **AND WHEN** the user saves the fixed file
- **THEN** watch writes a normal plan event and is still running

#### Scenario: Output closed

- **GIVEN** a running watch whose stdout reader has exited
- **WHEN** watch fails to write an answer
- **THEN** watch exits 2

### Requirement: WFF-V1-002 Warm state drops on any changed or unknown input

Source: PROD-TEST-007, PROD-TEST-002. Every item watch keeps between saves MUST record the inputs it
was derived from (file content hash, HEAD commit, index state, runner config hash, flow intent tree
hash, as applicable). Watch MUST reuse an item only when every recorded input matches. When an input
changes or cannot be read, or HEAD moves (checkout, stash, rebase, commit), watch MUST derive the
item again. File modification time alone MUST NOT prove an item fresh. A warm plan MUST equal the
cold plan for the same tree.

#### Scenario: Checkout under a running watch

- **GIVEN** a running watch with warm state for branch A
- **WHEN** the user checks out branch B and saves a file
- **THEN** watch derives its state again and the plan equals a cold plan on branch B

### Requirement: WFF-V1-003 A step or outcome test link selects through its variations

Source: PROD-TEST-004, PROD-TEST-002. A test link whose `from` is a step or an outcome MUST be
treated as linked to every variation that lists that member. A flow hit MUST select that test link
when it selects any of those variations. `corvint flows impact` MUST use the same rule, so the two
commands give the same tests for the same change.

#### Scenario: Step-level link

- **GIVEN** a flow whose variation `checkout-card` lists step `pay`, and a test link from `pay` to
  `e2e/pay.spec.ts`
- **WHEN** a change reaches the code anchor of `pay`
- **THEN** the plan selects `e2e/pay.spec.ts` and `flows impact` names the same test

### Requirement: WFF-V1-004 A hit with no variation selects the whole flow

Source: PROD-TEST-004. When a flow hit's member is listed by no variation, the plan MUST select
every test link of that flow, with reason `member-without-variation`. It MUST NOT drop the hit.

#### Scenario: Member outside every variation

- **GIVEN** a flow with outcome `receipt-sent` that no variation lists, and three test links
- **WHEN** a change reaches the code anchor of `receipt-sent`
- **THEN** the plan selects all three tests with reason `member-without-variation`

### Requirement: WFF-V1-005 Old step and outcome test links run until moved

Source: PROD-TEST-005, PROD-TEST-004. No tool SHALL convert a step-level or outcome-level test link
into an accepted variation link record. Each such link MUST be listed under
`e2e.link_not_variation` in the plan and in the flows gap report. Its test MUST be treated as having
no accepted variation link, so it runs as `unassigned` in every plan until a person links it to a
variation.

#### Scenario: Unmoved step link

- **GIVEN** a test link from step `pay` to `e2e/pay.spec.ts` and no variation link for that test
- **WHEN** a change reaches no anchor of the flow
- **THEN** the plan still runs `e2e/pay.spec.ts` as `unassigned` and lists the link under
  `link_not_variation`

### Requirement: WFF-V1-006 The plan names changed paths that reach no flow

Source: PROD-TEST-004, PROD-TEST-002. When a repository declares flows, the plan MUST list under
`e2e.unanchored` every changed path that has a graph owner, is not a global input, and reaches no
link target of any flow. The list MUST give the exact count and at most 50 paths in sorted order,
with `truncated: true` when cut. The plan MUST also select every E2E test, under the one reason
`change-unanchored`, when the list is not empty (R42 (panel pick) A1).

#### Scenario: Server handler outside every flow

- **GIVEN** flows whose anchors are all client files, and a changed owned file `server/orders.go`
- **WHEN** the plan is built
- **THEN** `e2e.unanchored` has count 1 and lists `server/orders.go`

#### Scenario: Unanchored path widens to every E2E test

- **GIVEN** flows with three E2E tests and a changed owned file `server/orders.go` that reaches no
  link target
- **WHEN** the plan is built
- **THEN** all three E2E tests are selected with reason `change-unanchored`

### Requirement: WFF-V1-007 The deprecated flows reader never omits a reached consumer

Source: PROD-TEST-002, PROD-TEST-004. While `.corvint/e2e-flows.json` is still read, a consumer
whose commands any changed path statically reaches MUST be selected. A reviewed link, of this flow
or another, MUST NOT explain the path away. The deprecation notice MUST stay.

#### Scenario: Link of another flow

- **GIVEN** a reviewed link of flow A that covers `lib/price.ts`, and flow B whose consumer command
  statically reaches `lib/price.ts`
- **WHEN** `lib/price.ts` changes
- **THEN** the plan selects flow B's consumer

### Requirement: WFF-V1-008 A snapshot plan says its E2E part is unknown

Source: PROD-TEST-003, PROD-TEST-002. When a plan is built on a tree that is not the repository
checkout and the repository declares flows (a flows intent or `.corvint/e2e-flows.json`), the plan
MUST carry `e2e = {scope: UNKNOWN, unknown: [E2E_SNAPSHOT_UNJOINED]}`. It MUST NOT omit the `e2e`
field and MUST NOT claim the plan is bounded.

#### Scenario: Guidance review snapshot

- **GIVEN** a repository with a flows intent
- **WHEN** guidance review builds an affected plan on a scratch snapshot
- **THEN** the plan's `e2e.scope` is `UNKNOWN` with `E2E_SNAPSHOT_UNJOINED`
