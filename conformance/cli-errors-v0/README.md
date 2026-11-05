# CLI error and help conformance

`contract_test.go` builds every shipped `cmd/` binary and launches every public node with
both help spellings, checking offline usage, required inputs, examples, catalog parity and
shared version lines (CAP-V0-7700). It also checks protocol wire preservation and the opt-in
message/code lint.

`native_contract_test.go` retains the existing per-binary refusal oracle (ACX-V0-859/904,
TCL-V0-859). Core keeps its argparse-compatible refusal; Tasks keeps its native stdout
result and exit 1; companions keep their owned JSON or text refusal and status. Newly
cataloged single-mode binaries keep their positional parsers and package-owned checks.
ACX-V0-7707 preserves these contracts, including exact JSON shapes and plain diagnostics.

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -run Test ./conformance/cli-errors-v0
```
