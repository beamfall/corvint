<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection baseline self-review repair

Owner intent: the sbgate self-review repair request names three gaps, within
V1-4724 and V1-4329. Preserve frozen outputs, assertions and lane deferral.
This entry supplements the original selection-baseline-gating qualification;
it does not claim a new full-closeout wall measurement or ticket completion.

## Decisions and acceptance

- **ASD-V0-944, manifest-only dependency changes:** conservatively run for
  participating workspace modules and local replacements rather than attempting
  to enumerate their manifest, sum and transitive replacement graphs. Root-only
  manifests cannot bound these builds. Existing pinned external module handling
  remains unchanged. `TestBaselineModuleManifestOnlyASDV0944` uses actual Git
  diffs and Go compilation closures with only `lib/go.mod` changed, for both a
  workspace module and a root-local replacement importing `lib/engine`, then
  repeats with no package imported from that module. Inspect the full Go module
  graph: unimported module requirements can still affect version selection.
- **ASD-V0-944 / ASD-V0-1990, outer batch skip:** recipes using the shared
  `SELECTION_BASELINE_INPUTS_BASE` preflight always reach that recipe outside lane
  closeout. An affected plan cannot validate the full reader declarations or
  bound compilation uncertainty. Keep historical recipe selection and its
  assertions for recipes without that shared protocol. Batch test group 15
  adds declaration-only valid/invalid cases with usable/unusable plans, plus a
  doc-only edit with an unchanged invalid declaration. Existing lane deferral
  cases remain in group 17.
- **ASD-V0-925 / DTL-V0-006, memory bound:** resolve `/proc/self/cgroup`
  membership through `/proc/self/mountinfo` for v1 memory controllers and v2.
  Take the minimum remaining memory across the leaf and every visible ancestor,
  using each ancestor's usage (including sibling consumption). Handle subtree
  mounts and escaped mount paths. An exhausted bound admits one worker;
  unreadable identified bounds cap the available-memory estimate at two GiB.
  `TestBaselineCgroupMemoryASDV0925` covers nested v1/v2 leaf and parent limits,
  subtree and escaped mounts, exhaustion and unavailable files.

## Verification and limits

The base-built Corvint affected plan was collected against immutable
`28a3ff86a2c3d17662765da12cc9251c06466f5f` before tests. The complete existing
and added `./tools/selection-baseline` tests pass through `script/heavy.sh`,
one package with no `-run` filter. Raw receipts and command logs remain under
`/home/russell/lanes/sbgate-evidence/repair-*`; final shell/spec/CEM results and
the delivered head are retained in the handback.

The first shell regression run failed in the added fixture: a blank line ended
its Make recipe before the preflight marker, and the cloned fixture omitted an
untracked interop directory. Corrected the fixture recipe and directory, and
isolated it from the existing lane-deferral fixture. Existing assertions were
preserved. The failed run remains in `repair-batch-tests.log`.

The module-graph extension initially encountered a workspace graph that Go could
not enumerate in readonly mode (`repair-package-tests-final.log`). Detect active
workspaces with `go env GOWORK` first and run conservatively; enumerate the full
module graph only outside workspaces. The manifest regression's exact uncertainty
assertion was preserved.

Final focused evidence: whole `./tools/selection-baseline` package PASS (1.255 s
test wall); batch-precheck groups 15 and 17 PASS; staged spec regeneration reports
no generated changes; `make -s spec-requirements-check line-citations-check` PASS.
The spec check's heavy runner explicitly excludes one Darwin-only lrfrepo source
on Linux; no Darwin qualification is claimed. Raw logs stay outside the repository.

No golden, assertion, JSON field, error code, exit code or core-freeze output
was changed. Independent review and the host's wider checks remain assigned to
the orchestrator. No full replay, doc-gates, whole-repository selection, Tasks
write, publication or seal is claimed in this repair. Original pre-change
context is reused; its missing impact receipt remains NOT_PRODUCED. Rollback:
run replay conservatively for all inputs and set `SELECTION_BASELINE_JOBS=1`;
do not restore an unsound skip or weaken a test.
