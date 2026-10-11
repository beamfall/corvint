# V1-1175 Playwright discovery listing omissions explained by project grep

Date: 2026-10-11. Base: `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`. GitHub #790.
Scope: `corvint affected discovery --playwright-config --playwright-list` refused every caller-run
listing of a config whose projects split one `testDir` by tag with `grep`/`grepInvert`, because a
project whose filter drops every case of a file omits that (project, file) pair from the listing
and the `TJAA-V0-019` membership check read every omission as staleness. New requirement
`TJAA-V0-021`, proposed and pending owner acceptance (V1-1175); `TJAA-V0-019` and the receipt
format paragraph are amended only to defer to it. No decision file.

## Design

- Grep is runtime case filtering (`TJAA-V0-014`), so a grep-explained pair stays a unit: the
  receipt still describes file membership and reconciles with static selection.
- The producer rebuilds each listed case's grep title from the JSON report (project, file suite,
  describe titles, test title, `@`-prefixed tags), as Playwright 1.63 `_grepTitleWithTags` builds it
  (read from the package source; no live Playwright run). A pattern that can neither match a space
  nor anchor to the whole title matches the joined title exactly when it matches one part, so
  evaluation is per part and avoids reproducing the exact separator layout.
- A static, partwise-evaluable pattern explains an omission only when it excludes every listed
  case of the file; anything else that is evaluable still refuses, with a message naming grep.
- A dynamic or non-partwise pattern cannot be decided, so the pair is kept and recorded in the new
  optional `unverifiedUnits` member, which a `MATCHED` plan surfaces as the `EXECUTION` unknown
  `playwright:discovery-omission-unverified`. A dynamic grep also leaves the existing config-syntax
  unknown, so the plan stays `STATIC_UNRESOLVED`.

## Limits

- Per-unit argv for a grep-only (project, file) unit can report "No tests found" when run alone;
  excluding such units would change `TJAA-V0-014` and needs an owner decision.
- The per-test listing producer (`playwright_listed_tests.go`) keeps the strict omission check.
- A top-level `grep` remains outside the static subset; a tag containing whitespace is read as
  the listing reports it.
- Evidence is hand-built listings in `playwright_grep_omission_test.go`; a live Playwright
  listing of the issue layout is `NOT_RUN`.
