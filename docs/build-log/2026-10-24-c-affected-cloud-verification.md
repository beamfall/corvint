# 2026-10-24 cloud lane C: affected/retrieval P1 tickets already on main

Lane `c-affected` (cloud), base origin/main `0949876e`. Tickets V1-2467, V1-2468, V1-2536 and
V1-2295. This entry records independent verification. It changes no code, spec, requirement or
baseline. Every ticket's fix or attribution had already merged before this lane started. The
orchestrator owns closing the tickets.

## Findings

| ticket | landed on main | requirement IDs | verified here (linux/cloud, go1.27.3) |
|---|---|---|---|
| V1-2467 (GH #1593) | `3ccbea4b`, review fix `53d83c45`, merged by #1596 | TJAA-V0-038..040, TJAA-V0-045 | `go test -run 'GH1593\|GH1594\|PR1596' ./internal/liveverify/affected/typescript/` ok. End-to-end `corvint affected` on a scratch Playwright repo whose helper has `s.replace(/[^/]+/, '').replace('/x', '')`: no `typescript:dynamic-import` or `typescript:scan-unresolved`. The selection narrowed to the dirty helper and its one importing spec, with the other spec excluded. Each `typescript:*` frontier detail names `path:line`. |
| V1-2468 (GH #1594) | same commits and PR | TJAA-V0-041..044 | Same test run. The same scratch config had a `'global-setup'` project, `process.env.TZ ??= 'UTC'`, top-level `require('dotenv').config()`, `process.env['HOSTNAME'] \|\| '…'`, an `=== 'true'` ternary, `Number.parseInt(process.env.X \|\| '60000')` and `executablePath: process.env.X`. It raised no `playwright:browser-identity-unresolved`. |
| V1-2536 | `cad1682b` (LQP-V0-022), merged through batch-1024c (#1684) | LQP-V0-022 | Root cause with code evidence is in `2026-10-24-lsp-combined-relation-states.md`: a `sourceBase`-built CLI uses the lexical `InlineSection` order, and `--limit 20` cuts the gold row at index 21 of 32. `python3 script/measure-lsp-quality.py --self-check` PASS. It needs the frozen `sourceBase` `cf93522e` fetched into a shallow clone first. |
| V1-2295 | attribution `2026-10-21-recall-1021s-pair-budget-anchor-attribution.md` (#1515), decision `2026-10-21-anchors-default-stays-on.md` (`b83a884a`) | none | Read only. Frozen bench numbers not re-measured here (see NOT_RUN). |

V1-2295 attribution as recorded, frozen Agent Retrieval Bench v2: trace2code r20 0.7937
(`242fde6`) rose to 0.8036 (`8f4b866`), from gin `ddc4c496` leaving abstention. It fell to 0.7690
at `b8fe75d`, the #444 merge of the TCP-V0-022 anchors default flip `366c3756a`. `b8fe75d` with
`CORVINT_CONTEXT_ANCHORS=off` gives 0.8036, which is its first parent `d761a403f` exactly. The
further step to 0.7640 is gin `2097c0a5` losing `path.go` at `76f7f2ac4` (#639), outside the
range. The diffusers `c7ea7d6f` code2test docs rows also first appear at `76f7f2ac4` (#639,
V1-0859 documentation share). The re-baseline was decided by the orchestrator under the owner's
delegation, and the owner can revisit it.

## NOT_RUN

- V1-2536 WSL2 r20 rerun (NEEDS-HOST): no gopls or WSL2 host here. Command, per
  `benchmarks/lsp-quality/README.md` with the CLI built from the candidate tree:
  `python3 script/measure-lsp-quality.py --manifest benchmarks/lsp-quality/public-v0.json --root <clean cf93522e worktree> --corvint <candidate corvint> --gopls <gopls v0.23.0> --output <out.json> --repeat 20`;
  expect `allGoldPassed=true`.
- V1-2295 frozen bench rerun: the bench JSONL and corpus snapshots are not in the container, and
  `tools/retrieval-bench` downloads nothing.
- Dogfood targets (cloud), `make gate`, `windows-vet` (no Go file touched).

## Rollback

Delete this file.
