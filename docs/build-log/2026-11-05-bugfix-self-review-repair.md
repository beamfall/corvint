# Bugfix self-review repair

Owner repair input: four findings against `35ef58bc3823a39267cc78137b76d37ddcbef1dd`.
Lane base remains `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`. This entry records
repair decisions; no new capability or production coverage promotion is claimed.

For LPCV-V0-7601/954 (V1-4380, imported through V1-4664), `-cover=true` does
not prove module-library instrumentation. Go build information omits the
`-coverpkg` setting. The scratch collector therefore freezes the effective build
flags before launch and records an executable digest only after a successful
observed build using the exact module-wide selector and collector overlay.
Unknown flags, quoted GOFLAGS and unobserved builds conservatively lack proof.
A later launch must match that digest, its coverage destination and completed
child counters. The existing refusal vocabulary and index format remain intact.
`TestUninstrumentedChild` covers a partial main-package build and replacement of
a previously verified output with partial instrumentation at the same path.
This chooses bounded build provenance rather than treating any child counter as
proof of instrumentation completeness; external/prebuilt qualification remains
conservative.

For LPCV-V0-7600 (V1-4379), counter resets are removed. Cumulative entry and
cleanup snapshots produce window differences while the ordinary exit profile
retains an independent reset-free suite union. Reach in gaps, setup and teardown
has unknown ownership and is retained for every test, even if another isolated
test reaches the same block. Missing, invalid or decreasing snapshots retain the
suite reach for every test rather than allowing incomplete attribution to narrow.
`TestBetweenWindowReach` models execution after A's dump and before B's entry,
including an isolated B overlap, checks A survives index construction and checks
the actual overlay never clears counters. Existing fault-injection and exact
selection assertions remain unchanged. Concurrent goroutine attribution and
production exclusion admission remain unqualified.

For PWV-V0-8302/8102 (V1-4663 and merged V1-0507), the owner's repair request
resolves the contradictory inventory in favor of the already merged operator
create/refine surface. The malformed top-level payload regression now requires
`invalid-input` and no native call. It does not remove validation assertions or
change runtime behavior. The README describes the existing workflowRevision
implementation and native payload/CAS boundary. The required package-version
command synchronizes Pi to 0.3.15; no JSON shape changes.

For LPCV-V0-7607/960 (V1-4372), Linux directory watches cover untracked trees
at startup and refresh on directory creation, movement and lost events. An
overflow re-adds every path, including a deleted/recreated inode at the same
name; obsolete descriptors are removed. Failure to restore complete watching
ends automatic watching. `TestNativeNewDirectorySaves` verifies three separate
saves after draining parent events for existing, created and moved trees.
`TestNativeLostDirectoryEvents` verifies overflow, same-name inode replacement,
lost-watch restoration and loss of the root. Git metadata stays outside the
watched tree.

Used: existing pre-change context/query/impact captures and the affected plan
against the fixed base. The initial and final plans select the touched Go
packages with scope UNKNOWN: 196 selected, 108 unknown (91 LANGUAGE_FRONTIER,
seven NO_SELECTABLE_TEST, ten UNOWNED_DIRTY_PATH). These unknowns remain visible;
selected package checks do not claim whole-repository verification. Ranking and
learning evaluations do not apply to these repairs. Whole touched Go packages,
the Pi suite, required Windows vet and documentation/citation checks are recorded
in the external handback with raw logs and digest manifest under
`/home/russell/lanes/bugfix-evidence/self-review-*`. Initial failed runs are retained.
The Pi host is the cached 0.99.1; native Tasks write fixtures are disabled under
the lane's no-Tasks-writes instruction. Independent review belongs to the
orchestrator; no reviewer process or subagent is run here.

Rollback reverts the repair and its synchronized version/spec changes together,
then rebinds the delivered diff; the four findings return. Original ticket
qualifications, frozen outputs and outstanding host-side filesystem verification
are not promoted by this repair. Reuse the original pre-change capture, commit
as the named owner, rebind/check, and stop without seal, Tasks writes or publication.

Observed focused evidence (raw logs remain outside the repository):

| Check | Result | Raw log SHA-256 |
| --- | --- | --- |
| `script/heavy.sh go test -count=1 -timeout 30m ./internal/linecov` | PASS; 73.566 s | `1938f1d4b2901880fb1ccaa8267dc0944fc6ac89d92fcf11e32944cf4d06e53f` |
| `script/heavy.sh go test -count=1 -timeout 30m ./tools/linecov` | PASS; 6.050 s | `30133fae3538688b3493edc451cf75d4756713b8e43972f2bded01e2fe58a217` |
| Full Pi suite with cached Pi 0.99.1 and `CORVINT_PI_TASK_WRITES=0` | PASS; 166 passed, 11 skipped | `3b6126ef03cc64573ab3d1af18138e6b86b5b253dabf4020abc1f7ee2d32ad5f` |
| `script/heavy.sh --admit-only make windows-vet` | PASS; Core and interop | `c2b7c39d4f2251f830c8c2936e75f736bb10b3c52864a671028a8c07da279535` |

The first child-proof experiment failed the existing covered-child assertions
because Go omits `-coverpkg` from build information; the digest-bound build
proof fixes that observed failure. The first Pi run retained one failure because
`pi` was absent from PATH; the cached pinned executable resolved it. No assertion
was weakened to hide either failure. `make spec-requirements-check` passes,
including whole `internal/specindex` and the spec-enumeration units. Citation
repinning moved three unchanged spans in two documents; a second repin found
no further moves, and `make line-citations-check` passes. Requirement definitions,
traceability, executable fixture writes, read scopes, unbounded readers,
walk/SkipDir and package-version checks also pass. The whole CLI-package outcome,
remaining environment qualifications and final CEM/check identity are retained
in the lane handback; no success is inferred for that package from these checks.

The required whole `cmd/corvint` run finished with rc=1 (package 767.300 s;
heavy run 781.8 s, queue 57.8 s). Log SHA-256: `3486c56f192b236b2c48179132dad20b98ba518698697f68213dea72b2a0b76e`
(66,407 bytes). Failures retain the sandbox's group-writable temporary-directory
ancestor and read-only repository marker qualifications: SOURCE_UNQUALIFIED,
source/profile root misclassification and blocked scratch/executable admission.
No security check or assertion is changed. The local outcome remains blocked
pending host-side whole CLI-package qualification.

The first full Pi invocation omitted CORVINT_PI_TASK_WRITES=0 and consequently
ran native Tasks writes in disposable pi-tasks-native/pi-tasks-interrupt fixture
repositories. That exceeded the lane restriction. The tests bind their native
runner cwd to those temporary repositories and register cleanup; this is not a
live project-ticket completion write. The corrected final run explicitly disables
those fixtures. This mistake remains recorded rather than being reported as
Tasks writes NOT_RUN.
