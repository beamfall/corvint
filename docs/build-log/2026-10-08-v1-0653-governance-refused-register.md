# 2026-10-08: Register the `coverage.governance_refused` values (V1-0653)

## Intent

Ticket V1-0653 found that the authority screen (TCP-V0-055..058, V1-0414) writes
`context` `coverage.governance_refused` rows whose `relation`, `trust` and `warnings` values have no
CCF-V1-007 (d) register row; the freeze spec still called them NOT_PRODUCED. The owner chose option
(a), a CCF-V1-006 additive amendment, over rolling the emission back (decision 0468).

## Decisions

- **Three register rows.** `relation` (open: the three reserved row kinds), `trust` (closed:
  `repository-content` for a downgraded row plus the two tainted classes) and `warnings[]` (open:
  the three authority-screen codes). `reason` is free text and stays out of the register.
- **One frozen mode.** `context downgraded governing row` commits an `AGENTS.md` that hides
  bidirectional controls. Its golden reaches `governing`, `repository-content` and
  `hidden-unicode`, so the register test sees every row reached.
- **N-1 skip.** 0.8.1 has no authority screen and emits an empty array for that fixture, so the
  mode joins `coreN1Skips`; the member itself already existed, so no profile version moves.

## Evidence

- `go test -run TestCoreVerbsEmitTheFrozenProfiles ./cmd/corvint` and the contextindex
  authority-screen tests pass (see the commit's lane log).
- The local doc gates pass.

## Limits

Only the `governing` / `repository-content` / `hidden-unicode` combination is reached by a frozen
mode; the other registered values are cited from the code paths that write them.
