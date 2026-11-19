# Line evidence CI repair: area ownership and frozen MCP help

Ticket: V1-5697. Repair input: `644ce4611fd98374bd72ce7a7b33ae7620d6f9fb`.
Lane base: `9f4d5d89ab0648425db40e617f3d890598f9ff79`.

## Decision and compatibility

Map `internal/lineevidence` to the tests area, layer 1 (IRT-V0-001/002/006).
Its production imports include `internal/linecov`, `internal/jslinecov`,
`internal/rubylinecov` and `internal/testinventory`, all tests-layer packages. Its consumer is
`cmd/corvint`, also tests-layer. Shared ownership would introduce wrong-way imports.
No baseline edge was added. The mapping also satisfies AFP-V0-2792 without changing shard costs
or any `.github` file.

The MCP mismatch was unrelated to affected result fields: adding `line-evidence` to the embedded
CLI catalog expanded `corvint.help`'s derived command enum. Comparing each of the three committed
MCP list goldens with that catalog found exactly this added name and no removed names.
LEV-V1-012 requires a shared affected reader; it does not authorize expanding the frozen
MMR-V0-001 list. Exclude this experimental command from that enum. MCPV0-953 command-specific
help also admits only advertised names, preserving the existing invalid-arguments refusal.
The full help catalog still includes line-evidence, and CLI help remains available.

This is a restoration of the frozen contract, with no new profile, field, error or golden.
`TestMMRV0001ExperimentalCLIHelpKeepsFrozenEnum` holds the catalog/enum/refusal boundary.
Existing assertions and all committed goldens are unchanged. Rollback is to revert the repair;
that restores the unmapped package and the unapproved help-enum expansion, including their CI
failures. No generated golden replacement is an authorized rollback or fix.

## Observed evidence and limits

Whole-package tests through `script/heavy.sh`, Go 1.27.4, no `-run` filter:

- `tools/import-ratchet`: PASS, 0.952s; includes IRT-V0-005 spy controls and
  `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs`.
- `internal/mcp/bridge`: PASS, 5.843s, including the added boundary regression.
- `internal/lineevidence`: PASS, 13.120s; uses the retained JavaScript runner installation.
- `internal/mcp/cli`: FAIL, 7.592s, only the two spare-directory cases in
  `TestMCPV08300UnavailableLaunchHandshake`. All three chartered tools/list tests passed.
- `.github/cishards`: FAIL, 8.352s, only
  `TestIsolatedHelperBuildsCopyWhatTheyNeed_AFPV02726`. The chartered
  `TestLayerDriftLive_AFPV02792` passed; no `.github` patch is needed.

The MCP spare-directory fixtures require scratch space outside a repository. The mandated
`/home/russell/lanes/lev.tmp` lies below `/home/russell/lanes/.git`; the status path instead
detects that enclosing `.git` entry although Git itself reports no repository. The isolated helper
builds similarly fail obtaining VCS status.
Do not alter these assertions or disable VCS stamping to hide the environment conflict.
Linux vet of the checked packages and `make windows-vet` passed. The terminal CLI package run,
remaining checks and host-owned independent review are recorded in the lane handback; this
entry does not claim a clean full-suite result or promote the underlying LEV partial delivery.
CEM bind/check remain unavailable under the retired `docs/DOGFOOD.md` contract (decision 0594).

Raw logs stay in `/home/russell/lanes/lev-evidence/`. SHA-256 digests:

- `ci-repair-import-ratchet.log`:
  `7a1569a14671b2fc58c81cca9c267c51243b820c5e833f64b24a4510dbf6a698`
- `ci-repair-mcp-bridge.log`:
  `9d54f93581d90cf4420e38b6e3bee53e87679ed3e0060c8330137da2baccfdf7`
- `ci-repair-mcp-cli.log`:
  `cf60ed2bdafebe44993ed2f44e467efe68b65bef61100f40804470fca0b07eb4`
- `ci-repair-lineevidence.log`:
  `d7b6ab60c26abbbff2726b2687ab756ec459f575937003b22e3e054c4bef2567`
- `ci-repair-cishards.log`:
  `dae260af91a08c65498686b8f6c5154c5f9f6d1e9d336cedaac5d9a49d7e3144`
- `ci-repair-vet.log`:
  `44a063d63a2aead413122f1a24ac961af1a3b595903f809a89dce6005515baab`
- `ci-repair-windows-vet.log`:
  `7e53d6a450e36df17de2bede2e8c98a407206f67b7eba8ddb314095c70bb8440`
