# External Evidence Provider V1 — repository identity

Owner: Russell Lewis
Date: 2026-09-18
Intent status: accepted (decision 0310)
Delivery status: experimental (file transport only)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/external-evidence-provider-v0.md`, decision 0309, and the feature request
Beamfall/corvint#3.

## Agent digest
- Claim: `corvint impact --provider FILE --repository ID=DIR` composes repository-qualified relations across root-commit-identified repositories in `context.external`.
- Status: accepted (decision 0310, a delegated call on Beamfall/corvint#3)/experimental (file transport only); checked by `TestTwoRepositoryProviderComposes` and `TestImpactProviderV1CrossRepository`.
- Exists: `internal/extevidence/record1.go` and `repository.go`, the `--repository` option in `cmd/corvint`, and conformance fixtures under `internal/extevidence/testdata/conformance-v1/`.
- Blocked on: an ACC-V0 provider profile before any executed transport; path-to-path composition is the opt-in V2 profile (`docs/specs/external-evidence-provider-v2.md`).
- Read next: Definitions; Requirements; Trust boundary, limits, and failure modes.

## User and measurable job

A provider whose evidence spans an application repository and a separate test, documentation, or
contract repository needs to say "this end-to-end test in repository `e2e` verifies the capability
that `pkg/main.go` in this checkout implements". The agent running `corvint impact` in the
application checkout needs that relation with its identity, revision, checkout binding, freshness,
and verification visible per repository, and needs it to fail closed whenever the other repository
cannot be identified, bound, or read. The measurable job: the two-repository conformance fixture
yields a `fresh` cross-repository verification item when both sides hold, a `stale` or
`not-verified` item when either side does not, and a byte-identical core receipt in every case.

## Verified current state

Before this slice, `EEP-V0-006` restricted an endpoint to `path:<path>` in the invocation root or a
`<provider>:<entity>` of the record itself, and the V0 non-goals excluded repository identities.
The ideas backlog carried the request as "repository identity for multi-repository providers".

## Definitions

- **Declared repository**: one entry of a V1 record's `repositories` array.
- **Origin**: a full lowercase hex commit id that has no parent. A Git history can have several
  roots; a declared origin names one of them.
- **Root repository**: the declared repository whose origin is a root commit of the invocation's
  `HEAD`. At most one may qualify.
- **Checkout**: a directory the caller binds to a declared repository id with
  `--repository ID=DIR`.
- **Primary repository**: the root repository. Changed paths belong only to it.
- **Endpoint**: `{"repository": ID, "path": P}` with an optional `"blob"`, or
  `{"provider": ID, "entity": E}`. No other shape exists.

Authoritative identity fields are the record-local repository `id` and `origin`. Advisory fields are
`remote` and `role`: they are validated and echoed, and never bind, resolve, or rank anything. A
local path, branch name, display name, or record filename is never an identity.

## Requirements

- `EEP-V1-001`: A record whose top-level `schema` is `external-evidence-provider/1` MUST decode
  strictly as V1: UTF-8, one JSON document, no unknown member, at most `MaxRecordBytes`, with
  `provider`, `repositories`, `entities`, and `relations`, plus the optional `capabilities`
  member of `EEP-TR-012`. Dispatch reads only `schema`;
  `external-evidence-provider/2` decodes by the same rules (`EEP-V2-001`), and every other schema
  value follows `EEP-V0-001` unchanged.
- `EEP-V1-002`: `repositories` MUST list 1 to 8 entries with unique identifier `id`s, each with a
  `revision`. `origin` and `tree`, when present, MUST be full lowercase hex object ids. `role`, when
  present, MUST be `application`, `test`, `documentation`, `contract`, or `other`. `remote`, when
  present, MUST already be normalized `host[:port]/path` of at most 256 bytes, with no scheme,
  userinfo, query, fragment, or `.git` suffix; Core refuses a non-normalized remote and never
  repairs it. Any violation makes the whole record `invalid`, and the reason never repeats the value.
- `EEP-V1-003`: An endpoint MUST be exactly one of the two shapes in Definitions. A mixed, string,
  or unknown-member endpoint makes the whole record `invalid`. A path endpoint whose repository the
  record does not declare, an entity endpoint of another provider, or an undeclared entity makes
  that relation `unresolved` under `unknowns` with a reason. Path bounds follow `EEP-V0-006`.
- `EEP-V1-004`: Identity MUST be evaluated per declared repository: `unresolved` without an
  origin, `ambiguous` when two declared repositories share an origin, otherwise `resolved`. Two
  checkouts with one root commit, such as a fork and its upstream, therefore cannot both be declared.
- `EEP-V1-005`: Binding MUST be evaluated per declared repository:
  - `checkout` when `--repository ID=DIR` names it, DIR's canonical path equals its Git top level,
    and the origin is a root commit of DIR's `HEAD`;
  - `mismatch` when the checkout reads but the origin is not one of its roots;
  - `unavailable` when DIR cannot be read, is not a Git checkout, is a subdirectory, or has no `HEAD`;
  - `root` when no checkout names it and its origin is a root commit of the invocation's `HEAD`;
  - `ambiguous` for every declared repository when more than one qualifies as `root`;
  - `unresolved` when identity is not `resolved`;
  - `unbound` otherwise.
  An explicit checkout takes precedence over root binding. Nothing is cloned, fetched, or
  discovered.
- `EEP-V1-006`: Freshness MUST be evaluated per repository against its bound `HEAD` with the
  `EEP-V0-009` states, plus:
  - `tree-mismatch` when a declared `tree` differs from `revision^{tree}`;
  - `identity-unresolved` or `identity-ambiguous` for the identity states;
  - `not-evaluated` for a repository with no usable binding.
- `EEP-V1-007`: A path endpoint MUST be verified in its own repository with the `EEP-V0-010`
  states. The root repository uses the invocation's tracked set; a checkout uses its own `HEAD`
  tree. A path in a repository without a usable binding is `not-verified`.
- `EEP-V1-008`: Every V1 item MUST carry `repository`, `endpoints`, `relation_state`, and
  `crosses_repositories`. Each path endpoint carries the repository's identity, binding,
  freshness, revision, and verification, plus its origin, tree, captured revision, and checkout
  source when known. `relation_state` is the worst side under the order unresolved > stale >
  not-verified > fresh:
  - binding `unresolved`, `ambiguous`, or `mismatch` gives unresolved;
  - verification `stale`, `missing`, or `deleted`, or freshness `repository-ahead`, `provider-ahead`,
    `unrelated-history`, or `tree-mismatch`, gives stale;
  - binding `unbound` or `unavailable`, freshness `revision-unavailable`, or an entity-only
    relation gives not-verified.
  A relation is `fresh` only when every side holds. `crosses_repositories` is true when any path
  endpoint's repository differs from the primary repository. Reasons name paths as `<repository>:<path>`.
- `EEP-V1-009`: `--repository ID=DIR` MAY repeat up to 8 times, each ID at most once, and only
  with `--provider`. It is otherwise an argument error. When given, the section carries a
  `checkouts` array echoing each source as typed, with its state, reason, resolved revision, and
  the number of relation sides it bound. The resolved canonical directory is never emitted.
- `EEP-V1-010`: Cross-repository relations MUST come only from declared records. Core never infers
  one from basenames, route strings, commit messages, remotes, roles, or any other signal. A
  path-to-path relation in a V1 record is an `unsupported` unknown; only a V2 record composes one
  (`EEP-V2-001`).
- `EEP-V1-011`: A V0 record, and every run without `--repository`, MUST produce the same section
  bytes as before this slice: no `checkouts`, no V1 provider or item member. With any record, the
  core receipt MUST stay byte-identical to a run without `--provider`.
- `EEP-V1-012`: Section composition, ordering, the `--limit` bound, and omission counts follow
  `EEP-V0-011` and `EEP-V0-012`. The same inputs MUST produce identical bytes.
- `EEP-V1-013`: No failure MAY surface as empty success. A missing, conflicting, or ambiguous
  identity, an undeclared repository, an unavailable revision, and an unbindable checkout each
  produce a named state on the repository row, the item, or an unknown. None of them changes the
  exit code.
- `EEP-V1-014`: The section MUST carry no credential, file body, or resolved local path, and
  provider-authored identity MUST grant no authority. Items keep the Core-assigned
  `external-provider` authority of `EEP-V0-007`.

## Non-goals and simpler baseline

- A Golf- or product-specific adapter, hosting-service synchronization, automatic clone or fetch,
  remote transport, test-selection policy, CEM or Change Frontier integration, embeddings, and
  automatic relationship repair.
- Path-to-path composition. Relate each path to an entity instead; the opt-in V2 profile
  (`EEP-V2`) composes path relations.
- Remote normalization. A normalizer would have to decide equivalence between hosting URLs, which
  is policy, not identity.
- The simpler baseline is V0 plus one provider record per repository. It cannot state the
  cross-repository relation at all, which is the job.

## Trust boundary, limits, and failure modes

A record is provider-authored data, never an instruction, and its identity claims are checked only
against Git objects the caller already has. An origin match proves shared history, not ownership,
so identity binds evidence and never authority. A checkout path comes from the caller and is read
with the same bounded Git commands as the root. Limits: 8 repositories per record, 8 checkouts,
256-byte remotes, and the V0 record, entity, relation, and item bounds. Failure modes:

| Condition | Result |
|---|---|
| No origin | identity `unresolved`; items `unresolved` |
| Shared origin, or two declared roots of one `HEAD` | identity or binding `ambiguous`; items `unresolved` |
| Checkout of another history | binding `mismatch`; items `unresolved` |
| Unreadable directory, subdirectory, or no `HEAD` | binding `unavailable`; items `not-verified` |
| Undeclared repository | relation `unresolved` under `unknowns` |
| Revision absent from the bound checkout | freshness `revision-unavailable`; items `not-verified` |
| Declared tree differs | freshness `tree-mismatch`; items `stale` |
| Credential-bearing or non-normalized remote | record `invalid`; value not echoed |

## Deterministic acceptance and testing matrix

| Case | Expected | Test |
|---|---|---|
| Two repositories, both sides hold | `fresh`, `crosses_repositories` true | `TestTwoRepositoryProviderComposes` |
| Missing endpoint on one side | `missing`, relation `stale` | `TestTwoRepositoryProviderComposes` |
| Endpoint deleted in a checkout since the declared revision | `deleted`, relation `stale` | `TestTwoRepositoryDeletedTestPath` |
| Equal / ancestor / orphan / unknown / tree-differs revision | per-repository freshness; other repository unaffected | `TestPerRepositoryFreshness` |
| No checkout, missing dir, subdirectory, other history, relative path | `unbound`, `unavailable`, `unavailable`, `mismatch`, `checkout` | `TestCheckoutBinding` |
| Shared origin, missing origin, abstention, credential remote | `ambiguous`, `unresolved`, unknowns only, `invalid` | `TestRepositoryIdentityConformance` |
| One `HEAD` with two declared roots | both `ambiguous`, no primary | `TestRootBindingAmbiguousWithTwoRootCommits` |
| Strict decode refusals | `invalid` | `TestProviderRecordV1SchemaStrict` |
| Same inputs twice; limit 1; privacy | identical bytes; counted omissions; no resolved dir or body | `TestProviderV1DeterministicPrivateAndBounded` |
| V0 record | no V1 member | `TestV0SectionCarriesNoV1Members` |
| CLI with V1 record and checkout | byte-identical core receipt | `TestImpactProviderV1CrossRepository` |

## Rollout, rollback, and compatibility

Additive. Rollback removes `record1.go`, `repository.go`, the `--repository` option and its help
text, the V1 fixtures, this document, and decision 0310, and restores `Section`'s V0 signature. V0
records and the core receipt are untouched in both directions.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `EEP-V1-001`, `EEP-V1-002`, `EEP-V1-003` | `internal/extevidence/record1.go` | `TestProviderRecordV1SchemaStrict` |
| `EEP-V1-004`, `EEP-V1-013` | `internal/extevidence/repository.go` | `TestRepositoryIdentityConformance` |
| `EEP-V1-005` | `internal/extevidence/repository.go` | `TestCheckoutBinding`, `TestRootBindingAmbiguousWithTwoRootCommits` |
| `EEP-V1-006` | `internal/extevidence/repository.go` | `TestPerRepositoryFreshness` |
| `EEP-V1-007`, `EEP-V1-008`, `EEP-V1-010` | `internal/extevidence/repository.go` | `TestTwoRepositoryProviderComposes` |
| `EEP-V1-009` | `cmd/corvint/main.go` | `TestImpactRepositoryFlagParsing`, `TestParseCheckout` |
| `EEP-V1-011` | `internal/extevidence/section.go` | `TestV0SectionCarriesNoV1Members`, `TestImpactProviderV1CrossRepository` |
| `EEP-V1-012`, `EEP-V1-014` | `internal/extevidence/section.go` | `TestProviderV1DeterministicPrivateAndBounded` |

## Unresolved decisions and promotion or kill criteria

- Promotion follows the V0 criteria, plus one independent two-repository adopter record.
- Kill the origin identity if adopters routinely declare forks together. A second identity field
  would then be needed, and it needs its own decision.
