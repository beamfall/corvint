## 2026-10-09 V1-1039: MCP cem.report with a valid map over a partial clone refuses without fetching

### Intent

V1-0349 covered `corvint.cem.report` over a partial clone only on its refusal path: the sparse
clone had no map checked out, so the call stopped at `cem-map-unavailable` before any blob read.
V1-1039 asks for the path where the map is present and a blob the report cites is missing.

### Finding

No defect. Over a `--filter=blob:none` clone, the map is in the worktree and every HEAD blob is
local. Only the base blob of the changed file stays on the promisor remote. `cem.report` gets
past the map read to the canonical diff and refuses. The bridge code is `repository-unavailable`,
which MCPV0-025 specifies for Git read failures. The underlying CEM code is
`repository-object-unavailable` (CEM-CB-019), so the refusal comes from the missing object, not
from the map or another Git failure. No fetch reaches the remote, including through a Git shim
that unsets `GIT_NO_LAZY_FETCH`. Repository bytes, `.git` included, are unchanged. No spec or code
change was needed.

### Change

- `internal/mcp/bridge/flows_test.go`:
  `TestMCPCEMReportRefusesAMissingCitedBlobWithAValidMapWithoutFetching`. It reuses the
  `cemRepository` fixture and the V1-0349 sentinel upload-pack, plus the removed source and the
  shim Git. Two clones get the same map:
  - The partial clone must refuse.
  - An unfiltered clone is the serving control and must return `READY`.

  A final lazy `cat-file` with the file transport allowed leaves the sentinel. This shows the
  sentinel would record a fetch.
- `docs/specs/mcp-server-2026-07-28-v0.md`: the test is traced in the `MCPV0-016..019` and
  `MCPV0-024`/`MCPV0-025` rows. No requirement text changed.

### Verification

- The new test passes on base `b2a0b8c6`. It adds coverage and fixes no defect, so there is no
  fails-on-base run.
- Mutation: removing `GIT_ALLOW_PROTOCOL=` from `internal/cem/gitauth/object.go` `scrubbedEnv`
  makes the test fail, because with the shim the partial-clone call reaches the promisor remote
  (sentinel written). The mutation was reverted.

### NOT_RUN

- `make gate` and the full `go test ./...`. Only the focused package was run, per lane policy.
- A real Git older than 2.46. The shim models it.
- The compiled `corvint-mcp` process over a partial clone. The test drives the in-process registry
  that the server dispatches to.

### Rollback

Revert the commit. It only adds a test and trace text.
