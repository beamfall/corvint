# MCP multi-root profile V0

Owner: Russell Lewis
Date: 2026-10-07
Requirement prefix: `MMR-V0`
Intent status: accepted (decision 0441); MMR-V0-009 accepted (decision 0519; V1-1173); MMR-V0-010 and MMR-V0-011 experimental under the standing lane auto-accept (V1-3116); MMR-V0-012 to MMR-V0-017 experimental under the standing lane auto-accept (V1-3117)
Delivery status: experimental
Authoritative inputs: AGENTS.md invariants 2, 3, 4, 7 and 8; `mcp-server-2026-07-28-v0.md`
`MCPV0-001`, `MCPV0-015`, `MCPV0-039` to `MCPV0-041`, `MCPV0-114`; native tickets `V1-0938`, `V1-3116`, `V1-3117`.

## Agent digest
- Claim: One corvint-mcp process serves up to 32 operator-declared repositories, each named by an alias that every repository-scoped tool call must select.
- Status: accepted (decision 0441); MMR-V0-009 accepted (decision 0519; V1-1173); MMR-V0-010 and MMR-V0-011 experimental under the standing lane auto-accept (V1-3116); MMR-V0-012 to MMR-V0-017 experimental under the standing lane auto-accept (V1-3117) / experimental
- Exists: `internal/mcp/cli/roots.go`, `internal/mcp/cli/main.go` (`parseArguments`, `toolHandler.call`), `bridge.Registry.SharesRoot`; `internal/mcp/cli/worktree_notice.go`; per-root `--follow-ref ALIAS=REF` in `internal/mcp/cli/follow_ref.go`; `--tool-profile` tool groups, `--corpus-artifact` and `--workspace` in `internal/mcp/cli/groups.go` over the shared `internal/mcp/toolgroup`; tests in `internal/mcp/cli/roots_test.go`, `main_test.go`, `worktree_notice_test.go`, `follow_ref_roots_test.go` and `groups_test.go`.
- Blocked on: owner acceptance; no qualified host run; the dedicated `corvint-test-validity-mcp`, `corvint-docs-mcp` and `corvint-corpus-mcp` binaries stay single-root deprecated aliases for one release (`MMR-V0-016`).
- Read next: Requirements; Bounded state and measurements.

## Owner intent and scope

An agent working across several repositories needs one `corvint-mcp` child per repository today
(`MCPV0-001`), so a host with ten checkouts runs ten processes and lists ten copies of the tool
catalogue. This profile lets the operator declare several roots on one command line. Authority stays
with the operator: roots come only from argv, are validated and pinned at startup exactly as the
single root is, and no client capability or request field can add, remove or switch one
(`MCPV0-015` is unchanged). A single plain `--root ABSOLUTE_ROOT` remains the `MCPV0-001` server
byte for byte; the multi-root form is selected only by the aliased spelling below.

Roots are declared as `ALIAS=ABSOLUTE_ROOT` rather than derived from basenames, because linked
worktrees of one repository commonly share a basename and a derived name would change when a
directory is renamed. An explicit alias is stable, short in every schema, and never carries a path
into the tool catalogue.

## Requirements

- `MMR-V0-001`: (accepted by decision 0441; V1-0938) `corvint-mcp` MUST accept
  `--root ALIAS=ABSOLUTE_ROOT`, repeated, as the multi-root form. One aliased root already selects
  multi-root mode. A value is aliased only when the text before its first `=` is a valid alias
  (`MMR-V0-002`); otherwise the whole value is a plain root. A plain root MUST appear alone: exactly
  one plain `--root` is the unchanged `MCPV0-001` server, whose `tools/list` and tool results are
  byte-identical to the server before this profile, and a plain root combined with any other root is
  refused. An aliased declaration with an empty root is refused. The form composes with
  `--protocol-version`, `--tool-profile` and `--error-profile` exactly as the single-root form does.
