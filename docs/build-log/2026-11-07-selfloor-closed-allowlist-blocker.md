<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor closed certification: frozen acceptance conflict

V1-5000, AFP-V0-3121/3122/3128, owner orchestrator class charter of
2026-11-07. Base `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`; incoming
head `27d7c001ebc69382126ea807fb6a7bbec4b16723`.

## Independent review findings

The four reported examples already have repairs and regression tests at the
incoming head: ASCII `strip`, extensionless attached file options, executable
make options, and explicit GOFLAGS. Those instance repairs do not establish
the requested closed certification domain. This round is **PARTIAL**: the
owner's new rejection rules and frozen narrowing assertions conflict.

The proposed class is certification without a fully modeled function or command
operand grammar. The candidate used this make function allowlist: `strip`, `if`,
`or`, `and`, `filter`, `filter-out`, `findstring`, `word`, `wordlist`, `firstword`,
`lastword`, `join`, `addprefix`, `addsuffix`, `patsubst`, `subst`, `sort`,
`foreach`, and proven literal `wildcard` names. Unmodeled functions and unresolved
eager arguments retained uncertainty, including inside discarded conditions.
It preserved finite conditional alternatives and existing literal package edges.
Command certification separated no-operand flags, literal values, path values,
and package patterns, rejected unknown forms and attached unmodeled values,
and rejected every environment assignment, env command and unmodeled wrapper.
The candidate also closed shell option, test operator and jq filter admission.
These are candidate bounds, not an accepted or delivered amendment to the spec.

The focused run and whole affected-package run rejected the candidate on these
unchanged existing assertions in `driver_frontier_bound_test.go`:

- `TestFrontierSiteFaultInjection_AFP_V0_3120/info-never-expands`: requires an
  `info` expression containing an unresolved argument to leave no site.
- `TestFrontierSiteBoundedSetExcludesPath_AFP_V0_3122`: two cases require
  `env GOCACHE=...` and direct `GOCACHE=...` recipes to lose their site. The new
  charter explicitly requires both forms to remain uncertified.
- `TestFrontierSiteParseTimeCommandKeepsClaim_AFP_V0_3127`: the
  `unused-recursive-shell` and `recipe-shell-bash` cases use a GOCACHE assignment
  in their shared recipe and require no site.
- `TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127`: requires at
  least a quarter of root targets to lose their sites. The strict candidate
  retained all 199 targets in both variants, including previously admitted
  make-function and environment forms.

The whole affected package failed in 121.219 s. A new env -S fixture also lacked
an evaluated expression and consequently tested outside this slice's frontier
removal trigger. Its corrected candidate form was retained in the external patch;
that final patch was not subsequently qualified. No existing assertion or golden
was edited. No code, wire, output or governing requirement change from the
candidate remains in this worktree.

## Disposition and evidence

The hard repair rule requires reverting a change whose completion needs a frozen
contract change. The candidate was therefore restored to the incoming code and
tests. The independent findings remain open; the earlier handback's instance
repair success is not class closure. Required owner action: reconcile the strict
allowlist requirement with these exact narrowing expectations, then qualify the
candidate against the resulting acceptance contract. No task write was attempted.
A report or CEM pass cannot resolve that acceptance conflict.

Raw candidate and test evidence are outside the repository under
`/home/russell/lanes/selfloor-evidence/allowlist/`:

| Artifact | SHA-256 |
|---|---|
| `closed-allowlist-candidate.patch` | `49ed19d6f147318a6fb96c73cf6c1b571859b656a58da360ee25b042af97ff4c` |
| `affected-test.log` | `f7dd05866a175001cab5e2cdaec1424a2a9feee9b8b49a352f7591a2f0d03504` |
| `focused.log` | `c2f1fd22edf6e61490da2e0576ace8f5ff5a08d35f9b901e5761968ab7b7b53f` |
| `regressions.log` | `aceffd14ed64ebe4bcddeede91b2b0b7c309788b65a813d4df31c66548b960b3` |

The affected route was used to select the two lane packages; its installed-binary
staleness and UNKNOWN scope remain explicit. Existing pre-change receipts are
reused by the repair instruction. Final whole-package verification of the restored
code and CEM checks are recorded in the handback. Wider selection, frozen replays,
model-backed review, publication, task writes and seal are excluded by the owner.
The final evidence bind describes a PARTIAL disposition, not delivery of the new
allowlist capability. Rollback is complete for the rejected candidate; the external
patch preserves the proposed work for an authorized follow-up.
