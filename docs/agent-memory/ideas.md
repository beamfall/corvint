---
name: ideas
description: Broader future work, features, and unscheduled directions
updated: 2026-09-19
---

# Ideas

Future features and larger directions. Promote to `docs/specs/` when an idea matures; remove the entry once promoted or shipped. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-19 work-queue: let Corvint's own `decision-0046-v0` mapping qualify store scope
Decision 0321 limits WQO-V0-046 to `repository-worklist-v0`, so Corvint's self-dogfood observation stays `UNKNOWN/SOURCE_UNQUALIFIED` and `propose-wave` still abstains on this repository. The same byte-reproduction argument holds for `docs/worklist.json`. Done means the check in `workMappingReproduced` (`cmd/corvint/work.go`) accepts both mappings, the WQO-V0-021/025/032 final-check fixtures in `cmd/corvint/work_final_check_test.go` get their incomplete initial capture another way, and the WQO-V0-017 paragraph is amended.

### 2026-09-19 ci: run the AFP-V0-017 qualification once `main` has 201 first-parent commits
`pr-tests-qualification.yml` (decision 0320) cannot freeze a corpus until `git rev-list --first-parent --count origin/main` reaches 201; it was 15 on 2026-09-19, at about 12 merges a day. Then dispatch `rows=1` to measure one row, then `rows=all` (about 200 × 35–45 min of runner time), review `qualification.json`, and only then set `CORVINT_PR_TOOL_SOURCE`, `CORVINT_PR_QUALIFICATION_SOURCE` and `CORVINT_PR_QUALIFICATION_SHA256` in `ci.yml`. Done means narrowed PR runs admitted by a PASS qualification.

### 2026-09-19 ci: one test runner that behaves identically locally and in CI
Owner accepted the recommendation: `make gate-affected` (local) and `tools/corvint-pr-tests` (CI) plan the same selection through different drivers. A single runner would take a base, plan through `corvint affected`, select, run, and fall back to the full suite the same way on a laptop and a hosted runner, with CI adding only the trusted-pin admission. It needs its own spec after the AFP-V0-017 qualification, since qualification is tied to the current driver's identity. Done means an accepted spec and both call sites using it.

### 2026-09-18 extevidence: test selection beyond one hop and into checkout worktrees (ETS-V0 follow-up)
`docs/specs/external-test-selection-v0.md` stops obligations one relation hop downstream of a changed entity, and never reads a bound checkout's worktree (rows say `checkout-worktree-not-inspected`). Both keep the selection fail-closed only as far as the record is complete. Done means a bounded transitive obligation walk and a per-checkout dirty read, each with conformance cases proving they only ever widen.
