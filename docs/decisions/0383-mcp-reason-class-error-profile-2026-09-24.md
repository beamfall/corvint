# Decision 0383 — MCP refusal reasons ship as an opt-in closed-class error profile

Date: 2026-09-24. Status: accepted. Authority: the repository owner delegated the choice on
2026-09-24 ("use experts to figure it out"). A wire-contract expert and an agent-surface disclosure
reviewer each assessed the options; this decision takes the wire expert's option and the reviewer's
constraints. It applies decisions 0103 and 0374 and supersedes neither.

Issue #170: `corvint.status` over MCP returns only `repository-unavailable` when isolated Git status
refuses a repository. The CLI names the reason (`EAF-V0-011`), but the MCP tool-error object is
closed: its code is "never underlying Git, repository, or process text" (`MCPV0`, tool-error
object). An agent without a terminal cannot learn why.

The decision:

- `corvint-mcp --root ROOT` output stays byte-identical. Profile `corvint-mcp-tool-error/0`,
  conformance profile `/0` and the vector that expects `repository-unavailable` are unchanged.
- One closed argv selector, `--error-profile reason-class`, following the `MCPV0-021` and `MCPV0-026`
  rules, makes every tool error use `corvint-mcp-tool-error/1`: the `/0` object plus a required
  `reasonClass`. `code` is unchanged.
- `reasonClass` is a closed enum: `git-filter`, `config-include`, `attributes-file`, `ref-storage`,
  `worktree-config`, `config-malformed`, `submodule`, `split-index`, `gitdir-pointer`,
  `metadata-unreadable`, `metadata-limit`, `metadata-directory`, `metadata-drift`, `scratch-dir`,
  `root-unresolved`, `unclassified`. A value is set only from a typed class attached where
  `internal/gitstatus` builds the refusal, never parsed from message text. Any error without a
  class, including every non-status failure, is `unclassified`. Clients MUST read an unknown value
  as `unclassified`. Adding a value needs an amendment to this decision.
- No free text crosses the boundary. The one repository-controlled part of a CLI reason, the filter
  driver name (at most 32 characters of `[a-z0-9_-]`), is enough to carry a short instruction to an
  agent, so it never appears in the MCP object.

Options set aside:

- Pass the reason text through: breaks the closed error object and opens the driver-name channel.
- New error codes, or `/1` for every client: an existing client matching `repository-unavailable`
  or pinning `/0` sees a changed object for a condition it already handles, the silent broadening
  decision 0374 rejected.
- No change with a "run `corvint status`" note: an agent with no terminal still learns nothing.

## Applied

Planned in the same change:

- `internal/gitstatus`: the refusal carries its class; `RefusalClass(err)` returns it. A table test
  maps every refusal site to a value other than `unclassified`.
- `internal/gokernel` (and `internal/contextindex` if it reaches the bridge): the kernel error keeps
  the class next to its code.
- `internal/mcp/bridge`, `cmd/corvint-mcp`: the selector and the `/1` object.
- `docs/specs/mcp-server-2026-07-28-v0.md`: `MCPV0-027` (selector) and `MCPV0-028` (closed
  `reasonClass`); non-goals state the default bytes are unchanged.
- `conformance/mcp-2026-07-28`: profile `/2` with its own cases file. It reuses the executable-config
  and worktree-redirect fixtures and asserts the exact `/1` object with and without the selector.
- `docs/MCP-SERVER.md` documents the selector. Shipped host examples may opt in once the profile is
  qualified.

## Rollback

Remove the selector, the `/1` object, the class plumbing and the `/2` manifest and tests. The default
output never changes, so no client, stored state or `/0` vector needs migration.
