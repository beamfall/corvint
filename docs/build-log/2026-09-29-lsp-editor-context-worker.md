# Experimental editor task-context projection and owned native worker

The owner asked to continue the Go/gopls agent and VS Code/Neovim direction. This slice uses the
accepted platform architecture at public base `5e28547ec751d1c70f222da847b0904864cbd8d0`; its
new LEC wire/profile spec is proposed, not accepted promotion authority. Source is isolated on
`codex/lsp-editor-context`. Astra/medium was selected for process ownership and evidence-binding
uncertainty. Model billing/cache savings are NOT_OBSERVED.

## Delivered candidate and limits

An explicit `corvint/context` request preserves the existing task-review bridge object, with
separate random-session/capture/version/digest overlay observation. Root, Git content and symbolic
HEAD observations bracket Core; exact overlay capture and cancellation are checked before reply.
A same-native-binary one-shot worker bounds the known noncooperative Core CPU tail. Its process-local
startup-only policy makes both Core Git helpers inherit the worker's owned group while ordinary
Core retains private groups. The policy is not a client/config toggle. The xcrun resolver has no
custom process attributes and inherits the worker group; no other executable launch is reached
by the selected task-review context operation. The worker never selects the LSP bridge.

The outer runner's 20-second operation timeout and 2-second configured shutdown budget do not
promise a strict 22-second end-to-end deadline: request setup, observation/drain and OS scheduling
remain additional phases. Results require successful joining, drain and known scoped cleanup;
unknown/partial cleanup refuses. Observed PID absence is not hostile escape containment. Root/branch
observations cannot detect unseen ABA or mutations after the final sample. Binary digest checks
are endpoint observations, not exec-fd protection against hostile replacement. The default
in-process Core cancellation issue remains open as V1-0485.

## Evidence retained for independent review

Raw local receipts and reproducible scratch drivers are under
`/tmp/lsp-editor-context-evidence/` (operator-local development evidence, not portable qualification):

- `context.json`, `affected-baseline.json`, `affected-final.json`: Corvint context/selection used.
  `dogfood-start.log` retains expected NOT_PRODUCED CEM/outcome reasons at change start;
  `enrollment.json` records keyed enrollment before edits and frozen selected checks.
- `baseline.log`: pre-change lspstdio, command and MCP bridge tests passed.
- `native-proof.log`: production NewTaskReview/ToolContext worker READY, cold timed cancellation,
  drained/joined group cleanup and healthy retry on tiny and 3,000-file tracked fixtures. Tiny
  normal one-shot wall observation was 402 ms; it includes observer/IPC work and is neither an
  isolated overhead measurement nor a latency distribution or promotion floor.
- `cpu-proof.log`, `cpu-witness`, `cpu-overlay.json`: scratch-only stack observation witnessed real
  lexicalRows before cancellation. The native worker was cancelled, joined and drained; observed
  descendants were absent. Sampling changes scheduling; production code contains no sampler.
- `groups.log`: isolated subprocess tests invoke real Git under both actual configure helpers,
  confirm private groups by default and the worker group when enabled, then cancel and join.
  `actual-git-proof.log`, `actual-git-groups.jsonl`, `git-group-overlay.json`: scratch observation
  of 42 actual production contextindex Git launches, all with the worker PGID and no lookup error;
  the real runner's cancellation/retry/cleanup proof passed. The helper tests additionally cover
  gokernel's inherited mode. The selected Core context fixture used contextindex's launcher.
- `editor-proof-final.json`: native stdio/gopls READY → CANCELLED → healthy READY transcript and
  editor EOF during an observed live native context worker. Every previously observed PID was
  absent afterward; reader joined. EOF-to-exit timing is recorded as a diagnostic, not a bound. Worker stdin
  EOF after its one JSON message remains normal. This is a protocol harness, not a real editor.
- `focused-final.log`: all selected gitstatus, contextindex, gokernel, lspstdio and command units
  passed. `adapter-final.log`, `adapter-race-final.log`, `fixture-final.log` cover subsequent
  adapter/test refinements; `race-final.log` covers both Core process helpers. `vet-final.log` and
  `adapter-vet-final.log` passed. `runner-tests.log` passes reused runner timeout/cancel/overflow,
  descendant retirement and interruption tests. `docs-final.log` and `specindex.log` passed.
- `candidate-bindings.json`: native binary SHA-256, relevant source digests, Go 1.27.1 darwin/arm64
  and gopls v0.22.0. The portable native-worker test compares exact bridge Object bytes on frozen
  fixture inputs, resolves subject blob/commit/tree against Git, and checks every non-Git fixture
  file's bytes and status remain unchanged, including absent new private state.

Retained losing observations: `binding-proof.log` exposed the CLEAN/MIXED-versus-lowercase probe
state mapping, repaired without changing Core bytes. The external Python Git sampling probe first
missed short-lived processes, then received OS PermissionError for its group signal; its failed
`git-membership.err` remains retained and does not count as cleanup evidence. The established Go
runner plus scratch launch observation supplied the successful owned-group proof. An initial
fixture put its owning spec outside the established docs/specs route and a test incorrectly
expected the runner's internal timer flag for parent deadline; those test assumptions were repaired.
Initial trace checking refused unstaged new test names; it passed after staging the witnesses.
The final documentation check also detected old repository.go line citations displaced by the
inline Wait branch (`docs-line-drift-failure.log`). Extracting only that conditional Wait at the
file end preserved those source locations; `final-repair-tests.log`, `final-repair-vet.log` and the
repeated native transcript verify that narrow repair.

## Remaining boundary and rollback

Independent source review, final post-commit CEM/OCM/check/report/seal, root-owned actual-client
probes and publication remain coordinator-owned. No final frozen gate ran during source edits.
The repository-wide gate is NOT_RUN under the owner's scoped-work preference. Frozen retrieval/
learning evaluations are not applicable: Core ranking, task-context semantics and learning inputs
are unchanged. Shared feature routes for test selection and change evidence were used; provider,
mutation, learning, console and unrelated tool families were not applicable. This is no general
read-only, whole-tree containment, client qualification or world-class outcome claim.

Rollback disables the optional companion or removes this request/marker and internal worker
profile. It requires no persistent migration and leaves default Core and the existing definition
experiment available. The ticket remains open until the coordinator retains its required
integration, acceptance evidence and native completion write.
