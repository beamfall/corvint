# Corvint 1.0 scope and first documentation proof

Date: 2026-09-28. Source base: `2e281305c49badc68d1a46035ca4ff966cbaff7c`.
Tickets: V1-0461..0465. Decision 0426 records accepted owner product scope only.

The native Tasks runtime built from this base exposes submit, gate run and complete, unlike the
older PATH installation. The primary native store records the expanded v1-0 release at revision 6,
with 18 required tickets. Receipt 859 reports CONSISTENT/AGREES; authentication, historical
acceptance, liveness and runtime qualification remain NOT_OBSERVED. It is still a fixture queue
with executionCutover false; no Beamfall production authority was changed.

Independent read-only plan review found release-blocking gaps in candidate review, multi-repository
completion, legacy history/live-state reconciliation, writer fencing/recovery after native writes,
and qualification using actual Beamfall gates. V1-0462..0464 retain those obligations. The recovered
TCP review contract requires VERIFIED independence or owner adjudication; the proposed external-agent
review replacement is pending owner choice. Distinct local actor labels are not authentication.

## First documentation path

The compiled native binary produced and consumed a draft from the committed source-documentation
spec and `internal/docmaintain`. Preview left the scratch page byte-identical. Apply retained its
human prose; repeated apply reported `applied: false`, `skipped: unchanged` and preserved exact bytes.
This proves source-reference generation and bounded maintenance, not useful human documentation,
complete behavior, HDC rendering or the entire auto-documentation release criterion.

Retained local evidence is under `/tmp/corvint-1-0-orchestrator-20260928/`:

| Artifact | SHA-256 | Observation |
|---|---|---|
| `docs-draft.out` | `f8f8bc21f680d891c7870ea4846f1da8631bd990e16764d86ff7e9d3e65657b8` | 9,504-byte generated source orientation |
| `docs-consume.out` | `a1d06c10d3277e70ef519cd3c3341e06442d688d48e3ba25d5b6684098afb9eb` | Native consumer accepted actual emitted bytes |
| `docs-preview.out` | `65799f80e4d7f1f4bfe2b6f75c96f22c9e2bff1f1645f7a8b7159c76d9f5b9a2` | Non-writing preview |
| `docs-apply.out` | `45feca92709a236fa94d1e51b654deb6ec061ce002bd76033d3b0090b94f63d8` | Human prose preserved |
| `docs-repeat.out` | `4eb225f85f4f4bde6a13db6ddbf2c18a3e096412fba2b06e37f99de1924c6cf5` | Idempotent no-op |
| `auto-docs-example.md` | `12b58d26cac6346a1a8861224f0475d886c9d966e1104d7a6b276be5614b8d9e` | Actual generated developer-reference page |
| `beamfall-docs-corpus.out` | `4834ef4c08c791b8c5672d597ae6a2f8835405db11310c71dfc158c11270027e` | Corpus compiled from committed roadmap-runner documentation |
| `beamfall-docs-search.out` | `6d6aa7e97073c18cfa9fc93a0de9d2a969b002c548e163e52d9985793c9585bc` | Bounded search for independent-review content |
| `beamfall-docs-render.out` | `72ca7ebd5243a2e55f66bdca0bbf707c49ec0bab5c32e686b16ed967cfb1982a` | Revision-pinned Markdown render plan |

The first Beamfall build refused an absolute manifest path (`invalid local input path`); using
repository-relative scratch inputs passed. Both attempts remain retained. All created repository
scratch files were moved to the evidence directory after the proof. Broader utility, source-change
drift, concurrent-edit, flow-document and compiled-MCP qualification remain pending.

## Self-use and uncertainty

Used: current native query and impact before edits, Tasks native mutations/readback/receipt audit,
docs draft/consume/maintain, and corpus build/search/render. The query omitted four ranked results
and withheld one test-path candidate; impact omitted fifteen ranked results and reported reverse-import
uncertainty for documentation paths. Original packets remain in the worktree's private Git evidence.
The required change-start coordinator ran before edits and reported NOT_PRODUCED for an empty
base-to-target CEM range, missing enrolled intent scope and absent outcome. These are not successful
completion evidence. Final CEM/OCM binding and scoped validation follow the delivered source commit.

Not applicable to this source-scope amendment: retrieval tuning, mutation testing, learned weights,
hosted providers, browser execution and full release qualification. Existing failed held-out Core
results remain unchanged. No new feature or stable release has been declared delivered.
