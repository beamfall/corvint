# 2026-10-20 V1-2104, V1-2108: stub-decided assertions and flag conditions

Tickets: V1-2104 (GitHub #1339) and V1-2108 (GitHub #1340).

## Decision

`TSA-V0-430..442` are added to `docs/specs/test-source-analysis-v0.md`, accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). The block starts above TSA-V0-419, the
highest ID on origin/main at lane start; lanes whose PRs were not open then could still collide.

- V1-2104: `RB-STUB-DECIDES-ASSERTION` (abstained) flags a negated change matcher whose observed
  value a stub on the same receiver fixes, or whose state-changing operation a stub replaces (a
  bang method or a closed list of write operations). `RB-MESSAGE-CHAIN-PREFIX-ONLY` (abstained)
  flags an `allow(...).to receive_message_chain` stub asserted only by `have_received` of a
  prefix message, with no expectation naming the final message.
- V1-2108: stubs of a flag lookup (built-in list, or a committed `.corvint/flag-lookups.json`,
  `corvint-flag-lookups/0`, with optional declared defaults) are conditions, not collaborators. Each
  gives the advisory `RB-FLAG-CONDITIONAL` and a `conditions` entry with key, forced value, declared
  default and the qualifier `conditional on flag KEY=VALUE`; a forced value that differs from a
  declared default adds the advisory `RB-FLAG-FORCED-NON-DEFAULT`. `tests validity`,
  `tests example-join` and the discovery source rules carry `conditions` additively.

## Narrow readings

- Reachability of the stub from the code under test is not read, so the stub rules abstain and
  never make an example INELIGIBLE (the issue asked for a flag, not a refusal).
- The "replaces the operation" case fires only for bang methods and a fixed write-operation list;
  a raising or `false`-returning stub of any other method was dropped as too likely to be a valid
  failure-injection test.
- A code default comes only from the project declaration; application code is never read, so with
  the built-in list no default is known and no non-default warning is given.
- Discovery binds no declaration and uses the built-in list.

## Limits

Not read: `Flipper.enable` and other setters, `ENV[]`, `receive_messages`, block-form stubs,
multi-line stubs, shared-group inclusions, and a generic stub overriding a keyed one. A dirty
example-join receipt reads the declaration at the join revision. Real-suite precision and recall
are NOT_MEASURED; no live Rails-suite qualification was run (NOT_RUN).
