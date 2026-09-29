# Go LSP integration through CLI and MCP — V1-0476

Date: 2026-09-29. Owner request: “get it built” after the proposed first milestone of explicit
Go semantic enrichment through CLI and one agent integration. Base:
`edafa4d152926f3537746af2c73ad52cc032cdea` (`origin/main`). Detailed extension remains
proposed/experimental; multi-language support, automatic activation, ranking changes and broad
agent-productivity promotion are excluded.

## Contract and result

`context --lsp gopls|off` overrides the legacy environment setting. The separate
`task-review-lsp` MCP profile requires `lsp: "gopls"` per call and preserves every existing
profile's descriptor and process boundary. Both routes use `internal/lspevidence` and retain
external authority, query provenance, default-off behavior, bounded gopls execution and explicit
unavailable-provider rows. The shared path compares the index observation before and after
external evidence assembly; checkout drift withholds the packet.

Owning clauses: TCP-V0-043..046 and new TCP-V0-051..053; EEP-V0-023..027; MCPV0-016,
MCPV0-024..026 and new MCPV0-029..030. Consumer setup and rollback: `docs/LSP.md`. The Go/gopls
operator environment is trusted local execution, with relative PATH entries removed and telemetry,
module downloads and toolchain switching disabled. It is not a hostile-program sandbox.

## Independent review

One native reviewer, Astra/medium, reviewed the plan and implementation (no nested delegation).
Gate A found two HIGH gaps: the provider ancestry read bypassed the MCP Git pin, and merely
serializing an index binding did not revalidate the live workspace. Both were addressed with the
bounded/pinned Git runner and `contextindex.Observe` comparisons, then the plan passed.

Implementation review found no HIGH and one MED: the qualification script's default subprocess
exception cleanup could kill the CLI before its gopls process group was retired. Repair round 1
added graceful owned-process retirement and an actual interrupted-harness test. The independent
repair review confirmed resolution with no remaining actionable findings. It inspected supplied
results without claiming an independent rerun of those tests.

## Verification observed before the final binding

- Baseline selected tests passed for `internal/lspprovider`, `internal/extevidence`,
  `internal/mcp/bridge`, `cmd/corvint-mcp` and `cmd/corvint` (live gopls installed).
- Focused regression suite passed across those packages plus `internal/lspevidence` and
  `internal/procgroup`. It covers CLI explicit off/invalid selectors, default bytes, old MCP
  selector refusal, startup executable pinning, HEAD and worktree drift through both frontends,
  actual cancellation with a descendant and private-cache removal, and bounded provider behavior.
- `go vet` passed for provider, shared attachment, all MCP packages and both commands.
- Required focused documentation checks passed after repinning unchanged source citations shifted
  by insertions. Three unrelated traceability rows remain marked planned by the existing gate.
- `script/qualify-lsp.py` passed with Go 1.27.1 darwin/arm64 and gopls v0.23.0. A compiled CLI and
  compiled MCP server produced identical packets in committed module and go.work fixtures.
  The module exposed 5 semantic edges and the workspace 6, including `a.A -> b.B` definitions and
  the `d.D -> a.A` caller witness, plus a cross-module workspace edge. Rankings and every other
  packet field matched off mode; dirty-file omission passed. SIGTERM of the qualification harness
  while its CLI owned a live fake-server descendant retired that descendant and removed its cache.

The first live sample measured module CLI off/on at 0.524/0.317 seconds and workspace at
0.077/0.390 seconds, with off/on packet bytes 3,681/12,781 and 4,868/15,506 respectively.
These are single ordered synthetic observations with warm-up effects, not performance estimates
or a savings claim. Complete raw reports, binary hashes, fixture commits and test logs are retained
under `/tmp/corvint-lsp-integration/`; the final enrollment reruns the frozen checks on the committed
candidate and retains them in the worktree's private dogfood evidence. The script now records both
candidate commit and diff digest, so a precommit sample cannot imply a clean-commit binding.

## Self-use, failures and remaining uncertainty

Pre-change query and path-impact receipts were retained against the base tree. Query was READY
but did not surface the LSP owning contracts; bounded original-source lookup supplied them. Its
four omitted results and nine withheld test-symbol candidates remain explicit. Path impact kept
all four requested sources but omitted 167 ranked results, including three MCP companion callers;
`go vet ./internal/mcp/...` covered their compilation. The selected feature routes were context,
path impact, affected-test advice and the enrolled CEM/OCM workflow. Other language servers,
ranking trials, browser/provider services and autonomous task dispatch were inapplicable.

The initial changed-code compile failed because the host disk filled. About 5 GB of regenerable
Go cache entries older than one day were reclaimed; subsequent selected checks passed. No source
or active task artifacts were deleted. A first documentation check failed on shifted citations;
original cited text was read and preserved at its new location, then the check passed.

NOT_RUN: repository-wide `make gate` under the owner's scoped-work preference; full-subset
retrieval benchmark and paired agent-productivity trial; other language servers. NOT_OBSERVED:
billed tokens/cache cost, broad usefulness, extra gold retrieval, native-host FULL support.
V1-0168's older Caddy/etcd package-metadata benchmark question remains open. Synthetic qualification
does not promote those missing outcomes. Ticket completion/landing remains separate from CEM
closure and draft publication; the installed Tasks runtime does not expose submit/gate run/complete.

Final binding surfaced two native evidence-format limits. OCM requirement anchors must occur in
named test cases, not only function comments; repair round 2 wrapped six existing assertion bodies
in requirement-named subtests. The reviewer found no assertion or execution-order changes, and the
four affected test packages passed. The native OCM reader cannot verify Python claims, so
TCP-V0-053 retains that explicit linkage gap alongside its required live-check report. The legacy
outcome recorder also refuses a regex containing `|`; its summary lists the actual vet, docs and
live commands while enrolled selected-check receipts retain the exact focused-test argv/results.
