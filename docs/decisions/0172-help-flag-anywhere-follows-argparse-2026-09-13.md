# Decision 0172 — `COMMAND ... --help` follows the oracle's argparse scan

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The release claim audit (commit c70cd1a8) found `corvint COMMAND --help` printed help only when
`--help` was the sole argument after the command: `query --task x --help`, `impact PATH --help`,
`index --if-stale --help` and similar exited 2 with `unrecognized arguments: --help`
(`parseHelpInvocation`, `cmd/corvint/help.go`). The audit lane documented that restriction in
`README.md` rather than choose the contract.

Governing contract. `GPK-V0-001` makes help and validation precedence compatibility surface frozen
at the pinned migration base. The retired Python oracle (`src/corvint_cli.py` at `54735d98^`) built
every command and nested action with `argparse` `add_parser` and the default `add_help=True`, so an
`-h/--help` token anywhere before `--` printed that parser's help and exited 0, unless an earlier
left-to-right error fired first. No `cli-parity-v0` fixture pins a help case, and `GPK-V0-059`
only requires `help VERB` and `VERB --help` to agree. The sole-argument restriction is therefore a
Go divergence from the frozen surface, not a contract.

The call: add `GPK-V0-062`. `--help` anywhere after a public command and before `--` prints the
byte-identical `COMMAND --help` output, exit 0, without reading stdin or inspecting the repository.
It is not help when it is the pending value of an option that takes one (argparse refuses that
with "expected one argument" before help), when it follows `--`, or when a nested `cem`, `ocm`, or
`harness` choice before it is invalid. Positional and value type errors do not precede help in Go:
help never validates repository input (`GPK-V0-059`). `-h` and argparse's option-prefix
abbreviations stay unsupported, as before; they are recorded as a follow-up, not added here.

Consequences: `parseHelpInvocation` scans left to right with a boolean-option table in
`cmd/corvint/help.go`; `TestHelpFlagAnywhereFollowsArgparse` pins both halves; `README.md` states
the new behavior in place of the audit restriction.

Rollback: revert the commit. That restores the sole-argument parser, removes `GPK-V0-062` and its
traceability row, and returns the README to the audit lane's restriction sentence.
