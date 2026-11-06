<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU constructed caller directory-cache repair

Independent review found a completeness hole under V1-4687 / AZU-V0-004 and
AZU-V0-8901. With declared `run/a.sh` and `run/run.sh`, the normalized literal
join matcher compared the complete path with one script, while completeness
cached its result by directory. An undeclared Go caller executing embedded
Python could use `os.path.join(".", "run", "r\u0075n.sh")` inside a raw string.
The first script cached false for both scripts, and literal-reader matching
could not recover the escaped basename. The resulting plan certified BOUNDED,
had no unknowns and excluded the undeclared caller's tests.

Decision: retain the existing directory cache and make normalized fully literal
joins compare their resulting directory with the callee directory. All matcher
branches now depend on the directory alone. This follows the existing guard's
conservative directory-construction contract and avoids rescanning every
candidate for every basename. It supersedes the exact-path comparison described
in the earlier normalized-join repair entry. This guard widens possible callers;
it does not infer an execution edge or authority to exclude another caller.
No wire shape, diagnostic, golden or existing assertion changes.

`TestNormalizedJoinSharedDirectoryRetainsUnknown_AZU_V0_8901` builds the reported
two-script fixture with embedded Python in a Go raw string. It requires foreign
invocation UNKNOWN and selection of the known caller, rejects the incomplete
declaration, checks matching for both basenames and admits the caller when
declared for both scripts. The owning spec's existing AZU-V0-004 traceability
row names the regression; no requirement ID or accepted intent changes.

The whole shell package reproduced the regression before repair (rc 1,
0.817 s). With the repair it passed, including all existing tests (rc 0,
0.708 s). The whole affected integration package also passed (rc 0,
152.594 s), preserving repository declaration completeness and existing bounded
selection assertions. `script/check-line-citations.sh --repin` found no moved citations;
`make -s spec-requirements-check line-citations-check` passed. Raw evidence stays
under `/home/russell/lanes/azu-new-evidence/independent-directory-cache/`:

| Log | SHA-256 |
|---|---|
| `shell-before.log` | `9aeb01eebcbfa9a930c83627fd5e8626d0617c687f2b8d5be12d8e76d090a96a` |
| `affected-final.log` | `0e409a17411a79616c88f817f4dee045e6b9042427af2cc97374a0340957395f` |
| `shell-final.log` | `e422a8ce1bb182258f7512024f35020b14ae02df5dba202573af2144873aad3c` |
| `repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `doc-checks.log` | `c84d2350ae93aa62c50273cf95be21889cebe9daf8f70823b313e3f415eceea2` |

Self-development routes reuse the existing pre-change capture. The exact owner
base `f50a0e3447a2bbadbfadbe7f8cc1019ae1636ac5` affected plan selected the shell
and affected integration packages (UNKNOWN, 186 selected, 86 unknowns), with
the installed binary's staleness explicitly retained. Focused whole-package
tests use the configured heavy wrapper and private lane TMPDIR, without an
exported Git identity. One terminal CEM binding and dogfood-check follow final
edits. The handback retains their outcomes and remaining limitations.

Broad gates, replacement capture, publication, Tasks writes, model-backed
review and sealing remain excluded by the repair instructions. Independent
review and host verification belong to the orchestrator. Earlier historical
replay qualification limits remain unchanged. Rollback must retain uncertainty
for escaped literal joins; a future alternative may cache per script, but must
pass the same integrated regression and preserve known caller selections.
