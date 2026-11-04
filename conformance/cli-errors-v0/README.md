# CLI error and help conformance

`contract_test.go` builds every shipped `cmd/` binary and derives its inventory from the
committed command trees, with no per-binary exemptions or hand-maintained rows. It launches
every public node with both help spellings, checks offline usage/required inputs/examples,
pins catalog parity, and requires one coded explanatory stderr refusal with exit 2 for
unknown verbs, unknown flags and unknown help topics. It also exercises native Core/Tasks
and plain companion failures, shared version lines, typo suggestions and message/code lint.

CAP-V0-7700 and ACX-V0-7707 supersede the earlier per-binary refusal exceptions. Structured
native error evidence is retained; successful JSON help and analyzer/MCP protocols keep their
owned shapes. Builds disable VCS stamping so the existing confined source-read scope is enough.

```sh
script/heavy.sh go test -count=1 -run '^(TestCAPV0(7700|7707)|TestCLIErrorContracts_ACXV0859)' ./conformance/cli-errors-v0
```
