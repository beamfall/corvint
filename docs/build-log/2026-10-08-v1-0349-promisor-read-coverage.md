## 2026-10-08 V1-0349: partial-clone reads never fetch, and CEM codes a missing promised blob

### Intent

V1-0349 asked whether `corvint.flows.*` blob reads lazily fetch a missing promisor object. It also
asked that every Core and MCP read refuse such an object with a coded envelope and no fetch.

### Findings

The flows suspicion is refuted on `origin/main` (`f33ea8ef`). Commit `5cd9419a` (2026-09-26,
`2026-09-26-rc1-git-runner-hardening.md`) added the empty `GIT_ALLOW_PROTOCOL` to
`gokernel.SanitizedGitEnvironment`, which the flows reads use. Core already codes the refusal as
`repository-object-unavailable` under CCF-V1-004.

The fixture: a source with `uploadpack.allowFilter`, `--filter=blob:none` clones (sparse and
full), and an upload-pack command that writes a sentinel. A shim Git that unsets
`GIT_NO_LAZY_FETCH` models Git before 2.46. All runs were on Git 2.54.
- Indexed Core reads (`index`, `query`, `context`, `impact`, `prove`) exit 2 with
  `repository-object-unavailable` and do not fetch. `affected` reads no blob and plans; on the
  sparse clone the scope is `UNKNOWN`.
- Over MCP, `query`, `impact` and `context` refuse with `repository-unavailable`. `cem.report`
  refuses with `cem-map-unavailable`, and the flows tools with `flows-refused`. None of them fetches.
- **Confirmed defect 1 (fetch).** `cem`, `ocm` and `frontier` read through `gitauth.Open`. Its
  environment set `GIT_NO_LAZY_FETCH=1` but added the empty `GIT_ALLOW_PROTOCOL` only for a pinned
  Git binary. With the shim, a gitauth blob read fetched from the promisor remote (sentinel
  written).
- **Confirmed defect 2 (code).** With the guard in place, `cem status` and `cem verify` over a
  blob:none clone refused with `git-diff-failed`, carrying raw Git stderr. CEM-CB-019 requires
  `repository-object-unavailable` for a missing promised object. The diff exits non-zero, and
  `canonicalDiffError` mapped every Git exit to `git-diff-failed`.

### Change

- `internal/cem/gitauth/object.go`: `scrubbedEnv` always sets `GIT_ALLOW_PROTOCOL=`, so no
  gitauth read can use any transport. The pinned-binary-only append is removed as redundant.
  gitauth runs no fetch, push, clone or other transport command.
- `internal/cem/gitauth/diff.go`: on a Git exit from the canonical diff, `diffExitError` walks the
  verified change set and its blobs, without fetching. If an object is missing, it returns that
  `repository-object-unavailable` refusal; otherwise it keeps `git-diff-failed`. The batch read
  exits under a Git that ignores the guard, so the blobs are then read one by one, where an exit
  names the object. The success path is unchanged.
- Tests:
  - `TestPromisorObjectNeverFetchedByAGitThatDropsTheLazyFetchGuard` (`internal/cem/gitauth`)
    covers `CanonicalDiff`, `CanonicalDiffWithCreateDestinations` and `BlobBytes`, guarded and
    with the shim.
  - `TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` (`cmd/corvint`) covers `cem status` and
    `cem verify`.
  - `TestMCPFlowsCoverageRefusesAMissingPromisorObjectWithoutFetching` (`internal/mcp/bridge`)
    asserts `flows-refused` on the sparse clone, and `READY` with no fetch on the full clone.
  - `checkPromisorObjectRefusals` also runs `affected`.
- The pinned-binary test now also expects the ordinary environment to carry `GIT_ALLOW_PROTOCOL=`.
- Trace rows: CEM-CB-017..020, CCF-V1-004 and MCPV0-016..019. There is no new requirement or code:
  CEM-CB-019 already requires `repository-object-unavailable`.

No promisor-specific MCP `reasonClass` was added. The closed set (MCPV0-028) would need an
amendment to decision 0383.

### Verification

- The gitauth and CLI promisor tests failed before the fixes and pass after them.
- Mutation: removing `GIT_ALLOW_PROTOCOL=` from `scrubbedEnv` makes both tests fail with the
  sentinel written. Removing `diffExitError` makes the diff assertions fail with `git-diff-failed`.
- Focused package and `-run` results are in the lane report.

### NOT_RUN

- `make gate` and the full `go test ./...`. Focused tests only, per lane policy.
- A real Git older than 2.46. The shim models it.
- `ocm`, `frontier`, `dogfood` and the experimental CLI `flows` verbs end to end over a partial
  clone. They share the gitauth or appflows readers covered above.
- MCP `cem.report` with a valid map over a partial clone.

### Rollback

Revert the commit. Without it, gitauth reads can lazily fetch on Git before 2.46, and missing
promised diff inputs revert to `git-diff-failed`. There is no data or wire migration.
