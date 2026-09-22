# Decision number gate V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `DNG-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md` "Required
capability-spec shape", `../decisions/0048-four-owner-calls-2026-09-04.md` (which names the
grandfathered pair and makes this check a `make gate` prerequisite), `../decisions/README.md`
"Checking numeric uniqueness", and `../agent-memory/fixes.md` (which lists this script among the
gate scripts still without an owning spec).

## Agent digest
- Claim: Every decision file claims a four-digit number no sibling shares, with exactly one grandfathered pair.
- Status: accepted (owner instruction 2026-09-07) / implemented
- Exists: `script/check-decision-numbers.sh`, `make` target `decision-numbers-check`, wired into `make gate`.
- Blocked on: nothing for this gate. The sibling gate scripts named in `../agent-memory/fixes.md` remain unspecified.
- Read next: Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

`docs/decisions/` files are named `NNNN-topic-date.md`; the leading number is the decision's short
identity, used for terse cross-references ("decision 0012") inside the lineage that recorded it.
Two independent branches can mint the same number before either sees the other's history, and a
merge then leaves two files claiming one number. `docs/decisions/README.md` records exactly one
surviving case of this: decision 0012 collided across lineages, and after resolution the two
`0016-*.md` files are the one pair still coexisting under a shared number, cited by full filename
from frozen evidence and grandfathered by `0048-four-owner-calls-2026-09-04.md`. Any other
collision is a naming defect: it breaks a bare-number citation and must be renumbered before it is
cited from anywhere outside its own lineage.

Affected user: an agent or engineer adding or merging a decision file. Measurable job: for the
current `docs/decisions/` directory, fail when any leading number is claimed by more than one file
other than the one grandfathered pair, and pass otherwise, including on a fresh checkout with no
decisions directory populated yet.

The owner instruction of 2026-09-07 places repository gate tooling in scope for AGENTS.md invariant
8. This spec discharges that for the decision-number gate only; the sibling gate scripts named in
`../agent-memory/fixes.md` are a follow-up and remain governed by nothing.

## Verified current state

`script/check-decision-numbers.sh` is wired into `make gate` through the `decision-numbers-check`
target. At `b7c3dbe` `docs/decisions/` holds 80 files matching `NNNN-*.md`; the only leading-number
collision is the grandfathered `0016` pair (`0016-change-anchored-packet-2026-09-01.md` and
`0016-packet-5-current-pin-requalification-2026-08-31.md`), named in
`0048-four-owner-calls-2026-09-04.md`. Running `script/check-decision-numbers.sh` directly at this
commit exits 0 with no output, matching the `../reviews/r13-truth-audit.md` record that the gate
passes with only that pair present.

## Requirements

- `DNG-V0-001`: The gate MUST scan only direct children of `docs/decisions/` whose name matches
  `[0-9][0-9][0-9][0-9]-*.md`; it MUST NOT recurse into subdirectories and MUST NOT consider a file
  whose name does not begin with exactly four digits followed by a hyphen.
- `DNG-V0-002`: When no file in `docs/decisions/` matches that pattern, the gate MUST exit 0
  immediately and check nothing further, so a fresh checkout or a test fixture with no decisions
  directory populated yet does not fail.
- `DNG-V0-003`: The gate MUST derive each matched file's decision number as the first four
  characters of its basename, group matched files by that number, and treat any number claimed by
  two or more files as a collision candidate.
- `DNG-V0-004`: Before grouping, the gate MUST sort the matched file paths under the `C` locale, so
  that the file order within a number, and therefore the joined string compared against the
  grandfathered exemption, is identical across shells and locales.
- `DNG-V0-005`: A collision candidate MUST be treated as exempt, and MUST NOT fail the gate, only
  when the full set of colliding files, joined in sorted order, is exactly
  `docs/decisions/0016-change-anchored-packet-2026-09-01.md
  docs/decisions/0016-packet-5-current-pin-requalification-2026-08-31.md`. A collision on number
  `0016` involving a third file, or any collision on any other number, MUST NOT match this
  exemption and MUST fail.
- `DNG-V0-006`: On a non-exempt collision the gate MUST report, for every colliding number found in
  the run (not only the first), a header line, the colliding number, and the full path of each file
  claiming it; the gate MUST evaluate every number before exiting rather than stopping at the first
  collision.
- `DNG-V0-007`: The gate MUST exit 0 when no non-exempt collision exists (including the case where
  the only collision present is the grandfathered pair) and MUST exit non-zero when any non-exempt
  collision exists, so that `make gate`, which runs this check through the
  `decision-numbers-check` target, fails with it.
- `DNG-V0-008`: The gate MUST perform no write to any file and MUST produce the same result
  regardless of the working directory it is invoked from, resolving its own location to find the
  repository root before reading `docs/decisions/`.

## Non-goals and simpler baseline

