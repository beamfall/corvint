<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor independent review: literal exports and filesystem conditions

V1-5000, AFP-V0-3121/3122/3128. Owner repair charter of 2026-11-07;
base `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, incoming head
`928cd0a1c06875c433aeea23dd9cfbaf0a0d21d1`.

## Findings and decision

The independent review found two remaining omissions: export effects survived
only as uncertified state, which a literal recipe did not promote to a frontier;
and filesystem condition values could discard the filesystem dependency itself.
Both can omit the target and its callers after a hidden input changes.

Environment certification now retains an unbounded site independently of make
expansion. Shell export/declaration mutations have no admitted transition forms.
Nonempty or unresolved GOFLAGS also retains the site through standalone,
command-local and env forms. Make exports retain the existing closed name
allowlist: CORVINT_REQUIRE_NODE, HELP_BASE, LANE, DOGFOOD_FINISH_ENTRY. Their values
must have no unknown effects, shell execution or filesystem reads. All other
names, export-all and unsupported forms retain the file's targets as sites.
The ordinary literal-command selection policy is otherwise unchanged.

Function effect certification uses a closed allowlist separate from value
calculation: strip, if, or, and, filter, filter-out, findstring, word, wordlist,
firstword, lastword, join, addprefix, addsuffix, patsubst, subst, sort, dir, notdir,
basename, suffix, error, warning, info, bounded foreach, origin, flavor, words,
separately scanned shell, and checked literal wildcard. Nested effects still
propagate. Every other function remains unproven even if its value is discarded.
In particular realpath and abspath keep effect uncertainty; no path resolution
or external filesystem execution is introduced.

The existing bounded-wildcard assertion requires a sound explicit edge rather
than blanket refusal. The only admitted wildcard operands are existing relative
repository paths composed of ASCII letters/digits and /._-+. Each read is
retained independently of the returned value, including through conditions,
recursive references and immediate parse-time evaluation. Path imports reach
owners and anchored tokens reach unowned files and directory descendants.
Patterns, absent paths and other syntax fail closed. New function value support
cannot silently certify its effects; it must enter the separate allowlist.

## Acceptance and rollback

`TestFrontierSiteLiteralExportEffects_AFP_V0_3128` exercises eleven literal
recipes: shell and make exports, unknown names/options, assignment/export,
command-local and env GOFLAGS, export-all, target exports and an effectful
allowed control. Hidden AUDIT_INPUT and an unrelated CEM select check and its
make/CI callers with FRONTIER_SITE.

`TestFrontierSiteFilesystemConditionEffects_AFP_V0_3121` exercises realpath,
literal wildcard, wildcard patterns and a future function, with MODE present
or absent and recursive or immediate evaluation (sixteen cases). Every MODE
change selects check and its callers. The bounded literal wildcard uses an
edge and excludes an unrelated CEM; all other forms retain FRONTIER_SITE.
The initial new regression run failed on both reported classes before the fix.
Existing assertions and goldens remain intact.

This is the charter's explicit test-selection soundness repair, with no new
wire shape, field, error/exit code, stderr or core-freeze output. Rollback reverts
this repair's code, added regressions and spec clarification together, restoring
the known omissions. No aggregate AZU-V0 qualification or precision claim is made.
The orchestrator owns independent review, host integration and ticket completion;
no Tasks writes, publication or seal are authorized. Original pre-change captures
are reused as instructed, with their provenance limits retained.

Raw evidence and final binding reports stay outside the repository under
`/home/russell/lanes/selfloor-evidence/review7/`; the handback records commands,
results, unavailable evidence and final CEM checking.

## Verification evidence

Go 1.27.3; every Go test ran through script/heavy.sh with the configured shared
semaphore and identity variables unset. Final whole-package runs (no -run):

- ok   github.com/Beamfall/corvint/internal/liveverify/affected 111.698s (PASS).
- ok   github.com/Beamfall/corvint/internal/gateselect 1.818s (PASS).

All frontier-focused tests passed (1.591 s) before the final literal-character
allowlist restriction; the final whole-package run includes that restriction.
The required spec-requirements-check and line-citations-check passed; staged
repinning found no moved citation. No pre-change capture, broad gate, independent
model-backed review or seal ran. Original regression failures are retained.

| External log | SHA-256 |
|---|---|
| `regressions-before.log` | `e7368f7b55b24daa4d236ff9c1ec28a18bfd3a180c04dd6b398ce718aa8a0ba2` |
| `frontier-after.log` | `0e474ad734ccd617de0dfc32569c6dc52435c6e74ac7d6216c0fa1a77b41e295` |
| `affected-final.log` | `0e55acdabddde6e7a76d0223a9ceef4f50c12c4907197ce7b95b5be8141943cf` |
| `gateselect-final.log` | `11dca7e36a9cfc3234ab0b2a6de1192bc9b30d6bf1b43c1707714cd66e1ae2e7` |
| `spec-citations.log` | `c0f05e29c1fdf8eb83803db2614fe8cc144cb07a7ee389f55c27e9f6af3a97b3` |
| `repin-final.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
