---
name: ideas
description: Broader future work, features, and unscheduled directions
updated: 2026-09-21
---

# Ideas

Future features and larger directions. Promote to `docs/specs/` when an idea matures; remove the entry once promoted or shipped. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 gate: per-package cross-worktree key for resolved packages (GL-V0 follow-up)
`docs/specs/gate-ledger-v0.md` keys the 104 unresolved packages on the whole tree and leaves the resolved ones to Go's test cache, which never hits across worktrees because its test log hashes absolute paths. A per-package content key (transitive source plus the literal-bounded files the affected-plan index already knows) would let a resolved package hit from any worktree. Done means a proven-complete bound per package and a hit measured from a second worktree.

### 2026-09-21 cmd/corvint: bound the one `os.Getwd` so the largest package leaves the unresolved set
`gate-affected-select -unresolved` lists `cmd/corvint` for `os.Getwd` in `dogfood_record.go`, so every tree change reruns it (most of the suite's wall time) under the GL-V0-004 tree key. Bounding that read moves it under Go's cache for same-worktree reruns. Done means the package absent from `-unresolved` and its tests still passing.

### 2026-09-19 ci: run the AFP-V0-017 qualification once `main` has 201 first-parent commits
`pr-tests-qualification.yml` (decision 0320) cannot freeze a corpus until `git rev-list --first-parent --count origin/main` reaches 201; it was 15 on 2026-09-19, at about 12 merges a day. Then dispatch `rows=1` to measure one row, then `rows=all` (about 200 × 35–45 min of runner time), review `qualification.json`, and only then set `CORVINT_PR_TOOL_SOURCE`, `CORVINT_PR_QUALIFICATION_SOURCE` and `CORVINT_PR_QUALIFICATION_SHA256` in `ci.yml`. Done means narrowed PR runs admitted by a PASS qualification.

### 2026-09-19 ci: one test runner that behaves identically locally and in CI
Owner accepted the recommendation: `make gate-affected` (local) and `tools/corvint-pr-tests` (CI) plan the same selection through different drivers. A single runner would take a base, plan through `corvint affected`, select, run, and fall back to the full suite the same way on a laptop and a hosted runner, with CI adding only the trusted-pin admission. It needs its own spec after the AFP-V0-017 qualification, since qualification is tied to the current driver's identity. Done means an accepted spec and both call sites using it.

### 2026-09-18 extevidence: test selection beyond one hop and into checkout worktrees (ETS-V0 follow-up)
`docs/specs/external-test-selection-v0.md` stops obligations one relation hop downstream of a changed entity, and never reads a bound checkout's worktree (rows say `checkout-worktree-not-inspected`). Both keep the selection fail-closed only as far as the record is complete. Done means a bounded transitive obligation walk and a per-checkout dirty read, each with conformance cases proving they only ever widen.

### 2026-09-18 extevidence: command, MCP, and remote provider transports (EEP slice 2)
Decision 0309 ships only the file transport for `docs/specs/external-evidence-provider-v0.md`. A provider that is a local command, an MCP tool, or a remote service reaches outside the local boundary (AGENTS.md invariant 7), so each transport needs its own accepted profile under the analyzer capability contract before `--provider` accepts anything but a file path. Done means an accepted profile plus the same strict record decode, freshness, and separation tests running over the new transport.
