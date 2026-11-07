<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4729: partition compatibility and fixture exclusion repair

Date: 2026-11-06. Repair start: `97d5061669cf637c84b4de6d9a7a24a244bf08e7`.
Charter base: `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`.
Owner self-review findings: partitioned content-only dry runs acquired ordinary
package reasons, and terminal fixture exclusions denied the unowned paths in
the same receipt. This entry supersedes the earlier sharded-reason decision's
instruction to copy every package reason. No historical replay is requalified.

## Decisions and requirement coverage

AZU-V0-9900: propagate partitioned reasons only when GoSlice.FullReason marks
the explicit ownership obligation. Preserve that marker when FULL removes the
narrowing pattern. The existing producer sets it only for the named ownership
ratchet. Ordinary content selection reasons remain absent from partitioned
units under universe, balanced and measured placement. This restores omitted
reason/reasonCode fields and their original manifest digest without changing
execution coverage, JSON shape, codes, goldens or existing assertions.
TestOwnershipPartitionedContentOnlyBytes_AZU_V0_9900 compares canonical
manifest bytes and manifest digests against the prior no-reason partition
behavior across all three placements, ordinary source and test-function
selections, and narrowed/FULL plans. Every case confirms multiple partitioned
execution units. Existing ownership partition and FULL tests remain intact.

AZU-V0-9901: reuse the fixture membership/selected-host proof to identify its
selected Go hosts. Only that exception receives a new explanation: terminal
traversal plus the SourceBoundFrontiers or FixtureClosedFrontiers classification
of each excluded entry. The ordinary FrontierExclusionReason is unchanged.
TestOwnershipFixtureTerminalReaders_AZU_V0_9901 now asserts the certificate
names go:pkg and the applicable classifications, including the foreign-fixture
case that may exclude only the source-bound reason. The same receipt must still
contain UNOWNED_DIRTY_PATH. Existing resolution-input, dynamic, unbounded,
other-unowned-path, testless and content-only controls remain unchanged.
The owning spec's acceptance table names these witnesses; no requirement is
amended. Rollback reverts these scoped repairs without a migration, restoring
the two reported defects; it must not adjust a golden or weaken an assertion.
AZU-V0-9902 is unchanged and is not newly qualified by this repair.

## Verification and limits

Both added regressions fail before their implementation fixes. After repair,
whole-package script/heavy.sh go test -count=1 -timeout 30m passes for
./internal/sessiontest (1.770 seconds) and ./internal/liveverify/affected
(53.017 seconds), including every existing test in both touched packages.
Go is go1.27.3. Tests use the prescribed private TMPDIR, no exported identity,
and the configured heavy semaphore without overrides. No extra package test
or wider gate is inferred from these results.

Before tests, corvint affected --base at the charter base selected both touched
packages. The installed binary's self-binary-stale diagnostic is retained:
this selection is its advice, not final-tree freshness evidence. It retains
three LANGUAGE_FRONTIER and two UNSELECTED_BINARY_CONSUMER unknowns. Existing
pre-change receipts are reused as instructed. Other retrieval/learning
experiments are outside this compatibility/certificate repair.

Staged citation repinning moves one unchanged select.go citation by six lines;
a confirming pass finds no further move. The permitted make -s
spec-requirements-check line-citations-check passes, including its existing
heavy-wrapped package checks. Gofmt and staged git diff --check pass.

The two self-review findings are repaired. V1-4729's host qualification and
native completion remain external: the prior whole CLI package failed 45
names, and this round does not touch or rerun that package. The aggregate
local outcome therefore remains failed rather than claiming qualification.
Independent review belongs to the orchestrator. Full repository tests/vet,
Windows vet, doc-gates, selection-baseline-check, corvint test, gate-affected,
model-backed review, Tasks writes, publication and seal are NOT_RUN by charter.
No new pre-change capture is produced. Final CEM binding/checking follows all
edits and checks and stops after dogfood-check. Structural closure does not
establish passing qualification, semantic completeness or ticket completion.

## Retained evidence

Raw evidence stays in /home/russell/lanes/ratchetsel-evidence/.

| Artifact | SHA-256 |
| --- | --- |
| `self2-sessiontest-before.log` | `6e53f52ba4a770f577d94f30614ee51ec7e83906fa4494bdd4a83afcf13956c9` |
| `self2-affected-before.log` | `fb660f493f343f1992ad8270621fd7676cc5545feca5e9be2be03dd5bd5b71da` |
| `self2-sessiontest-whole.log` | `4c54401ec0165e3013d442d02e3f53540ec14ddbd2fc0f56376be828954b90d7` |
| `self2-affected-whole.log` | `d0659c418902a3aa160f56866bf131f7ae475741fb0fb327c3bcc27bcc3d24fe` |
| `self2-spec-citations.log` | `36bc8272985ce02ae0ca1fd4458ca99557d67f5beb3174102813301b28922f48` |
| `self2-affected.json` | `17447b8646a5c167328039d565cc332b8ea866fd48a70975fc4578efe3447327` |
