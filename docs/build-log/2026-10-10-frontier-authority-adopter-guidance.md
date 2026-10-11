# V1-1143 — Explain unavailable Frontier authority to the adopter

Date: 2026-10-10. Scope: documentation and regression evidence for GitHub #785; related V1-0869
and V1-0243. No production behavior, wire, decision or accepted intent changes.

## Finding and delivery boundary

The harness unconditionally starts every successful event's degradation list with
`frontier-authority-unavailable` (`internal/gokernel/harness.go:439@a4869098`), without consulting
`internal/frontier`. Stop returns a fixed unavailable/non-continuing result
(`internal/gokernel/harness.go:478-483@23c0a748`). The observations and adapter vocabularies admit
the code (`internal/observations/observations.go:40-47@493847da`,
`cmd/corvint/host_adapter.go:724-733@c1243f78`). CEM/OCM presence is not a diagnostic input.

`docs/AUTOMATION.md` now explains the benign absence-of-workflow case, the loss of an assessable
change/intent review queue, the minimum explicit artifact-authoring and read-only preview steps,
causes/remedies, and the difference between host notice presentation and suppression. The docs
index links directly to that section. CEM/OCM artifacts enable advisory Frontier composition,
not execution authority: linked test obligations remain open
(`docs/specs/change-frontier-v0.md:40-48@9a2ff71f`); accepted decision 0009 retains that boundary
(`docs/decisions/0009-harness-authority-boundary.md:32-36@5685d76e`).

This explains existing contracts under the spec-driven proportionality rule. No new numbered
requirement or Delivery-table row is needed, and the generated REQUIREMENTS.tsv is unchanged.
The origin/main lookup confirmed the existing LPCV-V0-070; this slice allocates no IDs and creates
no decision file. Rollback removes this guidance, its navigation link and the focused test.

## Evidence and limits

- Baseline: worktree HEAD `e49ba3ffa3a2debb8db80fb13503cd6dfe8edeca`, already containing V1-1140
  above the supplied origin/main `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`. Existing work preserved.
- Model routing: existing session retained for this bounded prose/test task; no panel needed.
  Account usage telemetry was not available from the exposed tools (`NOT_OBSERVED`).
- Corvint **used**: pre-change `query --task` with limit 1; it returned an unrelated flow-doc
  decision, with four results omitted and six test candidates withheld. The supplied original
  source paths and repository routes supplied the fallback context. `affected` ran against the
  supplied base and again against the task-start HEAD; non-Go/unowned-path unknowns remain open,
  and the broad inherited range is not this ticket's test scope. Raw local outputs are retained
  outside tracked source at `/private/tmp/claude-501/w5-docs-query.json` and
  `/private/tmp/claude-501/w5-docs-affected-{baseline,final}.json`.
- Snapshot refresh **unavailable**: the explicit `index --if-stale` attempt could not write the
  shared Git snapshot directory outside the sandbox; reads continued without a refreshed snapshot.
- Installed tools: Core `1.0.0-rc.3 (build 407)`; Tasks
  `0.0.0-tcp01-unverified+build.407`. Both official update checks with network permission failed
  `process-after-start-failed: descendant snapshot unavailable`; available release versions and
  freshness remain `UNKNOWN`. No update verification was bypassed or binary replaced.
- Baseline `go test -count=1 -timeout 30m ./internal/gokernel`: PASS. The new
  `TestFrontierUnavailableWithoutAdopterArtifacts` exercises all six successful event kinds in a
  repository without artifacts, checks successful FALLBACK receipts, string degradation, Stop
  abstention/non-blocking behavior and unchanged Git status. Indexed retrieval uses a successful
  test seam; this is not end-to-end host or artifact-authority qualification.
- Final verification: PASS for `go test -count=1 -timeout 30m ./internal/gokernel` and the focused
  `./cmd/corvint` tests `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice`,
  `TestClaudeAdapterFaultKeepsNotice`, `TestAHI019ClaudePostToolReceiptNamesEveryDegradation`, and
  `TestRunFrontierExitCodesSeparateValidStatesFromFailure`. PASS for all nine requested targets:
  `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, `use-case-receipts-test`.
  Separate `sh script/repin-use-case-receipts.sh --check`: PASS, all 18 receipts current.
  Commands used `GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`.
  The native `mktemp` default ignored TMPDIR and failed before document validation; a temporary
  PATH wrapper supplied `-p` for implicit templates only, preserving explicit templates. No gate
  logic was modified. Logs: `/private/tmp/claude-501/w5-docs-gates.log` (first three passing checks,
  then the resolved template-path failure) and `w5-docs-gates-remaining.log` in the same directory
  (remaining six passing checks). An affected-plan drift refusal during editing was rerun on the
  stable worktree and succeeded; its unknowns remain advisory limits.
- Independent native-agent review: PASS, no actionable findings; checked source claims, authority
  limits, CLI usage, navigation and all-six-event regression coverage. No runtime changes needed.
- Staging to the real worktree index is blocked by sandbox permissions on the shared Git directory.
  Documentation checks use a temporary index/object store under `/private/tmp/claude-501/w5-docs-git`
  populated from the original index plus exactly this task's files. This does not stage the real
  worktree. No production files, commits, pushes or ticket-store writes are made.
- Post-commit CEM/OCM binding, seal, frozen evaluations and repository-wide `make gate`/`./...`:
  `NOT_RUN`; this task is an uncommitted documentation handoff and explicitly excludes full gates.
  Optional mutation, learning, corpus generation and provider qualification are not applicable.

## Still unmet

A per-degradation machine-readable `reason` and an adopter acknowledge/suppress option are not
implemented. The existing Stop `frontier.reason` only repeats the generic code. Proposed follow-up
work: separately specify and test a diagnostic reason contract, and an opt-in acknowledgement or
presentation-suppression contract that retains uncertainty and machine evidence. These are proposals,
not filed tickets or delivered features. Ticket completion, integration and any follow-up filing
remain with the coordinator; the native store is untouched as requested.
