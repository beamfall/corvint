# Decision 0205 — native verb refusals: lease codes and action order, reads toggles, witness root and names

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

A `cmd/corvint` bug hunt (2026-09-13, `docs/agent-memory/ideas.md`) filed five unconfirmed
hypotheses against `lease`, `witness`, `frontier`, `work`, and `reads`. Each was reproduced against a
built binary. None of these verbs existed in the retired oracle, so `GPK-V0-001` parity does not
decide them; each call follows the verb's own spec and the argparse behavior its messages already
imitate.

The calls:

(a) Defect. `lease acquire --holder h --ttl 1m` with no `--path`/`--ticket` exited 2 with code
`internal-error`, because `scopelease` returned a plain error and `emitError` labels every uncoded
error that way. The TTL bound, an empty holder, a malformed path, and a malformed lease id took the
same route. `scopelease` now marks these request refusals with `ErrInvalidRequest`, keeping each
message, and `runLease` reports them as `invalid-arguments`. `SCL-V0-010` is amended.

(b) Defect, narrower than filed. `witness --bogus x` already said `unrecognized witness option`
after decision 0196, but `witness --bogus` and `witness --bogus --json` still said
`missing value for --bogus`, because the value was consumed before the name was checked.
`parseWitnessOptions` now looks the name up first. Both messages already existed. `AGW-V0-001` is
amended.

(c) Defect for `witness` only. An explicit `--root` naming no Git repository printed an uncoded
`Git error` after the index build began. Twenty other native verbs refuse that root through
`resolveExplicitRoot` (`not a Git repository: PATH`, `invalid-arguments`); `witness` now does too.
Not a defect for `frontier` and `work`: `frontier` already emits the code-only
`invalid-frontier-input` that `CF-V0-024` requires, and `work` already emits the closed stdout
`ERROR/MALFORMED_INPUT` mapping its spec requires. Neither prints an uncoded Git error.

(d) Defect. `reads --limit 5 enable` enabled the marker and dropped the limit, while
`reads enable --limit 5` was refused. `--limit` belongs to the digest alone (`URE-V0-007`), so it
is now refused beside `enable` or `disable` in either order, with the existing
`unrecognized arguments: <token>` message. `URE-V0-007` is amended.

(e) Defect. `lease bogus --x` reported `unrecognized arguments: --x`. argparse consumes the action
positional before it reports an unrecognized option, so it names the invalid choice first, and the
lease verb already copies argparse's wording. `runLease` now checks the action before parsing flags,
with the existing `argument action: invalid choice` message. `SCL-V0-010` is amended.

No requirement IDs are added. No new message or error code is introduced.

Consequences: `TestLeaseCommandLabelsRequestRefusalsAsInvalidArguments`,
`TestLeaseCommandReportsAnInvalidActionBeforeItsFlags` (`cmd/corvint/lease_test.go`),
`TestRunReadsRefusesLimitBesideAToggleInEitherOrder` (`cmd/corvint/reads_test.go`),
`TestParseWitnessOptionsNamesAnUnknownOptionBeforeItsValue`, and
`TestParseWitnessInvocationRefusesARootThatIsNotARepository` (`cmd/corvint/witness_test.go`)
each failed before the change. `TestParseWitnessInvocationInterceptsAheadOfTheSharedParser` now uses
a directory with a `.git` marker instead of `/tmp`.

Rollback: revert the commit. That restores the `internal-error` label, flag-first lease refusal,
the silently dropped `reads` limit, witness's value-first option parsing and syntactic root, and the
pre-amendment spec text, and re-files the ideas.md entry.
