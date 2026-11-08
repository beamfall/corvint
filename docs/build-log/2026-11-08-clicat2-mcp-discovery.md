# V1-4846: complete offline MCP discovery

The clicat2 remainder charter accepts the additive `mcpTools` member of
`corvint-capability-catalog/0` under CAP-V0-8800. Each entry carries `name`,
`availability` and `summary`; `tools/list` remains authoritative for input schemas
and actual configured availability. Reusing CLI nodes for protocol methods would
misrepresent their invocation, so the catalog keeps an explicit tool inventory.
No protocol descriptor, native refusal, exit, golden or core-freeze output changes.

The live capability check observes 54 commands, 55 binaries and 44 distinct MCP
tools. `CheckMachineCatalog` compares the root entries with the independent source
inventory and refuses missing, duplicate, stale or incomplete entries. The tests
also add a descriptor outside the customary namespace, proving it cannot bypass
the check. The root catalog build and embedded snapshot carry identical entries.
`llms.txt` and `docs/CAPABILITIES.md` explain this route. All ten shipped skills
across Claude (7), Codex (1), Gemini (1) and OpenCode (1) already point to the catalog;
the existing routing test covers them, so no skill text needs changing.

The suspected P3 did not establish a native analyzer regression: the Go analyzer's
entrypoint calls its protocol handler directly, without `cliapp.Answer`. The shared
helper did nevertheless consume exact analyzer node-help argv. It now defers such
requests with untouched streams (ACP-011); the additive discovery profile continues
to provide help. Tests cover exact and unknown analyzer nodes and supported root
help. The deeper-child regression pair exercises known `docs corpus` help and an
unknown child beneath it through `corvint-cli`, for both help flags.

Self-development routes used: impact, query, affected, catalog inspection and final
CEM binding/checking. The first query invocation omitted `--task` and was refused;
the corrected query ran after edits began, so it does not establish pre-change
query chronology. The initial dogfood pass reported missing input files and absent
agent pre-change receipts. Affected selected the touched packages but retained
`scope=UNKNOWN`, 76 `LANGUAGE_FRONTIER` unknowns and `self-binary-stale`; this is not
an exhaustive-test claim. Retrieval/learning evaluations, external provider live
qualification and Tasks writes are not applicable to this discovery change or are
explicitly reserved to the orchestrator. Independent review, repository-wide gates,
sealing and ticket completion remain host responsibilities under the lane charter.

Raw evidence is outside the repository at `/home/russell/lanes/clicat2-evidence/`:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| affected.json | 198409 | 9c23cf9a5be4bd3a51b776ca8eeac6d76ded7f8551bffcd7eceae7575d808bac |
| prechange-impact.json | 10816 | 1842436c369ae36419e863beb38e80c2f103ac9d8083a6733e66f9a2b16dcf32 |
| prechange-query.json | 12773 | bc6febe8160b50b2d6099a0899de82744889bb654ceb3b89b65319ba1f4d62b2 |
| capabilities-check.log | 64 | 04733e1cc818c594f3eb0cd0ae7b3ad489b647da82341ac9068f0ec1d3e6312c |

Rollback removes the additive member and its inventory checker together, restores
the former discovery documentation, and reverts the helper guard and new tests.
Existing native compatibility and protocol contracts remain governing throughout.
