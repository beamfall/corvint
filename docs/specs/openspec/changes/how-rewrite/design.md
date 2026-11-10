# Design: `how-rewrite`

## Decisions

- The change re-specifies today's behaviour; no behaviour change is proposed. This reading is an
  assumption the owner has not yet confirmed (see "Open questions" below).
- HOW-V1-010 no longer requires `how` to refuse an unknown flow-file schema. `how` does not read
  `.corvint/flows.json`; it reads `--map`, `--source` directories, the ledger and the corpus
  (`internal/appmap/site_knowledge_how.go`, integ 9ec3a29d0), so that clause could not be tested.
  The requirement now states the one-home boundary as a testable check (review docs F6).
- `--workspace FILE` and `--no-workspace` stay in HOW-V1-002. Their meaning is now read from the
  appmap help text in `internal/command/flows/flows_appmap.go` (integ 9ec3a29d0) and recorded in
  `proposal.md`.
- The corvint-alone smoke test is the `NOT_INSTALLED` case: with corvint alone, `corvint how` must exit
  2 naming corvint-docs (HOW-V1-007). It is not a successful run (review docs F6).
- MCP: docs tools live on one `corvint-docs mcp` server and `corvint mcp` keeps the testing tools, with
  one table fixing every tool name and schema (owner decision R30, 2026-11-09). HOW-V1-011 pins `how`
  to that table; the table itself belongs to change `mcp-tool-table`.
- Flow-file edits do not force a full test run; they select the tests linked to the changed flows
  (owner decision R31, 2026-11-09). `how` reads no flow file, so R31 changes nothing here beyond the
  HOW-V1-010 rewrite.

## Failure modes

- corvint-docs is not installed: `corvint how` exits 2 with `NOT_INSTALLED` and names the binary
  (CMS-V0-913).
- An older corvint-docs without `how`: it answers with an argument error (decision 0595, inferred,
  NOT_VERIFIED); this spec requires a test for it.
- A goal over 256 bytes, zero or several GOAL arguments, `--budget` with `--full`, or an unbound
  `ALIAS:` in `--source`: refusal, exit 2, nothing written.
- A map, ledger or corpus of an unknown profile: refusal, exit 2.
- Map or corpus evidence older than the sources: state STALE with `stale-evidence`, never COMPLETE.
- An invalid workspace declaration: refusal `workspace-invalid`, exit 2 (appmap help text, integ
  9ec3a29d0).
- An MCP client asks `corvint mcp` for `how`: the tool is unknown there; only `corvint-docs mcp`
  serves it (HOW-V1-011).

## Acceptance evidence

- Golden tests for HOW-V1-002 to 008 at the base commit, plus the existing decision 0595 tests.
- A smoke test with corvint installed alone, which must report `NOT_INSTALLED` (HOW-V1-007), and one
  with an older corvint-docs. (Earlier wording: "A smoke test with corvint installed alone (G5) and
  with an older corvint-docs"; corrected because alone, `how` cannot succeed.)
- One run on a fixture application map covering each state.
- The HOW-V1-010 boundary test and the HOW-V1-011 tool-table test.
- Open questions for the owner: confirm "rewrite" means re-spec with no behaviour change; confirm
  `how` keeps the `site-knowledge-how/0` profile name rather than moving to a new one.

## Open questions

- PROD-DOCS-007 promises steps "each with the recorded run that showed it". `how` reads no
  `test-run-evidence/0` today. Either amend PROD-DOCS-007 to the current output, or add a later
  requirement that cites a recorded run per step when an evidence file is given (review docs F6,
  D13). Not decided by the owner.

## Rollback

Revert the spec and test commits. The code is untouched, so behaviour is unchanged; HOW-V1 IDs are
retired, not reused, and PROD-DOCS-007 returns to having no living requirement.
