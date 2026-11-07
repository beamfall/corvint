# cx-gates review fixes: V1-4538, V1-4539, V1-4540, V1-4542, V1-4543

Human intent: the 2026-11-04 fix-round charter and its independent CHANGES_REQUIRED findings.
Base remains `6c33df33bc5670131482f5263b410cb980580657`; reviewed starting head was `ae739c23d`.
Only the listed findings and their regressions are changed. No new requirement IDs or design fork.

## Findings and decisions

- P1 / V1-4542 / SRG-V0-982: preserve ASD-V0-925 unchanged. The Makefile starts with
  `line-citations-check`, then `spec-requirements-check`; the remaining targets are bytewise
  sorted and unique, one per line. The comment and shape checker enforce that exact contract,
  including duplicate prefix targets. Family 5 checks valid order, reversed/missing prefix,
  unsorted/repeated suffix, inline format, and staged versus unstaged repair. Its suffix assertion
  explicitly uses `LC_ALL=C sort -u`. The SRG-V0-982 requirement and trace row now state this shape.
- P2-1 / V1-4538 / SRG-V0-745: compute each lane's new IDs against its merge-base with BASE,
  then union them onto BASE's IDs. Do not require BASE to be an ancestor of every lane. The
  generator regression uses lane-a as the advanced 256-ID BASE and the older lane-b, proves the
  258-ID overflow diagnostic (not an ancestry refusal), and retains deduplication and deletion
  checks. Missing commits or merge-bases still fail closed; no merge or checkout is performed.
- P2-2 / V1-4539 and V1-4542 / SRG-V0-970, SRG-V0-982: static_checks now invokes the same
  staged shape checker as spec-status-check. Family 1 rejects malformed staged targets and passes
  their staged repair. Family 2 expands the known doc-gates recipe with several ordinary and
  order-only prerequisites; expansion matches the recipe without hard-coding prerequisite names.
- P3 and extra summary finding / V1-4543 / ASD-V0-780: STATIC_ONLY exits the family subshell
  and leaves the single summary to top-level finish. Family 5 checks exactly one summary and
  only family 1 selected, even with the retained baseline assertion failure. Timings below
  qualify the measured families individually; family 2 is excluded from a "well under 180 s"
  claim. The spec still mandates that bound only for CASES=4.
- V1-4540 / ASD-V0-640 needed no additional implementation repair; family 5 reruns its existing
  committed/staged/unstaged/untracked lane-test-selection and failed-test refusal regressions.

## Verification and timing

All shell suites below ran through `script/heavy.sh`; real Go test commands remain wrapped,
selected and one package per command. `corvint affected --base` ran before testing and reported
UNKNOWN (80 selected units, 80 unknown entries). Its unknowns remain advisory limits.
Raw logs, timing files and bind inputs are retained in `/home/russell/lanes/cx-gates.fix-round1`.
The environment reuses the shared writable heavy lock directory and home-disk Go cache from the
original lane evidence. Wall times include wrapper/setup overhead on this Linux host, with some
fixture builds overlapping; they are observations, not an isolated-host benchmark or an SLA.

| Command after `script/heavy.sh` | Wall seconds | Result |
| --- | ---: | --- |
| `CASES=1 script/batch-precheck_test.sh` | 8.56 | retained CI parser failure only; static-shape regressions pass |
| `CASES=2 script/batch-precheck_test.sh` | 146.80 | retained CI parser failure only; multiple-prerequisite regression passes |
| `CASES=3 script/batch-precheck_test.sh` | 75.15 | PASS |
| `CASES=4 script/batch-precheck_test.sh` | 2.59 | PASS |
| `CASES=5 script/batch-precheck_test.sh` | 12.12 | PASS, including shape checker and STATIC_ONLY regressions |
| `script/gen-spec-requirements_test.sh` | 10.19 | PASS, including advanced-BASE overflow |

The timing observation supports "well under 180 s" for families 1, 3, 4 and 5 on this host;
family 2 took 146.80 s and is not included. Both failing families complete all their assertions.
Their only failure is the pre-existing trusted-batch CI shell-block parser mismatch, reproduced
against BASE in the original `cx-gates.evidence/base-sync.log`; the workflow and parser are unchanged.
No full-suite PASS is claimed. The current logs retain every failure line rather than masking it.

`make spec-requirements` regenerated the staged specs successfully with unchanged generated files.
`make spec-status-check spec-requirements-check`, the standalone shape check through heavy.sh,
Bash 3.2 checking (239 scripts), requirement-definition and traceability checks pass. Spec checks
also ran their selected specindex/lrfrepo checks through heavy.sh, one package per command.
The final line-citation check passes after repinning one unchanged generator span from 531 to
537 (same content hash); the generator spec was already in scope. CEM evidence is recorded
by the handback after commit and rebinding.

Dogfood routes used: change-start query/impact receipt reuse, an actual task-context read, affected
selection and CEM/OCM binding/report inspection. Context built an in-memory snapshot (1.832 s)
because no index matched the starting tree. No retrieval/ranking/profile implementation changed,
so frozen learning evaluations, optional providers and external live qualification are not applicable.
Billed tokens, source-open telemetry and independent behavioral qualification remain NOT_OBSERVED.
The orchestrator owns independent review; no model-backed reviewer ran in this lane. Exhaustive
gates, Windows vet (no Go/module changes), Tasks writes, publication and sealing are NOT_RUN by
instruction. The final handback preserves the dogfood-check result, including any sandbox EXDEV.

## Friction with reproductions

1. `script/heavy.sh CASES=1 script/batch-precheck_test.sh` and CASES=2 exit 1 on the unchanged
   workflow's trusted-batch shell block, which --sync-check classifies as unsupported doc-gate
   commands (set/case/worktree/build/batch-check). Compare `cx-gates.evidence/base-sync.log` and
   the new case1/case2 logs; new scoped assertions pass. Parser changes are outside this fix round.
2. `/usr/bin/time script/heavy.sh CASES=2 script/batch-precheck_test.sh` took 146.80 seconds.
   The broad all-family speed claim is not qualified; only families 1, 3, 4 and 5 are included.
   Tasks writes are prohibited, so no follow-up ticket was filed here.
3. The configured heavy lock path `/home/russell/bench/rt/.locks` is outside the writable roots.
   Reuse `source /home/russell/lanes/cx-gates.evidence/env.sh` before heavy commands to use the
   host-shared `/home/russell/lanes/.heavy-locks`; retain this override for host-side checks.
4. Staging the generator exposes one positional citation move: `make line-citations-check`
   names `script/gen-spec-requirements.sh:531@e23d2d1f` moved unchanged to line 537. The standard
   `script/check-line-citations.sh --repin` updates only the in-scope generator spec; the rerun passes.
