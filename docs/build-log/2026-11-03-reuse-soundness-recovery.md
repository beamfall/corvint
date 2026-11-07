# Verified reuse: soundness corrections and execution-evidence hold

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Items: V1-1097 and V1-1099.
This entry qualifies the earlier `2026-11-03-reuse-verified-results` measurements and decisions.
Both capabilities remain experimental; the retrospective replay does not authenticate execution
or qualify hosted reuse.

The frozen historical snapshot contains 47 merged PRs with usable re-push outcome pairs. A
read-only refresh found 84 merged PRs versus the snapshot's 82, only two additional PRs; even if
both qualified, the retained 47 would become 49. AFP-V0-2333's 50-PR threshold remains
NOT_PRODUCED, as does owner acceptance of the experimental spec/amendment.

## Decisions and findings

AFP-V0-2321/2322 now discover the test closure with the recorded build flags, reject unsupported
flags and invalid values, and bind the architecture tuning settings. Inputs marked
assume-unchanged or skip-worktree count as dirty even when Git status omits them. Git observation
disables replacements, filesystem monitors and the untracked cache. Negative controls cover a
dependency file selected only by `-race`, both hidden index flags and unsupported `GOFLAGS`.

AFP-V0-2326 now reopens admitted record paths and recomputes eligibility and identity for the
actual local plan invocation before removing any command. Record names retain their supplied
paths. Harness identities preserve directory markers so re-evaluation discovers additions.
Missing records, malformed re-evaluation, changed inputs and changed identity run the work.
`TestHarnessDirectoryReevaluation` covers an added harness file; the shell policy fixture covers
decider failure, malformed output and moved HEAD.

AFP-V0-2342 now matches changed paths anywhere below each compiled package directory. This
conservatively covers embedded subtrees, including deletions; the sibling-directory negative
control still skips. Existing Windows-only dependency and interop formatting fault injections
continue to require RUN and demonstrate the downstream failure.

A further AFP-V0-2322 fault probe reproduced a false reuse under an external `go.work`:
changing only its `godebug panicnil=0` directive to `panicnil=1` left the digest equal, while
the actual package test failed. Identity observation now refuses every workspace other than
`off`, empty, or the hashed repository-root `go.work`, with `unmodeled-workspace` and no decision.
`TestWorkspaceInputsFailClosed` covers external and nested workspace refusals, an unchanged root
workspace positive control, and a root directive change that changes the digest. The original
failure is retained in `resume/workspace-probe.log`; `resume/workspace-fixed.log` proves exit 3
and no output decision after the repair. The later source edit requires renewed focused checks
and terminal gates; the earlier passes are not presented as checks of that edit.

AFP-V0-2327 remains blocked by **UNAUTHENTICATED_EXECUTION**. The earlier caller-boundary decision
does not establish trusted execution: PR test processes can overwrite their recorder, outcomes
or generated artifact in the same runner. The producer and consumer are both hard-disabled;
repository variables cannot enable them. A protected collector and its falsification evidence
require a separate control-plane review. This is an unresolved delivery boundary, not a repaired
authentication claim. The CI hold ships in its own commit. AFP-V0-2344 job skipping remains dark
behind its existing opt-in and control-plane review.

## Verification and local outcome

The focused `internal/testreuse`, `internal/jobinputs`, `tools/ci-reuse-plan` suites, selected
reuse/job CLI, use-case and help tests, and both shell policy groups pass. Those Go tests ran
through `script/heavy.sh`, one package per command. Windows vet passes. `gofmt -l` is empty and line
citations pass. The initial documentation gate found unstaged regression tests; after staging,
the focused traceability gate passes. Final gate results and command logs are retained in the
lane handback.

An integrated local fixture used the built binary, a real passing Go invocation, its observed
outcome and a real saved reuse decision. `heavy.sh --plan` reused the clean package. Two successive
dirty edits each restored the command, and the injected test failed when run. Restoring the
fixture permitted reuse again; changing `GOAMD64` or removing the record restored the command.
The final-build receipt is `/home/russell/lanes/reuse-evidence/resume/local-e2e-final.log`.

