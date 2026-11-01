# Lane spectool-sp2: spec tooling friction (2026-11-01)

Branch `claude/lane-spectool-sp2-1101`, base `origin/main` `8daec98ce4349c1583186692726d2c3a782ac647`,
Unraid host. Requirement block 950..954 in each governing prefix (`SRG-V0`, `TTG-V0`, `ASD-V0`,
`OCM-V0`), each checked free with `script/requirement-ids-free.sh PREFIX 950 954 origin/main` and on
every local branch before use.

## Problem

Fourteen v1-0 tickets from lane friction reports about the spec tooling: the requirement-index
generator refused one problem per run, named neither the frozen header line nor where a new range
belongs, misread note paragraphs and continuation-line notes, had no `--check` mode, and only warned
about unstaged specs (V1-3575, V1-3602, V1-3563, V1-3553, V1-3617, V1-3510); the traceability gate
failed the spec-first commit order (V1-3586); parallel lanes collided on requirement IDs with no
check over unmerged local work (V1-2909, V1-1009, V1-3614); `dogfood-check` called requirements
untraced although a range row and `_TCPV0991` test names traced them (V1-3455); `ocm link` erased a
requirement's first test file when a second was linked (V1-0585); a build-log citation of a
nested-module test did not reproduce (V1-3512); and the experimental-ID list looked authored in three
places (V1-3593).

## Decisions

- One refusal pass (SRG-V0-951): every check in `script/gen-spec-requirements.sh` now pushes onto
  `@refusals` with its category's repair, printed once after the lines; the projection-header checks
  moved before the single exit so `--index`/`--readme`/`--all` report them with the rest. A refusal
  that leaves nothing to project (an unreadable INDEX.json or README table, a new spec without a
  title) still dies at once.
- Frozen-line refusals (SRG-V0-950) name `path:line` of the header or digest line and say a new range
  belongs on each requirement's definition line, the acceptance and traceability rows and the build
  log. The charter-side rule lives in `docs/LANE-ORCHESTRATION.md` section 3, the repository's
  charter guidance; the orchestrator's `~/lanes/charter-*.md` templates are outside the repository.
- Note paragraphs (SRG-V0-952): an indented, unbulleted paragraph after a blank line while a list
  item is open ranks like an SRG-V0-548 wrapped citation. Chosen over "any indented line" so an
  indented block outside a list still counts. The committed corpus regenerates byte-identically.
- Continuation notes (SRG-V0-953): refused rather than read, consistent with SRG-V0-046 (only the
  definition line is read); the refusal names the continuation `path:line`.
- `--write`/`--check` (SRG-V0-954): two new modes rather than turning the `--all` warning into a
  refusal, because `--all` has other callers (`spec-requirements-resolve.sh`, the dogfood snapshot,
  `tools/dogfood-inherit-citations`). `make spec-requirements` and `spec-requirements-check` call
  them; `--write` refuses modified-unstaged and untracked specs. The `SPEC_GENERATE` Makefile
  variable is gone.
- Traceability range (TTG-V0-950): `--range BASE..HEAD` reads each first-parent commit's specs
  against HEAD's test functions and names a missing test once, at its first citing commit. The index
  check is unchanged apart from a hint naming the route; `(PLANNED)` stays valid.
- ID collisions (ASD-V0-950..952): a new `script/requirement-id-collisions.sh` instead of extending
  `script/requirement-ids-free.sh`, which the lane-scripts lane owns; the only shared-file edit is one
  added recipe line in the `requirement-ids-free` Makefile target (the `--block` check). New
  definitions are compared through each ref's committed `REQUIREMENTS.tsv` (cheap, about 6 s with the
  generator for this checkout) and this checkout's generator view of its index, so a lane can run it
  before committing. Branches without shared history (the pre-snapshot lineage) are skipped. The
  test runs inside `make spec-requirements-test` to avoid editing `GATE_STEPS`.
- `dogfood-check` traced forms (OCM-V0-950): range tokens with the ID's own prefix and test-name
  spellings `TCPV0991`/`TCP_V0_991` bounded so `XTCPV0991` and `TCPV09910` do not match; the note
  ends with the accepted forms. Reproduced on the merged ctxpkt-w1 content (`task-context-packet-v0.md`
  row `TCP-V0-990..993`, `_TCPV0991` tests) as a fixture; the Mac lane head itself is not on this host.
- `ocm link` (OCM-V0-951): no new flag. A link replaces only its own test path's earlier claims and
  keeps other paths' claims (with the union of hunks while any are kept), so the CLI surface, the
  cli-parity manifest and the capability catalog do not change. Removing a kept claim is `ocm mark`
  then relink.
