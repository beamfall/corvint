# V1-5623 second review repair: unknown reach through unowned paths and link targets

The independent fix review of the V1-5623 repair (lane `claude/r1flows`, reviewed head
`824456761`) returned CHANGES_REQUIRED: nine of ten findings fixed, the unknown-reach finding
(AFL-V1-007, APK-V0-010) only in part. This entry records what the second repair decided; the
rule is in the change design's "Decisions made while building". The intent of
`docs/specs/application-flow-status-v1.md` stays `proposed`.

## Findings and decisions

- B1. A changed path with no owning unit counted as resolved when a link targeted it. With
  `src/components/Cart.vue` linked by `checkout` and imported by `src/search.ts`, a commit changing
  only `Cart.vue` named `checkout` alone. The link resolves that one link, not what else imports
  the path, so the exemption is gone: any changed path with no owning unit is listed in
  `reach_unknown` and every flow is named with `reason: reach-unknown`; the resolved hit is kept.
- B2. A link target with no owning unit gave no reach at all. With `Cart.vue` (linked) importing
  `src/lib/price.ts`, a commit changing only `price.ts` gave `flows: []`. What an unowned target
  imports is unknown, so every changed path's reach is unknown: all are listed in `reach_unknown`,
  every flow is named, and the graph's `unknown` gains `LINK_TARGET_UNOWNED: PATH` per such
  target. A link target that no longer exists is the same case.
- Flow inputs. The first repair left out everything under the intents directory. Only the flow
  declaration and the `.json` files directly in that directory are flow inputs; any other file
  there is under the rule.
- Not closed. Reach through a file with no adapter that is neither changed nor a link target (an
  owned link target importing `Cart.vue`, which imports the changed owned file) still resolves as
  not reached, and the graph's `unknown` does not name it; a planner unknown or a dirty worktree
  still leaves the list as resolved under an `UNKNOWN` scope (AFU-V1-163). Both predate the change.
  Closing them needs the graph to give the units whose imports it could not resolve and a ruling on
  AFU-V1-017's narrowing, which are the owner's.
- Store names. `flows status` split the store's name of a held test at its first ` @`, so a test
  title holding a tag (`pays @smoke`) lost its `run-evidence-corrupt` mark. Each name is now read
  whole, as a key with no project, and split at its last ` @`.
- Still open for the owner: the RVD-V1-010 and AFL-V1-006 reconciliation of the first repair,
  including a stored record that cannot be read at all (skipped and marked, not refused).

## Evidence

Each test was run against the unrepaired code first; the failing output and the package runs at
the repaired head are kept with the lane handback. `impact.golden.json` is unchanged.
