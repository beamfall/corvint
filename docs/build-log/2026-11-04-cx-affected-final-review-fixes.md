<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4544 / V1-4549: final review corrections

Base: `6c33df33bc5670131482f5263b410cb980580657`. Round starts at
`5bde9ab1b3d57b9bbe42dacda316919975056158`. This entry supersedes the build-log exclusion
and acceptance claims in `2026-11-04-cx-affected-precision-and-flakes.md`; earlier replay counts
remain historical observations of an unsound exclusion, not evidence of safe narrowing.
The owner's final-round instruction accepts conservative over-selection. No new design profile.

## P1: withdraw the unsound build-log exclusion (TSP-V0-1061)

Decision: remove `docs/build-log/` from both `.corvint/unbounded-reader-exclusions.json` and
`.corvint/runtime-readers.json`. The old reason was false: `TestReferenceScannersMatchTheirRegexes`
in `internal/contextindex/reference_scan_test.go` resolves `../..`, runs `git ls-files -z` without
a path filter, and reads every listed regular file within `maxSourceBytes`. Dated Markdown
records are actual test inputs. Restore the root-locator floor rather than infer narrower scopes.
TSP-V0-1061 now requires that conservative behavior. The regression requires contextindex and
its UNBOUNDED_READER witness alongside cmd/corvint and flow-guide-check. No test is suppressed.

Exclusion audit against the requested BASE:

| Entry | Disposition | Original evidence |
|---|---|---|
| `docs/build-log/` | REMOVED from both declarations | The only exclusion added by this lane (`git diff BASE 5bde9ab1b -- .corvint/unbounded-reader-exclusions.json .corvint/runtime-readers.json`); the contextindex self-corpus test reads it. |
| `.corvint/change.cem.json` | Pre-existing, unchanged; no lane addition retained | Present at BASE in both declarations. This review does not introduce or renew its historical safety claim. |
| `.corvint/changes/` | Pre-existing, unchanged; no lane addition retained | Present at BASE in both declarations. This review does not introduce or renew its historical safety claim. |

The other flagged root resolvers were inspected, without turning their narrower behavior into a
blanket claim about every reader. `internal/cemchain/chain_test.go` resolves `../..` in both chain
tests to build the Core, Tasks, runner and portable consumer; subsequent CEM and artifact reads
use `newCheckout`'s disposable root. `internal/opencodequalification/process_test.go` resolves
`../..` to build `tools/qualify-opencode` and runs its probe with a temporary root.
`internal/postmergehost/launcher_test.go` resolves `../..` to build the paired author; its copied
fixture, product and control roots supply subsequent file reads.
`tools/test-exec-writes-check/main_test.go` calls `run("../..", ...)`; that implementation in
`main.go` lists only `*_test.go` with `git ls-files -z -- *_test.go` and reads those paths.
None of these observations repairs the demonstrated contextindex omission. No new exclusion
survives this round. Rollback must not restore the withdrawn exclusion.

V1-4544 remains **PARTIAL**: AC1's removal of the build-log floor is withdrawn because it misses
an actual reader; AC2's declared-consumer explanation is retained and corrected below; AC3's
mixed-union reduction remains unmet (V1-4535). Safe omission cannot be established by a reduced
unit count. The earlier isolated 35-to-11 reduction is explicitly not accepted safety evidence.

## P2.1: name a changed shell declaration (TSP-V0-1062)

`readScopeCause` now handles `ShellReadScopesPath` before matching an entry, mirroring the Go
declaration branch. Both `declaration` and `entry` name `.corvint/shell-read-scopes.json`, rather
than presenting the shell unit's own directory as the cause. The fifth table case in
`TestReadScopeCausePairsDeclarationAndEntry_TSPV01062` pins that exact pair. Selection is unchanged.
Rollback removes that branch and reintroduces misleading attribution, so it is not recommended.

## P2.2: unresolved detached failures (V1-4549 AC2)

V1-4549 is **PARTIAL**, including **AC2 PARTIAL**, not just the future hosted-report criterion.
The two original detached-process failures have no established cause or qualifying repair:

- `TestATRV0059_ClaimCarriesDrainNotice`: follow-up [V1-4637](ticket:corvint:main:V1-4637).
- `TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck`: follow-up
  [V1-4638](ticket:corvint:main:V1-4638).

These are native ticket identities supplied by the owner. Both the local projection and staged
read-only queue snapshot predate them; ticket bodies/liveness are NOT_OBSERVED here. No Tasks
writes were attempted. The prior race repetitions passed but do not explain SUPERVISOR_LOST;
neither test is quarantined, skipped, removed, serialized or relabelled fixed in this round.
The next weekly hosted report also remains NOT_OBSERVED. Earlier lease, PID publication and
service-home fixes and their retained qualifications remain unchanged.

## Verification and limits

Evidence for this round is under `/tmp/cx-affected-evidence/fix2-*`. The rebuilt binary's
`affected --base 6c33df33bc5670131482f5263b410cb980580657 --format text` selected both test
packages before execution: UNKNOWN scope, 164 selected, 86 unknowns at the dirty implementation
step. `--base` includes the whole original lane; this is not a full validation of that selection.
All Go tests use `TMPDIR=/tmp HEAVY_LOCK_DIR=/tmp/corvint-heavy-1000 script/heavy.sh`, one package
per command, GOTOOLCHAIN=local (go1.27.3):

- `go test -count=1 -timeout 30m -run 'TSPV0106|ReadScopeCause|NewBuildLog' ./internal/liveverify/affected/`
- `go test -count=1 -timeout 30m -run TestReferenceScannersMatchTheirRegexes ./internal/contextindex/`

Final results, clean-HEAD no-cache path replay, CEM inspection and dogfood-check are retained in
the lane handback; no passing result is inferred before its command finishes. The staged spec
was followed by `make spec-requirements`. Required focused gates are
`make traceability-tests-check line-citations-check unbounded-readers-check`.

Self-development routes: **used** the existing bound prechange query/impact receipts at the same
BASE, rebuilt affected selection, spec traceability and CEM/OCM dogfood binding/checking.
**Deferred** independent review to the orchestrator by explicit instruction. **Not applicable**
retrieval/ranking/learning frozen evaluations, external interoperability and provider profiles:
no changes to those contracts. Optional unconfined guarding tests are NOT_PRODUCED; focused
execution is separate evidence. Exhaustive gate, unrelated selected units, publication, native
completion and seal are NOT_RUN by lane instruction. Both tickets stay open; a bound CEM does
not establish acceptance of their unresolved criteria.

Friction newly observed in this round:

1. A bare start `make dogfood-change BASE=...` retains bound prechange query/impact but exits
   not-complete until citation, intent and verification inputs are supplied. It is not the final bind.
2. Installed `corvint affected --base ...` warns self-binary-stale (28 Go source paths at the
   starting head). Build this checkout to scratch and repeat selection before tests.
3. `corvint-tasks ticket show V1-4637` / V1-4638 refuses absent local projection/journal;
   retry with the offered CORVINT_TASKS_SNAPSHOT still refuses because the snapshot predates
   both tickets. Preserve owner-supplied references and NOT_OBSERVED bodies; do not create them.
