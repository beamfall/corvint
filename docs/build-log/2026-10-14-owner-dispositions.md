# 2026-10-14 Owner dispositions: V1-1283 criterion 4, V1-0812 consent, merge-queue build cap

The owner decided three items in chat on 2026-10-14. Each decision is also recorded in its
ticket's operator note. This entry is the repository record and changes no contract.

## V1-1283 criterion 4: deviation accepted

Criterion 4 asks for one real ticket to be completed through the `cem-verdict` gate "on a migrated
map". V1-1348 completed natively through the gate on a native `cem/1.0` map
(`2026-10-14-v1-1283-native-completion-through-cem-verdict.md`, PR #869, merge 61ac7fe44). A
migrated map cannot meet the clause: the gate refuses it with `CEM_VERDICT_UNVERIFIABLE`
(stable-not-accepted). The owner accepted the native-map completion as a deviation and did not ask
for a provenance-aware gate mode. V1-1283 stays completed.

## V1-0812: consent to narrow, conditional on the bar

The owner consented to narrowing pull-request race tests behind the merge queue (decision 0431
items 3, 4 and 8). Narrowing may start only once the retained AFP-V0-029 shadow bar is met (20
complete forward rows, at least 5 with failing packages, no non-FLAKY miss) and V1-0760 has merged.
The consent waives none of V1-0812's criteria: the allowlist and every FULL condition fail closed,
the kill switch and rollback R1 to R3 are required, and promotion out of experimental still needs
its 20-PR measurements. On 2026-10-14 no forward row had been retained yet.

## Merge-queue `max_entries_to_build` raised from 2 to 3

The owner approved raising the build cap and chose 3. Ruleset 23699808's `merge_queue` rule now
has `max_entries_to_build: 3` (`updated_at` 2026-10-14T08:15:18-04:00). GraphQL
`mergeQueue(branch: "main")` reads back `maximumEntriesToBuild 3`, and the other rules and queue
parameters are unchanged from `2026-10-12-merge-queue-enabled.md`.

- Expected effect: only the first queue entry on an unmoved `main` can reuse its identical-tree
  pull-request run (V1-1360). Each speculative entry behind it runs FULL and holds 13 of the free
  plan's 20 concurrent hosted jobs, so the third build often waits for runners.
- Measure queue wait and runner contention before any further raise, which needs the owner again.
- Rollback: `gh api -X PUT repos/beamfall/corvint/rulesets/23699808` with the same body and
  `max_entries_to_build: 2`.
