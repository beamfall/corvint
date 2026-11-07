# CLI error and help conformance

CAP-V0-9300 / ACX-V0-9301 make universal discovery and refusals explicitly additive:
`corvint-cli --binary BINARY ARGS...`. `profile_test.go` checks every binary/node through
that profile, alongside exact native analyzer frames and the existing native error table.
Direct invocation keeps all frozen bytes, streams and statuses; no golden is amended.
The twelve restored analyzers' generated-help assertions use the adapter, with unchanged
assertions and separate exact native controls. Common version discovery (CAP-V0-9302)
also uses the adapter for native version receipts that cannot be rewritten.

`contract_test.go` builds every shipped `cmd/` binary and launches the retained generated-help nodes with
both help spellings, checking offline usage, required inputs, examples and catalog parity
(CAP-V0-7700). The four folded shims are checked with both catalog spellings. Separate
process controls pin native analyzer rejection frames and the excluded shims’ native help. It also checks protocol wire preservation and the opt-in
message/code lint.

`native_contract_test.go` retains the existing per-binary refusal oracle (ACX-V0-859/904,
TCL-V0-859). Core keeps its argparse-compatible refusal; Tasks keeps its native stdout
result and exit 1; companions keep their owned JSON or text refusal and status. Newly
cataloged single-mode binaries keep their positional parsers and package-owned checks.
ACX-V0-7707 preserves these contracts, including exact JSON shapes and plain diagnostics.

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/cli-errors-v0
```