- `MMR-V0-002`: (accepted by decision 0441; V1-0938) An alias MUST match
  `^[a-z][a-z0-9-]{0,31}$`, aliases MUST be unique, and at most 32 roots may be declared. Each root
  keeps the `MCPV0-001` bounds (absolute, clean, at most 4,096 UTF-8 bytes). A duplicate alias, more
  than 32 roots, or any argv shape outside `MMR-V0-001` MUST exit 2 with
  `corvint-mcp: invalid arguments` before any MCP byte is written.
- `MMR-V0-003`: (accepted by decision 0441; V1-0938) Every declared root MUST be validated
  and pinned at startup by the same `MCPV0-001` rules and code as the single root, under the selected
  tool profile, before the server reads stdin. Any root that fails exits 2 with
  `corvint-mcp: repository unavailable`. The same directory declared under two aliases, under any
  spelling (symlink, case-folded path), compared by the pinned canonical root identity, MUST exit 2
  with `corvint-mcp: invalid arguments`. Distinct linked worktrees of one repository are distinct
  roots.
- `MMR-V0-004`: (accepted by decision 0441; V1-0938) In multi-root mode `tools/list` MUST
  return the selected profile's tool set, each input schema equal to the single-root schema plus one
  property `repository` of `{"type":"string","enum":[ALIASES]}`, aliases in byte order. `repository`
  MUST be required for every tool except `corvint.status`, where it is optional. No root path appears
  in the catalogue. `cacheScope` and `ttlMs` are unchanged.
- `MMR-V0-005`: (accepted by decision 0441; V1-0938) A tool call MUST reach only the
  repository its `repository` alias names; the selector is removed before the bridge's closed
  argument decode, so every other argument is validated exactly as in single-root mode. A missing
  selector (except on `corvint.status`), a non-string selector, an undeclared alias, a case variant,
  or a path in its place MUST be refused as JSON-RPC `-32602 Invalid params`, the same mapping the
  bridge's `invalid-arguments` code already has. Client `roots` capabilities, `_meta` members and any
  `root` or `roots` argument MUST NOT add, remove or switch a repository; the latter two remain
  closed-decode refusals, including on `corvint.status` without a selector.
- `MMR-V0-006`: (accepted by decision 0441; V1-0938) `corvint.status` with no arguments
  MUST return every declared binding as the closed object
  `{"schema":"corvint-mcp-multi-root-status/0","tool":"corvint.status","mutates":false,"repositories":[...]}`,
  one `{"repository":ALIAS,"result":RESULT}` entry per alias in byte order. Each `RESULT` MUST equal
  the `structuredContent` a `corvint.status` call naming that alias returns at the same moment: the
  bridge result object, or the `corvint-mcp-tool-error/0` (or `/1`) object for that alias's failure.
  The text block is the canonical JSON inside the `MCPV0` untrusted-data envelope; `isError` is
  `false` because the listing itself succeeded. Cancellation is `-32603 Internal error`, as in
  single-root mode. Per-alias results carry no alias member; their closed `/0` shapes are unchanged
  and a client correlates them by request id.
- `MMR-V0-007`: (accepted by decision 0441; V1-0938) Every result MUST bind to its own
  root's startup pin and current revision, re-verified per call as `MCPV0-001` requires. No result,
  abstention or cached value may be shared or mixed across roots: the bridge registries are
  independent, and the process-wide snapshot caches key on absolute path, size, modification time
  and content digest. A commit in one root changes only that root's binding. Interleaved calls across
  roots in one process MUST each return their own root's revision.
- `MMR-V0-008`: (accepted by decision 0441; V1-0938) Per-root resident state MUST be only
  the startup identity pin (paths and `os.FileInfo` identities; no file descriptor, index, goroutine
  or timer). Index handles stay per call and are bounded by the server's 64-request concurrency
  ceiling; the only cross-call retention is the existing process-wide snapshot mapping cache of 4
  pack plus 4 sectioned entries, independent of the root count. An idle server performs no background
  work. The status-all listing probes roots sequentially.
