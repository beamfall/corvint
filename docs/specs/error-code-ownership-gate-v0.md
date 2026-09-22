# Error code ownership gate V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `ECO-V0`
Intent status: accepted (decision 0100, 2026-09-12)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../decisions/0100-error-code-ownership-ratchet-and-doccompiler-detail-codes-2026-09-12.md`,
`diagnostic-repair-contract-v0.md` (`DRC-V0-006`, which keeps every present code spelling), and
`decision-number-gate-v0.md` as the shape a gate spec takes here.

## Agent digest
- Claim: Every kebab-case code Corvint emits is named by a spec or listed in a checked-in allowlist that can only shrink.
- Status: accepted (decision 0100, 2026-09-12) / implemented
- Exists: `script/check-error-code-ownership.sh`, `script/check-error-code-ownership_test.sh`, `script/error-code-ownership.allowlist`, `make` targets `error-code-ownership-check` and `error-code-ownership-test`, both in `gate`.
- Blocked on: nothing for the gate; the allowlisted codes are the remaining burn-down.
- Read next: Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

Corvint refusals carry a stable kebab-case code, and callers key on it (`SOL-V0-007` ledgers the
`unsupported-*` family). A sweep on 2026-09-12 found hundreds of such codes that no spec names, so
their meaning was defined only by the Go that emits them, and nothing stopped the count growing.

Affected user: an agent or engineer adding a refusal, and any caller that keys on a code. Measurable
job: for the tracked tree, fail when an emitted code is named by no spec and is not already on the
recorded list of unowned codes, and fail when a listed code has since been owned or retired, so the
list converges to empty.

## Verified current state

At the introducing commit the extractor finds 474 distinct emitted codes, 362 of them named by no
tracked `docs/specs/*.md`; the allowlist starts at those 362 and the same change series shrinks it
as codes gain owning spec rows. `script/check-error-code-ownership.sh --list` prints the current
unowned set with each code's first site. On 2026-09-13 the three genesis emission forms (named
string-type conversions, later `code` parameters, and `reason` assignments) raised the extracted set
from 508 to 580 distinct codes; the 22 of those no spec named gained owning rows in the same change,
so the allowlist stayed empty.

## Requirements

- `ECO-V0-001`: An emitted code is a string literal matching `[a-z][a-z0-9]*(-[a-z0-9]+)+` in a
  tracked Go file under `cmd/` or `internal/` whose name does not end in `_test.go`, standing in one
  of six positions: the value of a `Code:` or `Reason:` composite-literal key; the value of a
  `"code":` or `"reason":` map-literal key; the argument of `errors.New`; the argument, at that
  parameter's index, of a call to a function, method, or assigned closure with a parameter named
  `code` or `reason` on its single-line signature, discovered from the same file set, where a
  parameter after the first is matched only in files directly under the declaring directory; the
  operand of a conversion to a type declared as `type <Name>Error string` or `type <Name>Code string`
  in the same file set; or the value at the same index of a single-line `=` or `:=` assignment whose
  target list names `code` or `reason` at that index (at most three targets before it). A call
  argument before the literal may contain one level of nested parentheses. The gate MUST NOT carry a
  list of constructor, type, or variable names (amended 2026-09-13 for the genesis emission forms).
- `ECO-V0-002`: A code is owned when any tracked `docs/specs/*.md` file (direct children only)
  contains it as a whole token, bounded on both sides by a character outside `[a-z0-9-]`.
- `ECO-V0-003`: `script/error-code-ownership.allowlist` MUST hold, one per line, sorted under the
  `C` locale with no duplicate, exactly the emitted codes that are not owned. The gate MUST fail
  naming each emitted unowned code absent from the list together with its first site, and MUST fail
  naming each listed code that is owned or no longer emitted. A new code is owned by adding its row
  to the owning spec, never by adding it to the list.
- `ECO-V0-004`: Sources, specs, and the allowlist MUST be read from the Git index, not the worktree,
  so an untracked file changes nothing and a staged change is what the gate measures.
- `ECO-V0-005`: A `git grep` exit status other than 0 or 1 MUST fail the gate naming `git grep`,
  never be read as an empty set.
- `ECO-V0-006`: A spec row that owns a code MUST describe only what the emitting code path checks,
  citing it by `path:line`, and MUST NOT assign it a meaning the code does not implement.
- `ECO-V0-007`: A passing run MUST print nothing and exit zero; the gate MUST be read-only and a
  member of the `gate` target, as MUST its fixture test. `--list` MUST print the unowned set as
  `code<TAB>path:line` and exit zero.

## Non-goals and simpler baseline

The simpler baseline is the one-off sweep recorded in the backlog: accurate the day it ran and
silent about every later code. This gate does not type-check, does not decide which codes reach a
wire envelope, does not validate a spec row's prose against the code, and does not rename or remove
any code (`DRC-V0-006`). It does not scan Python, conformance runners, tools, or tests.

## Trust boundary, limits, and failure modes

Inputs are tracked Go sources and specs; the gate spawns `git grep` and `git cat-file` and writes
only a private temporary directory it removes on exit.

| Failure | Behavior |
|---|---|
| New emitted code named by no spec and not listed | fail, naming code and first site (ECO-V0-003) |
| Listed code is now owned or no longer emitted | fail, naming the code to remove (ECO-V0-003) |
| Allowlist unsorted, duplicated, or not in the index | fail, naming the file |
| `git grep` enumeration fails | fail, naming `git grep` (ECO-V0-005) |
| Code routed through a named constant or a constructor whose parameter is not `code`/`reason` | not extracted; an accepted under-count of the syntactic method |
| Multi-line signature, call argument with two nesting levels before the literal, or a later-parameter call from another directory | not extracted; the directory bound keeps an unrelated same-named helper (a `git(ctx, limit, "rev-parse")` wrapper) from reading subcommands as codes |
| Internal reason that never reaches an envelope | extracted and must be owned; an accepted over-count |
| Code spelled as an ordinary prose word in some spec | treated as owned; whole-token matching cannot tell a name from a word |

## Acceptance criteria and testing matrix

`script/check-error-code-ownership_test.sh` builds a fixture repository; the live tree is covered
by `make error-code-ownership-check`.

| Requirement | Evidence |
|---|---|
| ECO-V0-001 | test case 2 (a code reached through a discovered `code`-parameter closure), case 4 (test files excluded), case 6 (a `...Error` string-type conversion), case 7 (a third-parameter `code` argument after a nested call, and a same-named helper in another directory not extracted), and case 8 (a `class, reason = ...` assignment) |
| ECO-V0-002 | test cases 1 and 2, owning a code by naming it in a fixture spec |
| ECO-V0-003 | test case 2 (new unowned code named with its site) and case 3 (stale entry named) |
| ECO-V0-004 | test case 4, an untracked Go file with an unowned code passing |
| ECO-V0-005 | test case 5, a failing fake `git grep` refused by name |
| ECO-V0-006 | review of each owning row; the rows added with this gate cite their emitting line |
| ECO-V0-007 | test case 1, a passing run printing nothing; `gate` membership |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| ECO-V0-001 | the constructor and type discovery, the `position` pattern, and the per-directory call search in `script/check-error-code-ownership.sh` | cases 2, 4, 6, 7 and 8 |
| ECO-V0-002 | the `docs/specs/*.md` word set and `comm` in the same script | cases 1 and 2 |
| ECO-V0-003 | the allowlist comparison in both directions | cases 2 and 3 |
| ECO-V0-004 | `git grep --cached` and `git cat-file blob :<allowlist>` | case 4 |
| ECO-V0-005 | the `search` wrapper's exit-status check | case 5 |
| ECO-V0-006 | owning spec rows | review |
| ECO-V0-007 | the exit paths, `--list`, and `error-code-ownership-check`/`error-code-ownership-test` in `gate` | case 1; `make gate` membership |

## Rollout, rollback, and drift

The gate is green from the commit that introduces it, because the allowlist starts at the measured
unowned set. Rollback is removing the two `make` targets, the two scripts, and the allowlist; no Go
changes. Drift rule: renaming a code, retiring it, or moving its owning row edits the allowlist in
the same change, because the gate fails in both directions.

## Unresolved

A typed inventory that separates wire codes from internal reasons belongs to `DRC-V0-006` and
`DRC-V0-011` once that contract is accepted; this gate is the syntactic floor until then.
