# External Test Selection V0 — fail-closed advice from verification relations

Owner: Russell Lewis
Date: 2026-09-18
Intent status: accepted (decision 0311)
Delivery status: experimental (file transport only)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/affected-plan-v0.md`, `docs/specs/external-evidence-provider-v0.md`,
`docs/specs/external-evidence-provider-v1.md`, decisions 0309 and 0310, and the feature request
Beamfall/corvint#5.

## Agent digest
- Claim: `corvint affected --provider FILE` adds `advice.test_selection`: fail-closed advice on whether external verification relations allow a narrower test run.
- Status: accepted (decision 0311, a delegated call on Beamfall/corvint#5)/experimental (file transport only); checked by `TestSelectionConformance`, `TestSelectionEvaluation`, and `TestAffectedSelectionAddsOneMemberAndKeepsEverythingElse`.
- Exists: `internal/extevidence/selection.go`, the `--provider`, `--repository`, and `--selection-profile` options of `corvint affected`, and conformance fixtures under `internal/extevidence/testdata/conformance-selection/`.
- Blocked on: an ACC-V0 provider profile before any executed transport; an independent adopter record before promotion.
- Read next: Definitions; Requirements; Trust boundary, limits, and failure modes.

## User and measurable job

An agent that changed a few files wants to run the tests that verify what it changed, not the
whole suite, and needs to know when that is unsafe. The generic, provider-agnostic input is any
EEP record that maps changed paths to entities and relates tests to those entities with
`verifies`, `asserts`, or `covers`. The job: in the labelled conformance corpus, no case that
should require the full relevant suite is reported as `narrow-selection-allowed` (unsafe-narrowing
rate 0), every narrow selection names only tests the label expects (precision 1), and every
abstention case is `unknown`. The rest of the affected receipt stays byte-identical.

## Verified current state

Before this slice, `corvint affected` (AFP-V0) advised repository-declared mandatory checks plus
one advisory Go command, and EEP V0 and V1 attached external relations to `corvint impact` only.
No surface consumed `verifies`, `covers`, or `asserts` to decide anything. The ideas backlog carried
this request as "EEP slice 3".

## Definitions

- **Obligation**: a changed path, an entity a changed path maps to, or an entity one relation hop
  downstream of such an entity. Narrowing must justify every obligation. `ETS-V1-001` replaces the
  one hop with a bounded transitive walk.
- **Qualifying relation**: a relation from a test path to an obligation entity whose type the
  profile admits and which meets every condition of `ETS-V0-005`.
- **Selected test**: the test side of a qualifying relation. **Candidate test**: the test side of
  any other relation to an obligation, listed with its exclusion code.
- **Weak evidence**: a context type (`navigates`, `candidate`), `inferred` or excluded evidence,
  or `covers` under the strict profile. It neither qualifies nor blocks.
- **Mandatory check**: an `advice.checks` entry of kind `mandatory` (AFP-V0-009).

## Requirements

- `ETS-V0-001`: `corvint affected` MUST accept `--provider FILE` (up to 4), `--repository ID=DIR`
  (up to 8, EEP-V1 binding rules), and `--selection-profile strict|coverage` (default `strict`),
  each as `--flag VALUE` or `--flag=VALUE`. `--repository` and `--selection-profile` without
  `--provider`, a repeated repository id, an unknown profile, and a missing value MUST exit 2 with
  empty stdout.
- `ETS-V0-002`: With `--provider`, `advice` MUST gain `test_selection` with schema
  `external-test-selection/0` and the members `schema`, `profile`, `admitted_types`, `state`,
  `state_reason`, `authority`, `mandatory`, `selected`, `candidates`, `uncovered_paths`,
  `uncovered_entities`, `blocking_reasons`, `unknowns`, `omitted`, `provider_evidence`, `scope`,
  `note`, and `untrusted_text_fields`. Without `--provider`, the receipt MUST be byte-identical to
  a run before this slice. With it, every other receipt member MUST be byte-identical.
- `ETS-V0-003`: `state` is exactly one of `narrow-selection-allowed`,
  `full-relevant-suite-required`, `blocked`, or `unknown`, decided in this order: any unavailable
  or invalid record → `blocked` (`provider-unavailable`); a plan scope that is not bounded →
  `unknown` (`incomplete-affected-scope`); no changed path → `unknown` (`no-changed-paths`); any
  uncovered obligation or blocking reason → `full-relevant-suite-required` (`open-obligations`);
  otherwise `narrow-selection-allowed`. The state MUST NOT depend on how many tests were selected.
- `ETS-V0-004`: The strict profile admits `verifies` and `asserts`; the coverage profile also
  admits `covers`. No profile admits a context type.
- `ETS-V0-005`: A relation qualifies only when its type is admitted, its evidence is `declared` or
  `observed`, its type is not namespaced, the test side's repository identity is `resolved` and its
  binding is `root` or `checkout`, the record's repository revision is fresh for that side, the
  test path is verified at that revision, and, for a root-side path, the path is not dirty in the
  worktree. A V0 record never qualifies (`no-repository-identity`).
- `ETS-V0-006`: Any record, identity, binding, freshness, verification, or unsupported-relation
  failure touching an obligation MUST block that obligation and appear in `blocking_reasons`. A
  record that cannot identify the root repository blocks every changed path
  (`unbound-root-repository` or `ambiguous-repository-identity`). Weak evidence MUST neither
  qualify nor block.
- `ETS-V0-007`: Every selected and candidate row MUST carry the provider, provider revision, entity
  and kind, test path (and repository and blob when known), relation, relation type, evidence,
  authority `external-provider`, `confidence` `unscored`, identity, binding, freshness, source and
  test revisions, verification, relation state, `crosses_repositories`, limitations, and a
  one-sentence reason.
- `ETS-V0-008`: Every candidate, uncovered obligation, and blocking reason MUST carry a named code.
  An obligation with no evidence carries `no-external-evidence`. A relation between two entities
  names no test path, so it leaves its affected entity uncovered with `entity-test-endpoint`.
- `ETS-V0-009`: `mandatory` MUST echo every mandatory check unchanged. Selection never removes,
  reorders, or rewrites `advice.checks`.
- `ETS-V0-010`: Rows are sorted by a total key. Every list is bounded at 64 rows and `omitted`
  counts each cut. The same inputs MUST produce identical bytes regardless of relation order.
- `ETS-V0-011`: The conformance corpus MUST cover positive, negative, stale, missing, ambiguous,
  unavailable, unsupported, widening, dirty, partial, and abstention cases, each labelled with its
  expected state, codes, selected tests, and relevant tests.
- `ETS-V0-012`: The evaluation MUST report precision, unsafe-narrowing rate, abstention accuracy,
  latency, and receipt size over the corpus, and MUST fail on any unsafe narrowing.
- `ETS-V0-013`: The member MUST carry no file body, credential, or resolved checkout directory. A
  checkout is echoed as given. `relation.rule` and `relation.reference` are listed in
  `untrusted_text_fields`.

## Non-goals and simpler baseline

- Executing, scheduling, or ranking tests; changing the exit code; CEM or Change Frontier changes;
  an MCP surface; a product-specific adapter; inferring relations.
- Inspecting a bound checkout's worktree. Rows from a checkout carry
  `checkout-worktree-not-inspected`. `ETS-V1-005` now reads it.
- Obligations beyond one downstream hop. `ETS-V1-001` now walks them.
- The simpler baseline is "run what `advice.checks` says". It stays the default and stays in force:
  this slice only adds advice about which additional tests are enough.

## Trust boundary, limits, and failure modes

A record is provider-authored data. It can justify a narrow selection only through Git-checked
identity, freshness, and path verification, and it never grants authority or removes a mandatory
check. An empty `selected` list is never proof. Limits: 4 records, 8 checkouts, 64 rows per list,
and the EEP V0 and V1 record bounds.

| Condition | Result |
|---|---|
| Record absent or invalid | `blocked`, `provider-unavailable` |
| Unbounded plan scope (for example a language frontier) | `unknown`, `incomplete-affected-scope` |
| Changed path with no relation | `full-relevant-suite-required`, `no-external-evidence` |
| Stale record revision on the test side | `stale-provider-revision`, blocking |
| Test path missing or changed at the revision | `missing-path-reference` or `stale-path-reference`, blocking |
| Ambiguous identity or binding | `ambiguous-repository-identity`, blocking |
| Test repository with no checkout | `unbound-test-repository`, blocking |
| Namespaced relation type touching an obligation | `unsupported-relation`, blocking |
| Uncommitted change to a root path | `worktree-dirty-path`, blocking |
| Weak evidence only | coded candidate; obligation stays uncovered |
| Entity-to-entity relation to an obligation | `entity-test-endpoint`; entity stays uncovered |

## Deterministic acceptance and testing matrix

| Case | Expected | Test |
|---|---|---|
| 23 labelled cases in `conformance-selection/cases.json` | labelled state, codes, and selected tests | `TestSelectionConformance` |
| Corpus evaluation | unsafe 0, precision 1, abstention exact | `TestSelectionEvaluation` |
| Selected row fields | full provenance, `unscored`, limitations | `TestSelectionRowProvenance` |
| Mandatory checks | echoed unchanged | `TestSelectionMandatoryEchoedUnchanged` |
| Repeat run and reversed relations | identical bytes | `TestSelectionDeterministic` |
| Limit 1 | counted omissions | `TestSelectionOmissionAccounting` |
| Relative checkout | no resolved directory or body | `TestSelectionPrivate` |
| CLI with and without `--provider` | one added member, others byte-identical | `TestAffectedSelectionAddsOneMemberAndKeepsEverythingElse` |
| CLI dirty path, absent record, frontier | full, blocked, unknown | `TestAffectedSelectionFailsClosed` |
| CLI option errors | exit 2 with named message | `TestAffectedSelectionArguments` |

## Rollout, rollback, and compatibility

Additive and opt-in. Rollback removes `selection.go`, the three `affected` options and their help
text, the fixtures, this document, and decision 0311, and reverts the AFP-V0-009 wording. A run
without `--provider` is unchanged in both directions.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `ETS-V0-001` | `parseAffectedOptions` in `cmd/corvint/affected.go` | `TestAffectedSelectionArguments` |
| `ETS-V0-002`, `ETS-V0-009` | `compileAffected`, `affectedSelectionInput` in `cmd/corvint/affected.go` | `TestAffectedSelectionAddsOneMemberAndKeepsEverythingElse`, `TestSelectionMandatoryEchoedUnchanged` |
| `ETS-V0-003`, `ETS-V0-006` | `state`, `block`, `unrooted` in `internal/extevidence/selection.go` | `TestSelectionConformance`, `TestAffectedSelectionFailsClosed` |
| `ETS-V0-004`, `ETS-V0-005`, `ETS-V0-008` | `qualify` in `internal/extevidence/selection.go` | `TestSelectionConformance` |
| `ETS-V0-007` | `row`, `provenance` in `internal/extevidence/selection.go` | `TestSelectionRowProvenance` |
| `ETS-V0-010` | `result` in `internal/extevidence/selection.go` | `TestSelectionDeterministic`, `TestSelectionOmissionAccounting` |
| `ETS-V0-011`, `ETS-V0-012` | `internal/extevidence/testdata/conformance-selection/` | `TestSelectionConformance`, `TestSelectionEvaluation` |
| `ETS-V0-013` | `result`, `provenance` in `internal/extevidence/selection.go` | `TestSelectionPrivate` |

## Unresolved decisions and promotion or kill criteria

- Promotion needs one independent adopter's record and a measured corpus from a real change
  history, evaluated with the same metrics.
- Kill the narrowing state if any adopter-labelled case narrows unsafely. The member would then
  stay advisory-only, listing candidates without a state that allows narrowing.
