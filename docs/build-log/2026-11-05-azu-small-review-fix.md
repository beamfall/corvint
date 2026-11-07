# AZU small independent-review repairs

Date: 2026-11-05. Starting HEAD: `146657fd58709f3908a828b6fe91c3cf39e44f3f`.
Base: `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`.
Owner scope: the two P1 findings in the repair charter; V1-4689 / AZU-V0-006
and V1-4690 / AZU-V0-007. Other lane tickets retain their earlier disposition.

## Decisions and regressions

The current workflow recognizes a browser consumer only through the legacy
`tools/web-flows/test/` check containing `UNSELECTED_BINARY_CONSUMER`. A runnable
consumer's argv alone no longer matches. The charter excludes `.github` edits,
so CI session planning retains the existing legacy check alongside the exact
declared argv. Local execution still uses the project-owned argv. No manifest
member, error/exit code, diagnostic wording, golden or core-freeze output changes.
`TestAZUV0007CIRunnableBrowserConsumerRequired` uses the extended declaration for
`cmd/corvint-web-flows`, changes its dependency, and requires a complete NARROWED
CI manifest with both checks. It evaluates the current workflow predicate's
check branch and verifies that those predicate bytes remain in the workflow.
The existing local consumer execution assertions remain intact.

The coverage proof inspected the changed owner's caller closure, but examined
readers only for tests. A verified shell declaration can name an unowned Makefile:
that declaration removes the shell frontier without creating a modelled caller
edge. The proof now inspects reached readers attributed to the changed path and
walks their caller closures through the same uncertainty inspection as the owner.
Unowned declared callers and unresolved inbound frontiers prevent an `uncovered`
result. Existing test and execution observers still prevent absence claims.
`TestAZUV0006ReaderCallerUncertainty` covers literal, glob, scoped and unbounded
readers, directly and through a modelled caller. The public-CLI regression
`TestAZUV0006DeclaredReaderUnownedCaller` uses changed testless `data/data.go`,
`check.sh`, and a verified foreign-caller declaration naming `Makefile`; it
requires exactly the Go owner's NO_SELECTABLE_TEST uncertainty, UNKNOWN scope
and no uncovered entry, so invalid declaration fallback cannot mask the bug.

The existing experimental spec states both obligations and traces the regressions.
Rollback: revert this repair's implementation/spec/test commit and rebind the CEM;
doing so restores the two reported risks. A separate workflow PR can later admit
the runnable check directly; this repair does not depend on its landing.

## Verification and boundaries

The original pre-change receipts are reused under the repair instruction; no new
start capture is run. Installed `corvint affected --base` selected both touched
Go packages before tests. Its retained UNKNOWN entries include build constraints,
nested modules, binary-exec/read-scope declaration validation, unmodelled shell
callers and an unindexed hidden test path; no zero-unknown claim is made.
All Go tests use the configured shared `script/heavy.sh` semaphore, one package
per invocation, with Go 1.27.3 and private TMPDIR. Whole touched packages are the
terminal test evidence. The host-side orchestrator owns independent review.

Used self-development routes: affected selection and native CEM binding/checking
with the original context. Retrieval/learning evaluations, mutation campaigns,
browser execution and external interoperability are not implicated by these
repairs; existing qualification gaps remain visible. Full repository gate, Tasks
writes, publication and sealing are excluded by the charter. Cost/token savings
remain NOT_OBSERVED. Raw evidence stays outside Git under
`/home/russell/lanes/azu-small-evidence/review-fix/`.

## Observed checks

- Focused affected regressions: PASS (0.033s), after 45s of configured load admission.
- First focused CLI attempt: build failure from an incorrect `options.profile`
  reference in this repair; corrected to `input.RunnerProfile`. The retained retry
  passed all three named CLI regressions (0.626s). No assertion was weakened.
- Entire `internal/liveverify/affected`, including existing tests: PASS (260.664s).
- Entire `cmd/corvint`, including existing tests: FAIL (519.021s), 46 top-level
  failures. Their names exactly match both the previous whole CLI run and the
  original-base failure set retained in `wave1105b/failure-attribution.json`.
  There are no new failure names. This comparison reuses historical evidence at
  its original base `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`; it is not a fresh
  base run and matching names alone do not prove matching causes. Current logs
  explicitly show unsafe group-writable `/home/russell/lanes` ancestry, Git root
  detection through that parent, source qualification refusals and JavaScript
  adapter failures. Full CLI host qualification remains blocked; the local
  outcome remains `failed`. All three new regressions pass separately.
- Spec metadata, requirement definitions, traceability and line citations: PASS
  (16.0s admitted command). Metadata did not need regeneration. Repinning staged
  edits found no unchanged citation content moved; no further repin was requested.
  The first unstaged repin note and its staged correction are retained.
- No inherited Git author/committer identity variables were present in test
  commands. The required TMPDIR is mode 700; its parent ancestry remains mode 775.

SHA-256 digests of completed raw evidence (all files remain outside Git):

- `affected.json` (145106 bytes): `8bcb78fa94358fd703b2b07b0f8939176d12c5763b6107bca0c583582ddf416e`.
- `test-affected-focused.log` (923 bytes): `a07708809453a9de7a824660e06c8466aa890a66191f09e36811b437a9ad7c47`.
- `test-cli-focused.log` (1135 bytes): `beec2188033b222f5a3ec05117e3138058f0777242d2335893e2a12723c80d0e`.
- `test-cli-focused-retry.log` (764 bytes): `13c416444cd85e3418f2a53003912b9414b28c1b45f61803ecd7d586aa00f28f`.
- `test-affected-whole.log` (798 bytes): `cb948149291b46c2af8da4119240b850eb569c62834b57996fad7e322a76b740`.
- `test-cli-whole.log` (66217 bytes): `a05cc66c6efe9243d6ff2b3272255d8f078ca61c776eba8a32a88134b6be024c`.
- `failure-comparison.json` (570 bytes): `50a3fabc179a4987d35bff48c12cb394f8ecf53150870141d599b313c25983d2`.
- `scoped-gates.log` (3933 bytes): `1c7e2991dc32a44a53a1dbb840105f79f72535d2f6b1aea6cb2308c7231fab2d`.

The frozen selection baseline passed: 33/43 merged cases narrowed (floor 30),
zero misses across 26 failing packages from 15 failed CI runs, zero test omissions,
and zero test misses. The admitted foreground command took 932.5s, with no output
beyond admission until completion. This is shared-host verification, not a latency
or quiet-host comparison. The final staged citation repin and line-citation gate
also passed. No new line movement was reported.

The tested Go diff against the starting HEAD has SHA-256
`8b80889625ec08bbd6e5a140fa03cd859edbcfb141cd20f2958c683af8c2b44c`;
no Go source changed after the whole-package runs. Final CEM binding and checking
are post-commit and recorded in the external handback. They retain the failed
local outcome and stop before sealing.

- `selection-baseline.log` (10438 bytes): `3918d0884d3331eded19afd4acdb85a32d6d6f1f8f71d9e5bc3535cffbdc4983`.
- `selection-baseline.json` (927770 bytes): `1df4ec8535d9d96db31c0534551316149819a9b9facafbb8b7090aa3d4b057b3`.
- `citations-repin-final.log` (190 bytes): `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`.
- `citations-check-final.log` (118 bytes): `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba`.
