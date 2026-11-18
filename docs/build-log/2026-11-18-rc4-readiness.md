# 2026-11-18: v1.0.0-rc.4 readiness record (RCF-V1-003), first pass

Requirements: RCF-V1-003. Change: `rc4-candidate`, task 4.2.

## Result

HOLD. The tag `v1.0.0-rc.4` MUST NOT be pushed: six rows are `NOT_RUN` and one is `FAIL`. No candidate
commit is chosen yet. This pass reads `origin/main` at `55decc6a0` plus branch
`claude/rc4-qual-run` (promotion commits `101d0951a`, `e3f988b98`, `9c7e969a9`). It must be
redone on the candidate commit before the tag. No row is `WAIVED`, because the owner has waived
nothing for rc.4.

| # | Item | Status | Evidence or blocker |
| --- | --- | --- | --- |
| 1 | M4 merged (H7) | NOT_RUN | M4, the layering and release merge, is planned for about 2026-12-08 (PRODUCT-PLAN-1.0 milestone M4). It has not landed on `origin/main`. |
| 2 | Removals done | NOT_RUN | The `source-removal` change has 1 of 17 tasks done. |
| 3 | Flow links signed (H6) | NOT_RUN | The `flow-link-proposals` change has 0 of 8 tasks done. H6 slips the tag until the links are signed. |
| 4 | Held-out pools sealed (PRR-V1-004) | NOT_RUN | There is no seal record. The draw is seeded by the tag commit (H3), so sealing comes just before the tag. |
| 5 | Release gates pass on the commit (V1-3177, V1-3055, V1-4850) | FAIL | All three tickets are OPEN. V1-3177's AC2, the N-1 row against a published predecessor, can run now that rc.3 is published, but has not run. V1-3055 and V1-4850: nightly `full-gate` is red on macos-15 and ubuntu-24.04, and nothing shows it green. |
| 6 | Tag workflow build job (REL-V1-005) | PASS | The `build` job in `.github/workflows/release-binaries.yml` built and published rc.3 (run 44356855906, attempt 2). It must run again for the rc.4 commit. |
| 7 | Three-target `binaries.json` (REL-V1-008) | PASS | `tools/release-binary/binaries.json` lists darwin/arm64, linux/amd64 and linux/arm64. |
| 8 | Core-set archives installable by `corvint-update` (V1-5473) | PASS | Live proof 2026-11-18 on darwin/arm64: `corvint-update apply --allow-network --component core` against published rc.3 replaced `corvint`, `corvint-docs`, `corvint-docs-mcp` and `corvint-mcp` (`switched=true`, freshness CURRENT; publisher identity NOT_VERIFIED). The ticket is IN_REVIEW. |
| 9 | RCF-D1 applied; V1-5545 retired | PASS | Task 4.1 is done: MTT-V1-010 is superseded and the three archives are kept. V1-5545 is COMPLETED. |
| 10 | Candidate-byte linux/amd64 qualification workflow (V1-0374) | NOT_RUN | V1-0374 is OPEN with no workflow delivered. |
| 11 | Commit authorship on `origin/main` (V1-5621) | PASS | The history rewrite was force-pushed on 2026-11-18 with owner approval ("Rewrite and force-push"). The backup ref is `backup/main-pre-authorfix-2026-11-18`. The ticket stays OPEN for closeout. |
| 12 | RCF-V1-006 to RCF-V1-011 delivered | PASS | See `2026-11-18-rc4-docs-qualification.md`: the qualification, the DCS-V1-404 twin run, the rerun trigger, the schema and format pins, the promotion, and the revert drill (RCF-V1-012). Task 5.1 is closed: three findings fixed, and P1-3 is rescoped to V1-5692 and held by row 13. |
| 13 | Qualification current for the candidate (RCF-V1-010) | NOT_RUN | Run the three-repository qualification on the candidate commit, or show that `git diff --stat 1f35e9f28 <candidate> -- cmd/corvint-docs internal/doccorpus internal/mcp` touches no ranking, corpus build, corpus format, verb or tool code. The selected unit tests skip the live run without `CORVINT_RCF_QUALIFY`, so a green affected run does not prove this row (independent review 5.1, P1-3). The repaired runner passed twice at `0bc3087bd` with identical reports (`51800e0d…`); see `2026-11-18-rc4-docs-qualification.md`. The row turns PASS once the chosen candidate shows no trigger-path diff from that commit. |

## What frees the tag

- Rows 1 to 4 follow the M4 and M5a milestone plan, about 2026-12-08. They are not rc.4-specific
  work. If they are not delivered, only an owner waiver, quoted with its date, can turn a row to
  `WAIVED`.
- Row 5 needs a green `release-gates.yml` run on the candidate commit, or an owner waiver per
  ticket.
- Row 10 needs V1-0374 delivered, or an owner waiver.
- Row 13 requires `CORVINT_RCF_QUALIFY` to name all three pinned checkouts and their artifacts,
  a PASS from `TestRCFV1008Qualification` using the repaired verifier on the candidate inputs,
  and two passing DCS-V1-404 runs with identical ranks. Missing configuration, skipped runs,
  missing evidence, or a later RCF-V1-010 trigger change keeps this row `NOT_RUN` and the
  candidate gate HOLD. A successful selected-package run alone cannot free this row.
