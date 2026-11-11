# Lane dmf: honest docs checks and usable docs MCP tools (V1-5409)

Delivers the OpenSpec change `docs/specs/openspec/changes/docs-mcp-hardening` (tasks 1 to 14) on
base `af393f1b1e20f59d536663276f30469bea688f6a`, into the new spec
`docs/specs/docs-mcp-hardening-v1.md` (DMF-V1-001 to DMF-V1-011 and DMF-V1-013; DMF-V1-012 moved to
`how-rewrite` as HOW-V1-012). The change is accepted automatically under G14 (decision 0596) and
recorded in `docs/specs/openspec/acceptance-log.tsv` (kind `change`, mode `auto`). Task 15, the
independent review, is left to the orchestrator.

## What each requirement changed

- DMF-V1-001: `flows docs` generation and `--check` read the same committed waiver file. A claim an
  unexpired waiver names that is not PROVEN renders `WAIVED until DATE (now STATE)` on the page and in
  the sidecar (`waived_until`, `mark`). The check no longer restores the committed form of a waived
  claim for the byte comparison, so a stale PROVEN page fails until regenerated. Expired waivers are
  listed in `expired_waivers`; a waivers file that does not parse names file and line.
- DMF-V1-002: `corvint-docs docs check` composes the markers, flows and citations checks and prints
  `corvint-docs-check/0`. Run on this repository at `37634f530` it exits 1 (one stale generated block
  in `docs/DOCUMENTATION-CORPUS.md:481`, no tracked flows manifest, 942 of 2368 citations failing);
  the report is kept in `.agent-evidence/docs-mcp-hardening/docs-check-corvint.json`.
- DMF-V1-003: `docs maintain --check` (read-only, no `--enable`) exits 1 naming each stale block by
  page line; `--preview` keeps exit 0.
- DMF-V1-004: the product citation check reports `missing-file` for any untracked cited path, skips
  fenced code (counted), lists unpinned citations, and gains `--json`.
- DMF-V1-005: new shared package `internal/docworktree` gives every check report a `worktree` member
  (`clean`, `dirty` with count and first paths, or `unknown` with a reason); answers stay about Git.
- DMF-V1-006: a flow-doc byte failure carries `byte_detail`: which file differs, each changed claim
  with old and new state (`UNKNOWN` without a committed sidecar), and the regenerate command.
- DMF-V1-007: `docs flows generate` records an output directory inside the root; `docs flows check`
  compares it by default and reports `output: NOT_CHECKED` with `clean: false` when it cannot.
- DMF-V1-008: each corpus docs tool has its own description and every property a description;
  `docs_draft` and `docs_consume` name each other. No tool is pruned (R42 B9).
- DMF-V1-009: `corvint.docs_recommend_tests` is removed; descriptions send test questions to
  `corvint.affected` on `corvint mcp` (R42 B2).
- DMF-V1-010: one error shape (`code`, `message`, `retryable`, `next`) on every tool error and every
  `-32602` data object; `server-busy` and `cancelled` replace `-32603`; a failed `corvint-docs`
  child keeps its last stderr line.
- DMF-V1-011: a rebuilt corpus artifact is verified with the first-call read and served without a
  restart; a failed verification is refused with its reason and the old artifact is never served. A
  live `corvint-mcp` session that rebuilt the artifact between two `docs_search` calls is kept in
  `.agent-evidence/docs-mcp-hardening/live-reload.txt`.
- DMF-V1-013: the `docs consume` stale refusal names the bound and current commit and tree and the
  re-draft command; byte binding stays strict (R42 B5).

## Decisions and differences from the proposal

The proposal was checked against integ `9ec3a29d0`; this tree differs as follows, and each choice
keeps the change's intent without widening it.

- Line numbers from the change are dropped in the new spec, which cites paths only: the
  line-citation gate requires an anchor on every citation in a non-legacy document.
- Superseded rules are amended where they live: AFU-V1-032's wire contract (waivers keep the
  committed form), AFU-V1-045 (optional byte checking), SDD-V0-633 (missing-file only under a
  tracked directory), MCPV0-952 (closed `-32602` data, now with four added members), DCP-V1-143 and
  DCX-V1-498 (restart on a changed artifact), DCS-V1-595, DCP-V1-195 and DCP-V1-310 (the removed
  tool). Existing tests that pinned the replaced behaviour were changed to the new contract in the
  same commits, each named in its commit message; none was loosened to hide output.
- "The flows manifest" of DMF-V1-002 is read as the tracked `docs flows` generation manifests. The
  `flows docs` claim pages need their intents directory and evidence files, which no committed file
  names, so `docs check` cannot discover them.
- `docs maintain --check` reads the page as committed at HEAD (DMF-V1-005 requires answers about
  Git content), while `--preview` keeps reading the worktree page.
- `flows docs maintain` still refuses `--waivers`: its receipt would have to bind the waiver file,
  which is outside this change.
- DMF-V1-009 removes the MCP tool only. The `docs corpus recommend-tests` CLI read and its
  doccorpus operation stay; removing them is not in the requirement.
- Tool descriptions live beside the corpus bridge's tool table. The `mcp-tool-table` change that
  would own a shared table has not landed.
- `docs flows check` still compares the output directory on disk, as before; its `worktree` member
  says when tracked paths there differ from HEAD.
- A missing `--output-dir` is now `NOT_CHECKED` (`missing-dir`) rather than an exit-2 refusal.
- The `tools/list` golden was re-recorded twice (descriptions, then the removed tool); both diffs
  were reviewed and change only descriptions or drop the one tool.

## Evidence

Focused tests by package, each through `script/heavy.sh` with `GOTOOLCHAIN=local`, are listed in
the lane handback with their results; `make changed-checks`, `make windows-vet`,
`make capabilities-check` and `make line-citations-check` ran at the end. `openspec validate` is
NOT_RUN: the OpenSpec CLI is not installed on the lane host.

## Rollback

Each requirement landed as its own spec and code commits and reverts on its own; reverting restores
today's behaviour as the spec's Rollback section says. DMF-V1 IDs are retired, not reused.
