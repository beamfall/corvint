# Dogfood flow lane, wave 2 (2026-10-31)

Lane `dogfoodflow` on the WSL lane host, base `6c967b21caa2ffb0324a681f873a891996d82e34`, branch
`claude/lane-dogfoodflow-w2-1031`. Fifteen tickets in priority order. New requirements:
`DCW-V0-880` to `DCW-V0-887`, `OCM-V0-880`, `LCP-V0-880` to `LCP-V0-885`, each an experimental
amendment under the standing lane auto-accept, with its spec commit before its implementing commit.

## Decisions

- V1-3183 (pre-change receipts). `dogfood change` must not write the agent receipts (`DCW-V0-026`,
  accepted), so capture is a separate recorder, `corvint dogfood prechange [--task TEXT] BASE
  [PATH...]` (`make dogfood-prechange`). It refuses once HEAD's tree is not BASE's or a tracked file
  is modified (no retrospective receipt), never overwrites a receipt it did not write, and writes
  `prechange-binding.json` naming BASE, its tree, the Git directory, the task and each receipt's
  digest and satisfying command. `dogfood change` then notes each bound receipt `OBSERVED bound`, or
  why not: `agent-receipt-unsupported tool=...` (a context receipt saved as the query receipt),
  `agent-receipt-binding-mismatch`, `agent-receipt-binding-other-base`,
  `agent-receipt-wrong-worktree`, `agent-receipt-binding-malformed`. The notes stay non-blocking and
  out of the report, because `dogfood check` reproduces the report from a clean worktree that has
  no private receipts. Every finish-lane pass prints them, so closeout shows them with no file
  forwarding.
- V1-3290. The intents grammar check is split out as `readIntentManifest` (no Git read) and
  finish-lane/inputs-check run it right after the verify input check, before any step, with an
  `LC_ALL=C sort -u -o FILE FILE` fix line for an ordering fault (`DCW-V0-882`).
- V1-3285. Confirmed, and the reproduction was the accepted `DCW-V0-037` fixture: its two stages
  cited `intent.md:5:5` for all 279 hunks and bound. `carriedUniform` refuses a stage that completes
  one span over hunks already citing only that span (`DCW-V0-883`); the fixture now cites two spans.
- V1-3289. A re-keyed unknown row's NOTE is carried prefixed `[stale: re-keyed from OLD, review]`,
  replacing an earlier mark (`DCW-V0-884`). Regenerating the author's note was rejected: it would
  discard the author's reason.
- V1-3293. Without a citation-plan binding, `--refresh` resolves ordinal rows through the one
  sealed CEM of `BASE..HEAD` whose hunks hold every ordinal row (span among basis evidence, or the
  unknown reason); the existing re-key then maps them, and only unmapped rows are dropped
  (`DCW-V0-885`). Two matching seals, or none, leave the rows unresolved.
- V1-3283. Path anchors are ranked: a requirement-bearing line of a governing spec, any line of one,
  a requirement-bearing line elsewhere, a bare mention. Governing specs are those whose
  requirement-bearing lines name the most distinct owed paths, counting specs the change modifies
  (never cited). A first count over all lines picked the unrelated spec in the fixture, so only
  requirement-bearing lines count. A cite outside every governing spec is kept with a
  `cited low-confidence (DCW-V0-886)` note (`DCW-V0-886`). Self-use on this lane's own change then
  named `affected-plan-v0.md` governing, because it names many paths, and showed that `git grep -o`
  hands the matcher only the match, so modified specs were never counted. Both were fixed: the
  BASE line is read, and when a modified spec names an owed path only modified specs compete.
- V1-2083. Reproduced by reading `docs/specs/test-source-analysis-v0.md`: `TSA-V0-410..419` sit
  under `## Example receipt join...` (line 2708), outside the one `## Requirements` section
  (29..1063), so `OCM-V0-001` enumerates none of them. The change coverage now also counts a
  requirement line the diff adds outside the section, traced by tests or table rows, and the check
  names them in `NOTE requirements-outside-ocm-section` (`OCM-V0-880`). Making them OCM obligations
  would amend `OCM-V0-001` and is left out.
- V1-2978. A passing inputs-check leaves `<git-dir>/corvint/inputs-check-pass` keyed by BASE, HEAD,
  tree, inputs, outcome, staging and the executable digest; the next finish-lane consumes it and,
  on a match, replays the prepare line with `reused=inputs-check` instead of preparing again
  (`DCW-V0-887`).
- V1-0581. A coordinator preservation hold (`dogfood hold --reason`) makes `dogfood cancel` of the
  held ACTIVE enrollment refuse `enrollment-preservation-hold`, naming the distinct-checkout route;
  only a release with `--release-authorization`, kept as a record, admits the cancel
  (`LCP-V0-880..882`). Hold files sit in `local-completion/holds/`, outside the session directory,
  whose bounded entries count generations.