- V1-3512: the convention paragraph went into `docs/SPEC-DRIVEN-DEVELOPMENT.md`; the ticket's AC2
  required editing one line of the historical entry `docs/build-log/2026-10-31-linux-w2-bugs.md`
  (line 88), which the charter otherwise forbids; only the citation was changed.

## Evidence

- `script/gen-spec-requirements_test.sh` cases 13-16 (SRG-V0-950..954): ok.
- `script/spec-requirements-conflict_test.sh` (refusal now carries `:LINE`), `spec-requirements-worktree-check_test.sh`,
  `check-requirement-definitions_test.sh`: ok.
- `script/check-traceability-tests_test.sh` (TTG-V0-950 spec-first fixture): ok; `script/check-traceability-tests.sh --range 8daec98ce..HEAD`: pass.
- `script/requirement-id-collisions_test.sh` (ASD-V0-032/036 and batch C replays, block check): ok;
  on this host `make requirement-id-collisions` reports `no-collision 6 new IDs here, 11 open refs`.
- `go test` (through `~/lanes/heavy.sh`): `internal/dogfoodflow` (with `TestOCMV0950TracesTestNameSuffixesAndTableRanges`),
  `internal/lrfrepo` (with `TestOCMV0951LinkKeepsOtherTestPaths`), `internal/dogfoodocm`, `internal/specindex`,
  `conformance/ocm-v0`: ok.
- `(cd tools/local-authority && go test -run '^TestPLEV0013CodecPackageAdmitsTestdata$' .)`: ok.
- `script/lane-scripts_test.sh` (HEAVY_MAX_LOAD=100000): 35 groups passed; `script/dogfood-bind-range_test.sh`: ok.
- V1-3593 ALREADY-DELIVERED on the base: the header is the one authored place; README.md and INDEX.json
  rows are generated from it (SRG-V0-012, `make spec-requirements`), the digest `Status` is checked
  against it (SRG-V0-047, `TestSRGV0047StatusDriftNamesBothFieldsWithLines`), and new IDs may not be
  added to either line at all (SRG-V0-980).

## NOT_RUN

- `script/batch-precheck_test.sh` fails at case 1b on this host with `node: command not found`
  (a Pi adapter version case); no `node` on the Unraid host, unrelated to this change.
- `corvint affected` selected 100 units with scope `UNKNOWN`; per the charter only the focused units
  above ran. `cmd/corvint` and the remaining dependency-path and unbounded-reader units are `NOT_RUN`.

## Rollback

Each requirement block reverts with its commits: SRG-V0-950..954 (generator, test, Makefile recipes),
TTG-V0-950 (`--range`, `TRACE_RANGE`), ASD-V0-950..952 (new script, its test, two Makefile recipe
lines, the doc paragraphs), OCM-V0-950 (two matchers and the note tail), OCM-V0-951 (the
per-path branch of `linkObligation`). No generated row moves except the new requirement rows.

## Outcome

- Delivered: V1-3575, V1-3586, V1-3602, V1-3563, V1-3614, V1-3617, V1-2909, V1-1009, V1-0585,
  V1-3553, V1-3510, V1-3455, V1-3512.
- Already delivered: V1-3593 (evidence above).
- Dogfood: pre-change receipts recorded by `make dogfood-change`; inputs checked with
  `make dogfood-inputs-check`; the CEM is not sealed in this lane (the orchestrator seals after review).

## Friction

- `corvint` on this host is stale (`DEGRADED self-binary-stale ... built from f1680433b107`), so every
  `corvint affected` run warns; repro: `corvint affected --base 8daec98ce4349c1583186692726d2c3a782ac647`.
- `corvint affected` selects 100 units with scope `UNKNOWN` for a change whose Go edits sit in two
  packages, because a Makefile edit brings in every language frontier (`UNOWNED_DIRTY_PATH`, 86
  `LANGUAGE_FRONTIER`); repro: any change touching `Makefile`.
- `heavy.sh` refuses a whole-package run of a package `corvint affected` itself marked
  `FALLBACK EXISTING_DECLARATION_CHANGED` until `HEAVY_WHOLE_REASON` is retyped by hand; repro:
  `~/lanes/heavy.sh go test ./internal/dogfoodflow` after editing a test there.
- `script/batch-precheck_test.sh` hard-fails without `node` instead of skipping the Pi case; repro:
  run it on a host without Node.
- `ocm link` claim selectors are normalized (`keeps` becomes `keep`, lower case), so a test guessing
  the selector from the `t.Run` text misses; the miss message only explains `/case:` fragments.
- The charter forbids editing historical build-log entries while V1-3512 AC2 requires correcting one.
