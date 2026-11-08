<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor self-review repair: operand and evaluation read effects

V1-5000, AFP-V0-3121/3122/3128. Owner self-review repair charter of 2026-11-07;
base `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, incoming head
`227c8f878252a341ee16f553331968ccb816f65c`.

## Independent findings and decision

The review reported three missed dependency classes: a literal prefix before a
shell glob, exported GOFLAGS surviving a simple command, and file reads discarded
while evaluating a make condition. Each could remove FRONTIER_SITE and omit a
reader and its callers. This round keeps the existing output and wire contracts,
retains claims where reads are unmodeled, and adds no filesystem execution.

File-command certification rejects unmodeled glob operands anywhere in a word,
including after `--`, in option values, redirections and shell file tests. The
closed admitted forms are literals plus `literal-dir/*literal-suffix` with an
explicit containing-directory read edge. Directory and suffix bytes are ASCII
letters/digits or `/._-+`; the directory is relative, nonroot and not an option.
Other patterns retain claims. The directory edge covers all matches, including
new extensionless files; no wildcard enumeration is claimed. Existing exact
option spelling/arity allowlists still apply. Quote removal may conservatively
reject quoted glob characters too.

Shell environment mutations (`export`, `local`, `readonly`, `declare`, `set`,
`unset`, `read`) have no admitted transition forms. Their effects keep the scan
uncertified across later commands. A standalone nonempty GOFLAGS assignment also
retains the claim. Make exports admit only `CORVINT_REQUIRE_NODE`, `HELP_BASE`,
`LANE`, and `DOGFOOD_FINISH_ENTRY`: these project controls are not interpreted by
the certified built-in commands. Their expanded values must have neither unknown
effects nor shell execution. All other names, computed names and export-all
forms retain claims. Target-specific modifiers and tab separators are parsed
before classification; unsupported exports conservatively affect the file.

Make evaluation now carries effect uncertainty separately from value ambiguity.
Enumerating an `if` condition cannot discard that state. The function effect
allowlist consists of the modeled string/word/path transforms (`strip`, `filter`,
`filter-out`, `findstring`, `word`, `wordlist`, `firstword`, `lastword`, `join`,
`addprefix`, `addsuffix`, `patsubst`, `subst`, `sort`, `dir`, `notdir`, `basename`,
`suffix`), conditional `if`/`or`/`and`, bounded `foreach`, checked literal
`wildcard`, conservative `realpath`/`abspath`, diagnostics `error`/`warning`/`info`,
metadata `origin`/`flavor`/`words`, and separately scanned `shell`. An admitted
function still propagates nested effects. Unknown functions and computed function
forms fail closed; `call`/`value` remain unproven. Effectful immediate assignments
and parse-time conditions keep the same uncertainty. This is an effect-domain
allowlist, not a claim of exact values for the conservative metadata/path cases.

The first expanded focused run found that rejecting all whitespace-containing
assignment names also rejected existing target-specific `override` assignments,
retaining all 199 root Makefile targets. That candidate restriction was removed;
explicit modifier parsing handles the export forms without that collateral rule.
The unchanged root narrowing assertion passes on the repaired implementation.
The first whole-package run also found a frozen narrowing assertion for
`.corvint/changes/*.cem.json`. Retaining every glob violated that existing
assertion. The final design preserves only the closed directory-glob form with
an explicit complete directory read boundary, and adds extensionless/new-file
and changed-directory edge regressions. It does not exempt that fixture by name.
No inherited test assertion or golden was changed.

## Acceptance and rollback

`TestFrontierSiteGlobOperandsKeepClaim_AFP_V0_3128` covers the reported LICENSE*
operand, question/bracket forms (including an unknown bracket class), operands
after `--`, input redirection, and an unknown option.
`TestFrontierSiteDirectoryGlobEdges_AFP_V0_3128` proves explicit read edges for the
sole admitted glob form and excludes an unrelated CEM.
`TestFrontierSiteExportEffectsKeepClaim_AFP_V0_3128` covers shell export and
standalone assignment, declarations, make exports, export-all, target modifiers,
tabs, unknown names/options, computed names, and effectful allowed-control values.
`TestFrontierSiteConditionEffectsKeepClaim_AFP_V0_3121` covers MODE reads, a future
function, call/computed dispatch, nested diagnostics/metadata/path functions, and
recursive versus immediate evaluation. Every case requires target/caller
selection for the hidden input and an unrelated sealed CEM. Existing positive
controls and root narrowing tests remain unchanged.

The affected receipt selected the affected package by DIRECT_SOURCE_CHANGE and
inherited gateselect tests by DIRECT_TEST_CHANGE. Its stale installed binary and
UNKNOWN scope are retained. Existing pre-change captures are reused by direction.
Whole-package test results, focused spec/citation checks and final binding evidence
are retained in the lane handback; raw logs stay under
`/home/russell/lanes/selfloor-evidence/review6/`. No independent semantic review,
historical replay qualification or aggregate AZU-V0 promotion is inferred.
The orchestrator owns independent review and host verification. No task writes,
publication, broad gates or seal are authorized. Rollback reverts this repair's
source, added tests and spec clarification together, restoring the known gaps;
it does not alter frozen outputs or weaken existing assertions.

## Focused verification evidence

All Go tests ran through `script/heavy.sh`, one package per command, with identity
variables unset and the configured shared semaphore. Final whole-package runs:
`go test -count=1 -timeout 30m ./internal/liveverify/affected` PASS (100.331 s), and
`go test -count=1 -timeout 30m ./internal/gateselect` PASS (1.825 s), both with
`GOTOOLCHAIN=local`. Final frontier-focused tests passed in 1.214 s. Go is 1.27.3.
`script/check-line-citations.sh --repin` found no moved citations;
`make -s spec-requirements-check line-citations-check` passed. The directory-glob
regression initially exposed filename-only matching for unanchored `data` tokens;
explicit directory anchoring repaired it before final verification. Earlier
failures remain in the logs, not counted as final passing evidence.

| External log | SHA-256 |
|---|---|
| `focused-final.log` | `09ac99cdfe9b2d1912b74e40a13e638e3d83bc0824d2f17a3b7c5abceca607fd` |
| `affected-test.log` | `71c0566d3c83f82d8469f910eeebb781c7db5f7a4b2fb571b68540bfdda58e2f` |
| `frontier-final.log` | `a4601d7db4f51425986da0df1b8f2024368dd5aec8b318859121689a6cbf049d` |
| `frontier-complete.log` | `14ad0be44eeb5dd35012d68d0abbce4c65017525f597d0f400b764c2e2d398a8` |
| `affected-final.log` | `485badde90dfc1f1b6c7051d3d58cfada4936eb364243784b97f26b9a5ac4d06` |
| `gateselect-final.log` | `e270a1283f0ea3fc7b0367e64e490cdb78ebe6ab85710e1cc257cfaa45da9df0` |
| `spec-citations-final.log` | `8af1ebf33aa1f4884352e61bb4b20059bc4ddac3a3f75d29e3dd02978045cc0e` |
| `repin-final.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
