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
- The producer reads each listed case's titles (file suite, describes, test) exactly from the JSON
  report. Playwright 1.63 (`_grepTitleWithTags`, `test.tags`, JSON reporter; read from the package
  source, no live run) puts only explicit tags in the grep title, while the report's tags also
  carry the `@\S+` runs of titles and split describe/config tags on whitespace. So the title runs
  are removed from the reported tags (Codex round 1 found the title-tag and whitespace-tag
  counterexamples). Codex round 2 showed a describe tag `@ smoke` is not reported at all yet
  appears in the grep title, so no negative `grep` decision is provable from a listing.
- An omission is explained only when `grepInvert` matches a title part of every case, which hidden
  tag text cannot undo. An evaluable grep that matches a listed title still refuses the listing.
- Every other exclusion (dynamic or non-partwise pattern, grep matching no title, exclusion via
  reported tags) keeps the pair and records it in the new
  optional `unverifiedUnits` member, which a `MATCHED` plan surfaces as the `EXECUTION` unknown
  `playwright:discovery-omission-unverified`. A dynamic grep also leaves the existing config-syntax
  unknown, so the plan stays `STATIC_UNRESOLVED`.

## Limits

- Per-unit argv for a grep-only (project, file) unit can report "No tests found" when run alone;
  excluding such units would change `TJAA-V0-014` and needs an owner decision.
- The per-test listing producer (`playwright_listed_tests.go`) keeps the strict omission check.
- A top-level `grep` remains outside the static subset. A case its explicit tags would keep is
  refused, the conservative outcome, although a partially reported tag could hide excluding text.
- Evidence is hand-built listings in `playwright_grep_omission_test.go`; a live Playwright
  listing of the issue layout is `NOT_RUN`.
