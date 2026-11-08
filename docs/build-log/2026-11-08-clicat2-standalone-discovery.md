# Standalone MCP descriptor discovery repair

V1-4846 / CAP-V0-8800, AC2. The self-review found that a standalone
`bridge.ToolDescriptor{Name: extraTool}` with `const extraTool = "other.new_tool"`
could escape both the registry inventory and the lexical namespace scan.

Decision: check standalone descriptor declarations in command sources before
accepting machine catalog completeness. The closed name-expression allowlist is
string literals and file-resolved constants with exactly one name and one direct
string-literal initializer. Parser object resolution preserves lexical binding.
Calls, variables, computed constants, unresolved identifiers and absent names
remain unproven and reject through the existing malformed finding category.
Cross-file constants are deliberately unproven; this pass does not claim to
evaluate Go or enumerate runtime registration control flow. Registry providers
continue supplying concrete offline descriptors.

`TestCAPV08800StandaloneDescriptorDiscovery` enters through `CheckMachineCatalog`
for struct and map declarations. It checks the reported constant outside the
corvint namespace, literal names, unknown calls, variables, computed constants
and unresolved identifiers. Adding a catalog row repairs only a proven name;
unknown expressions still reject. Existing assertions and protocol contracts
remain unchanged. Rollback removes this standalone proof pass and its tests;
it would reopen AC2's completeness gap.

Focused validation: the complete `tools/capabilities-check` package passed through
`script/heavy.sh` (10.260s, exit 0), selected by `corvint affected` against
`6339f980e13e36c8ff51a593568da3d935ffae77`. The plan retains scope UNKNOWN,
76 LANGUAGE_FRONTIER unknowns and the installed binary's stale-source notice.
Independent review and broader host gates remain with the orchestrator.
