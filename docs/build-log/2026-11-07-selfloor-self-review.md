<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor self-review repair: complete certified read sets

Lane `selfloor`, ticket V1-5000 (`FRONTIER_SITE` slice), base
`538e823f4a1d220a3fb5b12c0529248f18924823`, incoming head
`f77ffc31e5c32db468e5e617f7c6453d510cad5b`. The owner's self-review repair
request names four P1 omissions. The existing experimental requirements
AFP-V0-3121, AFP-V0-3127 and AFP-V0-3128 are clarified in the same change;
requirement IDs and public contracts stay stable.

## Findings, decisions and executable witnesses

1. **Filesystem conditions (AFP-V0-3121).** Passing `wildcard missing` through
   as `missing` chose the true branch even when GNU make chooses the false
   branch. Literal wildcard names proven to exist through the explicit
   repository Source retain their exact spelling. Patterns, absent names,
   `realpath` and `abspath` remain unknown. An unknown condition scans both
   branches; an unknown operand retains uncertainty. Immutable analysis uses
   its supplied filesystem, never an unrelated live host filesystem. Regression:
   `TestFrontierSiteFilesystemCondition_AFP_V0_3121` requires an absent
   `wildcard` or `realpath` false-branch package to select the target and callers.
2. **Extensionless operands (AFP-V0-3128).** Literal file operands of bounded
   commands, file tests and input redirections now record path imports and
   path tokens without requiring punctuation. Path imports reach owners;
   tokens also reach unowned files. General text tokenization keeps its
   existing heuristic. Regression:
   `TestFrontierSiteExtensionlessOperand_AFP_V0_3128` covers `cat LICENSE`,
   a quoted space-containing filename, a file test, an input redirection and
   a changed working directory. Each must select the target and callers on
   that operand and exclude them for an unrelated sealed CEM.
3. **Deferred trap execution (AFP-V0-3128).** Every `trap` remains uncertified:
   a deferred command can run after environment or directory changes. Parsing
   the body under the current environment cannot establish its read bound.
   `TestFrontierSiteDynamicTrap_AFP_V0_3128` uses `cat "$$INPUT"` in an EXIT
   trap and requires the frontier claim and dependent selections.
4. **Repository shell wrappers (AFP-V0-3127).** A shell basename alone cannot
   prove a repository executable's reads. Plain-shell certification accepts
   bare known shell names or their exact `/bin/` and `/usr/bin/` paths; other
   paths retain uncertainty. Regression:
   `TestFrontierSiteRepositoryShellWrapper_AFP_V0_3127` installs `tools/bash`
   reading `policy.json` and requires a frontier selection when policy changes.

The absent-wildcard and wrapper fixes widen conservatively; no new precision
or historical-replay claim is made. No existing test assertion, committed
golden, wire shape, field name, error/exit code, stderr contract or core-freeze
output changed. Rollback must preserve conservative claims for these cases;
restoring argument passthrough or basename certification reopens the omissions.

## Verification

Pre-change capture is reused as instructed. The existing affected route selected
390 units with scope `UNKNOWN` and 70 retained unknowns: the entire affected
package by `DIRECT_SOURCE_CHANGE` and gateselect by `DIRECT_TEST_CHANGE`.
Its installed binary reports `self-binary-stale`; this is selection evidence,
not current-tree replay parity. Toolchain: `go1.27.3`.

The first focused repair run failed all five extensionless cases: path imports
alone did not reach unowned files. That failed evaluation is retained in
`review4/regressions.log`. Adding path tokens resolved it; the focused four-test
run passed (0.071 s). No assertion was weakened.

The first whole affected-package run failed the existing bounded literal
wildcard control (124.895 s). No assertion was changed: literal existing paths
are now checked through Source.Stat, while patterns and missing paths stay
unknown. The complete focused frontier suite then passed (0.866 s). The new
absent-filesystem-condition test also covers an immutable in-memory source.
The first gateselect whole run passed (2.607 s); final verification below uses
the corrected source and includes all existing tests again.

Final verification, with identity variables unset and
`TMPDIR=/home/russell/lanes/selfloor.tmp` (0700):

- `script/heavy.sh go test -count=1 -timeout 30m ./internal/liveverify/affected`:
  PASS, **133.653 s**, all existing tests and final regressions.
- `script/heavy.sh go test -count=1 -timeout 30m ./internal/gateselect`:
  PASS, **2.064 s**, whole package against the final source.
- Both whole runs set `HEAVY_WHOLE_REASON` to their affected-plan selection;
  one package per command, standard heavy.env, no lock-directory override.
- `script/check-line-citations.sh --repin`: PASS; no moved citation.
- `make -s spec-requirements-check line-citations-check`: PASS after final spec
  edits; its Go checks use heavy.sh. Generated indexes unchanged.

Raw evidence directory: `/home/russell/lanes/selfloor-evidence/review4/`.

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| `affected-before.json` | 157655 | `151fb677cd43693ca3ab049aadddcf32f9b3a2173ce8e225a1b8e6c3c61803cb` |
| `regressions.log` | 2957 | `5662f76b90baa677f63e935e3819fdcc32b41b95dd2f3b1f2181d863080ba58d` |
| `regressions-final.log` | 836 | `f883b874e15b4db39a540ae00a2ba2d60e81e9a5bbaa8a5c0bdd5619fd7c67a2` |
| `affected-test.log` | 1447 | `8635d37f5826fa59319627afe4676ffe4337769a1a26e8710dee8ccd73e47095` |
| `frontier-final.log` | 1045 | `3e238fc9a60031d4080c5a269d5cff68e351355ad24f872d9d9999e15bdca16e` |
| `affected-final-test.log` | 1049 | `1821a2dc8f8ff8d7b96d4001b83df59063475d29d9d31c5c038780bf00b30cc9` |
| `gateselect-final-test.log` | 843 | `e2f5f13c308a8b019183776da20b8bf94d06ea9f9a2b071dd2cce4363502ed11` |
| `repin-final.log` | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `spec-final-check.log` | 3794 | `801ef0e51ee0487617ee9826c7d94fbde66204a4f681cddb293e33ef8d91fdfe` |

## Limits and friction

Independent review remains NOT_OBSERVED, delegated to the orchestrator by the
repair charter. Pre-change capture was not rerun. Repository-wide tests,
selection-baseline-check, doc-gates, corvint test, gate-affected, make gate and
Windows vet are NOT_RUN: this round authorizes focused package/spec checks,
with broader host verification afterward and no push. Darwin qualification
is NOT_OBSERVED. No Tasks writes, push, PR, auto-merge or seal.

Friction, with reproducible commands: (1) the affected command above reports
self-binary-stale at the incoming head; (2) heavy.sh runs under concurrent host
load queued for up to 150 s in the final spec check (use the same commands and
shared heavy.env to reproduce admission delay); (3) heavy.sh reports /tmp free
2 GiB below its 3 GiB warning floor even with the required lane TMPDIR; (4)
`script/check-line-citations.sh --repin` skips 180 basename references not
tracked at repository root. No override or false coverage claim resolves
these observations. Binding and dogfood-check results are retained in the
external handback after all tracked edits are final.
