# Corpus diagnostics, builder upgrades, eval cost and ranking residuals

Date: 2026-10-24. Lane: claude/docs-corpus-gh1662 (base e78c1c610). Issues: GitHub #1662 and
#1664 to #1667, from a caller-labelled adoption on main-730c6e0. Requirements `DCP-V1-470..477`
(`docs/specs/documentation-corpus-v1.md`) are proposed and experimental. They await the
orchestrator's acceptance, because a lane cannot accept. `DCP-V1-478` (#1662) is drafted only
and awaits owner intent.

## Decisions

- **Located refusals (DCP-V1-470).** A closed JSON refusal of a named local input keeps the
  `invalid closed JSON input` prefix and adds the input, a JSON pointer (when json/v2 reports
  one) and a byte offset. The artifact decoder keeps its bare message, because the streamed and
  in-memory readers must refuse alike.
- **Builder upgrades (DCP-V1-471).** The engine now records a derivation version
  (`native-contextindex/<schema>/derivation-<N>/<go>`). A verified read admits an artifact whose
  builder differs only in an attested revision. A dirty or unrecorded revision, a changed
  derivation version, a changed schema or a changed Go version still refuse. `--verify-rederive`
  still compares bytes, builder included.
  - `TestDCPV1471DerivationVersionGuard` pins the digest of a fixture artifact with its builder
    blanked. A change to the derived bytes therefore fails CI until the version is raised.
  - Risk: Windows CI line endings could change that digest. The fixture is built in memory from
    Go string literals, so none is expected.
- **Aggregate bounds (DCP-V1-472).** Provider record shards and other inputs are charged to
  separate 128 MiB bounds. Every input is charged before the refusal, which names the class
  totals and the three largest contributors. The per-shard and artifact bounds are unchanged.
- **Eval diagnostics (DCP-V1-473).** A failing case lists its served rows. A missed case also
  reports `expected_rank` from one repeat at the maximum limit. That repeat is never served, so
  it skips the MCP frame bound; the first build without that skip spent 0.34 s CPU of a 50-case
  run escaping 256-row frames. It changes no score or latency.
- **Eval session (DCP-V1-474).** A suite opens the artifact and observes freshness once, then
  checks once after the last case. The per-case checkout stamp was the dominant cost: 1.37 s CPU
  of 2.60 s in the base profile.
- **Ranking (DCP-V1-475..477).** The work adds a phrase/identifier tier after exact matches.
  Claim terms match regular inflections through the existing postings, so the index encoding is
  unchanged. Ties break by summed idf, then shorter text, then natural ID order. The frozen
  evidence tokenizer is untouched.

## Measurements

The measurements used the synthetic 50-case fixture: a three-repository corpus with a 15 MB
artifact, and `docs corpus eval` from the CLI.
- The base binary was built at e78c1c610. The new binaries were built from this lane's
  commits, each artifact built by its own binary.
- The host was under heavy load (load average 110 to 180 on 12 cores). Wall times are therefore
  indicative only, and the binaries were run interleaved.

| Suite | Binary | Mean case latency_ms | Wall (s) |
| --- | --- | --- | --- |
| Labelled suite, 49 of 50 failing | base | 17.3 to 30.8 | 1.06 to 1.76 |
| Labelled suite, 49 of 50 failing | new | 15.7 to 21.6 | 2.15 to 2.34 |
| 50 copies of the passing case | base | 27.3 | 1.59 |
| 50 copies of the passing case | new | 14.7 to 15.7 | 1.11 to 1.25 |

- Per-case latency roughly halves.
- A passing suite is about 30 percent faster in wall time.
- A mostly failing suite is slower in wall time. Each failing case pays one ranked repeat at
  limit 256 for `expected_rank`, about 0.33 s CPU across 49 cases. The rest of that run's
  profile is the one-time index build. This diagnostic cost is bounded by one repeat per failing
  case and is outside `latency_ms`.
- `TestDCPV1404SearchRankingEvalFixture` still passes 6/6 file lookups and 6/6 claim lookups.

## Not delivered

- #1662: the caller-owned extractor profile (DCP-V1-478 is drafted only).
- #1664: the compact per-file anchor table, which is an anchor encoding change.
- #1665: a served `score` or rank group (row 3), and symbol subjects derived from indexed inputs
  (row 5).
- #1667: rebuilding from the embedded manifest.
