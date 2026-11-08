# RSP Go and shell integration repair

- Base: `fd95d9241` (integration repair base); branch `claude/rspfx-go`.
- Scope: RSP-V0-002, RSP-V0-003, RSP-V0-007, RSP-V0-035 and RSP-V0-036;
  Go selection adapter, selection mutation population, shell source guard and their
  existing Traceability rows. No qualification or production selection is enabled.

## Decisions and repairs

1. A Go module root is not necessarily the repository root. Resolve its nearest
   enclosing `.git` directory or regular worktree marker without launching Git or
   reading mutable source bytes. An absent, inaccessible or unsupported marker
   refuses with `go-repository-root-unresolved`. Bind the resolved boundary into
   the configuration digest. Inventory and loaded paths use that repository;
   coverage passes through the existing module-relative linecov adapter and then
   receives the module prefix before region assembly. Immutable Sources remain
   repository-relative. No configuration fields or evidence wire fields are added.
   This decision implements the charter's carry-prefix option. Git-exported trees
   without checkout metadata cannot establish the boundary and remain unsupported.
2. Ginkgo inventory stays absent and plans stay FULL with
   `go-ginkgo-inventory-unsupported` until a native inventory exists, per the
   orchestrator's decision in the repair charter.
3. Go skip explanations omit harness RUN/PAUSE/CONT/NAME/SKIP lines. A lifecycle-only
   transcript reaches the existing `go-skip-reason-missing` refusal.
4. Selection mutation rendering accounts for every failed body candidate by
   operator and source span, returning all such failures and no population. It
   does not reuse the legacy renderer loop that silently skips failed candidates.
   The existing legacy mutation path and its contracts are unchanged.
5. Shell source names containing `*`, `?` or `[` refuse with
   `shell-source-glob-unresolvable` before an inventory can be admitted.
6. The separate RSP-V0-036 Traceability row is merged into RSP-V0-033..037,
   preserving its test names and explicit lack of qualification.

## Blocked findings

- Item 6 / RSP-V0-036: `golang.Configuration` and `Capture` do not carry a plugin
  digest, selector version or verifier revision. `ExecutableDigest` identifies the
  Go executable, not the compiled-in plugin. `Assemble` receives producer metadata
  only after the inventory/configuration key has already been built. Adding those
  missing inputs is outside this charter's conditional authorization and frozen
  shape rule. Qualification remains absent; no identity is invented.
- Item 8 / RSP-V0-003: the frozen Bats TAP input cannot distinguish an unescaped
  directive-looking test-name suffix from a real skip. The upstream
  [Bats TAP formatter](https://github.com/bats-core/bats-core/blob/master/libexec/bats-core/bats-format-tap)
  emits names verbatim: passing name `first # skip why` and skipped name `first`
  with reason `why` both yield `ok 1 first # skip why`. A positional parser alone
  cannot recover the missing distinction. Locally, `platform/shell.go:parseBats`
  implements that interpretation; changing that file is also outside this lane.
  A bare `# skip` has no reported reason, and the existing
  `TestRSPV0003ShellTerminalRefusals` explicitly requires it to fail, consistent
  with RSP-V0-003. Accepting it would weaken a frozen assertion or invent a reason.
  No parser workaround is installed. Resolution needs an approved unambiguous
  reporter contract and a decision about reasonless skips, with corresponding
  permission to change the frozen assertion/platform parser.

## Acceptance evidence and limits

Regression tests written (execution NOT_RUN under the final build-only instruction):

- RSP-V0-002: `TestRSPV0002GoGinkgoInventoryUnsupported`,
  `TestRSPV0002ShellGlobSourceUnresolvable`.
- RSP-V0-003: `TestRSPV0003GoSkipRequiresNonLifecycleReason`.
- RSP-V0-007: `TestRSPV0007GoNestedModuleRepositoryPaths`,
  `TestRSPV0007GoUnknownRepositoryBoundaryRefusesEvidence`.
- RSP-V0-035: `TestRSPV0035GoUnparseableMutantsRefuseWholePopulation`
  uses deterministic renderer fault injection to retain multiple failures while
  refusing the entire population.

Only formatting, package vet and spec/line-citation checks are authorized in this
round. Package vet passed for all three touched packages; formatting and static spec/citation checks passed. The full spec-requirements-check target is NOT_RUN because its recipe also executes Go tests in internal/specindex and internal/lrfrepo. Exact command results are retained in the lane handback. Whole-package Go
execution, live qualification, independent review, CEM binding/checking/sealing,
exhaustive gates and promotion evidence remain NOT_RUN/NOT_PRODUCED for the host
orchestrator. The final build-only instruction supersedes the initial dogfood
change/bind instruction; the shared CEM and generated spec indexes are untouched.
Rollback is reverting this repair commit; all runners remain unqualified.
