# 2026-10-20 V1-2008: capability catalog and drift gate

Tickets: V1-2008, absorbing V1-0130, V1-1271 and V1-1821. Spec:
[`capability-catalog-v0.md`](../specs/capability-catalog-v0.md) (`CAP-V0-001..006`, accepted under
the owner's standing auto-accept of lane requirements, 2026-10-11; experimental).

## Decision

- `docs/CAPABILITIES.md` is the one agent-facing catalog. It has a row for each top-level `corvint`
  command in the root help (50), each `cmd/*` binary (55) and each `corvint.*` MCP tool (36). Each
  row gives the question it answers, when to use it, a minimal invocation, the output, its maturity,
  what it refuses or abstains on, and the owning spec. Invocations were checked against each
  command's `--help` text, not written from memory.
- The drift gate is `tools/capabilities-check`. `make capabilities-check` runs it, and its Go tests
  (`TestCAPV0002*`, `TestCAPV0003*`, `TestCAPV0004*`) run under `go test ./...`.
  - It reads `cmd/corvint/help.go`, `cmd/` and `internal/mcp` plus `cmd/corvint-*mcp`.
  - It fails on a missing, stale or duplicate row, on a command whose maturity disagrees with the
    root help's Core list, and on a broken relative link.
  - It only reads rows inside `<!-- capability-gate:... -->` regions, so prose and the "Start here"
    table are not gated.
  - It is not added to `GATE_STEPS`, because `go test ./...` already runs the tests.
- Routing:
  - `AGENTS.md` and the "What capability exists?" row of `docs/AGENT-ROUTES.md`
  - `docs/README.md` and the root `README.md`
  - all shipped skills: Claude Code (6), Codex, Gemini CLI (skill and `GEMINI.md`), OpenCode, and
    the Apache-2.0 OpenSpec skill under `protocol/integrations`
- Host packages whose content changed are version-bumped as the AHI-020 check requires:
  - claude-code 0.3.1 to 0.3.2
  - codex 0.2.4 to 0.2.5
  - gemini-cli 0.2.8 to 0.2.9
  - opencode 0.7.14 to 0.7.15
  The historical `hostVersionEvidenceAdapterVersion` 0.3.1 and qualification text are kept.

## Owner scope addition: README "What Corvint can do"

The owner asked for the root README to present the capabilities, not just link to them. A "What
Corvint can do" section near the top lists 13 headline capabilities, each with a one-line benefit,
a minimal command and a maturity label:

- core: governing context, context compilation, impact, affected tests, prove, CEM, OCM and review
  order
- experimental: docs corpus, MCP, flows, Tasks and gated learning
- language support: labelled as a core verb whose individual language providers are experimental

The gate is extended (`CAP-V0-004`). Every README row must resolve to a catalog row by longest
command prefix, binary or tool name, and must not claim a stronger maturity than that row.
`TestCAPV0004ReadmeRowsMapToCatalog` proves that an unmapped row (`corvint teleport --now`) and an
overstated row (`harness event` as core, `corvint-mcp` as implemented) both fail.

## Absorbed tickets

- **V1-0130:** already resolved on the base. `docs/PORTFOLIO-0.6.md` lines 210-211 carry the
  `flows` and `skill-export` rows. No change was needed; the catalog now also lists both.
- **V1-1271:** the Codex skill said impact was Go-only. Its path and range modes now say they admit
  Ruby, JS and TS under the experimental `non-go-syntax-v0` profile, and other suffixes are
  `unsupported-impact-path-suffix`. The Gemini evidence skill got the same correction.
- **V1-1821:** the `corvint-js-test-provider` help text said an existing test-evidence `.gitignore`
  is never overwritten. It now matches LPCV-V0-084 and `internal/testevidence/gitignore.go`: a
  non-empty file is kept, an empty one is repaired, and a non-regular entry is refused.
  `script/repin-use-case-receipts.sh --check` reported all 18 receipts current, because the help
  text is not a pinned subject, so no repin was needed.

## Self-check: five random rows

The five rows were chosen with Python `random.seed(2008)` over every gated row. Each minimal
invocation was run with lane-built binaries.

| Row | Invocation | Observed | Catalog result |
|---|---|---|---|
| `corvint step` | `step snapshot --declaration /nonexistent --host /nonexistent` | exit 2, `STEP_UNSUPPORTED` | matches "exit 2 on unavailable" |
| `corvint.map_plan` | `corvint-corpus-mcp --root R --artifact corpus.json` without `--map`, then `tools/list` | 17 `corvint.docs_*` tools listed, no `map_plan` | matches. Needing `--protocol-version 2025-11-25` for an `initialize`-first client was undocumented; a protocol note was added to the MCP section |
| `corvint-release-install` | `--help` | usage naming `-candidate` and `-store`, exit 2 | matches |
| `corvint frontier` | missing `--cem`/`--ocm`/`--artifacts` | exit 2, `invalid-frontier-input` | refusal was missing from the row; added |
| `corvint kernel` | `kernel --requirements CAP-V0-001` in an unindexed checkout | exit 2, `unsupported-kernel-authority` | refusal was missing from the row; added with "run `corvint index` first" |

Three of five rows were incomplete on refusal or setup detail, and all three were fixed. The gate
checks names, maturity and links, not refusal prose, so refusal text is still a human-review surface.

## Verification

- `make capabilities-check`: `ok (50 commands, 55 binaries, 36 MCP tools)`.
- The five `TestCAPV0*` tests pass. Synthetic omission, stale, duplicate, new-binary, maturity,
  broken-link and README cases are the fail-without-fix proof.
- These test packages pass:
  - `internal/specindex`
  - `protocol/integrations`
  - `cmd/corvint-js-test-provider`
  - the host adapter tests in `cmd/corvint` (`TestHostAdapter*`, `TestDogfoodEvent*`,
    `TestCodexHostAdapter*`, `TestClaudeNativeDogfoodLifecycle`, `TestAHI001*`)
- The first dogfood-change at base `c27d54e9` was all NOT_PRODUCED or NOT_OBSERVED:
  - prechange query and impact: `agent-receipt-absent`
  - cem-prepare: `git-diff-failed` on the uncommitted lane
  - cem-cite: `cem-map-not-produced`
  - ocm: `missing-intent-scope`
  - cem-status: `map-unavailable`
  - local outcome: `outcome-input-not-provided`

## Residuals and deviations

- **Pi:** routing is deferred, because the Pi package version is pinned in
  `cmd/corvint/local_completion_event.go`, so a skill edit would need a pinned-tuple change.
- **Not gated:**
  - subcommands, and the maturity of binaries and tools (the gate covers names only)
  - refusal and invocation prose
- **Small leftovers:**
  - the flows-profile code comment says "four" tools where five are served
  - only two of the four `corvint-mcp` profiles have goldens
  - the speckit command texts are not routed
  - the OpenSpec skill metadata version stays 0.1.0

## Codex review round (PR #1283)

The single Codex round found six issues. All six are fixed:

1. The MCP tool-name pattern now admits letters, digits, `_`, `.` and `-`.
   `TestCAPV0002ToolNamesWithDigitsAndHyphens` fails against the old `[a-z_.]+` pattern
   (it misses `corvint.docs_trace_v2`) and passes with the new one.
2. Commands also come from the `topLevelCommands` dispatch list in `cmd/corvint/main.go`.
   A dispatched verb with no root help line is reported as `dispatch-undocumented` (and as
   `missing` when no row exists). A help verb that is not dispatched is reported as
   `dispatch-absent`. `TestCAPV0002DispatchDriftFails` covers both.
3. Three catalog invocations are now runnable as written:
   - `corvint-dashboard-snapshot snapshot`
   - `corvint-release-gate` with its required `--commit`, `--git`, `--git-sha256` and
     `--policy-sha256` pins
   - `corvint-postmerge-metrics report` with all four flags
4. The Gemini and Codex skills match the catalog's impact admission. Suffixes the index admits,
   such as `.py` and `.md`, get positional evidence with missing-rule uncertainty. Only
   suffixes the index does not admit refuse. The Codex adapter moves to 0.2.6 and the Gemini
   adapter to 0.2.10.
5. CAP-V0-005 now cites `TestCAPV0005RoutingPointersNameTheCatalog`. It checks the AGENTS.md,
   README, docs README and AGENT-ROUTES pointers, and every shipped `SKILL.md` and `GEMINI.md`.
6. CAP-V0-006 now cites two tests:
   - `TestCAPV0006CheckIsReadOnly` runs the whole check over a synthetic tree. It asserts that
     every path, mode, mtime and content digest is unchanged.
   - `TestCAPV0006CheckImportsNoProcessOrNetwork` asserts that the check imports no package
     for processes or sockets.
