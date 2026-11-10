# 2026-11-09 lane seldisfix: review fixes for the selection-agreement lane

Lane seldisfix (WSL), branch `claude/sel-disagree`, on top of lane seldis's head `e14baa259`
(`docs/build-log/2026-11-09-seldis.md`). An independent review of seldis reported six findings;
each was checked against the code before any fix. New requirement IDs are the owning spec's next
free ID plus 1300: AFP-V0-11033 (`affected-plan-selection-v0.md`) and ASD-V0-5133 to ASD-V0-5135
(`corvint-self-development-selection-v0.md`).

## Dispositions

| Finding | Result | Requirement |
|---|---|---|
| 1 `affected jobs` and `affected audit` skip the stale-binary refusal | FIXED | AFP-V0-11033 |
| 2 pure deletion hunks and deleted test files narrow or vanish | FIXED | ASD-V0-5133 |
| 3 `$count == 0` runs before `eq "all"` | FIXED | ASD-V0-5133 |
| 4 hunk parsing needs the `b/` prefix and unquoted paths | FIXED | ASD-V0-5134 |
| 5 non-Go package inputs never run a package whole | FIXED | ASD-V0-5135 |
| 6 where the V1-4361 replay runs in CI is unrecorded | RECORDED (no code change) | AFP-V0-9732 amended |

## Finding 1: the jobs and audit forms emitted a stale binary's selection

Checked: `runAffectedJobs` (`cmd/corvint/affected_jobs.go`) called `compileAffected` and wrote the
job plan with no call to `degradeStaleAffected`; `runAffectedAudit` and `compileAffectedAudit`
(`cmd/corvint/affected_audit.go`) replayed each change with the binary's own selector likewise
unjudged. The review's wording "parsed but ignored" was not exact: both option parsers rejected
`--accept-stale-binary` as an unrecognized argument, so the flag could not be given at all. The
`--ownership-audit`, `discovery` and `cache` forms compute no selection and were left alone.

Fix: `parseAcceptStaleBinary` (`cmd/corvint/affected.go`) is the one parser of the flag, used by
the plan forms and both subcommands; `staleBinaryGate` (`cmd/corvint/self_binary.go`) is the
AFP-V0-9730 judgement for a document that is not an `affected-plan/1` receipt: the refusal (exit
2, `self-binary-stale`, the AFP-V0-846 detail, a `fix:` clause naming `go run ./cmd/corvint
affected jobs ...` or `... audit ...` and the flag) unless accepted, else the `degradations` and
`degradationDetails` members for the document and the stderr `DEGRADED` line when the streams
differ (AFP-V0-871). `jobs` gates after its plan compiles (an invalid declaration still exits 3
with the `DECLARATION_INVALID` document, which selects nothing); `audit` gates before any change is
cloned. `jobinputs.Plan` and `affectedAuditReport` carry the two optional members. Help, the
command-tree snapshot (`make capability-catalog`) and `docs/CAPABILITIES.md` name the forms.

Test: `TestStaleBinaryRefusesThePlanUnlessAccepted_AFPV09730` gains the `jobs --base` and `audit
--changes` forms on separate and merged streams, the accepted documents (exit 0 and 3) with the
members and the stderr line exactly when the streams differ, and the repeated flag. Before the fix
it fails on `affected jobs` with exit 0 and a `corvint-ci-job-plan/0` document (observed); after
it passes with `TestAffectedJobsCLI` and the audit tests.

## Findings 2 to 5: `script/test-run-pattern.sh`

Checked, finding 2: a `+N,0` hunk looked at new-side lines N and N+1 and skipped them when no
declaration covered them, so deleting `func init()` whole narrowed to the next test (observed on
the fixture: `why TestG1 ... changed via helper` after the init removal) and `+++ /dev/null`
dropped the file. Finding 3: `$count == 0` was evaluated with `$count` = `all`, which perl
coerces to 0 with the warning `Argument "all" isn't numeric`, so an untracked test file holding a
type declaration printed a narrow `-run` (observed). Finding 4: the hunk header regex required
`b/`, so `diff.noprefix=true` dropped every hunk (observed: the edited-test case printed no
selection) and a path with a non-ASCII byte was quoted by `core.quotepath` in every listing.
Finding 5: only `.go` paths reached the source rule; a `.s`, a fixture file and `go.mod` printed
nothing new (observed).