Actual feature use: the resumed orientation retained `index --if-stale`, `affected --base` and
the original requirement-bearing `context --task` packet. The installed binary reported
`self-binary-stale`; its affected scope was `UNKNOWN`, so focused checks were chosen explicitly.
The existing pre-change receipts remain in the worktree's private dogfood directory. CEM/OCM
binding and the reviewer report remain the required local closeout. Retrieval/learning evaluations,
optional providers, consoles, migrations and release routes are not applicable: neither ranking,
learning, those integrations nor a release changed. Independent review of the final fixes belongs
to the orchestrator. Publication, task-store writes and sealing are excluded by the lane charter.

## Repeated historical measurement

The same frozen history was replayed after the six original corrections, using one binary for
each replay's baseline records and decisions: SHA-256
`e8cf73c0ab8e7bbd9ec998d340f46e0ecca1fddd6bbfefd2fa27383a4ad3bb6d`.
Original results are retained under `/home/russell/lanes/reuse-evidence/resume/`.

| Measure | Result |
|---|---|
| V1-1097 re-push pairs / merged PRs | 81 / 47 |
| Later-run packages / observed failures / reused failures | 30094 / 22 / **0** |
| Reused package passes | 10125 |
| Counterfactual runner minutes saved | 2830.5 of 13219.8 (21.4%) |
| Modelled 16-shard wall, summed over 81 pushes | 849.2 to 723.5 minutes (14.8%) |
| V1-1099 planned runs / unsafe skips | 100 / **0** |
| Archive skips / saved observed runner minutes | 34 / 185.0 of 701.0 (26.4%) |
| Interop skips / saved observed runner minutes | 88 / 32.4 of 35.3 (91.8%) |
| Containment skips | 0 (`PLAN_SCOPE_UNKNOWN`) |

The test figures match the earlier replay; the corrected subtree rule reduces archive skips from
36 to 34 and saved minutes from 200.5 to 185.0. Runs 41565726798 and 41569979658 now RUN for a
changed child path, `internal/jstestprovider/externallive/loopback_test.go`, under a compiled
package directory. This is a deliberate conservative inclusion, not a claim that the child file
is itself embedded. No declared job failed in this historical window, so its zero-violation
result does not supply a hosted failing positive case; local fault injections remain separate.

After the external-workspace fault was repaired, the final binary
`a8fd32cb53dc3a0a9202d514179b3d8413d3ba1ece73c99d2bb7aad56ba15514` rechecks the test decisions.
The existing passed records are reused under AFP-V0-2324: all bind `GOWORK=off`, and that record
schema and input digest are unchanged by the new workspace refusal. Only record bytes are copied;
every decision is recomputed into `resume/tests-final-out`. This disposition preserves the record
admission contract instead of claiming the two executables are byte-identical. The job-input
algorithm did not change after its repeated replay. Final refresh results are retained in the
handback. These are retrospective predictions, not observed hosted savings or proof of trusted
execution.

## Friction and retained limits

The requirement-ID checker refuses the assigned four-digit range (`bad-range 2320..2359`); a
direct immutable-base search found no collision. Its three-digit-only restriction is retained,
not worked around by claiming the checker passed. `spec-requirements` regenerated successfully.

The configured semaphore directory `/home/russell/bench/rt/.locks` is outside the writable
sandbox. Commands used the existing writable `/tmp/corvint-heavy-1000` directory, eight slots,
and the normal disk-backed Go cache; coordination with processes using the configured directory
is not established. The earlier resume also hit tmpfs exhaustion; this resume used the
disk-backed temporary directory. Existing replay worktrees use disallowed shared Git metadata
paths, so replay uses an independent private clone. Its initial hardlink clone failed with
`Invalid cross-device link`; `--no-hardlinks` succeeded.

Commits report a read-only `/home/russell/projects/corvint/.git/packed-refs.lock` warning after
landing successfully. HEAD and author were read back; no commit was repeated. The first resumed
dogfood preparation exposed an inherited unused import, which was removed before verification.
No additional shared Git metadata path was made writable.

`make capability-catalog-check` expands to an unwrapped six-package Go test invocation, contrary
to this lane's test rule. It was invoked before inspecting that recipe; its passing result is
retained as a procedural deviation. The same six selections were rerun individually through
`script/heavy.sh`; their logs are `resume/cap-*.log` in the lane evidence directory.
