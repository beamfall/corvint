# V1-5178: Greptile fix round 1

Base: `317c8542734d01d83e3b9e507ad1e17b2d3e38b2`, branch
`claude/v1-5178-tsv-readers`. Scope: the two P1 comments in the owner's
`greptile-5178.json`. The owner requires recall over precision and authorizes
restoring the pre-lane conservative behavior. No Tasks calls, subagents, pushes,
network publication, or agent-started background jobs. Heavy tests use the approved
runner. The existing model/effort is retained for this bounded repair.

## Findings and repair

- `39687752-0`: `for f in "$ROOT"/script/*_test.sh; do cat "${f%_test.sh}.tsv"; done`
  lost `script/foo.tsv` despite an existing `script/foo_test.sh`.
  `TestShellDerivedGlobReaders_AFPV03333/review_example` reproduces that exact
  command; a suffix-substitution variant also fails before repair. Keep wildcard
  metadata but restore the conservative tokens from the same word. No shell data
  flow proof is claimed. AFP-V0-3333 now explicitly forbids replacing those tokens.
- `39687752-1`: `"$READER_BIN/cp" first.txt script/input.tsv` lost the second
  argument. `TestShellQualifiedUtilityReaders_AFPV03332/review_example` reproduces
  that exact command; computed `mv`/`mkdir` and project-qualified commands cover the
  same basename error. Restore the pre-lane exact command-name switch. Qualified
  commands retain their operands, including absolute utility paths. AFP-V0-3332
  and the previous precision tests now require this conservative behavior.

These changes restore literal reader selection, not a new known-value proof or
FRONTIER_SITE suppression. The earlier lane's wildcard and qualified-copy
precision claims/measurements are superseded where affected by these reversions.
No before/after repository FRONTIER_SITE measurement was made: the installed
binary predates this lane, and a candidate binary was not built for this repair.

## Evidence

Local evidence lives in `.repair-5178/` (untracked, retained for handoff).
All commands below use `-count=1` and the required 30-minute package timeout.

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestShell(DerivedGlob|QualifiedUtility)Readers_AFPV0333' ./internal/liveverify/affected/shell/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestShell(DerivedGlob|QualifiedUtility)Readers_AFPV0333|ReadScope|Shell|TestPathTokens|TestWriteOnlyWords' ./internal/liveverify/affected/shell/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestShell.*AFPV0333|TestRepositoryShellDataReaders_AFPV03330|ReadScope|Shell' ./internal/liveverify/affected/
```

1. FAIL before production edits: both exact review examples and the related
   table cases are excluded (`fail-before.log`, repeated with the final root-level
   fixture in `fail-before-final.log`). All eight retained cases fail on the lane
   commit. A ninth exploratory case described below also fails there.
2. PASS after repair: shell package, 0.211s test time on WSL Go 1.27.3
   (`shell-pass.log`); includes the existing lane regex and path-token/output tests.
3. PASS after repair: affected package, 25.329s test time on Darwin
   (`affected-after.log`); the heavy runner selected local execution because of
   Darwin-specific package files. Go metadata code is unchanged by this repair.

`gofmt` and `git diff --check` pass. `make changed-checks
BASE=317c8542734d01d83e3b9e507ad1e17b2d3e38b2` exits zero, but its index-based
line-citation check explicitly omits the unstaged spec edit. Its pre-existing
`internal/tasks/cli` unbounded-reader warning remains. No broad gate or push is
requested; Windows vet is a pre-push obligation and was not run.

Corvint path impact was used with retained bounded omissions (`context.json`).
The initial affected read raced test-file creation and refused with
`unsupported-affected-drift`; a stable retry succeeded (`affected.json`) with
`self-binary-stale`: installed Core is 1.0.0-rc.3.dev build 137, commit
`447bdcbd5963683f5f2b4934a45b8f48fc6742a3`. The official updater's Core check
could not prove process cleanup in this sandbox, so available-version freshness
remains UNKNOWN and no update was installed. Tasks version checks are excluded
by the explicit owner instruction. `make dogfood-change` failed creating the
private Git evidence directory with `Operation not permitted` (`dogfood.log`);
CEM/seal remain NOT_PRODUCED. No account-usage telemetry tool was available.

Commit attempt stopped at `git add`: the sandbox refused the worktree's private
`index.lock` with `Operation not permitted`. No commit was created, no alternate
index/worktree was used, and all edits remain in the requested worktree.

## Separate pre-existing limitation

The additional case `for f in script/*_test.sh; do cat "${f%_test.sh}.tsv"; done`
with `reader_test.sh`, `script/foo_test.sh`, and `script/foo.tsv` at the repository
root still drops the TSV reader. Restoring the entire pre-lane `shell.go` from
`HEAD^` reproduces only this case's failure while both reviewed examples pass
(`pre-lane.log`). It is therefore outside the two regressions introduced by this
lane, and is not silently converted into a passing expectation or included in
the delivered regression suite. The original table is preserved in
`reader-recall-with-preexisting-case.txt`. Expected acceptance: the reader is
selected for a dirty `script/foo.tsv` without relying on a global fallback.
Follow-up filing is outstanding because the owner explicitly forbids Tasks calls.

Rollback: restore the two reader rules and accompanying expectation/spec changes
from the base commit, which also restores the reviewed recall defects. No state
migration or declaration change is part of this repair.
