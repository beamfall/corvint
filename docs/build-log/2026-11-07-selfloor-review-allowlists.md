<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selfloor independent review repair: explicit certification allowlists

Lane `selfloor`, ticket V1-5000 (`FRONTIER_SITE` slice), base
`538e823f4a1d220a3fb5b12c0529248f18924823`; incoming head `34cf9ffba`.
This entry records the 2026-11-07 orchestrator decision under the owner's
2026-11-02 delegation. The three P1 findings require closing the permissive
certification classes with allowlists. AFP-V0-3125 and AFP-V0-3128 are amended
in the existing experimental spec; no requirement IDs or public contracts change.

## Findings and decisions

1. **Immediate-recursive assignment (AFP-V0-3125).** `:::=` expands immediately
   but remains recursive for a subsequent `+=`. Treating it as simple captured
   `ci` instead of the later `local`. Rather than approximate another flavor,
   only `=`, `:=`, `::=`, `?=` , `+=` and `!=` retain modeled semantics. Other
   colon-assignment flavors retain every target as a computed site. This also
   covers `define`, target-specific forms and an unused or computed variable
   name; an unsupported flavor cannot disappear during the alternative walk.
   `TestFrontierSiteUnsupportedAssignmentKeepsClaim_AFP_V0_3125` checks six
   forms and selection on both a local-package change and an unrelated CEM.
2. **Go flag-value injection (AFP-V0-3128).** An unknown `-o` value can expand
   to `out -toolexec=script/audit.sh`. Every Go flag must now be recognized,
   with a literal value and correct arity. Unknown, executable and incomplete
   flags and unbounded values are refused, including attached and quoted values;
   shell glob expansion is also refused. The previous positive output control
   keeps all assertions and uses a literal cache value. Its original unbounded
   form is included in the negative regression, so its coverage is retained
   under the amended intent.
   `TestFrontierSiteGoFlagAllowlist_AFP_V0_3128` checks eleven forms.
3. **Command-option abbreviations (AFP-V0-3128).** Substring rejection allowed
   `sha256sum --chec` to read unrecorded manifest payloads. Bounded file commands
   now admit only explicitly listed, exactly spelled options with known arity.
   Unknown short options, unknown/abbreviated long options, manifest checking,
   incomplete values and values that can split through globbing retain claims.
   Commands without an option table admit no options. Pure commands retain the
   existing no-file-read boundary.
   `TestFrontierSiteCommandOptionAllowlist_AFP_V0_3128` checks fifteen forms,
   including unrecognized constructs, and requires a `FRONTIER_SITE` selection
   for the target and its dependents on an unrelated CEM (run-all fallback).

The earlier five reviewed repairs remain intact. No existing assertion was
removed or weakened. No JSON shape, field, error, exit status, stderr, committed
golden or core-freeze output was edited. Unsupported constructs deliberately
widen selection; this round makes no new replay or precision claim. Rollback
must keep conservative claims for these constructs; restoring permissive
certification restores the reviewed missing-edge risks.

## Verification and evidence

The pre-change capture is reused; `make dogfood-change` was not rerun at start.
`corvint affected --base 538e823f4a1d220a3fb5b12c0529248f18924823 --format json`
selected 390 units, with scope `UNKNOWN` and 70 unknown entries. It selected the
whole `internal/liveverify/affected` package by `DIRECT_SOURCE_CHANGE` and the
whole `internal/gateselect` package by `DIRECT_TEST_CHANGE`. The installed binary
reported `self-binary-stale` (built at `cb403735903b`); this receipt is selection
from that executable, not a claim of current-tree replay parity.

Commands and observed results (identity variables unset during tests/checks):

- `GOTOOLCHAIN=local go env GOVERSION`: `go1.27.3`.
- `script/heavy.sh go test -count=1 -timeout 30m -run TestFrontierSite ./internal/liveverify/affected`:
  PASS on the initial repair; 0.869 s test time, 91 s host-load admission wait.
- `script/heavy.sh go test -count=1 -timeout 30m ./internal/liveverify/affected`:
  PASS on final source and tests, including existing whole-package tests; 118.800 s.
- `script/heavy.sh go test -count=1 -timeout 30m ./internal/gateselect`:
  PASS, whole package; 1.947 s, 9 s semaphore wait.
- Whole-package commands set `HEAVY_WHOLE_REASON` to the selected package and
  reason. No `HEAVY_LOCK_DIR` override was used. All commands use
  `TMPDIR=/home/russell/lanes/selfloor.tmp`.
- `script/check-line-citations.sh --repin`: PASS after staging; no moved citation.
- `make -s spec-requirements-check line-citations-check`: PASS after the final
  spec edit. These targets run their own focused Go checks through `heavy.sh`.
- `git diff --cached --check`: PASS.

Raw evidence stays outside the repository in
`/home/russell/lanes/selfloor-evidence/review3/`:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| `affected-before.json` | 157319 | `6368af675aae7233f78d1e658a4e2ad452e59a393ccad616351ae279ec027722` |
| `affected-test.log` | 798 | `5d525245c3e6aa4a638c0be8ca138c8b1df2580080ac7183627e471e2b8c3806` |
| `gateselect-test.log` | 631 | `e781b0d4331a91e1c70ce07e30de0723a9824016716b30a130aa1d3407d72c8f` |
| `spec-check-final.log` | 3247 | `663ce8cf4c3a81649d75a35b256b23bbb5d4e325a0d4e1309f5dfb434d6c9f9c` |
| `repin-final.log` | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |

## Limits and handoff

Independent review and host-wide checks remain the orchestrator's work.
`make doc-gates`, selection-baseline-check, `corvint test`, gate-affected,
whole-repository tests/selection, `make gate` and Windows vet are NOT_RUN in
this repair: only the authorized focused package and spec checks run, and no
push is requested. Darwin execution is NOT_OBSERVED on this Linux host.
No Tasks writes, push, PR or seal. Final binding/checking results belong in
`/home/russell/lanes/selfloor.handback.md`; the workflow stops after dogfood-check.

Friction: the installed affected executable is stale (reproduce with the
selection command above); `heavy.sh` waited 91 s for host load and 9 s for a
slot (reproduce under the same shared host load); citation checks skip 180
basename-only references not tracked at root (reproduce with the repin command).
These observations do not establish a failure of the repaired certification.
