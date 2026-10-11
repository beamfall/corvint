# 2026-10-11 w6 appmap-impact lane

Branch `claude/w6-appmap-impact`, from `codex/w5-appmap-a` with `codex/w5-appmap-b` merged.

- V1-1170: two-constant regression test pins the V1-1144 failed-hop state leak fix.
- V1-1167 (ANL-V0-031, AFP-V0-009 amendment): `corvint affected --appmap-impact` adds the learned,
  advisory `advice.app_navigation` member; unknown or uncommitted paths fail closed to
  `full-suite-required` with an `APP_NAVIGATION_FULL_SUITE_REQUIRED` unknown. Checks and plan are
  never changed.
- V1-1162 (ANL-V0-032, ANL-V0-016 amendment): a changed template/component file outside every anchor
  span is placed only through a bounded reference graph whose mention resolves as a quoted path
  literal; comments, dynamic literals and bound overruns stay named unknowns.
- V1-1169 (AMAP-V0-078): React `createBrowserRouter(IDENT)` reads one same-file `const` array
  literal; imported and computed arrays are named unknowns. Rails and imported resolution remain.
- V1-1166 (ANL-V0-033): `test-plan page-state --check --baseline` reports added, removed, changed
  and blocked-changed group IDs against a digest-verified baseline and exits 1 on drift.
- V1-1163 (ANL-V0-034): several maps of one app aggregate into one row with the most conservative
  verdict; a repeated map refuses.

Not delivered: V1-1164 (page-object effect classification needs owner intent for spec-to-page-object
mapping; deriving effects would invent certainty) and V1-1168 (runtime deep-link entry mode).
All new requirements are proposed, pending owner acceptance.
