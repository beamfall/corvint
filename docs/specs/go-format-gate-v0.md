# Go format gate V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `GFG-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8 and its Verify block, the owner ruling of
2026-09-07 that repository gate tooling is in scope for that invariant, and
`documentation-citation-gate-v0.md` as the shape a gate spec takes here.

## Agent digest
- Claim: Every Go file this repository tracks is gofmt-clean under the pinned toolchain's gofmt, and `make gate` fails naming any file that is not.
- Status: accepted (owner instruction 2026-09-07) / implemented
- Exists: `script/check-go-format.sh`, `script/check-go-format_test.sh`, `make` targets `go-format-check` and `go-format-test`, both in `gate`.
- Blocked on: nothing. Three sibling gate scripts outside `make gate` remain unspecified (`../agent-memory/fixes.md`).
- Read next: Requirements; Non-goals; Trust boundary, limits, and failure modes.

## Human intent and scope

`make gate` ran `go test` and `go vet` and nothing that reads formatting. `go vet` is a correctness
analyser and says nothing about layout, so an unformatted Go file could land and stay landed. One
did: `internal/contextindex/blob_shards_open_test.go` had a goroutine literal on a single line that
gofmt splits across four, and it went unnoticed until an unrelated change happened to run
`gofmt -l`. Nothing in the repository would ever have reported it.

Affected user: anyone reading or diffing Go in this repository. Measurable job: for one working
tree, refuse any tracked Go file whose bytes differ from the pinned toolchain's gofmt output, and
name every such file in one run.

Scope is deliberately the *tracked* set rather than a directory walk. `gofmt -l .` from the
repository root descends into `.corvint-benchmark-cache/` and `extensions/vscode/node_modules/`, which
between them hold thousands of Go files this repository neither owns nor formats — the first run of
such a walk returned scratch-tree paths before it reached a single owned file. The tracked set is
also the correct semantics: the gate governs what gets committed.

## Verified current state

At `9bd64b8` the repository tracks 1,173 Go files across three modules — the root,
`interop/cem01-go`, and `benchmarks/snapshot-reader`. Exactly one,
`internal/contextindex/blob_shards_open_test.go`, was not gofmt-clean. It is formatted in the same
change that adds this gate, so the gate is green from the commit that introduces it.

## Requirements

- `GFG-V0-001`: The gate MUST check every Go file in the repository's tracked set, obtained from
  `git ls-files`, and MUST NOT walk directories. An untracked Go file MUST NOT fail the gate,
  because scratch and vendored trees are outside what this repository formats. A failed
  `git ls-files` enumeration MUST fail the gate; it is never an empty, clean tracked set.
- `GFG-V0-002`: The tracked set spans every Go module in the repository. The gate MUST NOT carry a
  list of modules or directories, so that a module added later is covered without editing the gate.
- `GFG-V0-003`: The formatter MUST be the `gofmt` of the toolchain `go env GOROOT` names under
  `GOTOOLCHAIN=local`, never whichever `gofmt` appears first on `PATH`. gofmt's output changes
  between Go releases, so a gate whose formatter depends on the caller's environment decides
  nothing. When that gofmt is absent the gate MUST fail, naming the path it expected.
- `GFG-V0-004`: A file fails when its bytes differ from that gofmt's output for it. The gate MUST
  report every failing file in one run, not the first, so one run enumerates the work.
- `GFG-V0-005`: The failure MUST name each file on its own line and MUST print the exact command
  that repairs it, including the resolved gofmt path.
- `GFG-V0-006`: The gate MUST be read-only. It MUST NOT rewrite, stage, or otherwise modify any
  file on any path, including its failure path; repair is the author's explicit act.
- `GFG-V0-007`: A run with nothing to report MUST print nothing and exit zero, and any failure MUST
  exit non-zero so that `make gate` fails with it. The gate MUST be a member of the `gate` target,
  as MUST its own fixture test.

## Non-goals and simpler baseline

The simpler baseline is what existed before: nothing, with formatting left to each author's editor.
That is what allowed the one unformatted file to persist.

This gate does not format anything, does not run `gofmt -s` or any simplification pass, and takes no
position on import grouping beyond what gofmt itself does. It does not check non-Go files. It does
not check untracked files, generated output, or any Go inside `.corvint-benchmark-cache/`,
`extensions/`, or another module vendored under this tree. It is not a lint gate: `go vet` remains
the separate, unchanged correctness check.

## Trust boundary, limits, and failure modes

The gate reads tracked working-tree files and spawns `go env` and `gofmt`, both from the pinned
toolchain. It writes nothing.

| Failure | Behavior |
|---|---|
| A tracked Go file is not gofmt-clean | fail, naming every such file and the repair command (GFG-V0-004, GFG-V0-005) |
| An untracked Go file is not gofmt-clean | pass; out of scope by GFG-V0-001 |
| gofmt cannot parse a tracked file | fail, printing gofmt's own message; a file that does not parse is not a formatting question |
| A tracked path is absent from the worktree | fail through gofmt's read error; the deletion should be staged |
| The pinned toolchain is not installed | fail, naming the gofmt path expected (GFG-V0-003) |
| Nothing to report | silent, exit zero (GFG-V0-007) |

The gate reads the working tree, not a revision, so it judges what an author currently has rather
than what is committed. That matches `make gate`'s other checks and is what makes it useful before
a commit rather than after one.

## Acceptance criteria and testing matrix

`script/check-go-format_test.sh` builds a fixture repository and covers the scope boundary; the
corpus itself is covered by `make go-format-check`.

| Requirement | Evidence |
|---|---|
| GFG-V0-001 | test case 3, an unformatted file outside the tracked set passing; test case 5, a failing `git ls-files` failing the gate |
| GFG-V0-002 | `make go-format-check` over 1,173 tracked files across three modules at `9bd64b8` |
| GFG-V0-003 | the resolved `GOROOT/bin/gofmt` printed in the case 2 failure text |
| GFG-V0-004, GFG-V0-005 | test case 2, the failure naming the unformatted file and not the clean one |
| GFG-V0-006 | the script performs no write; case 1 and case 3 leave the fixture unchanged |
| GFG-V0-007 | test cases 1 and 3, a passing run printing nothing; `gate` membership |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GFG-V0-001, GFG-V0-002 | the `git ls-files -z -- '*.go'` pipeline in `script/check-go-format.sh` | case 3 |
| GFG-V0-003 | the `go env GOROOT` resolution and its `-x` guard | case 2 failure text |
| GFG-V0-004, GFG-V0-005 | the `gofmt -l` capture and its per-line report | case 2 |
| GFG-V0-006 | absence: no write, no `-w`, no staging on any path | inspection; cases 1 and 3 |
| GFG-V0-007 | the exit codes, and `go-format-check`/`go-format-test` in the `gate` target | `make gate` membership |

## Rollout, rollback, and drift

The gate is green from the commit that introduces it, because that commit also formats the one file
that failed. Rollback is removing the two `make` targets and the two scripts; no Go source has to be
un-formatted, since gofmt output is what every editor already produces.

Drift rule: the pinned Go toolchain and this gate move together. Raising `go-version` can change
gofmt's output, so a toolchain bump must be accompanied by whatever reformatting the new gofmt
wants, in the same change.

## Unresolved

Whether `gofmt -s` — the simplification pass — should also be required is not decided here. It is a
strictly larger rule that would rewrite existing code, and it is a style judgement rather than the
mechanical "matches the formatter" one this gate makes.
