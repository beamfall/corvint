<!-- SPDX-License-Identifier: Apache-2.0 -->
# Slowtests verifier repair 1

The host verifier rejected the V1-4381 cache-helper comment because it names
the heavy wrapper without declaring the file in the shell foreign-callers manifest.
TSP-V0-005 requires every tracked source mention to be declared even when it
only appears in a comment. Add `cmd/corvint/work_adopt_test.go` to the existing
sorted mentions array. It invokes no wrapper and adds no dependency edge.
The manifest profile, shape, callers and reason remain unchanged.

The existing `TestRepositoryShellForeignCallersComplete_TSP_V0_005` passes
through the heavy wrapper after this correction (package test time 2.513 s).
The whole existing `internal/liveverify/affected` suite also passes without a
test-name filter (105.321 s); line-citations-check passes without any repin.
Raw evidence is outside the repository at
`/home/russell/lanes/slowtests-evidence/repair1/`; the handback records the
full owning-package result and final binding/check outcome. No Go source,
assertion, golden, requirement or product behavior changes in this repair.
V1-2740 scheduling and V1-4381 speed measurements retain their previous limits.
Rollback is reverting the added mention along with the helper comment that
introduced it; reverting only the mention restores the completeness failure.
