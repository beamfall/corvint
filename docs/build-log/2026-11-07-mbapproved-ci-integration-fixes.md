<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbapproved: five CI integration failures of the batch merge

CI on the mbapproved batch (entry `2026-11-06-mbapproved`) failed in five places. Each came from
one merged lane meeting `origin/main` or another lane, never from a lane alone. Each was reproduced
before its fix: four under the CI read-scope wrapper (`test-confine`, built as `ci.yml` builds it,
Landlock ABI 7 here) and the unmapped-package and fixture guards as plain package runs. No test was
weakened, no budget widened, and the import baseline is unchanged.

1. **Unmapped packages (IRT-V0-005, IRT-V0-006).** clicontract added `internal/cliapp`,
   `cliconformance`, `clierror` and `cliprofile`, and reuse added `jobinputs` (V1-1099) and
   `testreuse` (V1-1097), all without area prefixes. `clierror` is a leaf, and `cliconformance` imports only it: both go to
   kernel. `cliapp` and `cliprofile` import `companioncli` (integrations, layer 7), so they go to
   cli. `jobinputs` and `testreuse` import only `liveverify/affected`, so they go to affected. No
   edge goes the wrong way, so nothing needed inverting.
2. **Detached maintenance (V1-1030).** The `jobinputs` and `testreuse` Git helpers hide the global
   config, so CI's `maintenance.auto=false` and `gc.auto=0` no longer reached their commits. Both
   now pass the two flags command-locally.
3. **EEP-REMOTE-004.** The CLI contract made `cmd/corvint-remote-provider` import `cliapp` and
   `clierror`, which pull in `capcatalog`, `clitree`, `companioncli` and `versionline`. The
   `internal/extevidence` scope still declared only the old closure. It now declares the same six
   paths that clicontract added for the other adapters. `make test-read-scopes-check` passed
   before and after the fix: it does not see a `go build` whose argv starts with a variable
   (`compiler`).
4. **Tasks import closure (V1-4050).** Floor declared `internal/companionrelease`'s read scope
   (AFP-V0-2360) before main added `TestTasksExportCoversCorvintTasksImportClosure`. That test
   sets `GOFLAGS=`, which drops the wrapper's `-buildvcs=false`, so `go list -f` of the main
   package stamped VCS and failed on the denied `.git`. It now passes `-buildvcs=false`, as
   AFP-V0-2360 already requires of builds from the repository root. A linked worktree does not
   reproduce this, because Go finds no VCS root there; the reproduction used a clone with a `.git`
   directory.
5. **Staged-snapshot hint (ASD-V0-2421).** clicontract (`2046c94da`) declared
   `internal/tasks/cli`'s read scope, and its CAP-V0-9300 test needs that declaration, so it stays.
   `openStagedPath` pinned each path component with `safeopen.Root` and `SubRoot`, which open each
   directory for reading. The wrapper grants no read on `/` or on the checkout's ancestors, so the
   hint was never produced. ASD-V0-2421 is amended at `dffa6ec8a`: ancestors are pinned with
   traversal-only handles (new `safeopen` `PinTraversalRoot`, `StepDir`, `LstatIn` and
   `ReadlinkIn`; O_PATH on Linux, as `Root` already uses internally), and only the final open
   reads. The protected-folder check still runs before each component is inspected.

## Evidence

- `TestImportRatchet`, `TestRepositoryGitFixturesDisableDetachedMaintenance`: failed with the CI
  messages before the fix, pass after.
- `TestRemoteTransportConformance` under `test-confine`: `open .../internal/cliapp: permission
  denied` before, the whole `internal/extevidence` package passes confined after.
- `TestTasksExportCoversCorvintTasksImportClosure` under `test-confine` in a `.git`-directory
  clone: `error obtaining VCS status: exit status 128` before, passes after.
- `TestASDV02421StagedHintsAvoidProtectedFolders` and
  `TestASDV02421_JournalAbsentMissNamesStagedSnapshot` under `test-confine`: `fixture control
  failed: []` and the missing staged hint before, pass after.
- New: `TestASDV02421StagedHintsNeedNoAncestorRead` (Linux) puts an execute-only directory
  between the test temp dir and `HOME`. It failed before the fix with no staged snapshot found,
  and now passes, both unconfined and confined.
