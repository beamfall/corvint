# Decision 0499 — Exact directory key warning (CAL-V0-211) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept both", answering the request to accept HLQ-V1-010 and CAL-V0-211).

## Context

V1-1118 followed V1-1090 (decision 0493): a `PATH` key without a trailing `/` that names a
directory covers only that exact path under CAL-V0-021, so `submit` later refuses `OUT_OF_SCOPE`
for files beneath it, while `set-effects` and `claim` accepted the key silently. The V1-1118 lane
proposed `CAL-V0-211` in `docs/specs/corvint-tasks-agent-leases-v0.md`. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `CAL-V0-211` as written:

- When `ticket set-effects` commits a fresh transaction, or `claim` / `claim --next` commits a fresh
  attempt, the result adds one `warnings` entry for each distinct `PATH` key without a trailing `/`
  that names a directory in the base tree. For `set-effects` the keys are the payload's
  `touchPaths` and `PATH` resource keys against the tree of `HEAD`; for a claim they are the
  reserved scope's `PATH` keys against the attempt's base commit.
- Each entry starts `EXACT_KEY_NAMES_DIRECTORY: PATH key <key> `, names the tree and ends
  `use <key>/ to cover them`, in byte order of the key. File keys, absent keys, keys ending in `/`
  and an unreadable tree add none.
- The command still commits; its items, codes, outcome, stored effects and reserved scope are
  unchanged. `EXACT_KEY_NAMES_DIRECTORY` is a warning prefix, not a result code.

## Limits

This decision settles intent only. The evidence is the `TestCALV0211_*` tests in
`internal/tasks/store` and `internal/tasks/cli` and the store package run named in the lane's
build log. Live qualification against a real store and `make gate` are `NOT_RUN`. `ticket create`
and `refine` are not covered (V1-1125).

## Rollback

Revert this decision and return the `CAL-V0-211` status text to proposed. To withdraw the behavior,
also revert `internal/tasks/store/directory_keys.go`, the `ScopeWarnings` report field and its use
in `internal/tasks/store/lease.go`, and the `set-effects` warning in `internal/tasks/cli/mutate.go`,
then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
