# 2026-10-10: exact PATH key naming a directory warns at set-effects and claim (V1-1118)

## Intent

Native ticket V1-1118, found in RC3 lane C (V1-1090). Under CAL-V0-021, a `PATH` key without a
trailing `/` names one path. A key such as `docs/build-log` that names a directory therefore covers
nothing beneath it, and `submit` refuses `OUT_OF_SCOPE` only later (CAL-V0-024). The change adds
one proposed requirement, CAL-V0-211 in `docs/specs/corvint-tasks-agent-leases-v0.md`, pending
owner acceptance. No decision record exists for it.

## Decisions

- **Warn, do not refuse.** This follows the CAL-V0-192 `EFFECTS_UNBOUNDED` precedent: an envelope
  `warnings` entry with a stable prefix, `EXACT_KEY_NAMES_DIRECTORY`, added only after a fresh
  committed transaction. Replays, no-change results and refusals add nothing. No result code,
  payload, receipt, journal record, stored effect or reserved scope changes. The prefix is
  UPPER_SNAKE warning text, so the kebab-case error-code ownership registry does not apply to it.
- **Which tree.** A claim reads the attempt's base commit, which is the tree `submit` diffs from.
  `ticket set-effects` has no attempt, so it reads `HEAD` in the caller's checkout.
- **Which keys.** A claim checks its reserved scope's `PATH` keys, whatever the scope's source
  (declared, requested or derived). `set-effects` checks every touchPath and `PATH` resource key
  whatever the coverage, because the keys become the declared scope once the coverage is
  QUALIFIED.
- **One Git call.** `git cat-file --batch-check=%(objecttype)` with `<tree>:<key>` lines. An
  `ls-tree` pathspec was rejected: overlapping keys (`a`, `a/b`) make it descend and drop the
  directory entry. A failed read drops the warning instead of failing a committed write.
- **Placement.** The claim check runs in `store.claimedTicket`, which already decodes the posted
  attempt, and reaches the result through `Report.ScopeWarnings`. That keeps it off the
  CAL-V0-024 refusal hunks in `internal/tasks/transaction/lease_gate.go`, which PR #740 edits.

## Evidence

- `TestCALV0211_ClaimWarnsOnExactDirectoryKey` and `TestCALV0211_ExactDirectoryKeyWarnings`
  (`internal/tasks/store`). A requested scope `absent lib/ src src/a.go` warns once, for `src`,
  naming the base tree and `src/`. The replay adds nothing, a `doc/ tool/d.go` claim adds nothing,
  a claim based on an earlier commit does not warn for a directory that only `HEAD` has, warnings come in key order, and an unresolvable revision warns nothing.
- `TestCALV0211_SetEffectsWarnsOnExactDirectoryKey` (`internal/tasks/cli`). In a real Git
  checkout, set-effects with resource `lib` and touchPaths `absent src src/a.go` commits and warns
  for `lib` and `src`. The replay adds nothing, file keys plus `/` directory keys add nothing, and a CLI claim of the exact keys commits with no codes and carries the same two warnings.

## Limits

Owner acceptance of CAL-V0-211 is pending. Live qualification against a real store is NOT_RUN.
