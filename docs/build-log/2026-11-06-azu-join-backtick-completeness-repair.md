<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU join and backtick completeness repair

Self-review of `e2762b715aaae549064e24cc59f5778435c3c8b8` found two remaining
AZU-V0-004 / AZU-V0-8901 completeness gaps. For the declared `run/run.sh`,
`exec.Command("sh", filepath.Join("run/", name+".sh"))` with `name == "run"`
and ``exec.Command("sh", "-c", "run/`echo run`.sh")`` can execute the script
without its literal basename. The join guard rejected the trailing slash;
the formatted-path guard only noticed the earlier backtick regression because
its `printf %s` contained a format marker. Both integrated cases returned
BOUNDED with no unknowns and omitted the undeclared caller's tests before repair.

The completeness guard now permits the same optional directory slash for joins
as for concatenations and independently recognizes backtick substitutions
inside quoted paths. Raw-string closing delimiters alone do not count as
substitutions: the first repair candidate put backticks in the format-marker
class and incorrectly matched literal Markdown spans in `script/heavy.sh`.
The whole affected-package check exposed this overmatch (108.528 s, rc 1).
Separating quoted substitutions from raw-string formatting avoids that match.
A second candidate still matched backticks in diagnostic prose after a literal
path (`script/check-line-citations_test.sh`; 135.560 s, rc 1). The final pattern
stops at whitespace before the substitution, keeping it within the quoted path
token. Negative regressions protect both literal backtick-delimited paths and
backticks in prose after a literal path, without broadening project declarations.
This deliberately widens possible callers and retains uncertainty rather than
inferring execution edges. The existing declaration schema, diagnostics, goldens
and all existing assertions stay fixed. No new requirement or design fork is
needed: AZU-V0-8901 already requires constructed-directory completeness and
preservation of verified caller selections. Rollback must retain these unknowns
and existing caller edges; restoring either omission is unsafe.

`TestConstructedForeignCallers_AZU_V0_8901` checks admission when each new form
is declared and rejection when omitted, including `./run/` joins.
`TestConstructedDirectoryRetainsUnknown_AZU_V0_8901` exercises both reported
expressions through graph construction and selection, requiring foreign-invocation
UNKNOWN and selection of the known literal caller's tests. The whole shell
package failed before the repair and passed afterward (0.490 s), with every
existing test included. Raw logs remain outside the repository:
`/home/russell/lanes/azu-new-evidence/self-review-directory-backticks/`.

| Log | SHA-256 |
|---|---|
| `shell-before.log` (rc 1) | `70a720e9883f952a58b1c74263f8523f6e23f84bb520fa4db9f0f79faaf87667` |
| `shell.log` (rc 0) | `e8e6292f097d761cee935381b05f6600d9e6fe88fff72926d4266beff4dc2768` |
| `affected-overmatch.log` (rc 1) | `aea98117a99831b17787fb586e159d51ff4d5e6077578519a065721a0c031e79` |
| `affected-prose-overmatch.log` (rc 1) | `2b20a466e8c9e6fa2bbe85119f3896214cb8ea8a9ea2329b7e70e9159f11aa24` |

Self-development routes: the existing pre-change capture is preserved;
`corvint affected --base 9208cc7e25bb82503ef9d61b0ce6a5f02039e649`
selected the shell and affected integration packages (UNKNOWN, 186 selected,
86 unknowns). Its stale installed binary remains explicit degradation. Whole
package tests run through `script/heavy.sh`, without filters or exported Git
identity. Final integration and focused documentation results, terminal CEM
binding/checking, and limitations are retained in the lane handback. The host
orchestrator owns independent review and broad verification. Historical replay
qualification remains NOT_QUALIFIED; this repair does not establish aggregate
AZU-V0 admission. Publication, Tasks writes, replacement capture, model-backed
review, broad gates and seal are excluded by the repair instructions.
