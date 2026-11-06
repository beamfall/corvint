<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU root and nested computed basename completeness repair

Independent review round 4 found that the root-specific matcher replaced the
nested substitution patterns and omitted backticks. For declared `run.sh`,
``exec.Command("sh", "-c", "sh `echo run`.sh")`` therefore returned BOUNDED while
excluding an undeclared caller. The owner directed a class-level repair under
AZU-V0-004 and AZU-V0-8901, covering root and nested scripts with the same
substitution vocabulary. No wire or declaration change is authorized or needed.

The guard now appends shared substitution patterns after its directory-specific
join and concatenation checks. Root callees cannot exclude a computed basename
by directory. Nested callees also recognize a quoted command prefix before their
directory. Both recognize backticks, `$()`, `${}`, `$VAR` and percent formats;
quoted concatenations and joins retain their prior checks. This is conservative
completeness detection, never inferred execution authority. The nested backtick
path suffix still stops at whitespace, preserving the existing negative tests
for raw-string closing delimiters and backticks in prose after a literal path.
An initial unrestricted command prefix matched markers in later arguments and
prose; the whole affected package rejected existing declarations (126.190 s,
rc 1). A second candidate bounded all format paths, rejecting existing declared
mentions (129.453 s, rc 1). The final guard preserves the original quoted-path
format admission, including spaces. Only the new command-prefix extension is
bounded to command words and a single path token. Additive negative regressions in
`TestConstructedCommandPrefixBounds_AZU_V0_8901` protect these boundaries.
Frozen diagnostics, selection schema, goldens and all existing assertions remain
unchanged. No additional design fork is necessary. Rollback must retain these
unknowns and verified caller selections rather than restore unsafe completeness.

`TestConstructedBasenameSpellings_AZU_V0_8901` is the shared integrated table:
29 constructions per directory, 58 cases total, covering substitutions in host
quoted strings, shell single/double quotes and representable Go raw strings,
percent formatting, quoted concatenation, split separators and joins (including
raw strings, trailing slashes, dot prefixes, lowercase join and joinpath).
Every row requires foreign-invocation UNKNOWN and selection of the known literal
caller's tests. Existing integrated and declaration-admission tests remain intact.
Before repair, the table exposed 10 unsafe BOUNDED cases, including root backticks
and nested substitutions preceded by `sh`. The whole shell package passed after
repair (0.763 s). Logs are outside the repository:
`/home/russell/lanes/azu-new-evidence/independent-root-substitutions/`.

| Log | SHA-256 |
|---|---|
| `shell-before.log` | `fce7c05d0a137031507262ffc3948d4faee8c4aca5e70f8ebae06b893f422621` |
| `shell.log` | `fbfa8083c65ed73b6e7f8f9945c304abfad2b1c8f27af24362b86b1971db0363` |
| `affected-overmatch.log` (rc 1) | `e1c0ed437ced81472b62e5cb43ff394ccc8bb53d8b811e35ccce35d94b9c9d1b` |
| `affected-under-admission.log` (rc 1) | `fce1698d47872f6f6f7c1169c1862403a7bdb7c66f8bbb97679b495fb59a9387` |

Self-development routes: preserve the existing pre-change capture; use the
requested `corvint affected --base dba1f4d6c9a33098470ccf57794fb66f989f03c8`
selection for focused shell and affected integration tests. Its actual result was
UNKNOWN, 186 selected, 86 unknowns, with installed-binary staleness retained as
degradation. The current-source CLI was built externally for final binding.
Final focused checks, one terminal CEM bind/check cycle, independent-review
finding disposition and friction are retained in the lane handback. Historical
full-suite/mutation qualification remains NOT_QUALIFIED. Publication, Tasks
writes, replacement capture, model-backed review, broad gates and seal are
excluded by the repair instructions; the orchestrator owns independent review
and host verification. No aggregate qualification or native completion is claimed.