Fix (ASD-V0-5133): hunks carry the old-side range; removed lines are read from `git show
BASE:FILE`, parsed with the same `parse_decls` as the working file, and attributed as changed
lines are: inside a surviving test, benchmark or helper of the same name and kind, the declaration
is marked changed (narrowing kept); a removed declaration that is gone runs the package whole
(`removed-init`, `removed-counter`, `removed-helper`) unless it was a Test, Example, Fuzz or
Benchmark function, which selects nothing of its own; removed block and uncovered lines run whole.
A deleted test file runs its package whole (`deleted-test-file`) while the directory still has a
`.go` file. A pure deletion attributes nothing on the new side, and `eq "all"` is tested first.
(ASD-V0-5134): every Git listing runs with `-c core.quotepath=off`, the parsed diff with
`--src-prefix=a/ --dst-prefix=b/`, Git's trailing tab after a header path with a space is
stripped, and hunk records are tab-separated so a path with a space keeps its field count.
(ASD-V0-5135): a changed compiled non-Go source in a package directory with tests is
`source-edit`, any other non-testdata file there `package-file`, and a changed `go.mod`/`go.sum`
turns every package of its module the script would otherwise narrow into `whole: ./PKG
reason=module-edit FILE`. Retained limits are in the requirement texts: a path Git still quotes
(tab, quote, backslash, newline), an embedded file in a subdirectory without tests, and
`script/heavy.sh` admitting only `.go` source edits by itself (ASD-V0-2478), so the new whole runs
need `HEAVY_WHOLE_REASON` as `TestMain-changed` already does.

Tests: `script/test-run-pattern_test.sh` gains a second fixture whose BASE holds the declarations
the cases remove; 15 of its new cases fail on the previous script (removed init, var and helper;
removed benchmark; deleted file in three states; untracked type; noprefix; the quoted path twice;
`.s`; package file; the `go.mod` pair) and all pass on the new one, with the earlier cases unchanged.

## Finding 6: where `TestV14361RangeAffectedAndTestSelectTheSameGoPackages_AFPV09732` runs

Both commits (`ea09e3db5`, `2f4447b3c`) are ancestors of `origin/main` and of this head. In
`.github/workflows/ci.yml` the test runs only in the `go-product-shard` job: its checkout is
`fetch-depth: 0`, its shards run `go test -race` without `-short`, and `cmd/corvint` must be in
the shard's run set. The `go-static` job checks out at depth 2 but runs no `cmd/corvint` test;
`go-live` is depth 0 and runs the live tier only. So in CI the replay is run when a shard runs
`cmd/corvint` and `NOT_RUN` for a CI run whose shards omit it; it is never silently green on a
shallow checkout because the test skips with a `NOT_RUN` message. Recorded on the AFP-V0-9732 text
and its trace row.

## Note: CI's linecov planner and AFP-V0-9730 (no code change here)

The `linecov` step of `go-product-shard` builds `linecov-corvint` with `-buildvcs=true` from the
trusted main tools commit (`.github/workflows/ci.yml`, around line 1371 at this head) and runs it
with `--root "$PWD" affected --base ... --line-index ...` on the PR tree (around line 1401). After
AFP-V0-9730 that binary is `self-binary-stale` for every PR whose Go source or module files differ
from that main commit, so the step exits 2 and the shard takes the `FALLBACK` path until the
workflow passes `--accept-stale-binary` (the step's planner is deliberately main's build, so the
accepted, degraded plan is the intended one). That workflow change is a separate PR; nothing under
`.github/` was touched here.

## Commands and results

- `go test -count=1 -run 'TestStaleBinaryRefusesThePlanUnlessAccepted_AFPV09730$|TestAffectedJobsCLI$|TestAffectedAuditReplaysChangesAgainstOutcomes_AFPV0445$|TestAffectedAuditMissingObservationIsUnknown_AFPV0447$|TestSelfBinaryStaleIsReportedByAffectedAndTest$|TestStaleBinaryNoticeReachesEveryAffectedForm_ACXV0750$|TestStaleBinaryNoticeOncePerInvocation_ACXV0751$|TestAffectedStaleBinaryMergedStreamStillParses$|TestAFPV0825StaleCheckReusesThePlanHEAD$' ./cmd/corvint`: 9 PASS (5 s).
- `go test -count=1 ./internal/jobinputs`: ok.
- `script/test-run-pattern_test.sh`: ok; on the previous script the 15 new cases fail (listed above).
- `go build ./...`, `go vet ./cmd/corvint ./internal/jobinputs`: ok (with `TMPDIR` on the cache
  disk; the tmpfs `/tmp` had 1.2 GiB free and the linker ran out of space there).
- `make -s -k spec-status-check spec-requirements-check unbounded-readers-check
  traceability-tests-check line-citations-check capabilities-check` and `make changed-checks`:
  see the handback for the final run at the head commit.
- Not run: whole-package `./cmd/corvint`, `make gate`, the exhaustive repository gate,
  `make windows-vet` (no push from this lane), the V1-4361 replay test (unchanged code).

## Friction

- The unbounded-readers ratchet (AFP-V0-2745) read a bash pattern `*"tools/x"*` in the shell
  self-test as a path glob that admits `docs/build-log/` entries and failed the gate; the message
  names the test but not the literal, so finding it took a line-by-line bisection of the new
  block against the checker. Writing the pattern through a variable resolved it.
- A tmpfs `/tmp` with 1.2 GiB free fails `go build ./...` in the linker; `TMPDIR` must point at
  the cache disk, as `script/heavy.sh` already prints.
- The pre-commit self-test cannot run `go` (docs/SHELL-TEST-REGISTRATION.md), so the deletion
  cases are selection-only; the compile errors a removed `var` or helper would cause are the
  package build's to find.
