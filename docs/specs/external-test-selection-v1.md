# External Test Selection V1 — transitive obligations and checkout worktree reads

Owner: Russell Lewis
Date: 2026-09-19
Intent status: accepted (decision 0315)
Delivery status: experimental (file transport only)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/external-test-selection-v0.md`, `docs/specs/external-evidence-provider-v1.md`,
decisions 0311 and 0315, and the feature request Beamfall/corvint#12.

## Agent digest
- Claim: `advice.test_selection` walks provider-declared entity relations transitively within fixed bounds and reads each bound checkout's worktree; both only widen.
- Status: accepted (decision 0315, a delegated call on Beamfall/corvint#12)/experimental (file transport only); checked by `TestSelectionConformance`, `TestSelectionEvaluation`, `TestSelectionExtensionsOnlyWiden`, and `TestAffectedSelectionInspectsCheckout`.
- Exists: `walk`, `truncate`, `inspect`, and `checkoutCode` in `internal/extevidence/selection.go`; the `CheckoutStatus` reader `corvint affected` passes; `multihop*.json`, `cycle.json`, `deep.json`, and the `checkout-*` cases under `internal/extevidence/testdata/conformance-selection/`.
- Blocked on: an independent adopter record before promotion, as for ETS-V0.
- Read next: Requirements; Trust boundary, limits, and failure modes.

## User and measurable job

ETS-V0 stopped obligations one relation hop past a changed entity and never read a bound
checkout's worktree. A two-hop dependency and an uncommitted edit in a test checkout were both
invisible to it, so it could allow a narrow selection that a complete record would forbid. The
job: every labelled case whose evidence runs past one hop, or through a dirty or unreadable
checkout, is not `narrow-selection-allowed`, and adding either extension never turns a
non-narrow case into a narrow one. The ETS-V0 evaluation floors (unsafe-narrowing rate 0,
precision 1, exact abstention) still hold over the enlarged corpus.

## Verified current state

At `05e17d0`, `widen` in `selection.go` added only the direct neighbours of joined entities, and
`limitations` stamped `checkout-worktree-not-inspected` on every checkout-bound row. The ETS-V0
non-goals listed both gaps.

## Definitions

- **Obligation walk**: a breadth-first walk from the entities a changed path maps to, over the
  record's own entity-to-entity relations whose type is neither a verification nor a context type,
  in both directions. Every entity it reaches is an obligation.
- **Cut**: the walk stops with entities still unwalked, because it reached the depth bound or the
  next step would exceed the entity bound. The **edge** is each walked entity with an unwalked
  neighbour.
- **Inspected checkout**: a resolved `--repository` checkout whose dirty-path set was read.

## Requirements

- `ETS-V1-001`: The obligation walk MUST replace the one-hop widening. It is bounded at
  `MaxObligationDepth` (4) hops and `MaxObligationEntities` (256) entities per record, visits each
  entity once, and terminates on cycles.
- `ETS-V1-002`: A cut MUST block every edge entity with `obligation-depth-truncated` or
  `obligation-budget-exhausted`, so the state cannot be `narrow-selection-allowed`.
- `ETS-V1-003`: A cut MUST add one `unknowns` row per edge entity with `code`, `provider`,
  `entity`, `depth` (hops walked), and `unwalked` (entities left at the next hop).
- `ETS-V1-004`: The walk follows only relations the provider declared. It infers no relation,
  and a context or verification relation between entities never extends it.
- `ETS-V1-005`: `corvint affected` MUST read each resolved checkout's dirty paths once, with the
  same bounded, fail-closed Git status capture it uses for the root (`affected.DirtyPaths`).
  A side bound to a checkout with any dirty path fails with `checkout-worktree-dirty`, and one
  whose status cannot be read fails with `checkout-worktree-unreadable`. Both codes block.
- `ETS-V1-006`: A checkout is judged as a whole: any dirty path in it blocks every side it binds,
  because a test's result can depend on files it does not name.
- `ETS-V1-007`: A row whose every checkout side was inspected MUST NOT carry
  `checkout-worktree-not-inspected`. A library caller that passes no `CheckoutStatus` keeps the
  ETS-V0 behaviour and limitation.
- `ETS-V1-008`: Both extensions can only widen. For every labelled case, reading a clean, dirty,
  or unreadable checkout never makes a non-narrow state narrow, and a dirty or unreadable one
  never selects more tests. The ETS-V0-012 evaluation floors MUST hold over the enlarged corpus.
- `ETS-V1-009`: The member schema stays `external-test-selection/0`. The new codes and the
  `unknowns` row shape are additive, and no member carries a checkout directory or a dirty path
  (ETS-V0-013).

## Non-goals and simpler baseline

- A per-path checkout rule, a caller-set depth, reading ignored files, or opening any file body.
- Walking relations across records or providers. Each record walks its own relations.
- The simpler baseline, ETS-V0's one hop with a disclosed limitation, is what V1 replaces. Its
  failure was silent: a record with a two-hop chain could narrow.

## Trust boundary, limits, and failure modes

The walk reads only the record. The checkout read is a Git status report bounded at 8 MiB and
10 seconds per checkout, and at most 8 checkouts (ETS-V0 bound), each read once per run.

| Condition | Result |
|---|---|
| Chain longer than 4 hops | edge blocked, `obligation-depth-truncated`, unknown row |
| Walk would pass 256 entities | edge blocked, `obligation-budget-exhausted`, unknown row |
| Relation cycle | each entity walked once; the walk ends |
| Bound checkout has a dirty or untracked path | `checkout-worktree-dirty`, blocking |
| Checkout status fails, overflows, or times out | `checkout-worktree-unreadable`, blocking |
| Checkout unresolved | ETS-V0 binding code; not inspected |

## Deterministic acceptance and testing matrix

| Case | Expected | Test |
|---|---|---|
| Second hop unverified (`multihop.json`) | full, `no-external-evidence` on the second-hop entity | `TestSelectionConformance` |
| Every hop verified; cycle | narrow; the walk terminates | `TestSelectionConformance` |
| Seven-entity chain (`deep.json`) | full, `obligation-depth-truncated` | `TestSelectionConformance` |
| 257-entity fan-out | full, `obligation-budget-exhausted` | `TestSelectionWalkBudget` |
| Clean, dirty, unreadable checkout | narrow, full, full with the named code | `TestSelectionConformance` |
| Every case under each checkout state | never narrower | `TestSelectionExtensionsOnlyWiden` |
| Corpus evaluation | unsafe 0, precision 1, abstention exact | `TestSelectionEvaluation` |
| CLI with a real clean then dirty checkout | narrow without the limitation, then full | `TestAffectedSelectionInspectsCheckout` |

## Rollout, rollback, and compatibility

Additive to ETS-V0 behind the same `--provider` opt-in. A run without `--provider` is unchanged.
With it, a record whose chains are at most one hop and whose checkouts are clean gets the same
state and rows, minus the `checkout-worktree-not-inspected` limitation. Rollback restores `widen`
and `limitations` in `selection.go`, removes the `CheckoutStatus` field and its caller, the new
fixtures and tests, this document, and decision 0315, and restores the two ETS-V0 non-goals.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `ETS-V1-001`, `ETS-V1-004` | `walk`, `adjacency`, `reach` in `internal/extevidence/selection.go` | `TestSelectionConformance` |
| `ETS-V1-002`, `ETS-V1-003` | `truncate` in `internal/extevidence/selection.go` | `TestSelectionConformance`, `TestSelectionWalkBudget` |
| `ETS-V1-005`, `ETS-V1-006` | `inspect`, `checkoutCode` in `internal/extevidence/selection.go`; `compileAffected` in `cmd/corvint/affected.go` | `TestSelectionConformance`, `TestAffectedSelectionInspectsCheckout` |
| `ETS-V1-007` | `inspected` in `internal/extevidence/selection.go` | `TestAffectedSelectionInspectsCheckout` |
| `ETS-V1-008` | `internal/extevidence/testdata/conformance-selection/` | `TestSelectionExtensionsOnlyWiden`, `TestSelectionEvaluation` |
| `ETS-V1-009` | `result` in `internal/extevidence/selection.go` | `TestSelectionPrivate`, `TestSelectionDeterministic` |

## Unresolved decisions and promotion or kill criteria

- Promotion follows ETS-V0: one independent adopter's record and a measured corpus.
- Raise the depth or entity bound only with a measured adopter record that cuts at the current
  bound. A cut is fail-closed, so the cost of a low bound is lost narrowing, not unsafe advice.
