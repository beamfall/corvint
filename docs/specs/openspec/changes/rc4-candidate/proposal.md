# Change proposal: 1.0.0-rc.4 and the docs search promotion

Change name: `rc4-candidate`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-DOCS-001, PROD-DOCS-009 and PROD-DOCS-010, and the candidate
clauses of PROD-INT-012 to PROD-INT-014 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/release-candidate-4-v1.md`, a new spec with prefix `RCF-V1`
(absent from `docs/specs/REQUIREMENTS.tsv` and from every change folder, checked 2026-11-18)
Requirement IDs this change adds: RCF-V1-001 to RCF-V1-012
Requirement IDs this change modifies: the `Blocked on:` gate of DCP-V1
(`documentation-corpus-v1.md`) and its companions DCS-V1 and the verified-reads spec, for the
promoted scope only; the CMP-V1-005 preview list (MCP schemas), for three tools only
Owner answers this change implements: the owner's request of 2026-11-18 ("I'd like to see Docs
corpus and MCP search out of experimental"); R2, H1, H3, H6 and H7 as carried by
`release-process`; answer 8 (docs search for agents in 1.0); R30 (one `corvint-docs mcp` server)
Intent status: proposed (written 2026-11-18; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/release-candidate-4/spec.md`. It changes the release bar, so its acceptance-log row is
`pending-owner` until the owner accepts it.

## Why

`v1.0.0-rc.3` was published on 2026-11-18. No text on `origin/main` (1795bfc0d) says what the next
candidate is, when it is cut or what it must prove. The plan names two later tags, the M5a
qualification candidate (about 2026-12-08) and the M8 release candidate, but neither has a version
number, and rc.3's release notes leave several release-path gaps open.

The owner also wants the documentation corpus and its MCP search out of `experimental`. Today every
corpus spec is `Delivery status: experimental` and says "Blocked on: owner acceptance and
independent external utility qualification". A single-owner repository cannot close an
independent-actor gate (`docs/SPEC-DRIVEN-DEVELOPMENT.md`, proposed solo-falsifiability amendment),
so as written the corpus can never leave `experimental`. PROD-DOCS-009 already names a gate the
owner can close: "recorded searches on Corvint and on one outside repository, with every returned
passage verified".

## What exists today

Read on `origin/main` 1795bfc0d, 2026-11-18:

- Tags `v1.0.0-rc.1` to `v1.0.0-rc.3`; rc.3's notes (`docs/RELEASE-NOTES.md`, "Candidate status")
  record V1-0019 (untouched-repository validation) as `NOT_RUN`, the macOS full gate as advisory
  (V1-5298), and the tag pipeline's publish job as off until `GHA-V1-040` to `GHA-V1-042`.
- Open release-path tickets: V1-3177 (`release-gates.yml` cannot pass on the single-root
  repository), V1-3055 and V1-4850 (nightly full gate red), V1-5473 (no per-binary archives for
  `corvint-docs`, `corvint-docs-mcp` and `corvint-mcp`, so `corvint-update apply --component core`
  cannot install the core set).
- MTT-V1-010 (`mcp-tool-table-v1.md`) takes `corvint-mcp` off `tools/release-binary/binaries.json`
  and waits on an owner decision, because PROD-INT-006 still names `corvint-mcp` as a product
  binary. V1-5473 asks for the archive of that same binary.
- `README.md` capability table: "Documentation corpus ... experimental".
- `docs/COMPATIBILITY.md` (generated): `corvint-docs` verbs, flags and exit codes and the MCP tool
  names are `stable`; MCP input and output schemas are `preview` by name (CMP-V1-005); the corpus
  artifact format `corvint-evidence-corpus/2` has no row, so it is `preview`.
- `documentation-corpus-search-ranking-v1.md` DCS-V1-404: a committed ranking evaluation fixture
  (`internal/mcp/corpusbridge/testdata/search-ranking-gh1597/`) that "ranking changes to
  `docs_search` run before they ship".
- PROD-DOCS-009: Delivery experimental, State PARTIAL, "Open tickets: none found".
- V1-4859 (generated developer and flow documentation, qualified on Corvint and Beamfall) is open
  and is not part of this promotion.

## What Changes

- `v1.0.0-rc.4` is the M5a qualification candidate tag. Its commit seeds the held-out draw
  (REL-V1-002). A blocker rebuild takes the next `rc` number. The M8 release candidate takes the
  number after the last qualification rebuild.
- A readiness record lists every item that must pass, or carry a recorded owner waiver, before the
  rc.4 tag is pushed.
- V1-0019 runs on rc.4's published bytes, and the result goes into the rc.4 release notes.
- The docs search read path is promoted from `experimental` to `validated`:
  - the CLI verbs `corvint-docs docs corpus build`, `search`, `get` and `info`;
  - the MCP tools `corvint.docs_search`, `corvint.docs_get` and `corvint.docs_info` on
    `corvint-docs mcp`;
  - the artifact format `corvint-evidence-corpus/2`.
- The promotion gate is a frozen, owner-run qualification on three repositories: Corvint,
  Beamfall and one pinned public repository. Every returned passage must be verified, CLI and MCP
  must agree, and the DCS-V1-404 ranking evaluation must pass. Independent external utility is
  kept as an `EXTERNAL_DEPENDENT` gate: it is reported `NOT_RUN` and is outside promotion.
- On promotion, the specs' delivery status, the README row, the plan's PROD-DOCS-009 state and the
  compatibility table (the three tools' input and output schemas and the artifact format become
  `stable`) change in one commit.
- One owner decision settles the MTT-V1-010 and V1-5473 conflict before the rc.4 tag.

## Capabilities

### New Capabilities

- `release-candidate-4`: rc.4 identity, tag readiness, published-byte validation, and the docs
  search promotion gate.

### Modified Capabilities

- `documentation-corpus` (DCP-V1, DCS-V1, verified reads): the `Blocked on:` gate for the promoted
  scope.
- `compatibility-promise` (CMP-V1-005): three MCP tools' schemas leave the preview-by-name list.

## Impact

- New files: `docs/specs/release-candidate-4-v1.md`, a frozen query set and its expected-result
  file under `testdata/`, one readiness record and one qualification build-log entry.
- Changed files: the three corpus spec headers, `README.md`, `docs/specs/PRODUCT-PLAN-1.0.md`
  (PROD-DOCS-009 state only), `tools/compatibility-table/surfaces.json` and the generated
  `docs/COMPATIBILITY.md`, `docs/RELEASE-NOTES.md`.
- No change to ranking, the corpus wire, the MCP transport or any other docs tool.

## Non-goals

- No promotion of generation, flows, claims, drafts, navigation, journeys or any `docs_*` tool
  other than the three named (V1-4859 and the flow work keep their own gates).
- No ranking-order promise: result order stays `preview`; only fields and their meaning are
  `stable`.
- No new release mechanics: the tag workflow, calendar, demotions and artifacts stay with
  `release-process`.
- No publishing by anyone but the owner (G10, G15a).
- No claim of independent validation, market usefulness or adoption from owner-run evidence.

Failure modes, acceptance evidence and rollback are in `design.md`.
