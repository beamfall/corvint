# CLI error contracts v0

`contract_test.go` builds every binary that has a command tree in `internal/capcatalog/trees/`,
runs each one with the unknown verb `zz-no-such-verb`, and checks the exit code, the output stream
and the envelope against that binary's row (ACX-V0-859, TCL-V0-859). A binary without a row, or a
row without a binary, fails the test. Every row names a stream and a check, because each unknown-verb
refusal carries a code (ACX-V0-664). It also checks the near-verb suggestion of `corvint` and
`corvint-tasks`.

Every companion binary refuses an unknown verb with one shared envelope (ACX-V0-904): exit 2,
nothing on stdout, and one JSON line on stderr with `binary`, `code` `unknown-verb`, `error`
naming the word, `help` and `ok` false. `corvint` keeps its argparse-parity `invalid-arguments`
envelope (ACX-V0-859) and `corvint-tasks` its Tasks command result on stdout. Changing a
binary's refusal is a reviewed edit of its row.

```sh
GOTOOLCHAIN=local go test -count=1 ./conformance/cli-errors-v0
```
