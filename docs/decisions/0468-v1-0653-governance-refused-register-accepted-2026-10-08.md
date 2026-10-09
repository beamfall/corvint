# Decision 0468: register the `coverage.governance_refused` values

Date: 2026-10-08. Status: accepted (owner approval in chat, 2026-10-08: "accept (a) for V1-0653").
Ticket: V1-0653.

## Context

CCF-V1-007 (d) left the `relation` and `trust` of `context` `coverage.governance_refused` as
NOT_PRODUCED: no generator wrote a row there, because every reserved authority maps to the untainted
`project-authority`, and CCF-V1-006 would decide a value by review. The authority screen
(TCP-V0-055..058, V1-0414) now downgrades a reserved governing row that hides bidirectional controls
or modifies its own authority and names it in `coverage.governance_refused` with `trust`
`repository-content` and a `warnings` array. The values were therefore emitted with no register row.
The owner chose option (a): register them as a CCF-V1-006 additive amendment rather than roll the
emission back.

## Decision

1. CCF-V1-007 (d) gains three rows for `context`:
   - `coverage.governance_refused[].relation`, open: `governing`, `spec-mentioned`,
     `instruction-routed` (the reserved row kinds).
   - `coverage.governance_refused[].trust`, closed: `repository-content`, `external-provider`,
     `tool-output`. A downgraded row keeps its derived class; a tainted class is named as before.
   - `coverage.governance_refused[].warnings[]`, open: `hidden-unicode`,
     `hidden-unicode-unscreened`, `self-modified-authority`.
2. `reason` stays free text, outside the register.
3. The frozen mode `context downgraded governing row` commits a governing `AGENTS.md` that hides
   bidirectional controls, so each row is reached by a frozen mode. 0.8.1 has no authority screen and
   emits an empty array, so that mode is added to the N-1 skip list (`coreN1Skips`).
4. The `context` profile version does not change: the member already existed in 0.8.1 and the change
   adds values to it, which CCF-V1-006 treats as compatible.

## Limits

The `spec-mentioned` and `instruction-routed` relations and the tainted `trust` values are
registered from the code paths that can write them (`internal/contextindex/trust.go`,
`internal/contextindex/taskcontext.go`); only the `governing` / `repository-content` /
`hidden-unicode` combination is reached by a frozen mode. An open row may gain a value under
CCF-V1-006; the closed `trust` row may not without a profile decision.

## Rollback

Revert this decision's commit: the three register rows, the frozen mode, its golden and N-1 skip,
and the spec status lines return to the NOT_PRODUCED state. The emitted values are unaffected; the
register would again leave them to CCF-V1-006 review.
