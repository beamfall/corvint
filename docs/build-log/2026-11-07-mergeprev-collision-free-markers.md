<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5043 independent review repair: collision-free diff3 markers

Intent: the orchestrator's 2026-11-07 decision under the owner's 2026-11-02
delegation, supplied with the P1 independent finding. `DWC-V0-7700` requires real
edit conflicts to remain conflicted; `DWC-V0-7704` shares the same parser for batch
qualification. The prior input blacklist already refused the supplied pipe/equal
example, but pattern-by-pattern refusal did not implement the required construction.

Decision: scan all three inputs before invoking Git. Use the greater of 7, the
requested width, and one more than the longest leading run of any marker character
in any input line. Pass that width to `git merge-file --marker-size=N`. The parser
recognizes only that exact width, the supplied labels, and the ordered
start/base/separator/end sequence. Shorter runs remain literal content. Any
unexpected delimiter or malformed hunk returns the original Git output with
`clean=false`, including when an earlier hunk could have resolved. A conflict exit
without a parsed hunk also cannot become clean. Git's clean output still bypasses
parsing. This removes the blacklist without changing the empty-base rule, newline
handling, CLI errors, exit conventions, stderr, or wire shapes.

The requested marker width remains bounded by the existing invocation checks;
the internally chosen width can exceed 1024 to avoid colliding with long content.
Existing exact conflict-byte comparisons are retained. Their expected generated
markers grow by one where the fixture contains a run at its requested width,
as explicitly required by the orchestrator's decision. No assertion was weakened
or deleted and no golden was changed. The owning spec records the same change
under stable requirement `DWC-V0-7700` and names its executable witnesses.

Acceptance evidence:

- `TestAppendDWCV07700CollisionFreeMarkers` covers both reported examples, literal
  base/end delimiters, a 2048-character run, the minimum width, unchanged clean
  marker text, and literal text both inside and outside append-only hunks.
- `TestAppendDWCV07700DefaultMarkerProperty` checks 60 combinations: each marker
  character, five suffixes, and each of base/ours/theirs. Every case requires exact
  retained conflict bytes and `clean=false` for a real edit.
- `TestResolveAppendDWCV07700UnexpectedDelimiters` refuses orphaned, reordered,
  duplicate, nested, wrong-width and wrong-label markers, with and without an
  earlier resolvable hunk. The new regressions failed against the previous code.
- Whole `internal/mergedrivers`: PASS, 0.185s. Whole `tools/merge-drivers`: PASS,
  0.652s, including real Git literal-base/end edits and unchanged clean marker text.
  Both ran through `script/heavy.sh`, one package per command, without exported
  commit identity, using the configured semaphore and private TMPDIR.
- Staged `script/check-line-citations.sh --repin`: no moved content found.
  Staged `make -s spec-requirements-check line-citations-check`: PASS. The existing
  180 skipped citations remain explicit; no darwin qualification is inferred.
- `git diff --cached --check`: PASS. Toolchain: go1.27.3.

Self-development routes: existing pre-change capture reused by explicit repair
instruction; `affected` used at base c3b29cbb54d31c8526d3cce4ad74356b6fb9b943,
selecting both tested packages via DIRECT_SOURCE_CHANGE. The installed planner
reports staleness, scope UNKNOWN, 142 selections, 522 exclusions and 81 unknowns
(79 LANGUAGE_FRONTIER, 2 UNOWNED_DIRTY_PATH); those are not current-tree exhaustive
assurance. Final binding, report inspection and dogfood-check outcomes are recorded
in the lane handback. Independent review and host process-containment qualification
remain orchestrator-owned. Retrieval/learning evaluations, broad gates, publication,
Tasks writes and sealing are excluded by the repair scope. Rollback: revert this
repair; the earlier conservative blacklist and parser behavior return.

Raw evidence is outside Git under `/home/agent/lanes/mergeprev-evidence/`:

| File | SHA-256 |
|---|---|
| marker-size-repair-red.log | 8b23abf766041e9f2bfd8ea45d59c750c17eec9dd53809ba8a216d2861211671 |
| marker-size-repair-mergedrivers.log | 3c469185db425701344faa3c3f95ad6ef4ea373f8e74631967593b20570f74fd |
| marker-size-repair-tool-tests.log | 81326b4cd2a3f406ebc0d04bcaa5c1ee433d5a286fd70930d9daa65e85175d61 |
