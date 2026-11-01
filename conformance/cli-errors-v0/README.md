# CLI error contracts v0

`contract_test.go` builds every binary that has a command tree in `internal/capcatalog/trees/`,
runs each one with the unknown verb `zz-no-such-verb`, and checks the exit code, the output stream
and the envelope against that binary's row (ACX-V0-859, TCL-V0-859). A binary without a row, or a
row without a binary, fails the test. It also checks the near-verb suggestion of `corvint` and
`corvint-tasks`.

The rows record each binary's current contract; they are not one shared envelope. Changing a
binary's refusal is a reviewed edit of its row.

```sh
GOTOOLCHAIN=local go test -count=1 ./conformance/cli-errors-v0
```
