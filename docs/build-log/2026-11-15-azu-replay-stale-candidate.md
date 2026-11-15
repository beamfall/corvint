# AZU replay: stale candidate binaries, and why safety stays NOT_PRODUCED (V1-5124)

Lane `azu1115`, cut from `origin/main` 1a02c5f39. Spec: `docs/specs/affected-zero-unknown-v0.md`
(AZU-V0-009, AZU-V0-10031).

## Blocker found on BASE

Since AFP-V0-9730 (landed 2026-11-10 in c8bb373da), `corvint affected` refuses with
`self-binary-stale` when the binary's build commit and the planned checkout's Go sources differ.
The replay always plans historical merge trees with a binary built elsewhere, so on BASE it
stopped at its second frozen merge (`39c031980`, 1806 differing Go paths) and wrote no row past
the first. The replay could not measure anything, unknowns included.

AZU-V0-10031: `readPlan` retries a refused invocation once with `--accept-stale-binary`. A binary
that does not refuse, such as an older baseline without the flag, never sees it. Any other
failure, or a second refusal, still fails the replay unaccepted. The wall time recorded is the
accepted invocation's alone. Test: `TestReadPlanAcceptsStaleCandidate_AZU_V0_10031`.

## Eight frozen merges, lane build of 1a02c5f39 plus this fix

`script/affected-zero-unknown-replay.sh --through 8e2c13aa9...` (no baseline, no safety receipts),
2m32s, rc 1 (`AZU-V0-009 NOT_ACCEPTED`):

| Merge | Unknown | Selected | Tests | Wall ms | Safety |
|---|---|---|---|---|---|
| `8e2c13aa9` | 79 | 594 | 3506 | 9896 | NOT_PRODUCED |
| `39c031980` | 7 | 401 | 1843 | 7352 | NOT_PRODUCED |
| `d436dff76` | 76 | 561 | 3337 | 9983 | NOT_PRODUCED |
| `6c33df33b` | 58 | 527 | 3249 | 9281 | NOT_PRODUCED |
| `ea770fa17` | 62 | 331 | 1909 | 6403 | NOT_PRODUCED |
| `24d8c04f9` | 3 | 259 | 1717 | 6628 | NOT_PRODUCED |
| `f13f52b16` | 77 | 276 | 1770 | 7166 | NOT_PRODUCED |
| `9fdc2f02e` | 3 | 256 | 1695 | 7717 | NOT_PRODUCED |

The largest `unknownByCategory` totals over the eight are all TypeScript frontier classes:
`dynamic-import` 85, `bare-import-undeclared` 67, `runtime-flags-unresolved` 60 and
`executable-config-unresolved` 38. They are followed by `NO_SELECTABLE_TEST` 17 and
`UNOWNED_DIRTY_PATH` 13. `go:nested-module-frontier`, `shell:foreign-callers-invalid` and
`shell:foreign-invocation-unmodelled` appear once on every merge. The per-category count of
acceptance criterion 3 was already on BASE (`TestUnknownByCategory_V1_5124`, c8bb373da).

## Why no safety producer was built

The `safety` checker accepts a receipt only when both full suites hold a test-level result for
every unit in the plan's inventory, and only with changed-line mutants. At `8e2c13aa9` the
inventory has about 890 units: 229 `driver` units (Make targets and GitHub workflow jobs, which
carry no tests and cannot run locally), 402 Go packages, 107 TypeScript, 84 shell and 30 Python
units, plus Swift, .NET, Kotlin, Rust, Ruby, Java and C-family fixture units. This host has no
Swift, .NET, Kotlin, Rust or Ruby toolchain. A producer on this host cannot write a receipt the
checker accepts. One that skips those units, or runs only the selection, would prove nothing.
Two things are owner decisions: what a full-suite result is for a unit with no tests (a `driver`
job), and where the producer runs (a CI host with every toolchain, or a narrower local inventory
under a changed AZU-V0-009). Criteria 1 and 2 stay open and the code for them is unchanged.

Rollback: revert the AZU-V0-10031 commit; the replay then needs a binary that predates AFP-V0-9730.
