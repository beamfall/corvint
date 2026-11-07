<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5043 repair: literal diff3 delimiters

Owner intent: the 2026-11-07 self-review repair, P1 on append-union literal
delimiters. Requirement `DWC-V0-7700` requires genuine nonblank-base conflicts
to remain verbatim conflicted. The supplied finding shows that ours' literal
`||||||| example` followed by `=======` hid Git's real nonblank base and falsely
qualified a clean merge in both the local driver and shared batch replay.

Decision: refuse ambiguous input when Git reports conflicts, retaining its exact
output and `clean=false`. Start/base/end marker prefixes in any input, or an exact
separator line in the base, prevent parsing. This deliberately retains the whole
conflicted file, including otherwise resolvable append hunks. Equals underlines in
ours or theirs remain supported by the existing section-aware parser. When Git
reports no conflicts, return its output directly: literal complete marker blocks
in ordinary text must not be parsed or removed. Requested marker widths, labels,
wire fields, error codes, stderr and exit conventions remain unchanged.

`TestAppendDWCV07700LiteralDelimiters` checks exact conflict bytes with literal
start/base/end/separator lines in each input at widths 7 and 11, and unchanged
literal blocks in identical, one-sided and disjoint clean merges. It fails against
the original parser. After repair, the whole `internal/mergedrivers` package
passes (0.084s), including every existing assertion. Whole `tools/merge-drivers`
passes (0.647s); its extended `TestGitDriversDWCV07703` reproduces the supplied
literal-base edit and a literal-end edit through real Git and retains clean marker
text. Tests ran one package per `script/heavy.sh` command, Go 1.27.3, private
TMPDIR, configured semaphore and no exported commit identity.

The existing pre-change capture is reused. The affected route is used and retains
scope UNKNOWN, 142 selected units, 522 exclusions and 81 unknowns (79
LANGUAGE_FRONTIER, 2 UNOWNED_DIRTY_PATH), plus the installed-binary staleness notice.
The named packages are selected with BASE_UNREADABLE fallback. Final CEM/report
inspection and the local check outcome are retained in the lane handback; prior
host process-containment failures remain external qualification limits if they
recur. Independent review is reserved for the orchestrator. Retrieval/learning
evaluation, providers, broad gates, Tasks writes, publication and sealing are not
applicable or excluded by this repair's scope. Rollback: revert this repair; the
literal-delimiter false-resolution and clean-text corruption defects return.

Raw evidence stays outside Git in `/home/agent/lanes/mergeprev-evidence/`:

| File | SHA-256 |
|---|---|
| delimiter-repair-regression-before.log | `ce822542760bbf288b37faff9f056e42415e98451c4f71d10ffd0cb07ae96153` |
| delimiter-repair-mergedrivers.log | `c2f32751e5c30a15aff656806fcee300571bbc8cd0dcd5e29979b70f556cd878` |
| delimiter-repair-tool-tests.log | `76264df3ae8e29775e7c62b6c2d3fa2005562b451cdcca4c6cd40bfb6d4947bf` |
