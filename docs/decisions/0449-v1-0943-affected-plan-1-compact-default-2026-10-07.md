# Decision 0449 — `affected-plan/1` becomes the compact default; `--full` keeps `affected-plan/0`

Date: 2026-10-07. Status: recorded by the orchestrator under the owner's delegation of in-task
decisions (2026-10-05). This is the CCF-V1-006 breaking-change record. It does not accept intent:
AFP-V0-035 and the CCF-V1-002, CCF-V1-005 and CCF-V1-007 amendments stay proposed, pending owner
acceptance (AGENTS.md invariant 8).

## Context

Ticket V1-0943 asked for a lean default `corvint affected` document. `plan.excluded` changes from
an array to a `{count, digest, groups}` object and `selected[].tests` becomes `testCount`. CCF-V1-005
freezes those members, so under CCF-V1-006 the change is breaking and needs a new profile version,
an N-1 reader, a decision record, and a contract and test update in the same change
(`docs/build-log/2026-10-07-v1-0943-affected-compact-plan.md`).

## Decision

- The default worktree, `--base` and `--snapshot` forms write `affected-plan/1` (AFP-V0-035).
- `--full` writes the `affected-plan/0` document byte-for-byte as before; the renamed core-freeze
  goldens `affected-full-*.json` pin it. A consumer that needs full exclusion or test lists passes
  `--full`.
- The N-1 reader rule (CCF-V1-007) is met in-repo: `internal/companionrelease` core smoke,
  `.github/cishards` and `tools/corvint-pr-tests` accept both identifiers, and
  `tools/retrieval-bench` passes `--full`.
- The Playwright profile is unchanged; `--full` with `--playwright-config` is `invalid-arguments`.

## Limits

N-1 replay against 0.8.1 covers only the `--full` modes (0.8.1 has no `/1`). External consumers that
parse `/0` from the default form must add `--full` or read `/1`. No `make gate` was run.

## Rollback

Restore `affected-plan/0` as the default, delete `cmd/corvint/affected_compact.go` and the `--full`
option, restore the original core-freeze golden names, and mark this decision superseded.
