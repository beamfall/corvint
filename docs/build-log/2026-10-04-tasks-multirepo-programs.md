# Multi-repository supervised programs (issue 354, partial)

Date: 2026-10-04
Owner intent: GitHub #354; native V1-0475.
Requirements: CAL-V0-071, CAL-V0-072 (new, S21) and an amendment to CAL-V0-062 (S13) in
`docs/specs/corvint-tasks-agent-leases-v0.md`.

## Owner scope decision

On 2026-10-04 the owner split #354. The issue closes on (a) multi-repository programs and (b) Codex
continuation across longer runs, using the configurable effort already delivered (CAL-V0-062/063).
Claude Code and OpenCode supervisor hosts move to separate native tickets, which the coordinator
files. They are out of scope here, and S13 records the split.

## Decisions

- **Declaration.** Extra checkouts are pinned in policy `supervision.repositories` by path SHA-256,
  1..8 per program. The local config only names a declared checkout and cannot add one. Names start
  with a lowercase letter, so they never collide with the composite's `.queue` entry.
- **Isolation.** Each repository gets a detached sibling worktree `<worktree>@<name>`. The operator
  checkout's `HEAD` and index are never moved. Only the implement invocation receives the siblings
  as Codex `sandbox_workspace_write.writable_roots`, so review stays read-only.
- **Binding.** A composite candidate tree carries the primary tree at `.queue` plus one `160000`
  gitlink per repository candidate commit, written into the queue repository with `git mktree`. The
  existing candidate, review-tree and transaction fields therefore bind every repository without a
  new wire field on the attempt. Review recomputes the composite and refuses a mismatch.
- **Scope.** Extra paths are checked as `@name/<path>` against the ticket touch paths. A primary
  path beginning with `@` becomes a question rather than an ambiguous tree.
- **Immutability.** Program transactions refuse a changed repository name, checkout or identity.
  Only reassignment moves a base, and it clears the candidate.
- **Fail closed.** No cross-repository landing or crash-recovery contract exists yet. Gates and
  integration of multi-repository programs therefore refuse, and do not approximate a landing.
- **CAL-V0-062 low fix.** An empty or unknown base `effort` was accepted when `stageEfforts`
  overrode every stage. It is now refused. A recorded program with an empty base effort is refused
  at its next stage launch, but `drain` and `cancel` still work.
- **ID allocation.** origin/main defines up to CAL-V0-068/S18, and CAL-V0-069/S19 is used by
  coordinated unlanded work (issue 494). This slice takes CAL-V0-071..072/S21 and leaves
  CAL-V0-069..070 and S19..S20 for that work. The coordinator should confirm the allocation.

## Evidence

- **Focused tests**, all passing:
  - `TestCALV0071_PolicyRepositories` (intent)
  - `TestCALV0071_ProgramRepositoryRecords` (snapshot): single-repository bytes are unchanged
  - `TestCALV0071_RepositoryBindingImmutable` (transaction)
  - In store:
    - `TestCALV0062_BaseEffortRequired`
    - `TestCALV0071_CheckProgramConfigRepositories`
    - `TestCALV0071_WritableRoots`
    - `TestCALV0071_UndeclaredRepositoryRefusedBeforeMutation`
    - `TestCALV0071_ExtraRepositoryPathsAreScoped`
    - `TestCALV0072_MultiRepositoryGatesFailClosed`
  - `TestCALV0071_MultiRepositoryProgramFakeHost`, end to end with a pinned fake Codex host and
    Core, the test binary as lane leader, and real Git repositories. It proves:
    - the writable root appears in the implement argv only
    - the extra candidate is parented on its base
    - the operator checkout `HEAD` does not move
    - the composite gitlink names the extra candidate
    - review reaches `READY_FOR_INTEGRATION` bound to the composite
    - integration refuses
- **Package runs:** the full `internal/tasks/...`, `cmd/corvint-tasks`, `internal/specindex` and
  `internal/taskman` packages, plus `go vet ./internal/tasks/...`. All pass except the
  `TestPSR*` pool-sweep tests in `internal/tasks/store`. Those fail identically at base `cd70ba0a`
  when `TMPDIR` is the symlinked `/var/folders/...` (`UNSUPPORTED_FILESYSTEM: env file:
  non-directory or symlink parent`), and they pass on this branch with `TMPDIR` resolved to
  `/private/var/folders/...`. The failure is environmental and predates this change.
- **Affected plan:** `corvint affected` selected 172 units with scope `UNKNOWN`. The doc edits reach
  most of them through documentation readers. Units beyond the packages above are `NOT_RUN`, per
  the owner's focused-test preference.
- **Live qualification:** live Codex qualification of a multi-repository program is `NOT_RUN`.
- **Untested:** cleanup of extra sibling worktrees (`CleanupWorktrees`) is implemented but has no
  direct test.

## Remaining under #354 / V1-0475

- Checkpointed automatic continuation across runs, beyond WAIT/resume.
- Multi-repository gate evaluation over the composite candidate.
- Integration into each designated checkout, with cross-repository crash recovery.
- Per-repository Core context packets. Context is currently primary-only.
- A direct test of extra-worktree cleanup.
- Live Codex qualification of CAL-V0-062/063 and CAL-V0-071/072.

## Rollback

- Removing policy `repositories` refuses every later stage of a multi-repository program.
  Single-repository policy, config and `programs.json` bytes are unchanged.
- A binary without this slice cannot decode a `programs.json` that records repositories. Drain or
  cancel those programs before a downgrade.
- Candidate refs under `refs/corvint/tasks/` and the sibling worktrees are ordinary Git state.
