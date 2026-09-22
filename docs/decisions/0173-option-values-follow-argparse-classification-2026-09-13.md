# Decision 0173 — option values and `-h` follow the oracle's argparse classification

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/bugs.md` (2026-09-13) found that `parseQueryArgumentsForPlatform`
(`cmd/corvint/main.go:222@68f4a206`) and the sibling per-command parsers consume the next token as an
option's value unconditionally, so `query --task -x` and `query --task --help` run a query with
that literal text instead of refusing. `GPK-V0-062`/decision 0172 explicitly deferred `-h` as "a
follow-up, not added here." This decision resolves both, plus the option-prefix-abbreviation gap
`GPK-V0-062` also left open, as one owner call under `GPK-V0-001`/`GPK-V0-033`.

Governing contract. `GPK-V0-001` freezes help and validation precedence compatibility at the
pinned migration base. The retired oracle (`src/corvint_cli.py` at `54735d98^`) built every parser
with `argparse`. `argparse._parse_optional` classifies a token as option-like — never a value for
an option that takes one — when it starts with `-`, is longer than one character, is not a
negative number, and contains no space; such a token instead triggers `argument --X: expected one
argument` (exit 2). `argparse`'s `add_help=True` also registers `-h` as a full alias for `--help`,
and its prefix-matching accepts unambiguous option abbreviations (`--he` for `--help`).

The call:

(a) Add `GPK-V0-064`. Register `argparseOptionLike` (`cmd/corvint/help.go`, added by decision
0172) as the single classification helper, and use it everywhere a parser conditionally treats
`arguments[index+1]` as a value: `if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1])`
now guards that consumption in every command parser reachable from `parseQueryArgumentsForPlatform`
and its siblings across `cmd/corvint/*.go`. A refused token reuses the existing "expected one
argument" Go error path verbatim — no new message. `--opt=-x` (inline `=`) is unaffected: the value
is already bound to that token and is never re-classified. A handful of sites already carried a
weaker ad hoc `strings.HasPrefix(arguments[index+1], "--")` check; those are replaced with the
canonical helper rather than kept as a second copy.

(b) `-h` becomes a full alias for `--help` everywhere `--help` is accepted: root, every top-level
command, and nested `cem`/`ocm`/`harness` choices, with byte-identical output and exit code. This
completes the follow-up `GPK-V0-062` deferred.

(c) Option-prefix abbreviations (`--he`, `--ta`) stay refused in Go. This is registered as an
intentional, adjudicated divergence (`DR-0037`, `conformance/divergence-register.md`) rather than
resolved: accepting an abbreviation today would make adding any future option whose name shares
that prefix a silent breaking change for scripts relying on the shorter form, which argparse's own
users accept but Corvint's `GPK-V0-001` compatibility-surface goal does not require reproducing.

Non-goals. The `--root PATH` preamble-scanning idiom duplicated ahead of most per-command parsers
consumes its next token with the same unconditional pattern and is not touched by this decision;
it is filed as a follow-up in `docs/agent-memory/bugs.md` (2026-09-13, cmd/corvint) for a future
lane, since none of this decision's named test scenarios exercise it and folding it in here would
widen the diff past the reported bug.

Consequences: `TestQueryTaskValueFollowsArgparseOptionLikeClassification` (`cmd/corvint/query_test.go`)
and `TestArgparseOptionLikeClassification` / `TestHFlagAliasesHelpFlag` (`cmd/corvint/help_test.go`)
pin the new behavior; `TestCheckpointArgumentDispatchAndExclusivity`
(`cmd/corvint/prove_checkpoint_test.go`) is updated because it previously encoded the divergent
bare-flag-looking-value acceptance the fix removes. `conformance/cli-parity-v0`'s
`TestGPKV0002ManifestReplay` continues to pass, since no pinned fixture exercised the removed
divergence. `README.md`'s `--help`-anywhere sentence gains a mention of the `-h` alias.

Rollback: revert the commit. That restores unconditional next-token consumption, removes the `-h`
alias and `GPK-V0-064` plus its traceability row, and removes the `DR-0037` register entry.
