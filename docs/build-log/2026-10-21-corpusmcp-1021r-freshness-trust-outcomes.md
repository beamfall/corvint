# Corpus MCP: additional-repository freshness reuse, trust envelope anchor states, outcome counters

Lane corpusmcp-1021r; tickets V1-2328 (GitHub #1487), V1-2329 (#1492) and V1-2330 (#1493).
Requirements DCP-V1-280..286 in `docs/specs/documentation-corpus-v1.md` are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

- **#1487.** A corpus that declares `additional_repositories` keyed its live observation on the
  primary checkout only, so every warm MCP call recomputed freshness for all repositories (about
  10 s per call in the report).
- **#1492.** The trust envelope could not say which cited span drifted. It had no `verified_at`,
  and `docs_gaps` could not be filtered by kind.
- **#1493.** The corpus MCP kept no record over time of how it answered, so no answer-rate floor
  could be set or gated.

## Decisions

- **DCP-V1-280/281.** The observation key is built per declared checkout:
  - Each key is observed in parallel and concatenated length-prefixed with repository-ID prefixes.
  - A single-repository corpus keeps the primary key unchanged.
  - Any change recomputes freshness and the receipt names the repository.
  - A failed observation computes, never reuses.
- **DCP-V1-282.** Anchor states are derived from the drift the live overlay already observes, with
  no extra Git work:
  - Only a changed input is read, inside its checkout, under fixed byte bounds. A clean path must
    hash to its live blob.
  - The pinned span digest decides `live`, `moved` or `changed`, and a deleted path is `gone`.
  - Static readers, undeclared inputs and any read past the bounds report `unknown`.
  - A drifted citation makes the receipt `stale`. `Receipt.citations` is unchanged; the envelope's
    citations are those same anchors, each with `anchor_state` added.
- **DCP-V1-283.** `verified_at` is always `null` with a limitation, because no verification is
  recorded.
- **DCP-V1-284.** The `kind` filter accepts one kind or 1 to 32 distinct kinds, on `docs_gaps` only.
  `kind_totals` is computed before paging and is zero for a kind with no gaps.
- **DCP-V1-285/286.** The outcome log is an opt-in flag on `corvint-corpus-mcp`, recorded around
  `tools/call` without touching the result:
  - Tool names outside the listed set count as `unlisted`.
  - The file is rewritten atomically.
  - A restart continues only the same artifact's log. A foreign or inconsistent log refuses startup.
  - The floor CLI exits 1 below floor minus tolerance, and also when a floored scope has no calls.

## Evidence

- **#1487 scale fixture:** primary plus 2 additional repositories, 8,001 inputs.
  - First call: load 2,599 ms, freshness `computed` 750 ms.
  - Warm calls: `freshness: reused` with `freshness_ms` from 32 to 87 ms (median about 40 ms).
  - Warm total p95: 218 ms for `docs_get` and `docs_search`.
  - A one-file change recomputes, names the repository and is stale; later calls reuse again.
- **New tests:** `TestDCPV1280…`, `TestDCPV1281…`, `TestDCPV1282PerCitationAnchorState`,
  `TestDCPV1283…`, `TestDCPV1284GapsKindFilter`, `TestDCPV1285…` and `TestDCPV1286…` each fail
  with the fix removed.
- **Two pre-existing parity tests now compare with the live reader:**
  - The `cmd/corvint-corpus-mcp` transport test compares with the live reader the CLI uses.
  - The `cmd/corvint` envelope oracle compares envelope citations without `anchor_state`.
- **Package runs that passed:**
  - Full packages: `internal/doccorpus` (759 s), `internal/mcp/corpusbridge`,
    `cmd/corvint-corpus-mcp`, `internal/corpusindex`, `internal/corpusrepublish`,
    `internal/corpusserve`, `cmd/corvint-corpus-parity`, `cmd/corvint-corpus-republish`,
    `cmd/corvint-corpus-http`, `internal/appflows`, `internal/delta`,
    `internal/doccorpus/selfcorpus`, `internal/flowcoverage`, `internal/flowdocs`,
    `internal/playwrightminimize` and `tools/heading-nav-bench`.
  - `cmd/corvint` with `-run 'Corpus|Docs|DCPV1'` only.
- **Timing flake:** `TestDCPV1195IndexRefusalDerivedOncePerArtifact` failed once on its 50 ms bound
  while the host load average was about 62. It passed 3 of 3 times on rerun.

## NOT_RUN

- Outside the `-run` subset: the rest of `cmd/corvint`.
- `corvint affected` tier 2.
- Siblings that import no changed package: `internal/mcp/bridge`, `internal/mcp/docsbridge` and
  `internal/mcp/testvaliditybridge`.

## Residuals

- **#1492 is partly open:**
  - The optional caller revision that reports `diverged` (needs a Git ancestry reader).
  - Per-field `origin`.
  - A `kind` filter on the CLI and HTTP readers.
  - `verified_at` on the refusal envelopes of `cmd/corvint-corpus-mcp` and `internal/corpusserve`.
- **#1487 remaining cost:** reuse still costs about 35 to 45 ms of observation and about 70 ms of
  pin verification per call.

## Rollback

Revert the lane's commits. Nothing is migrated:

- the artifact format is unchanged;
- receipt fields are additive;
- the outcome log is off by default.