The simpler baseline is failing on any leading-number collision with no exemption; this gate adds
exactly one hardcoded exemption because the alternative would either permanently fail the gate over
an already-resolved historical merge or require renaming files that frozen evidence cites by exact
name. The gate does not check that a decision number is sequential, that no number is skipped, that
a file's number matches its content, or that a citation elsewhere in the repository uses the
correct number; it does not validate any other part of the filename (topic or date), and it does
not read Git history — only the Git index's list of tracked paths, which is not the same set as the
working tree's directory listing.

## Trust boundary, limits, and failure modes

`docs/decisions/*.md` filenames are the only input; the gate reads the paths `git ls-files`
reports as tracked under `docs/decisions/`, derives basenames from them, and never reads file
contents. An untracked file is therefore invisible to the gate, and a tracked file deleted from the
worktree alone is still counted, because `make gate` must measure the set a fresh clone contains
(`Makefile:18`). It spawns `git ls-files` and its own shell pipeline (`tr`, `grep`, `sort`, `awk`)
and performs no write. Every collision found in one run is reported before the gate exits;
there is no partial-success exit.

| Failure | Behavior |
|---|---|
| No tracked `docs/decisions/NNNN-*.md` file exists | pass (exit 0), nothing else checked |
| Every leading number is claimed by exactly one file | pass |
| A leading number is claimed by two or more files, and the sorted set is exactly the grandfathered `0016` pair | pass (exempt) |
| A leading number is claimed by two or more files, and the sorted set is anything else, including a third `0016` file | fail, naming the number and every claiming path |
| More than one leading number collides in the same run | fail, every colliding number reported, not only the first |

## Acceptance criteria and testing matrix

This script has no dedicated Go or shell test file; its evidence is the gate invocation itself
against the live `docs/decisions/` corpus, plus the historical record of the one exemption it must
keep passing.

| Requirement | Evidence |
|---|---|
| DNG-V0-001, DNG-V0-002 | `./script/check-decision-numbers.sh` at `1ad9616f`, exit 0 over the 89 tracked matching files among 90 tracked `docs/decisions/` paths, none of them in a subdirectory; separate fixtures with no matching file, and with a colliding pair tracked only in a subdirectory, both exit 0 |
| DNG-V0-003, DNG-V0-004 | the grouping and `LC_ALL=C sort` in `script/check-decision-numbers.sh`; deterministic pass at `b7c3dbe` |
| DNG-V0-005 | the `0016` pair present at `b7c3dbe` and passing, per `../decisions/0048-four-owner-calls-2026-09-04.md` and `../reviews/r13-truth-audit.md` |
| DNG-V0-006, DNG-V0-007 | `make decision-numbers-check` membership in `make gate`; a non-exempt collision fails, an exempt or absent one passes |
| DNG-V0-008 | the script's own root resolution (`cd -- "$(dirname "$0")/.."`) and read-only body; no write path exists in the script |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| DNG-V0-001 | the `git ls-files` enumeration and its `^docs/decisions/[0-9]{4}-[^/]*\.md$` filter in `script/check-decision-numbers.sh` | `decision-numbers-check` |
| DNG-V0-002 | that same filter matching nothing, which leaves `awk` no input and exits 0; the explicit empty-directory guard it replaced is gone | `decision-numbers-check` |
| DNG-V0-003, DNG-V0-006 | the `awk` grouping and reporting loop in `script/check-decision-numbers.sh` | `decision-numbers-check` |
| DNG-V0-004 | the `LC_ALL=C sort` pipeline stage in `script/check-decision-numbers.sh` | `decision-numbers-check` |
| DNG-V0-005 | the `grandfathered` variable and comparison in `script/check-decision-numbers.sh` | `decision-numbers-check` at `b7c3dbe` |
| DNG-V0-007 | the `Makefile` `decision-numbers-check` target and its listing in the `gate` target | `make gate` membership |
| DNG-V0-008 | the `root=$(...)`/`cd` prelude in `script/check-decision-numbers.sh` | code inspection; no write syscall in the script |

## Rollout, rollback, and drift

The check is additive over `docs/decisions/README.md`'s existing documentation of the same rule;
reverting it removes the mechanical gate and leaves the collision rule as documentation only.
Rollback is dropping the `decision-numbers-check` prerequisite from `make gate` and reverting the
script. Drift rule: the grandfathered exemption names an exact, closed set of two files and MUST
NOT be generalized into a pattern, a count threshold, or a growing allowlist; a new collision is a
naming defect to fix by renumbering, not a candidate for a second exemption.

## Unresolved

Whether a future cross-lineage merge should ever add a second grandfathered pair, versus always
requiring renumbering, is not decided here; the exemption's exact-match design assumes the answer
is "renumber," per `../decisions/README.md`. The sibling gate scripts named in
`../agent-memory/fixes.md` remain unspecified.
