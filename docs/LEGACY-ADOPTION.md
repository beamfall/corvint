# Legacy-project adoption

Status: proposed product contract. This document defines how Corvint may help a brownfield repository
recover documentation, candidate specifications, and pull-request knowledge without converting
observed implementation into invented intent.

## Decision

Corvint is an advisory, local-first evidence compiler for legacy projects. Generated prose is a view
over cited evidence, never an authority. A repository may promote an inferred specification only
through its existing review and ownership process.

Every atomic output carries:

- `class`: `PROVED`, `OBSERVED`, `INFERRED`, `CONFLICTED`, or `UNKNOWN`;
- authority and confidence;
- exact revision and source spans;
- deterministic derivation identity;
- exclusions and unsupported surfaces; and
- verification results where applicable.

Retrieval state and proposition status remain separate. A complete context packet may still produce
an `UNKNOWN` or `CONFLICTED` claim.

## Evidence classes

| Class | Corvint may say | Corvint must not imply |
|---|---|---|
| `PROVED` | A Git object/span exists; an accepted marker is present; an exact static relationship exists; a CEM is complete; a mechanical edit or drift state verifies. | The feature works, the evidence is sufficient, or the code reflects intended behavior. |
| `OBSERVED` | A pinned build, test, runtime trace, Git event, or deployment result occurred. | That an unobserved path cannot occur or that one passing execution proves complete behavior. |
| `INFERRED` | A flow, owner, purpose, side effect, or relevant test is a cited candidate. | That the candidate is authoritative or accepted. |
| `CONFLICTED` | Two bounded witnesses disagree. | Which witness is correct without a repository-owned decision. |
| `UNKNOWN` | A declared frontier was not established. | Global absence or safety. |

Reflection, dynamic dispatch, concurrency, generated or vendor code, external systems, missing build
inputs, parser failures, binaries, and unobserved paths remain explicit frontier items.

## Pipeline

```text
immutable Git snapshot
  -> zero-execution repository inventory
  -> typed, blob-addressed evidence fragments
  -> class-aware claim synthesis
  -> canonical JSON receipt
  -> deterministic Markdown view
```

The inventory detects languages, manifests, entry points, routes, jobs, configuration, schemas,
tests, specifications, decision records, and ownership files. It classifies every tracked path as
included, excluded, unsupported, or unknown and does not require a successful build.

Extraction reuses deterministic symbols, imports, specifications, CEMs, and adapter-declared edges.
Malformed or unsupported syntax creates an exclusion and unknown frontier rather than a partial
success claim. A logical graph may be compiled in memory; a graph database is not required.

Generated architecture, behavior, and flow pages partition their content into:

1. Authoritative records
2. Implemented structure
3. Verified or observed behavior
4. Inferred drafts
5. Contradictions
6. Unknown frontier

Each statement is independently cited. A changed blob invalidates only fragments derived from that
blob; a clean rebuild and an incremental rebuild must be byte-identical.

## Specification backfill

`corvint spec backfill` produces ordinary repository-native Markdown plus a canonical receipt. Every
new document starts with `status: inferred-draft`, the source revision, receipt digest, and citations.
It cannot serialize itself as accepted.

Promotion requires a normal pull request approved by repository-configured owners. Corvint records the
promotion evidence but does not become the specification authority. When stated, implemented,
verified, or observed claims disagree, Corvint emits `CONFLICTED` and preserves both witnesses.

## Pull-request knowledge

`corvint review --base SHA --head SHA` should compose, without silently widening certainty:

- the exact diff and Change Evidence Map;
- affected symbols, callers, routes, jobs, schemas, configuration, and feature flags;
- candidate side effects and explicit dynamic/unknown boundaries;
- tests, executable claims, and mandatory project gates;
- specifications, decisions, ownership, relevant history, and incidents/tickets when configured;
- contradictions and stale evidence; and
- a source-content-free reviewer summary with links into local evidence.

Git and repository ownership are the default knowledge sources. Ticket, PR, incident, and runtime
adapters are opt-in. Mutable external text retains its source, retrieval time, and revision or content
digest and never becomes automatic authority.

## Delivery ladder

### P0 — Inventory

`corvint adopt` classifies the repository and its unknown frontier without executing project code,
then performs one bounded local-history batch. `corvint adopt --continue` resumes deeper history
without repeating the fast pass. The same compiler is exposed as `corvint init` for new repositories.
It must work on incomplete and unbuildable fixtures.

### P1 — Documentation and candidate specifications

`corvint document` renders cited architecture/feature/flow views. `corvint spec backfill` emits
inferred-draft specifications. Both are read-only unless an explicit output path is supplied.

### P2 — Knowledge enrichment

Add local Git history and ownership first. Add tickets, PRs, incidents, and runtime evidence only as
optional provenance adapters over the same receipt model.

### P3 — Pull-request review

`corvint review --base SHA --head SHA` combines change evidence, bounded impact, test claims,
contradictions, ownership, and unknowns. It begins advisory; required enforcement is earned only by
the CEM outcome gates.

## Acceptance gates

| Surface | Required evidence |
|---|---|
| Inventory | Every tracked path is classified; unbuildable fixtures succeed; no behavioral assertion is emitted. |
| Documentation/spec backfill | Every atomic claim is classified and cited; inferred output cannot become accepted without repository-owned review; contradiction fixtures preserve both witnesses. |
| PR review | Mandatory project gates are preserved; matched trials show at least 20% fewer missed-evidence findings, at least 70% legitimately supported material hunks, and fewer than 5% incorrect hard failures. |
| Incremental refresh | Only changed blobs are reprocessed; clean and warm results are byte-identical; representative warm queries remain under one second. |
| Privacy | Network-denied and secret fixtures pass; no source, receipt, prompt, ticket, or runtime evidence leaves the machine by default. |

Generated-document precision below 0.70, anchored-inference precision below 0.85, or failure of the
CEM outcome gates kills or narrows the relevant synthesis. Correction rate, draft-promotion rate,
unknown resolution, critical recall, reviewer misses, false positives, latency, and output bytes are
reported separately; no aggregate score may hide a failed hard gate.

## Failure and rollback

Mixed or stale worktrees, parser ambiguity, missing history or credentials, oversized/binary/
generated/vendor sources, conflicting authority, and dynamic behavior route to an explicit unknown
or human review. They never produce a false-green result.

Rollout is inventory-only, then advisory documents, then advisory PR reports, then required CEM only
after measured gates. Rollback removes generated views, CI/comment integration, and disposable cache;
it never rewrites source, accepted specifications, or repository authority.

## Deliberate cuts

The first legacy slice does not require a graph database, hosted service, embeddings, UI, new
specification language, automatic spec acceptance, or automatic code editing. SARIF, MCP/LSP,
cross-repository knowledge, and real-time runtime twins wait for measured demand. The first build
needs only a generalized project-profile/status adapter, blob-fragment claim/drift compiler, and
local-history plus optional external-provenance seams.
