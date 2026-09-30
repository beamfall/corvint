# Proposed query trace abstention for structural closeout

The owner's 2026-09-30 request to merge the six experimental workflows exposed a closeout
blocker (V1-0523): the unchanged original `DOGFOOD_TASK` selects authority-start, which refuses a
clean tree containing a retained local trace. The exact native refusal is intentional under
GPK-V0-028. Rewording that task or deleting the earlier trace would hide evidence.

The proposed/experimental QAT-V0-001..005 contract adds a separate query abstention receipt and
amends DCW-V0-005/015/025 explicitly. The row stays `NOT_PRODUCED` with reason
`authority-start-trace-state-abstention`. Canonical artifact bytes bind original NUL argv/task,
base/target, exit 2 and raw stdout/stderr hashes. The checker rejects drift, duplicate/extra
members, duplicate query rows, invalid argv shape and a reused reason on another step. It runs
only a fixed query command through independently selected base/current and optional override
verifiers, never the recorded executable. An identical wrong result is insufficient. Existing
CEM/OCM, selected checks, clean worktree and local-outcome checks remain required. The query and
its coverage remain unavailable; no query, authority, release or usefulness claim is promoted.

Gate A independent review retained no HIGH findings. Its required concerns shaped the exact row
and successfully written artifact exemption, strict pre-replay argument validation, transition
cleanup tests, failed closeout-row tests and the owning daily-contract amendment. Original
failed receipts and trace bytes remain in the root coordinator's private evidence; the builder
did not alter `.corvint` maps or traces. Independent implementation review and live final CLI
qualification remain root-coordinated closeout requirements, not claims made by this entry.

Focused evidence: `go test -count=1 -timeout 30m ./internal/dogfoodflow` and
`go vet ./internal/dogfoodflow` pass (final package test: 1.751s). Spec requirements,
requirement definitions and traceability metadata checks pass; older requirement IDs and spec
index order are preserved. The line-citation check found one shifted reference in
`docs/SPEC-TOOLCHAIN-INTEGRATION.md`; its exact owning-path expansion and correction were
handed to the root coordinator before final closure. Tests cover exact replay; abstention-to-success and other
failure transitions without stale artifact/digest; corrupt/unreachable diagnostics, extra stderr,
nonempty stdout and wrong statuses; hostile artifact/report/argv shapes, including recomputed
artifact digests; base/current/override disagreement; unanimous wrong results; and blocking
failed local outcome/CEM/OCM rows. These synthetic tests are not live CLI qualification.

Self-development routes used: installed `corvint query --task 'query authority-start trace
abstention dogfood workflow' --limit 1` at builder start returned original governance/source
pointers with omissions and mixed-worktree uncertainty; bounded source/spec reads supplied the
owning contract. This builder orientation is distinct from the unchanged original user task.
`corvint affected --base 7e0950fe51cedb60e8179f20a6ff15f95cc776d5` retained its selected-test
advice and unknowns before focused checks. The root coordinator owns final CEM/OCM, original-task
qualification and frozen outcome evidence. Mutation, source-documentation and external-provider
routes are not applicable to this local completion exception. No full `make gate` was run for
this scoped issue, following the repository's standing focused-check preference; affected-plan
advice does not qualify exhaustive coverage. Installed tools observed by the builder were Core
1.0.0-rc.1 build 163 and Tasks build 202; current release checking belongs to the root session.

Rollback removes the query exception so the original refusal blocks completion again. Keep the
retained trace, original task, raw refusals and failed evaluations. No migration or history rewrite.
