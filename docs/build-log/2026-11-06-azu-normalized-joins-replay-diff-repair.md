<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU normalized join completeness and replay source coordinates

Self-review requested two repairs under existing contracts. P1 / V1-4687 /
AZU-V0-004, AZU-V0-8901 found that an undeclared Go caller could construct
`run/run.sh` with `filepath.Join(".", "run", name+".sh")`. The previous guard
required the full directory as the first operand; separate literals also
escaped reader matching, yielding BOUNDED while excluding that caller's tests.
P2 / V1-4692 / AZU-V0-009, AZU-V0-8903 found that `diff.noprefix=true` removed
the headers' canonical prefixes, losing changed-line mutation coordinates.
Neither repair needs a wire, error, diagnostic, golden or assertion change.

The completeness guard now also consumes complete initial literal join operands
and normalizes their combined path. A matching directory or ancestor followed
by an unresolved operand retains uncertainty; a fully literal join is compared
with the callee path. A dot prefix alone cannot prove exclusion. Unrelated
literal directories stay excluded, and identifier boundaries prevent treating
`driverJoin` as `Join`. This augments the existing conservative checks and
preserves verified caller selections, rather than inferring execution authority.
Unsupported quoted escapes retain uncertainty. The integrated nine-case
`TestNormalizedJoinRetainsUnknown_AZU_V0_8901` covers dot prefixes, raw strings,
parent normalization, empty prefixes, escaped UTF-8, split nested directories
and unresolved suffixes. Every
case requires foreign-invocation UNKNOWN and selection of the known caller.
`TestNormalizedJoinBounds_AZU_V0_8901` checks unrelated joins and identifiers,
fully literal paths, single quotes, escaped components and string separators.
The fixture source itself now conservatively matches all declared directories:
it contains a dot-prefixed unresolved join. The declaration lists that file as
a reviewed mention for each script; these tests build isolated example.test
graphs and never execute repository scripts. No production caller was relabelled.
Empty join prefixes also conservatively match two existing Perl text-join
classifiers, the citation checker and merge driver. They were already reviewed
mentions for scripts under `script/`; their existing non-executing role is now
also recorded for `.github/forward-rows/pin-lag.sh`. The intermediate whole
affected run exposed those two missing mentions (153.003 s, rc 1). The remedy
adds explicit reviewed mentions, preserving the stronger guard and every test.

The first integration run failed at 175.257 s because treating unsupported
single-quoted escapes as arbitrary paths also matched JavaScript/Perl string
join separators. The final decoder handles single-quoted strings, short NUL
separators and byte versus Unicode escapes. It keeps uncertainty for unsupported
escapes. Added negative separator regressions protect that boundary. Existing
repository completeness and bounded-selection assertions remain intact.

Replay uses one source-diff helper that forces `a/` and `b/` prefixes and disables
text conversion, preserving source coordinates independently of the reported
local prefix settings and attributes. `TestSourceDiffConfiguration_AZU_V0_8903`
creates real committed edits, configures no prefixes, mnemonic prefixes and a
textconv driver that would hide the edit, then requires exact parent/merge line
coordinates and admission of the bound mutation receipt. This is fixture
evidence, not historical replay qualification.

Before repair, all seven integrated join cases lost the required uncertainty;
the no-prefix and textconv cases lost mutation coordinates. The final whole
shell package passed (0.817 s), as did the whole replay package (0.355 s),
including existing tests. The final whole affected integration package passed
(163.992 s), including repository completeness and existing bounded-selection
assertions. Original assertions remain intact. Raw evidence is
outside the repository under
`/home/russell/lanes/azu-new-evidence/self-review-join-prefix/`:

| Log | SHA-256 |
|---|---|
| `shell-before.log` (rc 1) | `5c6855c3cea1b6c156e1e0dfac6c0e15913d3fa6a81beed18ec2c3f127d6bf92` |
| `replay-before.log` (rc 1) | `9460a1e41759ea7ddb62c01cd8cfd8bb565f707656efc42b91dfbd55d4d694c3` |
| `shell-final-encoding.log` (rc 0) | `fc0ab61b0150ba24e9101c536638b1c515a16784146603481bd215123d2aae9d` |
| `replay-final.log` (rc 0) | `518d4002b64d5cc359bf62116fe142ef1c932d28321b20a379a3902687f7aae8` |
| `affected-overmatch.log` (rc 1) | `01a0d403e5d2c958ca1cac39cb03d11d00701401dc70e70bf923293602736cd8` |
| `affected-terminal.log` (rc 0) | `3f00967f4616c449374898b5ee9276c8071f389235bc75fd55f2ac73aa089569` |
| `affected-empty-prefix.log` (rc 1) | `41ea59a9cf332831ae3a9bcf18a1d93c5314d181ce208c70e879b45a24bfe3ea` |

Self-development routes: used the owner's exact `corvint affected --base
dba1f4d6c9a33098470ccf57794fb66f989f03c8` selection (UNKNOWN, 186 selected,
86 unknowns, stale installed binary retained), focused whole-package tests and
the existing pre-change capture. The current-source CLI is built externally for
the one terminal CEM binding cycle. The lane handback retains the affected
integration result, focused documentation checks, CEM/check result and friction.
Replacement capture, broad gates, publication, native Tasks writes, model-backed
review and seal are excluded by instruction. Independent review and remaining
host verification belong to the orchestrator. Historical full-suite/mutation
qualification stays NOT_PRODUCED; no native ticket completion or aggregate
qualification is claimed. Rollback must preserve uncertainty for these joins
and source-bound receipt coordinates, rather than restore unsafe completeness
or relax receipt admission. No accepted intent or requirement ID changes.
