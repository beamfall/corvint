# 2026-10-21 affreaders-1021v: unbounded-reader cause and a bounded guard tool

Lane `affreaders-1021v`, base `f29ddc0db09c01c370cd3c9bcc5022a296ae3876`. Tickets V1-2075, V1-2224
(delivered), V1-1817 and V1-1006 (skipped, below).

## Problem

- V1-2075: an `AGENTS.md`-only change selected nine units in other repositories as
  `UNBOUNDED_READER` with no reason per unit; the plan did not say which file makes each unit read
  any path.
- V1-2224: `tools/dogfood-guard-hunks` (no tests) inherited an unresolved row from
  `internal/jstestprovider` through `internal/dogfoodflow` and added it to every plan's
  unresolved listing.

## Decisions

- AFP-V0-192 (accepted under the owner's standing auto-accept, 2026-10-11): every
  `UNBOUNDED_READER` witness carries `cause` `{unit, reads}`: the unit itself when it records its
  own `unboundedReads`, else the `locatesRoot` unit that starts the chain rule (d) followed. The
  acceptance allowed "selects no units or states the concrete read path per unit"; dropping the
  selections would be unsound because rule (d) reads are lexically unbounded, so the plan states
  the cause and keeps the selection (fail closed). `via` is unchanged, so ranking is unchanged;
  the member is omitted on every other witness kind.
- V1-2224: declare `tools/dogfood-guard-hunks` with an empty read scope in
  `.corvint/test-read-scopes.json`, as the ticket allows ("bounded or declared"). It has no tests
  and reads no files under test; the declaration is the existing AFP-V0-023 mechanism, covered by
  `TestDeclaredPackagesAreBoundByTheirScope`.
- V1-1817 skipped: shell path tokens are only path-shaped words, so `cd docs` leaves no token and a
  script can reach any directory through `find`, globs or variables. Narrowing a lone file name to
  the test's working directory has no sound bound without a new shell unit member recording that
  directory; that needs its own design.
- V1-1006 skipped: telling a data literal from one used as an executed or built path needs data
  flow, including helpers in other packages. Dropping the edge on a lexical guess could miss a real
  binary reader. Needs a design first.

## Evidence

- `TestInstructionFileChangeNamesEachUnboundedReadersCause_V1_2075` fails without the change
  (witness `cause` nil) and passes with it; package `./internal/liveverify/affected/` passes.
- `gate-affected-select -unresolved github.com/Beamfall/corvint .`: 105 rows at base, 104 after;
  `tools/dogfood-guard-hunks` is no longer listed.

## NOT_RUN

- `make gate` (not required for scoped lane work); tier 2 of `corvint affected` (batch CI).

## Rollback

Revert this change: drop `Witness.Cause`, `UnboundedCause`, `Graph.causes`, AFP-V0-192 and the
`tools/dogfood-guard-hunks` read-scope entry.
