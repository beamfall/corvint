# V1-0649 — Tasks directory traversal under declared read confinement

Source base: `02567f9ab07bd24ec11d9fd043b8b80ebe6e4c2f` (`origin/main`). The owner requested CI to
run only the tests a change needs; V1-0649 identifies `internal/tasks/authority` and
`internal/tasks/store` as expensive unbounded readers whose root-open path blocks AFP-V0-023
confinement. The previous 430 package-seconds attribution is an estimate, not a measured saving
from this change. The 16-merge replay is still pending.

## Decision and safety boundary

Linux ancestor directory descriptors use `O_PATH|O_DIRECTORY`, with the existing atomic
`O_NOFOLLOW` on every component. Final directories still open for reading; the descriptor bridge
and identity comparison stay intact. Darwin keeps the previous flags. Opening an unchecked
multi-component ancestor or granting read access to `/` was rejected because either would weaken
an existing boundary. No store format, canonicalization, filesystem allowlist or journal changes.
CTS-V0-004's implementation trace records the existing path authority; broader task-store contract
recovery V1-0310 remains open.

The Linux permission regression covers absolute `Root`, relative `InRoot`, and unreadable final
directories. A privileged runner explicitly skips when it can bypass those permissions. It also
ran as `nobody`, both unconfined and under PR438's wrapper, and passed without a skip. Existing
symlink/FIFO replacement, pinned-root and bridge tests retain their original checks.

## Qualification observations

The wrapper is copied without changes from PR438 head
`b6e38e477f3176e9e90d7cf949aa17db592b1597`, into an independent standard-library-only module.
`golang:1.27.1` reports Landlock ABI 4. The base `safeopen` package passes unconfined and fails
confined with `open /: permission denied`. Initial authority/store runs on Docker overlay fail
filesystem qualification, as expected; executable tmpfs is the supported fixture mount used for
subsequent runs. These failed attempts are retained, not counted as confinement results.
The corrected baseline driver exited successfully, but an early copy-all evidence runner overwrote
its package stream with an older failed stream. That stream is excluded from successful baseline
evidence; the final runner copies only its own outputs. The final candidate's complete paired
streams, rather than the overwritten baseline stream, govern confinement acceptance.

The candidate's full authority/store/safeopen runs pass both unconfined and confined. Exact JSON
comparison first exposed one dynamic subtest name: the INIT `durable-post:pinned/<digest>.json`
case embeds a digest derived from temporary paths. It is now labelled `durable-post:pinned`, with
the original fault-injection step unchanged. No comparison normalizes names or discards skips;
acceptance requires exact terminal outcomes and successful package terminals for all three packages.
The final source's selected verification commands and complete comparison results are retained by
the local dogfood workflow and reported on the source PR.

A separate clean checkout of PR438 committed empty scope entries for authority and store as
qualification commit `9fe7e391d6ddbfa28c470e0886e44a55688debab`, then changed only
`docs/specs/README.md`. Its branch-built planner excludes both packages with
`NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`. The plan retains all language-frontier unknowns and the unowned
docs path; these two exclusions do not establish whole-repository coverage.

## Review, delivery and rollback

Independent Astra/high Gate A and source review found no HIGH or MED issue. The permission test
addresses its initial LOW finding; the comparator now requires all expected package terminals.
The additional stable test label is independently reviewed before publication. Source review does
not qualify V1-0310 or complete native task authority.

This source fix is based on `origin/main`. Read-scope declarations remain a separately retained
patch for PR438, which is still blocked by its owner/admin control-plane status. Do not replace that
branch's existing declarations or move its reviewed head as part of this source fix. V1-0649 stays
open until source integration, declaration integration and the native completion disposition; the
recorded `full-gate` ticket gate is not claimed as passed. Owner policy selects focused tests for
scoped issue work, so `make gate` is `NOT_RUN`.

Rollback reverts the task-owned source, test and documentation changes. There is no store migration
or state cleanup. Reverting the Linux flags restores the confinement failure without changing
stored bytes. Owned containers use `--init` and EXIT/INT/TERM removal traps; no persistent service.

## Corvint use and limits

Used: installed Core rc1 build163 authority query, tracked Go path impact, `affected` before tests,
and the enrolled CEM/OCM local workflow in the isolated checkout. Original receipts, selected-check
observations, planner JSON, wrapper source, container scripts and failed/successful test streams live
under `.git/corvint/ci-0649/` (the pre-change receipts use the standard `.git/corvint/` names).
The installed release predates AFP-V0-023; only the PR438 build is used for declaration selection.
Context7 returned no matching syscall documentation within the three-command limit; Linux kernel
Landlock documentation and Linux `open(2)` documentation supply the primary platform references.

Deferred: live provider attachment, mutation experiments, retrieval learning and broad frozen
benchmarks; none is needed to alter directory traversal flags. Formal native-host support remains
FALLBACK, Frontier authority remains unavailable, and billed tokens/cost are NOT_OBSERVED.
Official update checks report Core rc1 build163 and Tasks developer build202 already installed;
unsigned prerelease/developer qualification limits are retained.

## Hosted amd64 repair

The first published source head a4f0f0ea passed the local arm64 Linux parity suite,
but hosted amd64 Go CI failed compilation: `syscall.O_PATH` is not exposed on
that architecture. An amd64 cross-compile reproduced the exact failure. Go's
bundled Linux syscall definitions and vendored x/sys definitions agree on
O_PATH=0x200000 for supported Linux architectures (sparc64 has a different ABI
but is not a supported Go Linux target). The repair uses that ABI constant
without adding a dependency; no traversal or permission behavior changes.
The prior seal is reverted by an ordinary forward commit to allow real source
repair, preserving original evidence and avoiding a force-push. Frozen
plan/base/key remain unchanged. Earlier checked/sealed identities and the
seal-stale reproduction remain retained; passing arm64 qualification was not
amd64 compile evidence. Verification must include both architectures before
publishing the repaired head.
