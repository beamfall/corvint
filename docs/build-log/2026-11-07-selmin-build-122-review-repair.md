# Selmin build 122 review repair

Requirements: AFP-V0-2202, AFP-V0-2203, AFP-V0-9010 (V1-5029).
Input head: 936ddec48938a8190458dff5c6848635ef6ffdf5.
Base: e19823d919cfa653ee0ed05708d03f5fd12ca929.

## Independent review findings

The six hidden-package failures are merge-forward expectation drift. AFP-V0-9010
requires Source independently of Changed and Module, including ignored .go text
that an unproven reader can parse. Keep exact equality of the complete result:
add the expected Source to each case without changing any Changed or Module
expectation. Existing import-selection, body-only exclusion, witness-chain and
live go-list assertions remain intact. Removing Source from production would
lose required unproven-reader coverage.

The package-documentation negative control was masked by mainline AFP-V0-2203:
a lone literal `lib` now independently selects the reader as PATH_LITERAL_READER.
The initial whole-package run confirmed that exact witness. Compute the fixture's
package name from bytes, using it in both the target and expected comment, so its
runtime behavior stays identical without a literal path witness. Preserve the
positive IMPORT_GRAPH_READER witness, exact lib-to-docs chain, guard exclusion,
and unconditional failure if docs remains selected after Source is removed.
Include the unexpected witness in that failure's diagnostic.

These are test integration failures, not newly admitted analysis forms. No
classifier or production behavior changes. The existing closed allowlist remains:
only proven confined go-list calls over ImportPath, Imports, Deps, Name, Standard
and DepOnly, with classified source channels and proven directory provenance,
qualify for header narrowing. Doc and unknown channels retain Source selection.
The whole package includes the existing unknown-import, unknown-directive,
unknown-directory and unknown-command-method regressions. No new denylist case,
wire shape, golden, error code or production stderr is changed.

## Verification and limits

`corvint affected --base e19823d919cfa653ee0ed05708d03f5fd12ca929` selected
internal/liveverify/affected/golang; scope UNKNOWN, 71 language-frontier unknowns.
The installed binary reported self-binary-stale (14 source paths); this is retained
as a selection limitation, not evidence that the current implementation is equal.
Go version: go1.27.3. Tests ran with no exported commit identity.

Whole-package `script/heavy.sh go test -v -count=1 -timeout 30m
./internal/liveverify/affected/golang` ran all existing tests. Both reported tests
PASS, including every hidden-package subtest and its live go-list check. The
package still exits 1 because unrelated Git-backed fixtures fail process cleanup:
`process-cleanup-failed`, `/proc quiet proof unavailable`, missing leader stat.
This sandbox failure needs the host verifier; no test assertion is weakened.

`script/check-line-citations.sh --repin` found no moved unchanged citation.
`make -s spec-requirements-check line-citations-check` PASS. Broader selections,
repository gates, independent review, task writes, publication and seal are
NOT_RUN by the repair charter. The existing pre-change capture is not rerun.
CEM binding/check results belong in the external handback after the final commit.

Raw evidence remains outside the repository in ~/lanes/selmin-evidence/:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| golang-before.log | 10435 | 0d1af5e04e7481c6c37ee08feedbce48c3b2123bb0091ba604d2c90f3c9199f4 |
| golang-after.log | 87603 | 9bc8fa3cb006ff651b19119c0289006b8926af15e29c8550e93267feafc77547 |
| docs-check.log | 1525 | 66c190fcb52816f3912546d2141b4a62102e4cae214072a136f7cadbcdf4b67c |
| repin.log | 190 | 85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133 |

Rollback: revert this repair commit; no previous commits are rewritten. V1-5026
and V1-5030 are unchanged by this bounded round. No task completion is claimed.