- `MMR-V0-009`: (accepted, decision 0519 (V1-1173); GitHub #781) At startup,
  after every root is pinned, `corvint-mcp` MUST read its own launch directory once (`os.Getwd`) and
  find its enclosing worktree, the nearest ancestor holding a `.git` directory or gitfile, without
  running Git. A binding is labelled exactly when that worktree is a different directory from the
  bound root and both resolve to the same Git common directory, that is, the server was started from
  another linked worktree of the bound repository. Every `tools/call` routed to a labelled binding,
  including its tool-error result, MUST append a second `text` content block holding the canonical
  JSON object `{"code":"launch-worktree-differs","message":TEXT,"mutates":false,"schema":"corvint-mcp-worktree-notice/0"}`,
  whose fixed message names no path. The first block, `structuredContent`, `isError`, the bound root
  and routing are unchanged; status-all and protocol errors carry no notice. The launch directory is
  local process state, not request data: it never selects, adds or switches a root, and client roots,
  `_meta` and arguments cannot set or clear the label (`MCPV0-015` and the client-authority non-goal
  hold). An unreadable launch directory, no enclosing worktree, the bound worktree itself or a
  subdirectory of it, or an unrelated repository yields no label, so those results stay
  byte-identical. Under the label this is a bounded exception to the `MCPV0` single text block and
  to `MMR-V0-001` single-root byte identity. The launch directory is the only caller-side signal the
  stdio transport offers without client authority; a host that starts the server elsewhere than the
  agent's worktree gets no label. Rollback: remove the notice and its startup read.
- `MMR-V0-010`: (experimental under the standing lane auto-accept; V1-3116, GitHub #2010; extends
  `MCPV0-039` to `MCPV0-041` and `MCPV0-114`) In multi-root mode `corvint-mcp` MUST accept
  `--follow-ref ALIAS=REF`, repeatable at most once per alias, where ALIAS is a declared root alias
  and REF keeps the `MCPV0-039` bounds. A value is `ALIAS=REF` only when the text before its first
  `=` is a valid alias (`MMR-V0-002`); otherwise the whole value is a bare REF, so a ref whose name
  holds `=` is spelled in full (`refs/heads/NAME`). Each named root is bound at startup, in alias
  order, by the `MCPV0-039` startup contract and code (a detached linked worktree on which REF
  resolves), and from then on behaves as the single-root follow-ref binding on its own lock: every
  call routed to that alias, result or tool error, moves its dedicated worktree before the answer
  (`MCPV0-039`, `MCPV0-040`) and appends one `corvint-mcp-follow-ref/0` block (`MCPV0-041`) that
  also carries `"repository":ALIAS`. Roots no selector names serve their checkout exactly as before,
  with no block. `corvint.status` without a selector probes each following alias under its binding,
  so its entry describes the moved commit or carries the `follow-ref-missing` tool-error object, and
  appends one block per following alias in alias order after the listing's first block.
  `corvint.help` binds nothing (`MCPV0-100`). `tools/list` marks every tool but `corvint.help`
  `readOnlyHint` false and appends one fixed sentence naming the following aliases in byte order
  (`MCPV0-114`); input schemas are unchanged. The unaliased single-root `--root ROOT --follow-ref REF`
  server, its block and its `tools/list` are unchanged byte for byte. Falsifier: a call naming a
  following alias answered from a commit other than REF's on a clean worktree, a non-following root
  moved or given a block, a multi-root block without `repository`, or any change to the single-root
  bytes. Rollback: drop `bindFollowSelectors`' aliased branch, `openFollowRefs`, `followFor`'s alias
  lookup and the status-all binding; the bare single-root form is unaffected.
- `MMR-V0-011`: (experimental under the standing lane auto-accept; V1-3116, GitHub #2010; refines
  `MCPV0-039` and `MMR-V0-002` for the `--follow-ref` path) Every startup refusal of a
  `--follow-ref` argument MUST exit 2 before any MCP byte with
  `corvint-mcp: invalid arguments: REASON`, where REASON names the cause: a missing value, a
  malformed REF or empty `ALIAS=` value, a bare REF repeated, an alias repeated, the selector beside
  `--version`, a bare REF with aliased roots, `ALIAS=REF` with one plain root, or an undeclared alias
  (naming the selector and the alias). A following alias whose root is not a dedicated detached
  linked worktree, or whose REF does not resolve, exits 2 with
  `corvint-mcp: follow-ref-not-dedicated: repository ALIAS` or
  `corvint-mcp: follow-ref-missing: repository ALIAS`. The single-root refusals
  `corvint-mcp: follow-ref-not-dedicated` and `corvint-mcp: follow-ref-missing` stay byte-identical,
  and argv refused outside the `--follow-ref` path (for example a plain root beside another root, or
  a failing `--protocol-version`, `--tool-profile` or `--error-profile` selector) keeps the bare
  `corvint-mcp: invalid arguments`. Falsifier: a `--follow-ref` refusal printed without its reason.
  Rollback: print the bare text again; no wire or persisted state depends on the reason.

- `MMR-V0-012`: (experimental under the standing lane auto-accept; V1-3117, GitHub #2011; extends
  `MCPV0-026`) `--tool-profile` MUST accept, besides one code profile, a comma-separated list of at
  most one code profile (`task-review`, `task-review-lsp` or `flows`) and distinct tool groups from
  `test-validity`, `docs` and `corpus`, in any order. An empty, unknown, differently cased or
  repeated token, two code profiles, the selector given twice, or the selector beside `--version` is
  the bare `corvint-mcp: invalid arguments` refusal. The code profile's tools are always served; a
  group only adds its tools. Without a group, every `tools/list` and call byte is unchanged, so the
  default tool set `MCPV0-008` freezes and the four profile goldens stay byte-identical; the groups'
  `tools/list` has its own deliberately added golden. Falsifier: a value without a group that changes
  any byte, a group listed without being named, or a malformed list accepted. Rollback: restore the
  single-value check in `extractToolProfile` and drop `groups.go`.
- `MMR-V0-013`: (experimental under the standing lane auto-accept; V1-3117, GitHub #2011) The
  `test-validity` and `docs` groups MUST be served per declared root by the `internal/mcp/toolgroup`
  code the dedicated `corvint-test-validity-mcp` and `corvint-docs-mcp` binaries also answer
  through, with each group's own tool-error profile, so a call returns the same result through
  either server apart from server-identity `_meta`. With aliased roots their tools take the required
  `repository` alias enum (`MMR-V0-004`), are routed by alias (`MMR-V0-005`), and under a following
  alias are bound, marked and given the follow-ref block exactly as code tools (`MMR-V0-010`); the
  launch-worktree notice applies (`MMR-V0-009`). `--error-profile reason-class` changes only code
  tools; a follow-ref sync failure on a group tool is the `corvint-mcp` tool-error object, as for
  code tools. A root a group cannot open is `repository unavailable` at startup. Falsifier: a group
  result that differs from the dedicated server's for the same root and arguments, or a group call
  on a following alias answered outside its binding. Rollback: drop the per-root group branch of
  `answer` and `multiRootTools`.
- `MMR-V0-014`: (experimental under the standing lane auto-accept; V1-3117, GitHub #2011) The
  `corpus` group and `--corpus-artifact VALUE` MUST each require the other; VALUE is given once,
  never beside `--version`, and is `PATH` with one plain root or `ALIAS=PATH` naming a declared
  alias. PATH keeps the `corvint-corpus-mcp --artifact` rules (an absolute path below the root, else
  a canonical root-relative local input path). Each refusal exits 2 before any MCP byte with
  `corvint-mcp: invalid arguments: REASON`. An artifact that opens at startup lists exactly its
  declared tools and answers through the same `toolgroup.Corpus` code as `corvint-corpus-mcp`. One
  that cannot be opened is noted on stderr, never a startup failure: every corpus tool is listed and
  each call reopens it, answering the corpus tool-error `corpus-refused` until it opens, while every
  other tool keeps serving. An artifact whose bytes change after opening is `corpus-refused` by the
  registry. The corpus tools are workspace-level: they take no alias, never bind or mark a followed
  ref, and keep their own `repository` arguments. `--map`, `--consolidation`, `--legacy-cases`,
  `--outcome-log` and `--outcome-summary` stay only in `corvint-corpus-mcp`. Falsifier: a missing
  or stale artifact that stops another group, or a corpus call answered other than corpus-refused
  while the artifact is unavailable. Rollback: drop `extractCorpusArtifact`, `bindCorpusArtifact` and
  `toolgroup.CorpusArtifact`.
- `MMR-V0-015`: (experimental under the standing lane auto-accept; V1-3117, GitHub #2011)
  `--workspace FILE` MUST read one `corvint-workspace/0` declaration through `internal/workspace`
  and serve it exactly as `--root ALIAS=ROOT` per declared alias in alias order; it is given once and
  never beside `--root` or `--version`. A declared default revision is not followed: the server notes
  each on stderr, and `--follow-ref ALIAS=REF` follows a ref explicitly; `manifest_repo` is unused.
  A refusal or an unreadable declaration exits 2 with `corvint-mcp: invalid arguments: REASON`.
  Falsifier: a `--workspace` server whose `tools/list` differs from its `--root` expansion, or a
  declared revision followed implicitly. Rollback: drop `expandWorkspace`.
- `MMR-V0-016`: (experimental under the standing lane auto-accept; V1-3117, GitHub #2011) The
  dedicated `corvint-test-validity-mcp`, `corvint-docs-mcp` and `corvint-corpus-mcp` binaries MUST
  stay for one release with unchanged argv and results, as deprecated aliases of the unified server.
  `--help` or `-h` alone MUST print usage naming the unified equivalent on stdout and exit 0 on all
  four servers; beside another argument it stays the exit-2 refusal. Falsifier: a dedicated binary
  removed or changed within the release, or `--help` exiting non-zero. Rollback: drop the usage
  constants and the `--help` branch.
- `MMR-V0-017`: (experimental under the standing lane auto-accept; V1-3117 review repair) The
  corpus group MUST pin its root's identity at startup and answer `corpus-refused`, never open or
  serve an artifact, once that root is no longer the pinned directory: a reopen after a missing
  artifact checks the root before and after opening, and a registry already opened refuses the
  replaced root. An artifact removed after it opened MUST also answer `corpus-refused`, as a missing
  one does, never the registry's `corpus-input-unavailable`. Every corpus tool name MUST take the
  corpus path while the group is selected, including one the opened artifact does not declare, so
  no corpus tool name binds a followed ref. Falsifier: a corpus answer from a repository that
  replaced the startup root, an unavailable artifact answered with any other code, or a corpus tool
  name that moves a followed worktree. Rollback: drop the root pin and refusal mapping in
  `toolgroup.CorpusArtifact` and list only the declared tools in `corpusTools`.
## Bounded state and measurements

The ticket asked for lazy opening with at most K open roots closed in LRU order. The implementation
records K = 0 retained per-root index handles instead, because nothing per root is held open between
calls: each call already opens, builds and drops its own context index. What stays resident per root
is the startup identity pin, which is under 1 KiB and holds no descriptor. Closing a pin and reopening
it later would re-resolve the root and could silently accept a directory swapped while it was closed,
which `MCPV0-001` forbids, so the pins are eager and permanent. Memory is therefore bounded by
32 pins, 64 concurrent call working sets, and 8 retained snapshot mappings; descriptors are bounded
by the concurrent calls alone.

Measured on Darwin arm64 at the implementing commit, empty `.git` fixtures, one `tools/list` per
process (`wc -c` of the response line; maximum RSS from `/usr/bin/time -l`):

| Profile | Roots | One multi-root server: bytes / processes / max RSS | N single-root servers: bytes / processes / max RSS (sum) |
|---|---|---|---|
| default | 1 | 3,772 / 1 / 20.7 MB | 3,603 / 1 / 20.2 MB |
| default | 3 | 3,820 / 1 / 20.9 MB | 10,809 / 3 / 60.6 MB |
| default | 10 | 3,991 / 1 / 20.3 MB | 36,030 / 10 / 202.0 MB |
| task-review | 1 | 5,429 / 1 / 20.5 MB | 5,138 / 1 / 20.7 MB |
| task-review | 3 | 5,509 / 1 / 21.1 MB | 15,414 / 3 / 62.0 MB |
| task-review | 10 | 5,794 / 1 / 20.3 MB | 51,380 / 10 / 206.6 MB |

A status-all call over ten committed fixture repositories took 0.44 s wall time at 22.5 MB maximum
RSS, against 0.07 s and 21.2 MB for one single-root status call. These are single local
observations, not a benchmark gate.

## Non-goals and failure modes

Non-goals: client-provided roots, `roots/list`, or any other client-driven authority; adding or
removing roots while running; a different tool profile per root; cross-root query, impact or context
aggregation (only status lists every binding); Streamable HTTP or any listener; and changing any
`MCPV0` requirement, vector or single-root byte, except the notice block of `MMR-V0-009` and the
named `--follow-ref` argument refusals of `MMR-V0-011`; a different ref per call, or a ref chosen by
the client, is also a non-goal (`MMR-V0-010` refs come only from argv).
The dedicated `corvint-test-validity-mcp`, `corvint-docs-mcp` and `corvint-corpus-mcp` binaries
stay single-root; multi-root use of their tools goes through `corvint-mcp --tool-profile`
(`MMR-V0-012` to `MMR-V0-017`). Removing them after the one-release alias window needs its own
decision.

Failure modes:
- Argv outside `MMR-V0-001` or `MMR-V0-002`, or one directory under two aliases: exit 2,
  `invalid arguments`, nothing on stdout.
- Any declared root unavailable at startup: exit 2, `repository unavailable`; no partial server.
- Missing or unknown alias on a call: `-32602 Invalid params`; no repository is touched.
- A root swapped, retargeted or replaced after startup: that root's calls abstain with
  `ROOT_IDENTITY_CHANGED` (`MCPV0-001`); other roots are unaffected, and status-all reports the
  abstention in that root's entry.
- One root's status failure: that entry carries the tool-error object; the other entries are intact.
- A following alias (`MMR-V0-010`) whose worktree is dirty, attached, swapped or fails to check out
  or reindex is never modified and its block is flagged `stale` (`MCPV0-040`); a ref that stops
  resolving is that alias's `follow-ref-missing` tool error, and other roots are unaffected. Calls to
  different following aliases hold different locks, so they move their worktrees independently; the
  status-all listing takes each following alias's lock in turn, never two at once.
- Limits: the status-all listing is sequential, so its latency grows with the root count; the
  flows-impact serialization lock is process-wide, so flows-impact calls serialize across roots.
  Evicted snapshot mappings are dropped from the cache without being unmapped (pre-existing,
  independent of this profile), so a long-lived process that rotates through more than four snapshots
  keeps the older mappings' address space until exit; this is recorded as a suspected defect, not
  claimed fixed here.

## Linked worktrees (V1-1139, GitHub #781)

Programs that run one agent per linked worktree already have a supported path: declare each worktree
as its own alias (`MMR-V0-003` treats distinct linked worktrees as distinct roots) and have every
call name its alias. Each result is bound to that worktree's own commit (`MMR-V0-007`), and
`corvint.status` echoes `commitRevision`, `worktreeState` and `dirtyPathCount` per alias; this is
tested with real linked worktrees. A server started from one linked worktree but bound by a fixed
`--root` to another (the #781 case of a host configuration that hardcodes the root) labels each
routed result with the informational notice of `MMR-V0-009` (V1-1173), which changes no authority.
Not delivered, because it needs an owner decision against `MCPV0-015` and the non-goals above (no
client-driven authority): an optional per-call `root`, `snapshot` or `revision` argument validated
against repository identity with a typed refusal (V1-1172), and cwd discovery per call.

## Acceptance evidence and rollback

Acceptance evidence: the maintained tests in the traceability table pass; the existing
`internal/mcp/cli` golden `tools/list` files and the `conformance/mcp-2026-07-28` black-box suite pass
unchanged; the measurements above are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-mcp-multi-root.md`. The owner accepted this intent in decision 0441.

Acceptance evidence for `MMR-V0-010` and `MMR-V0-011`: the tests named in their rows pass, the
existing `MCPV0-039` to `MCPV0-041`, `MCPV0-100` and `MCPV0-114` follow-ref tests pass unchanged, and
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-29-mcp-per-root-follow-ref.md` records the contract and forks.

Acceptance evidence for `MMR-V0-012` to `MMR-V0-017`: the tests named in their rows pass, including
the parity test against the three dedicated binaries built from the same tree; the four existing
profile goldens pass unchanged and `testdata/tools-list-groups.golden.jsonl` is added deliberately;
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-29-v1-3117-unified-mcp-tool-groups.md` records the forks.

Rollback: revert the implementing commit. A plain single `--root` is unchanged by this profile, so
existing configurations are unaffected; a host using aliased roots returns to one server per root.
No persisted state, index format or wire field of the single-root profile changes.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `MMR-V0-001` | `internal/mcp/cli/main.go` `parseArguments`, `run`; `roots.go` `splitAlias` | `TestParseArgumentsIsClosed`, `TestMMRV0001SingleRootBytesUnchanged`, `TestAFUV1034FlowsToolProfile` (golden single-root bytes) |
| `MMR-V0-002` | `roots.go` `validAlias`, `maxRoots`, `maxAliasBytes`; `main.go` `parseArguments` | `TestParseArgumentsIsClosed`, `TestMMRV0003StartupRefusals` |
| `MMR-V0-003` | `roots.go` `openRepositories`; `bridge.Registry.SharesRoot` | `TestMMRV0003StartupRefusals` |
| `MMR-V0-004` | `roots.go` `multiRootTools`; `main.go` `Handle` | `TestMMRV0004RepositoryEnumOnEveryTool` |
| `MMR-V0-005` | `roots.go` `route`; `main.go` `call` | `TestMMRV0005AliasRoutingAndRefusal`, `TestMMRV0005ClientRootsNeverSwitchRepository` |
| `MMR-V0-006` | `roots.go` `statusAll`; `main.go` `toolErrorObject` | `TestMMRV0006StatusReportsEveryBinding` |
| `MMR-V0-007` | `bridge.Registry` per root; `contextindex` path-keyed caches | `TestMMRV0005AliasRoutingAndRefusal` (interleaved calls across three roots), `TestMMRV0006StatusReportsEveryBinding` (commit in one root), `TestMMRV0007LinkedWorktreesBindTheirOwnCommit` (linked worktrees, V1-1139) |
| `MMR-V0-008` | `bridge.Registry` pin-only state; `contextindex` `packCacheCapacity` | Measurements above; `TestMMRV0005AliasRoutingAndRefusal` (interleaved calls stay bound) |
| `MMR-V0-010` | `follow_ref.go` `extractFollowRef`, `bindFollowSelectors`, `openFollowRefs`, `followState.block`, `followRefRootsTools`; `main.go` `followFor`, `call`; `roots.go` `statusAll` | `TestMMRV0010PerRootFollowRefServesBothRoots` (#2010 repro), `TestMMRV0010FollowersAreIsolatedAndStatusAllHoldsEachLock` (per-alias locks), `TestMMRV0010FollowRefBoundsTheRefNotTheValue` (REF bound), `TestMMRV0010SingleRootBlockUnchanged` (single-root bytes) |
| `MMR-V0-011` | `internal/mcp/cli/follow_ref.go` `followReason*`, `extractFollowRef`, `bindFollowSelectors`, `openFollowRefs`; `main.go` `run` | `TestMMRV0011FollowRefRefusalsNameTheirReason` (every refusal's exact stderr, the unchanged single-root refusals and the bare text outside the path; `--help` names both forms) |
| `MMR-V0-009` | `internal/mcp/cli/worktree_notice.go` `launchWorktreeNotices`, `withWorktreeNotice`; `run`, `toolHandler.call` in `main.go` | `TestMMRV0009LaunchWorktreeNotice` (real linked worktrees: labelled only from another worktree, first block and structuredContent unchanged, client roots ignored, per-alias label, none on status-all), `TestMMRV0009NoticeOnToolError`, `TestMMRV0009NoticeOnRoutedToolError` (real routed envelope-collision tool error) |
| `MMR-V0-012` | `internal/mcp/cli/groups.go` `splitToolProfile`; `main.go` `extractToolProfile`, `run` | `TestMMRV0012ToolProfileList` (grammar and bare refusals; code tools kept beside groups), `TestMMRV0012GroupsToolsListGolden`, `TestAFUV1034FlowsToolProfile` (four profile goldens unchanged) |
| `MMR-V0-013` | `internal/mcp/toolgroup` test-validity and `Docs` groups, `answer`; `groups.go` `openGroups`, `groupDescriptors`, `answerGroup`; `roots.go` `multiRootTools` | `TestMMRV0013UnionAcrossFollowedRoots` (#2011 acceptance: union across two roots, e2e following a ref; a draft answered from e2e at the followed ref changes when the ref advances between calls), `TestMMRV0013ParityWithDedicatedServers` (same results as the dedicated binaries) |
| `MMR-V0-014` | `internal/mcp/toolgroup` `Corpus`, `CorpusArtifact`, `RootRelativePath`; `corpusbridge.AllTools`; `groups.go` `extractCorpusArtifact`, `bindCorpusArtifact` | `TestMMRV0014CorpusArtifactMissingAndStale`, `TestMMRV0014CorpusArtifactRefusals`, `TestMMRV0013ParityWithDedicatedServers` (corpus rows) |
| `MMR-V0-015` | `groups.go` `expandWorkspace`; `internal/workspace` `Load` | `TestMMRV0015WorkspaceFile` |
| `MMR-V0-016` | `usage` and `run` in `cmd/corvint-test-validity-mcp`, `cmd/corvint-docs-mcp`, `cmd/corvint-corpus-mcp` | `TestMMRV0016HelpPrintsUsage` (each dedicated package), `TestMMRV0011FollowRefRefusalsNameTheirReason` (corvint-mcp `--help`) |
| `MMR-V0-017` | `internal/mcp/toolgroup` `OpenCorpusArtifact`, `CorpusArtifact.Call`, `open`, `unavailable`; `groups.go` `openGroups` (`corpusbridge.AllTools`); `main.go` `call` | `TestMMRV0017CorpusRetryRefusesReplacedRoot`, `TestMMRV0017CorpusArtifactRemovedAfterOpen`, `TestMMRV0017UndeclaredCorpusToolNeverBindsFollowedRef` |