- V1-0580. `dogfood export --output FILE` and `dogfood import --input FILE` carry an enrollment
  across an archive and restore: the bundle holds every session file and every artifact the state
  names, bound to HEAD's commit and tree; import refuses a moved HEAD, an existing enrollment,
  another owner or conflicting bytes, relocates saved paths into the new Git directory and
  re-digests the report set only when it held at the source (`LCP-V0-883..885`). Aggregate-outcome
  and sealed-continuation enrollments are refused for export: their ledgers name paths the bundle
  does not relocate.

## Evidence for measurement and already-delivered tickets

- V1-2810: seal reuse of a bound passing check (`DCW-V0-347`) and hunk-ID templates without a probe
  bind (`DCW-V0-350`) are on the base. This lane's closeout timing is recorded below.
- V1-2360: `propose-citations` on this lane's own change (71 hunks; this host, WSL, 32 cores, load
  about 1): 1.85, 1.86, 1.91 s without a map and 1.86, 1.87, 1.88 s with a prepared map (median
  1.87 s, under the 2 s bound for about 30 hunks). Before: the ticket's reported 13 s (Mac). No row
  falls back below an anchor: of 71 rows, 2 cite and 69 are unknown with a typed note (20 "no stable
  BASE line of a file this change leaves untouched names PATH", the rest naming a same-change
  requirement ID, `DCW-V0-440`).
- V1-1482: not reproduced. `internal/dogfoodflow` whole: 211 tests, package 19.6 s at load about 1;
  22.1 s pinned to one CPU with `GOMAXPROCS=1` as a contention proxy. The slowest tests:
  `TestBatchCheckCEMV1203RetiredSealHasNoCoverage` 0.68 s (0.80 s pinned),
  `TestDCWV0379InputsCheckRefusesEachPlanAndManifestDefect` 0.51 s,
  `TestProposeCitationsDCWV0441V0442RefreshRekeysReHunkedNewFile` 0.50 s;
  `TestCheckDCWV0047DeclaredOutputChange` 0.12 s. The package is about 90 times inside its 30 m
  budget. A run at load about 60 was not made: it would load a host other lanes share.
- V1-1645: not reproduced. Ten `corvint impact --base 6c967b21c --range-profile expanded-256
  --limit 20` runs on this lane's clean committed tree were byte-identical (`BUDGETED`, same
  SHA-256). `impact-range-drift` is raised only when the worktree status or identity changes during
  the read (`verifyRangeSnapshot`), so another process writing an untracked file during the rebind
  remains the likely cause; nothing here observed one.
- V1-3356: the base verifier build is cached by root tree (`dogfood_build_base_corvint`), and the
  guarding-tests row records why it is not produced (`NOTE guarding-tests NOT_PRODUCED
  guarding-tests-not-run`, naming the opt-in command, which runs tests unconfined for up to 1200 s).
  This lane's warm closeout time is recorded below; the Mac time is `NOT_OBSERVED` here.
- V1-3183 real change: in a scratch worktree at the base, `corvint dogfood prechange --task "Capture
  and bind pre-change agent receipts in the enrolled dogfood workflow" 6c967b21c
  internal/dogfoodflow/change.go` (lane build of commit `6cd9fce70`) recorded
  `prechange-query sha256=3d7e041f...` and `prechange-impact sha256=fb945f8e...` in 6.3 s; after a
  commit, `dogfood change` printed `NOTE prechange-query OBSERVED bound sha256=3d7e041f...
  command=corvint query` and the same for impact, and a second `prechange` refused
  `prechange-after-change`. This lane's own receipts were not recorded: the recorder did not exist
  when the lane started, and none may be written retrospectively.

## Closeout

Recorded after the seal; see the lane report.

## Friction

- The WSL host's `/tmp` (16 GB tmpfs) filled up during the lane: 14 GB is the shared
  `/tmp/corvint-go-build-cache`, the Makefile's default `CORVINT_GOCACHE`. Links, `go run` and
  `t.TempDir` then failed with ENOSPC. Repro: `df -h /tmp; du -sh /tmp/corvint-go-build-cache`.
  The lane ran with `CORVINT_GOCACHE`, `GOTMPDIR` and `TMPDIR` under `~/.cache` instead.
- `make capability-catalog` writes each catalog with a shell redirect, so a failed `go run` (here
  ENOSPC) leaves `internal/capcatalog/catalog.json` empty, and a following `git add` committed an
  empty catalog once. Repro: fill the temp volume, run `make capability-catalog`, check
  `wc -c internal/capcatalog/catalog.json`.
- `~/lanes/heavy.sh` refuses a whole-package run without `HEAVY_WHOLE_REASON`, and `corvint test
  --dry-run` then selects 189 packages for a dogfoodflow change (BINARY_EXEC and PATH_LITERAL_READER
  via `Makefile`, `catalog.json`, `REQUIREMENTS.tsv`), so the focused package needs a hand-written
  reason. Repro: `corvint test --base 6c967b21c --dry-run` on this branch.
- `heavy.sh` copies its index to `${TMPDIR:-/tmp}/heavy-index.*`; on the full `/tmp` it printed `cp:
  error writing ... No space left on device` and the test it wrapped still ran, so the warning is
  easy to miss. Repro: run `~/lanes/heavy.sh` with `/tmp` full and `TMPDIR` unset.
