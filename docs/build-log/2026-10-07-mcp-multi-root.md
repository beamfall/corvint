# 2026-10-07: one corvint-mcp process for several declared repositories

## Intent

Ticket V1-0938 asks for one `corvint-mcp` process to serve several operator-declared repositories,
so a host with N checkouts no longer runs N servers and lists N copies of the tool catalogue. The
proposed profile is `docs/specs/mcp-multi-root-v0.md` (`MMR-V0-001..008`), pending owner acceptance.

## Decisions

- **Operator-declared aliases.** Roots are `--root ALIAS=ABSOLUTE_ROOT`, repeated up to 32 times,
  aliases `^[a-z][a-z0-9-]{0,31}$`. Basenames were rejected as names because linked worktrees share
  them and they change on rename. A single plain `--root` stays the `MCPV0-001` server byte for byte,
  so no existing host configuration changes.
- **Same validation, same refusals.** Each root goes through the unchanged bridge constructor under
  the selected tool profile. A directory declared twice is detected through the pinned canonical root
  identity (`bridge.Registry.SharesRoot`, `os.SameFile`), which covers symlink and case-folded
  spellings. Startup refusals reuse the existing `invalid arguments` and `repository unavailable`
  stderr lines and exit 2.
- **Selector, not new codes.** Every tool schema gains a `repository` enum of the aliases, required
  except on `corvint.status`. The handler removes it before the bridge's closed decode. A missing or
  unknown alias maps to `-32602 Invalid params`, as the bridge's existing `invalid-arguments` does,
  so no new error code is introduced. A `root` or `roots` argument stays a closed-decode refusal, and
  client `roots` capabilities are ignored, as `MCPV0-015` already requires.
- **Status lists every binding.** `corvint.status` without an alias returns
  `corvint-mcp-multi-root-status/0`, with one entry per alias equal to that alias's own status
  result. The per-alias result and tool-error objects stay their closed `/0` shapes.
- **No LRU of open roots.** The ticket proposed lazy opening with K roots open and LRU close. The
  only per-root resident state is the startup identity pin, under 1 KiB with no descriptor. Indexes
  are already built per call and dropped. Closing a pin and reopening it would re-resolve the root and
  could accept a swapped directory, which `MCPV0-001` forbids. So K = 0 retained per-root handles,
  and the process-wide snapshot mapping cache stays at 4 pack plus 4 sectioned entries whatever the
  root count. An idle server does no background work.
- **Scope.** Only `corvint-mcp` changes. The test-validity, docs and corpus servers keep their own
  parsers and registries; reusing the alias, schema and routing helpers there is a recorded follow-up
  non-goal.

## Measurements

Darwin arm64, empty `.git` fixtures, one `tools/list` per process; bytes are the response line,
RSS is `/usr/bin/time -l` maximum resident set size.

| Profile | Roots | One multi-root server: bytes / processes / RSS | N single-root servers: bytes / processes / RSS sum |
|---|---|---|---|
| default | 1 | 3,772 / 1 / 20.7 MB | 3,603 / 1 / 20.2 MB |
| default | 3 | 3,820 / 1 / 20.9 MB | 10,809 / 3 / 60.6 MB |
| default | 10 | 3,991 / 1 / 20.3 MB | 36,030 / 10 / 202.0 MB |
| task-review | 1 | 5,429 / 1 / 20.5 MB | 5,138 / 1 / 20.7 MB |
| task-review | 3 | 5,509 / 1 / 21.1 MB | 15,414 / 3 / 62.0 MB |
| task-review | 10 | 5,794 / 1 / 20.3 MB | 51,380 / 10 / 206.6 MB |

Status-all over ten committed repositories: 0.44 s wall, 22.5 MB RSS, all ten entries `READY`. One
single-root status: 0.07 s, 21.2 MB. Single local observations, not a gate.

## Evidence

- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./cmd/corvint-mcp ./internal/mcp/...
  ./conformance/mcp-2026-07-28/...`: all eight packages ok, including the unchanged single-root
  goldens and the black-box conformance suite.
- `go vet` of the same packages with `GOOS` darwin, linux and windows: clean. `gofmt -l`: empty.
- Doc gates `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, `diagnostic-coverage-check` and
  `go test ./internal/specindex`: all rc=0.
- `corvint affected --base 79536edd8df3a76d1ffb0a420e3038f0a18ed1a2` (Corvint 1.0.0-rc.2 build 360)
  selected 80 units over the dirty paths. Only the MCP packages above ran; the other selected units
  are NOT_RUN. The only non-MCP-package code change is the additive, otherwise unused
  `bridge.Registry.SharesRoot` method.

## Limits

- Owner acceptance of the proposed intent and a qualified host run are pending.
- The status-all listing probes roots sequentially; the flows-impact lock is process-wide.
- Suspected pre-existing defect, not changed here: `contextindex.retainPack` and `retainSectioned`
  drop an evicted mapping from the cache without unmapping it, so a long-lived process rotating
  through more than four snapshots keeps the older mappings' address space. Multi-root rotates faster
  but does not cause it.
