# Design: `rc4-candidate`

## Decisions

- rc.4 is the M5a candidate. The plan already fixes a qualification candidate about 2026-12-08,
  rebuilt only for blockers (R2, REL-V1-002), and an RC published at M8 after the freeze. Naming
  the M5a tag `v1.0.0-rc.4` gives the qualification run published bytes. V1-0019 and the held-out
  repositories then judge the same bytes a user can install, as rc.1 to rc.3 did. A blocker
  rebuild takes `rc.5`, `rc.6` and so on, and cites its blocker ticket. The M8 RC takes the next
  free number after the freeze. Alternative considered: cut rc.4 now as an interim like rc.3. It
  was rejected because it would spend a candidate before M4 lands (H7) and before the release gates
  can pass on the single-root repository (V1-3177).
- Readiness before the tag, not after. REL-V1-002 makes the first pushed candidate tag the seed of
  the held-out draw, which is never redrawn (H3). A tag pushed too early cannot be taken back
  without changing the draw, so every readiness item is checked first. The readiness items cite
  their owning change or ticket and add no new mechanics:
  - M4 merged (H7);
  - removals done (2026-12-01);
  - flow links signed (H6);
  - held-out pools sealed (PRR-V1-004);
  - release gates passing on the candidate commit (V1-3177, V1-3055, V1-4850);
  - tag workflow build job (REL-V1-005);
  - three-target `binaries.json` (REL-V1-008);
  - core-set archives that `corvint-update` installs (V1-5473);
  - the candidate-byte linux/amd64 qualification workflow (V1-0374);
  - commit authorship on `origin/main` fixed or waived by the owner (V1-5621);
  - the docs search promotion (RCF-V1-006 to 011).
- Promotion scope. The narrowest set an agent needs to search and quote documentation:
  - build a corpus, search it, read a passage, and ask what it was built from;
  - on both surfaces, the CLI (`corvint-docs docs corpus build|search|get|info`) and MCP
    (`corvint.docs_search`, `corvint.docs_get`, `corvint.docs_info`);
  - the artifact format those read, `corvint-evidence-corpus/2`.
  Every other corpus verb and tool keeps its current label. The verbs and tool names are already
  `stable` in `docs/COMPATIBILITY.md`, so the promotion adds the schemas and the artifact format,
  each covered by a contract test (CMP-V1-002).
- Gate. PROD-DOCS-009's own acceptance evidence, made concrete and frozen before the run:
  - repositories: Corvint, Beamfall and one public repository from the pinned proof set
    (`proof-repositories`, PRR-V1);
  - queries: 25 per repository, committed with their digest before the first run, written as file
    lookups, symbol lookups and prose questions in roughly equal parts;
  - every returned passage verified against its pinned blob (the verified-reads spec);
  - the CLI and MCP records byte-equal per query;
  - the DCS-V1-404 evaluation passing twice with equal ranks;
  - `git status` unchanged.
  The gate measures correctness (verified, bounded, consistent), not usefulness. Usefulness stays
  an `EXTERNAL_DEPENDENT` gate under the solo-falsifiability rule, reported `NOT_RUN`.
- Why 25 queries and no relevance threshold. A relevance score needs labelled judgements, and a
  judge that is the owner's agent is not independent. The verified-passage rule has no threshold:
  one unverified passage fails. Ranking quality keeps its existing frozen fixture (DCS-V1-404)
  rather than gaining a new metric in this change.
- Binaries (MTT-V1-010 and V1-5473). Owner decision RCF-D1 (2026-11-18): keep `corvint-mcp`
  ("I like the MCP as its a good way to query the docs"). MTT-V1-010's removal of `corvint-mcp`
  from `binaries.json` is superseded, PROD-INT-006 is unchanged, and V1-5473 keeps all three
  archives (`corvint-docs`, `corvint-docs-mcp`, `corvint-mcp`). Settled before the rc.4 tag,
  because REL-V1-008 fixes the artifact list. The promoted docs tools are served by
  `corvint-docs mcp` either way.
- Schemas become `stable` additively. The three tools keep every documented output field's name,
  type and meaning within 1.x; result order is `preview` (non-goal). This narrows CMP-V1-005's
  "MCP input and output schemas are preview" for these three tools only. The owner accepted this
  (RCF-D2, 2026-11-18).

## Failure modes

- A returned passage that does not verify against its pinned source blocks the promotion; it is
  never sampled away or retried until it passes.
- CLI and MCP returning different records for one query blocks the promotion (PROD-DOCS-010).
- The DCS-V1-404 evaluation failing, or giving different ranks on two runs, blocks it.
- A readiness item with neither `PASS` nor a recorded owner waiver blocks the rc.4 tag.
- V1-0019 failing on rc.4 is recorded as failed in the release notes, and does not block rc.4's
  publication as a prerelease. It does block stable.
- Missing evidence is reported `NOT_RUN` or `NOT_OBSERVED`, never as a pass.

## Acceptance evidence

- The readiness record for the rc.4 commit, every row `PASS` or `WAIVED` with the owner's words.
- The qualification build-log entry: query-set digest, corpus artifact digests, per-query
  CLI/MCP record digests, verification counts (zero unverified), DCS-V1-404 output from two runs,
  and `git status` before and after.
- The promotion commit, with `make compatibility-check`, `make spec-requirements-check` and
  `make openspec-check` passing.
- The V1-0019 result in the rc.4 release notes.

## Rollback

Before rc.4 is published: revert the promotion commit; the three surfaces return to `experimental`
and `preview`, and no data migration is needed. After publication: the surfaces are `stable`, so a
removal follows the CMP-V1-004 notice rule. Reverting the readiness record or the rc.4 numbering
changes no shipped bytes.

## Owner decisions (answered 2026-11-18)

- RCF-D1: keep the `corvint-mcp` binary.
- RCF-D2: the three tools' MCP input and output schemas and the corpus format become `stable`.
- RCF-D3: rc.4 is the M5a qualification candidate.
- 2026-11-18, "Yes, add them": readiness rows for V1-0374 and V1-5621; V1-5545 retired.

## Risks

- The release gates may still be red at the planned M5a date. The readiness record then holds the
  tag, which slips M5a, as H6 already allows for flow links.
- Beamfall content may include private material. The qualification keeps only digests and counts
  for Beamfall, never passage text, in committed evidence.
- A ranking fix late in the window reruns DCS-V1-404 and the qualification (RCF-V1-010), which
  costs a day, not a redesign.
