<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5043 self-review repair: diff3 separator ordering

Owner intent: the mergeprev repair request, 2026-11-07. Requirement `DWC-V0-7700`
already requires nonblank-base and malformed hunks to remain verbatim conflicted;
this repair restores that behavior without changing the contract.

The independent finding supplied by the owner reproduces with base `old\n`, ours
`Ours\n=======\n`, and theirs `Theirs\n` at Git's default marker size 7. The parser
accepted the Markdown underline before seeing the diff3 base delimiter and falsely
reported a clean merge containing leftover markers. The separator now advances the
parser only from the base section. Markdown underlines in ours remain content;
missing base, separator, or closing delimiters retain the original conflicted bytes.
Both the local merge driver and parent-derived batch replay share this parser.

`TestAppendDWCV07700DefaultMarkerMarkdown` invokes real Git at marker size 7 and
checks exact bytes for edit conflicts with a heading on either side and a valid
empty-base heading addition. `TestAppendDWCV07700MalformedDefaultMarkers` checks
verbatim refusal of incomplete delimiter sequences. Before the fix, the heading
edit, heading addition, and missing-base cases fail. After it, the whole
`internal/mergedrivers` package passes (0.021s), as does `tools/merge-drivers`
(0.484s), using `script/heavy.sh`, Go 1.27.3, the configured shared semaphore,
and the required private TMPDIR without commit identity exports.

The existing affected route selected all three lane packages, with scope UNKNOWN,
139 selected units and 81 unknowns (79 LANGUAGE_FRONTIER, 2 UNOWNED_DIRTY_PATH).
Its installed-binary staleness notice remains explicit. The existing pre-change
capture is reused; no new capture was made. Fresh whole `internal/dogfoodflow`
results, final binding/check results and any external containment blocker are
retained in the self-review section of the lane handback. Independent review is
reserved for the orchestrator. Broad gates, Tasks writes, publication and sealing
are excluded by the repair request. No spec, golden, wire, error or exit contract
changed. Rollback: revert this repair; the false-resolution defect then returns.

Raw evidence is outside Git under `/home/agent/lanes/mergeprev-evidence/`:

| Evidence | SHA-256 |
|---|---|
| self-review-regression-before.log | `155de919e28fbe4a33536713e49196bdc3b3cf346c7e75b4de6d21f364d30ec1` |
| self-review-mergedrivers.log | `016554a844ebee2fea17b505595697541e7c4c31f07afdc753ed583b8c30eac2` |
| self-review-tool-tests.log | `e99d726203bad084c7487695818753d1f9d6ea953249b32aad8e2cc12f5232c7` |
