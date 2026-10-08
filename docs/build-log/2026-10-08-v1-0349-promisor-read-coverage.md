## 2026-10-08 V1-0349: partial-clone reads never fetch; regression now covers every Core and MCP read

### Intent

V1-0349 asked whether `corvint.flows.*` blob reads lazily fetch a missing promisor object, and
for every Core and MCP read to refuse such an object with a coded envelope and no fetch.

### Findings

The defect is already fixed on `origin/main` (`f33ea8ef`). Commit `5cd9419a` (2026-09-26, recorded
in `2026-09-26-rc1-git-runner-hardening.md`) added the empty `GIT_ALLOW_PROTOCOL` to
`gokernel.SanitizedGitEnvironment`, which the flows reads use, and added the regressions. Core
already coded the refusal as `repository-object-unavailable` under CCF-V1-004.

This lane reproduced the case again, on Git 2.54 with the built binary. The fixture was a source
with `uploadpack.allowFilter`, `--filter=blob:none` clones (one sparse, one full), an upload-pack
command that writes a sentinel, and the source removed afterwards.
- The sparse clone was read with `index`, `query`, `context`, `impact` and `prove`. The full clone
  was read with `impact --base` and `prove --base`. Each exited 2 with
  `repository-object-unavailable`, named the missing object and offered
  `git.fetch-promisor-objects`.
- `affected` exited 0 on both clones. It reads tree-level Git data (`diff --name-only --no-renames`)
  and the checked-out files, never a blob. On the sparse clone its plan is `scope: UNKNOWN`.
- No sentinel was written, and the `.git/objects` listings were byte-identical before and after.
- Over the MCP bridge, `query`, `impact` and `context` refuse with `repository-unavailable`.
  `cem.report` refuses with `cem-map-unavailable`, and the flows tools with `flows-refused`.
  `status` succeeds, because it needs no blob. None of them fetches.
- Two reads were outside the existing regressions. `corvint.flows.coverage` was added after
  `5cd9419a`, in `bbe7abd9`. The Core regression never ran `affected`.
- A mutation removed `GIT_ALLOW_PROTOCOL=` from `SanitizedGitEnvironment`, with the shim Git that
  drops `GIT_NO_LAZY_FETCH`. On this host,
  `TestAFUV1034CommittedFlowsReadNeverFetchesAPromisorObject`,
  `TestMCPReadsRefuseAMissingPromisorObjectWithoutFetching` and the new coverage test all fail
  (sentinel written). The earlier entry's note that the end-to-end tests miss the removed guard was
  load-dependent.

### Change

Tests and traceability only. No runtime, wire or code change:
- `TestMCPFlowsCoverageRefusesAMissingPromisorObjectWithoutFetching` (`internal/mcp/bridge`).
  `corvint.flows.coverage` over a sparse blob:none clone refuses with `flows-refused`, and over a
  full blob:none control clone it serves the page. Neither reaches the remote, with or without
  `GIT_NO_LAZY_FETCH`.
- `checkPromisorObjectRefusals` (`cmd/corvint`) also runs `affected --base` on both clones. It
  asserts a plan, and on the sparse clone `scope: UNKNOWN`. The shared sentinel check then shows
  no fetch, in both the guarded and the shim run.
- The MCPV0-016..019 and CCF-V1-004 traceability rows cite the added coverage.

A promisor-specific MCP code or `reasonClass` was not added. The closed `reasonClass` set
(MCPV0-028) needs an amendment to decision 0383, and the MCP refusals are already coded.

### Verification

- `go test -run 'TestIndexedCoreVerbsCodeAPromisorObjectWithoutFetching|TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway' ./cmd/corvint`: pass.
- `go test -run 'TestMCPReadsRefuseAMissingPromisorObjectWithoutFetching|TestMCPFlowsCoverageRefusesAMissingPromisorObjectWithoutFetching|TestFlowCoverageMCPParity' ./internal/mcp/bridge`: pass.
- `TestAFUV1034CommittedFlowsReadNeverFetchesAPromisorObject` (`internal/appflows`) and
  `TestSanitizedGitEnvironmentRefusesAPromisorFetch` (`internal/gokernel`): pass.
- Mutation (`GIT_ALLOW_PROTOCOL=` removed): the new coverage test fails on the sparse clone, and so
  do the appflows and MCP end-to-end regressions. The mutation was reverted.

### NOT_RUN

- `make gate` and the full `go test ./...`. Focused tests only, per lane policy.
- A Git older than 2.46. Its behaviour is modelled by the shim that unsets `GIT_NO_LAZY_FETCH`.
- The experimental CLI `flows` verbs over a partial clone. They share the appflows readers that
  the MCP flows tools exercise.
- `frontier`, `ocm` and `dogfood` over a partial clone, beyond the existing
  `repository-object-unavailable` tests for `cem` and `ocm`.

### Rollback

Revert the commit. It adds tests and edits traceability text only.
