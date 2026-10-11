# 2026-10-11 w7 appmap-rest lane

Branch `claude/w7-appmap-rest`, from `claude/w6-appmap-impact` (31fb799d8).

- V1-1220 (AMAP-V0-079, issue #773): `createBrowserRouter(NAME)` reads the array a sibling module
  exports when NAME is one relative named import; exactly one hop, other import forms and
  unresolved or ambiguous modules stay named unknowns. The Rails dialect already shipped (V1-1131).
- V1-1164 (ANL-V0-035/036, issues #776/#777): orchestrator decision of 2026-10-11 defines the
  classes. Each test-sequence step is navigation-only (goto, link clicks, waits, reads, assertions),
  state-effect (fills, submits, API requests, storage) or unknown, read from the spec span and the
  page-object methods it calls. Prefix lint replays only all-navigation-only, fresh chains; the
  sidecar can no longer upgrade a derived step.
- V1-1168 (AMAP-V0-080..082, issue #778): orchestrator decision: "runtime" means retained observed
  navigation evidence. `entries --ledger` reports each screen's shortest path of receipt-backed
  PROVEN ledger edges from a deep-linkable entry, NOT_OBSERVED otherwise; no URL is visited.

Limits: test-id and raw-selector clicks stay unknown (only `getByRole('link')` is navigation);
property-bound locators, inherited methods and spec-local helpers are not followed. Entry loading
itself stays STATIC_ONLY. All new requirements are proposed, pending owner acceptance.

Independent review round 1 (Codex, FAIL) found five fail-open paths, all fixed: computed and
optional invocations, `Object.assign` through a root whitelist, reassigned page-object bindings,
member-span freshness that missed import changes (steps now cite whole files), and contradicted or
empty-key receipts counting as proven edges.
