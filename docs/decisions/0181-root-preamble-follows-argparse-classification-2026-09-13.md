# Decision 0181 — the `--root PATH` preamble follows the oracle's argparse classification

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/bugs.md` (2026-09-13) found that decision 0173 applied `argparseOptionLike` to
every per-command option value but left out the `[--root PATH]` preamble that `cmd/corvint`
scans ahead of each command verb. That preamble was copied into roughly thirty places (the
per-command `parse*Invocation` detectors, `calibrateRoot`, `readsRoot`,
`commandPositionAfterRoots`, `parseHelpInvocation`, `parseCEMInvocation`, `queryCommandAfterRoots`,
and `parse()`), and every copy consumed the token after a bare `--root` unconditionally, so
`--root --task calibrate` resolved the root `--task` instead of refusing.

Governing contract. `GPK-V0-001` freezes validation precedence at the pinned migration base. The
retired oracle (`src/corvint_cli.py` at `54735d98^`) declares `--root` on its top-level `argparse`
parser, so `corvint --root --task ...` never binds `--task` as the root: argparse's
`_parse_optional` classifies it as option-like and refuses `--root` for lacking its one argument
before any command or `--help` is considered. This is the same classification `GPK-V0-064` already
applies to per-command option values.

The call:

(a) Amend `GPK-V0-064` to cover the root preamble; no new requirement ID. One shared predicate,
`rootPreambleValue` (`cmd/corvint/help.go`), decides whether the token after a bare `--root` may
be consumed: it must exist and must not be `argparseOptionLike`. Every preamble scan that consumed
the next token now calls it, a one-line guard change per copy (the per-command `parse*Invocation`
detectors, `calibrateRoot`, `readsRoot`, `commandPositionAfterRoots`, the `test-validity` scan,
`parseHelpInvocation`, `parseCEMInvocation`, `queryCommandAfterRoots`, and `parse()`). A bare
`--root` whose next token is missing or option-like ends the scan at that `--root`, so no
per-command detector sees its verb and dispatch falls through to `parse()`, which refuses. Loops
bounded by a position that a scan above already computed are unchanged, since that position covers
only well-formed pairs. Consolidating the loops into one scanner was considered and rejected: the
copies are not identical (they return different tuples, and `parse()` resolves every root in
order), and removing them shifted 85 pinned line citations across six specs.

(b) The refusal is the existing `parse()` message `missing value for --root`, exit 2, the same Go
error path already used for a trailing bare `--root`. No new message is introduced, as
`GPK-V0-064` requires. This decision does not change that message to argparse's
`argument --root: expected one argument` wording.

(c) Unchanged: a non-option-like value (`-`, a negative number, a token containing a space, any
ordinary path), the inline `--root=VALUE` form (including `--root=-x`), and `--`. A literal `--`
after `--root` is still consumed as the root value, as before. This lane was told not to change
`--` handling, and the oracle's treatment of `--` as an option argument depends on the Python
version.

Consequences: `TestRootPreambleFollowsArgparseOptionLikeClassification` (`cmd/corvint/help_test.go`)
runs `--root --task COMMAND` and `--root -h COMMAND` for every entry of `topLevelCommands` through
`run`, and pins the value/inline/`--` table through `calibrateRoot`. It fails against a candidate
that consumes an option-like token as the root: a mutant `rootPreambleValue` that ignores
classification produced 71 failure lines. `--root --help` is now a `missing value for --root` refusal rather than a root named
`--help`, which matches argparse's left-to-right order.

Rollback: revert the commit. That restores unconditional next-token consumption in each preamble
copy and the pre-amendment `GPK-V0-064` text and traceability row, and re-files
the bugs.md entry.
