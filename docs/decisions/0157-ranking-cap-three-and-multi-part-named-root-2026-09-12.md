# Decision 0157 — Admit three implementations per feature; drop the frontier only for a multi-part named root

Date: 2026-09-12. Status: accepted, delegated owner call. Authority: the coordinator's delegation to
promote the measured change in `docs/plans/ranking-cap-frontier-2026-09-12.md`.

## Decision

1. `EvalQuery` admits **three** implementation declarations per competitive feature record
   (`featureImplementationCandidates(…, 3)`), whatever `--limit` is. Amends `GPK-V0-040`.
2. `evalLinkedSymbols` drops the remaining confident frontier after the promoted dependency links
   only when the root declaration's identifier splits into **two or more** parts and the compacted
   task contains its compacted name. A one-part root such as `kill` keeps the frontier. Amends
   `GPK-V0-047`.
3. The oracle's pre-amendment behavior is registered as `DR-0036` (`python-defect`, known-divergent);
   `src/` is absent and unrepaired. `analyzerSchemaID` moves `corvint-analyzer/34` -> `/35`.

## Linked-root form: why the narrow one

Three forms were considered: the plan's full keep (never drop), a one-word exemption (this decision),
and the unchanged drop. On the corpus the drop fires in exactly one case: `execa-kill-descendants`,
task "terminate a subprocess and kill all descendant processes", root `lib/methods/main-async.js:kill`,
one part. So full keep and the narrow form are indistinguishable there (both 64/86, below). The narrow
form is chosen because it removes the rule only where the evidence shows it misfires, an ordinary task
word matching a one-word name, and keeps it where naming is unambiguous (`killProcess` written in the
task). `TestEvalLinkedSymbolsDropsTheFrontierOnlyForAMultiPartNamedRoot` discriminates both
alternatives: its one-part subtest fails against the old any-name drop, and its multi-part subtest fails
against a never-drop candidate (mutation-checked). No corpus case yet shows the multi-part drop is
protective; that remains unmeasured.

## Measured evidence

Harness `experiments/ranking-calibration/rankcap_harness_test.go.txt`, 23 frozen `query` cases on the 5
`benchmarks/manifest.json` repositories at their pinned commits, each case's own limit and budget.

| Variant | gold | must | critical miss | top-5 | precision | packets bytes / selectors |
|---|---|---|---|---|---|---|
| baseline (cap 2, any-name drop) | 60/86 | 25/25 | 0/22 | 23/23 | 0.6968 | — |
| cap 3 + never drop | 64/86 | 25/25 | 0/22 | 23/23 | 0.6965 | 4 / 2 |
| cap 3 + multi-part drop (chosen) | 64/86 | 25/25 | 0/22 | 23/23 | 0.6965 | 4 / 2 |
| multi-part drop alone (cap 2) | 63/86 | 25/25 | 0/22 | 23/23 | 0.6943 | 1 / 1 |

States, budget 23/23, abstention 5/5 and case checks 23/23 are unchanged in every row. After the
production change the harness was re-run with its baseline set to cap 3 plus the multi-part rule; its
parity assertion held (byte-identical to the modified `EvalQuery` on all 23 cases) at 64/86, so the
shipped code is the measured variant. Caveat carried from the plan: both gaining cases are in the
already-observed heldout partition, so +4 is in-sample evidence from 2 cases.

## Packet review

- `execa-kill-descendants` (`results`, `coverage`): better. The frontier kept after the `kill` root's
  links adds the gold rows `killDescendantsUnix`, `killDescendantsWindows` and `subprocessKill`, the
  code that actually kills descendants; no must or critical row moves.
- `natural-language-session-revocation` (`results`, `coverage`): better. The third implementation slot
  admits gold `internal/auth/auth.go:scopeFromCookies`.
- `ambiguous-reveal-navigation` and `blind-beamfall-immediate-hard-delete-abstention` (`coverage`
  only): neutral. Emitted selectors are unchanged; the counts grow because the admitted universe is
  larger, which is what `GPK-V0-040` requires them to report.

## Parity and reports

`cli-parity-v0` replay of `--only query-` (22 PASS, 5 PASS-WITH-KNOWN-DIVERGENCE, 2 UNSUPPORTED) and
`--only harness-` (19 PASS, 1 PASS-WITH-KNOWN-DIVERGENCE) against a worktree build: no FAIL, so no
frozen expectation changes and no parity row discriminates either amendment (owed by `DR-0036`).
Re-capture of the dated `benchmarks/results/v4-development-go*.json` reports is NOT_RUN: their capture
runner (`benchmarks/run.py --engine corvint`, cited by the V4 status record) is not in the tree.

## Rollback

Revert the commit: the cap returns to 2, the any-name drop returns, and `analyzerSchemaID` returns to
`/34`, which invalidates `/35` snapshots so indexes rebuild. No persisted or wire format changes.
