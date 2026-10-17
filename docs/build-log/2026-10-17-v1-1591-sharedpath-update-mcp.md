# V1-1591, V1-1599, V1-1603: shared-path policy, update service restart, MCP follow-ref

Three operator capabilities ship in one change. They follow the owner's standing auto-accept of lane
requirements (2026-10-11), which this entry records for every new requirement ID below.

- V1-1591 (#1034): new spec `docs/specs/shared-path-change-policy-v0.md`, `SPC-V0-001..008`.
  - `internal/sharedpath` provides `corvint changes check-shared` and `corvint changes split-shared`.
  - Findings name the commit, the path and the missing element: `marker`, `listing`, `run-citation`,
    `separate-commit`, `clean-merge`.
  - A merge-tree conflict on a configured generated file is an exempt conflict, not a finding.
  - The splitter rewrites only the named, checked-out local branch, through a private index, with a
    compare-and-swap on `--expect-head`. It never pushes. It keeps the final tree, the author, the
    dates and the message, and prints the old head and a restore command.
  - It refuses a dirty tree, a detached HEAD, a merge in range, a configured session marker and
    Git's own in-progress state.
  - Its only manual follow-ups are `notes-listing` and `run-citation`.
  - No repository config ships yet (residual).
- V1-1599 (#1042): `UPD-V0-011..014` in `docs/specs/operator-update-v0.md`.
  - `apply --restart-service PROGRAM` drives a generic `status|stop|start|live-attempts` control
    program around the switch.
  - Mode is `RESTART` for Core. For Tasks it is `ADOPT` when `version` formats are equal and `DRAIN`
    otherwise, following CAL-V0-130/131.
  - `--post-switch` hooks run with bounded timeouts and retries. A failed hook is reported and the
    switch stands.
  - A service that does not start on the new build triggers the verified rollback and a restart on
    the old build.
  - Residuals:
    - The control contract is not bound to launchd, systemd or a Tasks `service` verb.
    - A live platform service run is NOT_RUN.
    - `rollback` does not drive the service.
- V1-1603 (#1046): `MCPV0-039..041` in `docs/specs/mcp-server-2026-07-28-v0.md`.
  - `corvint-mcp --follow-ref REF` runs only on a detached linked worktree.
  - When the ref moves, a clean tree is checked out and the index snapshot is refreshed in process
    before the answer.
  - A dirty tree or a checked-out branch is never modified and is flagged stale.
  - A missing ref returns the typed tool error `follow-ref-missing`.
  - Every response echoes a `corvint-mcp-follow-ref/0` block that carries the resolved commit.

Rollback is per capability, as each spec states. Neither the CLI nor the spec changes stored state.

## Review fix round

An independent Codex review of PR #1059 reported 11 findings. Each fix has a regression test.

- Shared-path (`SPC-V0-001`, `003`, `004`):
  - Config and notes files are opened without blocking and read only when they are regular files.
  - The merge probe writes into a temporary object directory that is removed afterwards, so it
    writes no object into the repository.
  - Git runs with `GIT_OPTIONAL_LOCKS=0`, so planning leaves the index bytes unchanged.
- Update service (`UPD-V0-011`, `013`, `014`):
  - `status` uses exit 0 for running and exit 3 for stopped; any other result refuses before a stop.
  - No rollback is attempted when the new build cannot be stopped, or in `DRAIN` mode.
  - `DRAIN` mode waits even when the service is already stopped.
  - The service tests fail on this host before they reach the new code. `ps` takes 2-5 s at load
    70-80, which exceeds procgroup's 1 s descendant snapshot bound. The unchanged `TestUPDV0010`
    fails the same way, so these tests are NOT_RUN locally and left to CI.
- Follow-ref (`MCPV0-039..041`):
  - The binding is pinned to the registry's canonical root and its identities (`ROOT_CHANGED`).
  - Git runs with the sanitized environment.
  - One lock covers both the move and the answer.
  - The dirty and attached checks run even when HEAD is current.
  - Checkout uses `--no-overwrite-ignore`.
