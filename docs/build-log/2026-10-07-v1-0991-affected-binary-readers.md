# 2026-10-07: V1-0991 `corvint affected` selects the packages that run a command's binary

## Intent

Ticket V1-0991: `corvint affected` missed test packages that build or exec the corvint binary at
runtime (`conformance/host-lifecycle-v1`, `conformance/release-artifact-v0`). No import edge
reaches them and the plan stated no uncertainty for them. Requirement AFP-V0-037 (proposed) in
`docs/specs/affected-plan-v0.md`.

## Reproduction (before)

Base `0b5096ca`, a one-line scratch commit to `cmd/corvint/help.go`, `corvint --root . affected
--base 0b5096ca`: scope `UNKNOWN` (unknowns `go:build-constraint-variants`,
`go:nested-module-frontier` only), 48 units selected. `conformance/host-lifecycle-v1` was excluded
with `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`; it runs the binary it is handed through `--corvint`.
`conformance/release-artifact-v0` and `internal/archiveverify` were selected only by a path literal
(`PATH_LITERAL_READER`), so a change in a package `cmd/corvint` imports did not select them.

## Change

Option (a) of the ticket, a declared edge, combined with deterministic literal detection that
reuses the existing path tokens (AFP-V0-021); no new read of source.

1. `affected.Unit.Execs` names the command units whose built binary the unit runs. It is part of
   the graph digest projection only when non-empty, so graphs without it keep their digest.
2. The Go plugin records an edge when a path token of the unit names a `package main` directory
   exactly (`./cmd/corvint`, `example/m/cmd/corvint`, `../cmd/corvint`), or when the project-owned
   `.corvint/test-binary-execs.json` declares it. The declaration in this repository declares
   `conformance/host-lifecycle-v1 -> cmd/corvint`. A present but invalid declaration keeps no
   declared edge and raises the module-level frontier `go:test-binary-execs-invalid`, which
   `tools/gate-affected-select` treats as module-level (full run).
3. `Select` marks every command that the dependency closure or the enclosing-package rule reaches
   as built, then selects its exec consumers with witness `BINARY_EXEC`, one edge past the command,
   without traversing their importers.

## After

| Scratch change | Before: selected / host-lifecycle-v1 | After: selected / host-lifecycle-v1 | `BINARY_EXEC` |
| --- | --- | --- | --- |
| `cmd/corvint/help.go` | 48 / excluded | 49 / selected | 21 |
| `internal/diagnostic/diagnostic.go` | 99 / excluded (and `internal/archiveverify`) | 102 / selected | 19 |
| `tools/gate-ledger/main.go` (unrelated) | 55 / excluded | 55 / excluded (identical plan) | 0 |

The `help.go` case also turns `release-artifact-v0`, `internal/archiveverify`,
`conformance/cli-parity-v0`, `tools/corvint-pr-tests` and `internal/tasks` into `BINARY_EXEC`
selections. The plan stays `UNKNOWN` for the same two pre-existing reasons.

## Timing

The host had a load average of about 54 on 12 CPUs throughout, so every number is noisy.

- End-to-end `corvint affected --base` (five runs each, wall clock) took 8.0-12.8 s before and
  5.8-10.4 s after on `help.go`; 5.4-8.5 s before and 5.5-6.1 s after on `diagnostic.go`. Nearly
  all of that time is spent in the graph build and git reads, which this change does not touch.
- `TestIncrementalSelectionMeetsTheLiveBudget` (372 units, 160 selections) was run interleaved
  three times on a base export and on this tree. Base p50 was 35-97 ms and p95 169-219 ms. After,
  p50 was 81-94 ms and p95 189-210 ms. The change adds one map-backed pass over the reached set.
- That test's 100 ms p95 budget fails at base as well under this load, so it is retained as an
  environment failure, not a regression.

## Limits

- A literal that names a command directory as data (for example `internal/mcp/docsbridge`) also
  becomes an edge and over-selects.
- A command at the module root (`.`) is never a literal target.
- An exec consumer's importers are not selected through it.
- A dirty declaration file is an unowned path and widens the plan to `UNKNOWN`.
- A `_test.go`-only change to a command still counts as reaching its build; V1-0984 owns the
  semantics of test-only changes.
- Opt-in qualification legs that take the binary from an environment variable are not declared.

## Rollback

Delete `internal/liveverify/affected/execs.go`, `internal/liveverify/affected/golang/binaryexecs.go`
and `.corvint/test-binary-execs.json`. Then remove `Unit.Execs`, its digest field, the call to
`builtCommands`/`execUsersOf` in `reach`, and the frontier entry in the gate tool.
