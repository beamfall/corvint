# V1-4933: Greptile fix round 1

Base: `539e4e39183cfdffe25a1049f15e42ed1791afb0`, branch
`claude/v1-4933-record-readers`. The supplied `greptile-4933.json` comments
contain two P1 findings: declaration registration and a scoped ancestor walker.
They contain no shell/make command or value scanner finding.

## Findings and repairs

- `39687305-0`: register `.corvint/record-entry-read-scopes.json` as a closed
  planner declaration at the top level and `/units/*`. The table test
  `TestRecordEntryDeclarationClosedPositions_AFPV0710` first reproduced the
  unclassified-file rejection, then passed with the registration. It also
  rejects new members at both closed positions against a pinned declaration.
- `39687305-1`: `filepath.WalkDir("../docs", ...)` with a current scope declaring
  `Reads: ["docs/build-log/"]` passed the ratchet even though the planner
  classifies it as an ancestor-only walker. Addition-only scopes filter
  existing matches; they cannot create missing selections. Restore the
  AFP-V0-2747 rejection for every walker, including those with current
  addition-only scopes. Literal/glob reader admission is unchanged.
  `TestRecordEntryScopeCannotAdmitWalker_AFPV3424` checks fixture classification
  and valid source binding before the refusal assertion. Directory, exact-file
  and empty-scope table cases all reproduced the erroneous exit 0 before the
  fix and pass afterward. This deliberately retains conservative rejection
  even for a declared empty scope; no new selection proof is introduced.

The AFP-V0-3424 text and acceptance mapping retain the walker boundary. Rollback
of the walker repair would reopen a missed-reader risk; disable addition-only
scope admission entirely if the repair needs to be removed.

## Verification

All Go test commands used the approved `script/heavy.sh` runner, which offloaded
to WSL Linux/amd64 with Go 1.27.3. Each command used the same task worktree's
source bytes plus its uncommitted edits.

Fail-before commands, both exit 1 with initialized fixtures:

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^TestRecordEntryDeclarationClosedPositions_AFPV0710$' ./tools/declaration-compat-check/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^TestRecordEntryScopeCannotAdmitWalker_AFPV3424$' ./tools/unbounded-readers/
```

After the repairs, the new regressions and existing focused checks pass:

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^(TestClosedDeclarationMemberAgainstPin_V13600|TestDogfoodMapsAreNoPlannerInput_V13600|TestTestSelectionClosedPositions_AFPV0710|TestRecordEntryDeclarationClosedPositions_AFPV0710)$' ./tools/declaration-compat-check/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestAddedRecordEntry|TestRecordEntry|AFPV274[5-8]' ./tools/unbounded-readers/
```

Results: exit 0, 0.196s and 0.076s package test time, respectively.
`make declaration-compat-check` passed on the actual repository declaration.
`make changed-checks BASE=539e4e39183cfdffe25a1049f15e42ed1791afb0` exited 0:
unbounded-reader and test-read-scope checks passed, with existing stale baseline
entries reported as advisory. Its citation check reads the index and explicitly
did not cover the unstaged spec edit. After staging, `make line-citations-check`
passed on the updated spec and this entry. `gofmt` and `git diff --cached --check`
passed.

## Evidence limits

Corvint context was used for orientation, with lexical omissions and unresolved
ticket identity retained. `corvint affected --base
539e4e39183cfdffe25a1049f15e42ed1791afb0` ran before tests and after the repair;
the installed build 137 at `447bdcbd5963` reported `self-binary-stale` against
this lane. Those advisory receipts are not candidate planner measurements.
Before/after `FRONTIER_SITE` counts were not measured: these findings change
checks, not shell/make scanning, and no fresh candidate scanner was built.
No full gate, Tasks commands, subagents, publication or detached jobs were run.
This is a local repair handoff; post-repair independent review and integration
remain with the caller.
