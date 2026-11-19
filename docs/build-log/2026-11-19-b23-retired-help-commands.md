# Retired commands in MCP and corpus help

For V1-5694, the owner delegated the MMR-V0-001 compatibility choice to the lane
orchestrator. Its decision under RMV-V1-008 removes `test-validity`, `test-plan`
and `runs` from the advertised command values of `corvint.help` on every profile.
Clients receive the existing `invalid-arguments` refusal for each retired name.
The default, flows and test-evidence profiles pin the descriptor bytes. Rolling
back the advertisement requires restoring the three enum members and the
corresponding profile goldens together.

The core docs shim lists the two corpus joins served by the docs app, `impact`
and `affected` (DCP-V1-013, DCP-V1-036). For RSP-V0-025 and RNC-V1-004,
`TestRNCV1004CommittedLegacyPluginFile` plans from a repository containing only
a committed legacy plugin declaration. The inventory finds no runner and the
plan retains its empty selection.
