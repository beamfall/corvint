# Foreground Codex programs

Enable the optional `taskman-codex-supervisor/0` runtime in the native policy and its
`supervision` object before dispatch. The runtime pins executable path/content hashes and mode,
existing BUILDER/REPAIR/REVIEWER/VERIFIER roles, and worker capacity. `supervision` contains
`profile`, `contextRequired: true`, `maxRepairCycles` (0..2), and `program` caps for `turns`,
`wallClockMinutes`, `inputTokens` and `outputTokens` (canonical decimal strings). Optional
`efforts` maps `implement`, `review` and/or `integrate` to sorted arrays of `low`, `medium` and
`high`; a stage without an entry admits only `low`. Optional `stageWallMinutes` (1..240, default
60) bounds the config `wallSeconds`; larger values are refused (`LIMIT_EXCEEDED`), because a stage
never outlasts the lane `wallClockMinutes` cap, itself at most 240.

A local JSON config contains `profile`, absolute resolved `executable`, `executableSha256`,
explicit `model`, `effort` (default for every stage), optional `stageEfforts` (per-stage
override), `prompt` (reserved; generated stage prompts govern dispatch), absolute `workRoot`,
numeric `wallSeconds` (1..`stageWallMinutes`×60), `coreExecutable`, `coreSha256`, and
`ownIntegrationCheckout` (false unless the operator explicitly designates this clean checkout).
Optional `pool` selects the implementation pool. The policy remains authoritative. Core context
must be READY/fresh at the stage's exact Git tree; stated uncertainty is retained in prompts.
Every stage effort must be admitted by the policy: a new program is refused before its first
record, and an existing one is re-checked before each stage launch, so a narrowed policy never
blocks `drain`. A stage still ends at the minimum of `wallSeconds`, the lane `wallClockMinutes`
and the program's remaining `wallClockMinutes`; expiry returns a resumable WAIT handoff.
The config `effort` must itself be `low`, `medium` or `high` even when `stageEfforts` overrides
every stage.

Multi-repository programs: declare each extra checkout in the policy as
`supervision.repositories` `{"<name>": {"pathSha256": "<SHA-256 of the absolute path>"}}` (1..8
names, lowercase letter first, then `[a-z0-9-]`, at most 32 bytes) and list it in the config as
`"repositories": [{"name": "<name>", "checkout": "<absolute path>", "integrationBranch": "<branch>"}]`,
sorted by name; `integrationBranch` is optional. The program records each checkout's common Git
identity and current `HEAD` as its base; implement runs in the detached sibling worktree
`<worktree>@<name>`, which is the only extra writable root. Ticket touch paths address extra
repositories as `@<name>/...`. Each changed repository gets a candidate commit under its
`refs/corvint/tasks/`, and the program candidate is a composite tree (`.queue` plus one gitlink per
repository) that review binds. Every stage also needs a READY, fresh Core packet for each extra
repository, queried in its sibling worktree; without one the stage is refused
`repository <name>: CONTEXT_UNAVAILABLE` before the host starts, and the prompt carries the packets
as `repositoryContext`. Required gates run in the primary stage worktree with each sibling at
`<worktree>@<name>`, observe the composite tree, and refuse `DIRTY_WORKTREE` when any of those
worktrees has uncommitted or untracked changes.

Integration of a changed extra repository needs a designation: set `ownIntegrationCheckout` and
that repository's `integrationBranch` in the config before the program starts, and keep that
checkout clean, on that branch and at its recorded base. A changed repository without a designation
is refused `UNSUPPORTED` before any grant; an unchanged one is never moved. The integrator lands
every changed repository by fast-forward, in name order, before the queue checkout, and a restart
after an interruption skips any repository already at its candidate, so no candidate lands twice.
The store lock does not stop other Git writers: if one advances a later checkout after an earlier
repository landed, integration stops `TARGET_ADVANCED`, keeps that landing and waits until the
advanced checkout is back at its base or candidate. See
[S21](specs/corvint-tasks-agent-leases-v0.md#s21--multi-repository-supervised-programs-issue-354).

Claude Code host: set `"host": "claude-code"` in both the policy `supervision` object and the
config, and pin the Claude Code executable as the same `taskman-codex-supervisor/0` runtime (one
host per policy; absent means Codex). A config host that differs from the policy host, or an
unknown host, is refused `UNSUPPORTED` before any record; `--host` must match the config host. A
missing, unreadable, unpinned, non-executable or symlinked executable of either host is refused
`CAPABILITY_UNAVAILABLE` before any record or claim; pin the symlink's regular target instead (for
example the `claude.exe` that `/opt/homebrew/bin/claude` points to). An existing program reopened
after the policy host changes never claims or launches again, and keeps only drain and cancel on a
live attempt. To switch hosts back, cancel each `claude-code` program with its original config
while its pin is still in force, then edit the policy; a drained program must still be cancelled,
because a drain leaves its claim held. The lane leader runs the bytes it verified: a root-owned,
root-protected path with no ACL runs in place, and any other runtime runs from a private copy in the stage's
effect directory, so pin a self-contained binary such as `claude.exe`, not a script wrapper.
Stages run `claude -p --output-format json` with the stage effort, project settings only, no MCP
servers and no permission prompts; implement accepts edits, review and integrate deny Edit, Write
and NotebookEdit, and every stage adds the sibling worktrees with `--add-dir`. The single result
object must succeed and carry the handoff object as its `result`, with no repeated member and no
member that differs from a read one only by case; usage is observed from its
integer token counters or stays NOT_OBSERVED. Bash is governed by project permission rules, not
contained. Live Claude Code qualification is NOT_RUN; see
[S22](specs/corvint-tasks-agent-leases-v0.md#s22--claude-code-supervised-host-v1-0755-split-from-issue-354).

OpenCode host: set `"host": "opencode"` in the policy `supervision` object and the config, pin the
OpenCode executable the same way, and give `model` as one `provider/model` without a `#variant`
(each stage appends its effort as the variant). Multi-repository OpenCode programs are refused
`UNSUPPORTED`. Stages run `opencode run --standalone --format json`, resuming with
`--session S --fork`. OpenCode starts its session server and tool processes in process groups of
their own; the supervisor observes those escapes while the host runs, drains them at stop, and
reports a clean stop only when the server-held standard error (`OPENCODE_PRINT_LOGS=1`, error
level) reaches end of file. Escape groups are trusted only through a live recorded leader identity,
and recovery after a lost supervisor never proves a detached host quiet. The run never
passes `--auto`; an inline `OPENCODE_CONFIG_CONTENT` permission set denies sub-agents and directories
outside the worktree in every stage and edits in review and integrate, project configuration is
disabled and self-update is off. The last text part inside a step that finished with reason `stop`
must decode to the handoff; any host `error` event or a changed session is refused, and a resumed
stage advances only from a new forked session, otherwise it stays WAITING. Usage sums every
finished step's disjoint counters only when accounting is complete (no open step, a final `stop`,
output under the cap), or stays NOT_OBSERVED. Plugins loaded from user configuration still run
with host privileges and are not contained (a known limit). Live OpenCode qualification is NOT_RUN; see
[S23](specs/corvint-tasks-agent-leases-v0.md#s23--opencode-supervised-host-v1-0756-split-from-issue-354).

```sh
corvint-tasks run --program migration --config supervisor.json --role implementer --count 3 --host codex
corvint-tasks run --program migration --config supervisor.json --role reviewer --count 2 --host codex
corvint-tasks pending
corvint-tasks program show
corvint-tasks answer --program migration --config supervisor.json --question QUESTION_SHA --revision 1 --answer 'Approved clarification'
corvint-tasks resume --program migration --config supervisor.json --role implementer
corvint-tasks drain --program migration --config supervisor.json
```

Roles select eligible native attempts; an idle role exits without creating implementation work.
The foreground batch is bounded, keeps shared program counters across ticket reassignment, and
returns retained handoffs when no work is eligible or a cap stops dispatch. Required ticket roles
are an optional acceptance-relevant map with implement/review/integrate arrays. Independent review
requires both a different holder and different Codex session, plus every acceptance claim.

Before integration, use the existing `ticket grant-approval` command with operation INTEGRATE,
current acceptance revision and exact scope `taskman-integration:` followed by SHA256 of
`baseCommit + NUL + candidateTree + NUL + intentBranch`. A multi-repository program appends, for
each repository in name order, `NUL + name + NUL + integrationBranch` (empty when undesignated)
before hashing, so one grant names every landing. Then run the integrator role with
`--grant GRANT_ID`. Each designated checkout must still be clean at its original base; a later tip
requires a fresh candidate/review/gates/grant. No remote push is performed. Native completion and
receipt audit remain the delivery boundary.

`drain` retains WAIT and scope; `cancel` releases scope only after proved shutdown. `retry` is an
explicit operator continuation of a retained handoff. Exact process identity, group membership,
worktree registry and generation fence recovery; uncertain processes/resources remain blocked.
Pool allocations always enter quarantine after a stage and require explicit safe confirmation.
Every stage acquires a health-qualified member under the same native pool protocol; the bounded
stage prompt includes its exact allocation/member/config reference. Credentials are not inferred.

Checkpointed continuation (stages longer than one wall): optional policy
`supervision.continuations` (`"1"`..`"16"`) lets a stage that its own `wallSeconds` deadline stopped
continue in the same role invocation. The stage first stops cleanly into WAIT; an implement stage
commits its partial work to a per-turn ref under `refs/corvint/tasks/`. The supervisor then answers
`checkpointed continuation N of M` and resumes the recorded host session in the same worktree,
at most M times. Each continuation is another dispatched turn, so the lane `turns` cap, the program
`turns` and `wallClockMinutes` caps and ticket/policy revision fencing still apply; when one leaves
no room, or a `drain` or `cancel` is pending, or the program wall rather than the stage wall ended
the run, the attempt stays WAITING unanswered and the command returns the deadline error. An
integrate continuation reuses its recorded grant and lands once. Host support: Codex resumes with
`exec resume`, OpenCode with `--session S --fork`; Claude Code reports its session only in its final
result, so a policy with `continuations` is refused `UNSUPPORTED` for it before any record. No host
reports the token usage of an interrupted turn, so `continuations` also requires the lane and program
`inputTokens` and `outputTokens` caps to be `"0"`, otherwise it is refused `UNSUPPORTED`. `run`
never continues a waiting checkpoint by itself; after the cause is resolved, continue it with
`corvint-tasks retry --program P --config C --role implementer` (or `--role integrator`; an
integrate checkpoint keeps its grant, and a different `--grant` is refused `APPROVAL_MISSING`).
Live Codex and OpenCode qualification of continuation is NOT_RUN; see
[S21](specs/corvint-tasks-agent-leases-v0.md#s21--multi-repository-supervised-programs-issue-354).

Turns and active wall deadlines are enforced locally. Input/output tokens are observed from
qualified JSONL, with missing/cache dimensions reported NOT_OBSERVED. Token cutoffs prevent later
dispatch; they cannot stop provider consumption mid-turn and may overshoot by one admitted turn
per active lane. Required hard token enforcement refuses. Prompts/context and runtime output are
bounded and retained in the native journal evidence; truncated output never qualifies as success.

Scoped local Codex qualification is recorded in [the build log](https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-09-29-tasks-codex-supervision.md), including output-limit and unobserved audit boundaries. See
[the accepted contract](specs/corvint-tasks-agent-leases-v0.md#s10--foreground-codex-programs-issue-341).

# Continuous dispatcher

`corvint-tasks dispatch` keeps a configured roster of host workers (OpenCode, Codex, Claude Code or
any argv-launched agent) busy on the native queue while it runs. It is started by the operator and
stops with SIGINT/SIGTERM; workers it launched keep running and the next dispatcher adopts them.
It holds no queue authority: workers claim, submit and gate through the ordinary CLI under their
worker ID as holder, and the dispatcher writes only `release` (HANDOFF) and `reap`.

A `taskman-dispatch/0` config names `stateDir`, `workRoot`, `tickSeconds`, `globalCap`,
`killGraceSeconds`, `hosts` (absolute argv with placeholders such as `{prompt}`, `{ticket}` and
`{holder}`, plus optional env, `idleIgnore` and `activityPaths`), an optional `workState` reader,
`roles` (match by labels/kinds/idGlob/states/statuses/planSelected/pool, or a quarantined pool
`lane`; cap, priority, prompt, idle and wall seconds), `pinned`, `backoff` and `heal`. A role's
`priority` (0..1000) is an ascending sort key: when several roles match one ticket, the role with the
LOWER value launches it, and equal values fall back to the earlier role in `roles` (CAL-V0-054). A
role whose match a lower-valued enabled role provably covers is reported as shadowed: a `config`
event with outcome `ROLE_SHADOWED` at start and after each applied reload, plus `roleShadows` and a
result warning in `dispatch status`. The warning never refuses the config or changes launch order;
the check is conservative, so an overlap it cannot prove is not reported (CAL-V0-290). Text shared
by several roles can live once in a top-level `prompts` map; a role `prompt` may then be an array
such as `[{"fragment": "rules"}, "Implement {ticketLocal}."]`, joined with no separator at load
(CAL-V0-175..178, proposed). A fragment no role references is refused. A ticket that
records `requiresPool` matches only a role whose `match.pool` names that pool, with `{pool}` bound
for its host's `claim ... --pool {pool}`; other roles never see it (CAL-V0-097). With a `workState`
reader, a ticket whose known work state no ticket role's `states`/`excludeStates` admits is held: the
dispatcher replans its observation with it deferred `WORK_STATE_HELD`, so it takes no
`maxActiveAttempts` slot from actionable tickets; `UNKNOWN` and `NONE` keep today's window (CAL-V0-105).

```sh
corvint-tasks dispatch --program night --config dispatch.json
corvint-tasks dispatch status --program night --config dispatch.json --events 20
corvint-tasks dispatch unpark --program night --config dispatch.json --key ticket:acme:main:AT-0002
corvint-tasks dispatch salvage --program night --config dispatch.json --attempt ATTEMPT
```

`dispatch salvage` writes the secret-screened hand-off note of the program worker that holds the
attempt, whether its session still runs or something else stopped it, without changing the attempt;
the next launch on the ticket is offered it under the hard-stop note rules (ATR-V0-105). A host whose
`contextNotices` is `file` receives context warn and hand-off notices appended to the file named by
`CORVINT_DISPATCH_CONTEXT_NOTICE_FILE` (ATR-V0-125..127). A host whose `contextNotices` is `plugin` (opencode roles only)
receives them in-session through the Corvint OpenCode plugin, which runs `dispatch context-hook` when
the worker's context record changes (DSG-V0-008). `dispatch upgrade --check NEW_BUILD` reports,
without changing state, whether a restart on NEW_BUILD adopts the live workers (DSG-V0-007). `dispatch upgrade --apply NEW_BUILD` asks the
running dispatcher to hand over to NEW_BUILD in place: between ticks it execs NEW_BUILD under the same
pid with the lock passed through, so no worker is drained, signalled or orphaned and no lease or lock is
released. Sessions launched earlier finish on the build recorded in `workers/<id>/build.json` (their
`CORVINT_TASKS_BIN` names it); the `upgraded` event's `windowEndsBy` bounds that mixed-build window,
`upgrade-settled` closes it, and at most three builds may be live at once. A failed exec keeps the old
build running (`UPGRADE_FAILED`); a crash mid-upgrade is reported by the next start
(`UPGRADE_INTERRUPTED`), which adopts the workers. Roll back with `--apply PREVIOUS_BUILD` or a plain
restart on the previous build (DSG-V0-010..015).

Each tick observes the queue, supervises workers (whole-tree kill on wall cap, idle timeout or an
orphaned tree; a child process keeps a worker from the idle timeout only while it is new or uses
at least 1% CPU since the last activity, so sleeping helpers such as MCP servers do not, CAL-V0-218,
proposed), hands off live attempts of ended workers, reaps expired leases, accounts progress,
and launches the roster. A refused hand-off is retried a bounded number of times with backoff and the
attempt is reaped once its lease expires; only when both fail is it reported as `needs-owner`
(`heal.exitRecovery`, default true; CAL-V0-104). With `heal.reap`, a worker that is still running
but whose attempt's lease has been expired for longer than its role's `expiredLeaseGraceSeconds`
(0..86400, default 600) has that attempt reaped, fenced on the lease expiry it observed, which frees
its pool member through the usual quarantine and cleanup; only when the store reports that reap is
the worker stopped like a wall-capped worker, with a `lease-expired` event. A running worker whose
attempt the store holds as reaped is stopped on every tick, so a restart does not lose the stop
(CAL-V0-191, proposed). A role's optional `keepLease {intervalSeconds, leaseMinutes, maxIdleSeconds}` has the
dispatcher keep the attempt of each running worker that holds one: every `intervalSeconds` it records
the attempt heartbeat, and renews for `leaseMinutes` once the lease no longer covers two intervals.
It writes nothing for an exited worker or one whose supervision activity (output growth,
`activityPaths`, a busy child) is older than `maxIdleSeconds`, so that worker's lease runs out as
above, and it stops once, with one `lease-keeper` event, when the attempt is fenced, released or
regenerated; `dispatch status` shows each worker's `keeper` (CAL-V0-311..318). A run that changes no durable ticket state cools the ticket down; after
`parkAfter` such runs it is parked until its state changes or the operator unparks it. Exhausted
retries are reported as `needs-owner`; readmission stays the owner's `ticket reopen`. Every
decision is a plain-language line on stderr and in `events.jsonl`. The `finished` summary is the
worker's final text when the host emits a recognized event stream (OpenCode `run --format json`,
Codex `exec --json`, Claude Code `-p --output-format stream-json --verbose`, or `json` without
`--verbose`, Gemini CLI `-p -o stream-json` or `json`), otherwise the output tail; a worker that
did not complete carries only that final text or `(stopped before summary)` (see "The `finished`
event" below). A Claude Code host can look like this, with `corvint-tasks` on the worker `PATH`
and user-level settings and hooks excluded:

```json
"claude": {
  "argv": ["/opt/homebrew/bin/claude", "-p", "{prompt}", "--output-format", "stream-json",
           "--verbose", "--setting-sources", "project", "--strict-mcp-config",
           "--no-session-persistence", "--permission-mode", "dontAsk", "--max-turns", "40",
           "--allowedTools", "Bash(corvint-tasks claim *)", "Bash(corvint-tasks submit *)",
           "Bash(corvint-tasks gate run *)", "Bash(go test *)", "Read", "Edit", "Write",
           "--disallowedTools", "Bash(git push:*)", "WebFetch", "WebSearch"],
  "env": {"PATH": "/usr/local/bin:/usr/bin:/bin"}
}
```

With `--permission-mode dontAsk` a tool outside the allow-list is refused rather than prompted, so
a headless worker never blocks on approval; list every command the role prompt asks for.

A Codex host runs `codex exec`, which never prompts for approval:

```json
"codex": {
  "argv": ["/opt/homebrew/bin/codex", "exec", "--json", "--ephemeral", "--ignore-user-config",
           "--sandbox", "workspace-write", "--cd", "{workRoot}",
           "-c", "sandbox_workspace_write.writable_roots=[\"{workRoot}/.git\"]", "{prompt}"],
  "env": {"PATH": "/usr/local/bin:/usr/bin:/bin"}
}
```

The `workspace-write` sandbox keeps `.git` read-only. A worker's `claim` writes the store journal
and its filesystem probe in the git common directory, so it fails `UNSUPPORTED_FILESYSTEM` until
that directory is a writable root. When `workRoot` is a linked worktree, name the common directory
(`git rev-parse --path-format=absolute --git-common-dir`) instead. The writable root also lets the
worker write refs, objects, hooks, `config` and the store journal itself, so a sandboxed worker can
still plant commands that later run in the dispatcher's or operator's unsandboxed Git, and can edit
queue state without the CLI.

A review-only Codex run (`codex exec -s read-only`) is not read-only for the store on its own: with
the default `approval_policy=on-request`, a command Codex escalates runs outside the sandbox and can
write `.git/taskman` (V1-1237, V1-1441). Launch a review with `-c approval_policy="never"` and
`CORVINT_TASKS_READ_ONLY=1` in its environment; `corvint-tasks` then refuses every non-read command
`UNSUPPORTED` (ATR-V0-212). The variable does not stop a write that bypasses the CLI.

A Gemini CLI host runs headless with `-p`, where a tool call no policy rule allows is denied or
fails, so a `--policy` file is the allow-list. This example is derived from the Gemini CLI 0.54.0
source and is not live-qualified (see the
[build log](https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-01-tasks-dispatch-gemini.md)):

```json
"gemini": {
  "argv": ["/opt/homebrew/bin/gemini", "-p", "{prompt}", "-o", "stream-json",
           "--approval-mode", "default", "--policy", "/abs/gemini-worker.toml",
           "--skip-trust", "-e", "none"],
  "env": {"PATH": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"}
}
```

```toml
[[rule]]
toolName = "run_shell_command"
commandPrefix = ["corvint-tasks claim", "corvint-tasks submit", "corvint-tasks gate run", "go test"]
decision = "allow"
priority = 999

[[rule]]
toolName = ["read_file", "write_file", "replace"]
decision = "allow"
priority = 999
```

The `gemini` launcher runs `node` from `PATH`. Rules from Gemini settings (`tools.allowed`,
`tools.exclude`, `tools.core`) rank just above priority-100 `--policy` rules, so the allow rules use
999. A redirected shell command (`go test ./... 2>&1`) still needs confirmation and fails headless.
`-e none` loads no extensions. `--skip-trust` trusts `workRoot`, so its `.gemini/settings.json` and
`.env` load alongside `~/.gemini`. Because the policy allows file writes, a worker can widen its
next run's tools through those files. Gemini needs its own non-interactive authentication (for
example `GEMINI_API_KEY`). A host's own permission allow/deny-list or sandbox (Claude Code `--allowedTools`,
Codex `--sandbox`, Gemini `--policy`, OpenCode `OPENCODE_CONFIG_CONTENT`) is the host's
responsibility, not a containment guarantee. See
[the accepted contract](specs/corvint-tasks-agent-leases-v0.md#s11--continuous-dispatcher-issue-431).

## The `finished` event

Each ended worker writes one `finished` event to `events.jsonl` (keys `profile`, `seq`, `at`,
`program`, `kind`, `ticket`, `role`, `worker`, `message`, `detail`). `message` is for people:
`<role> worker <id> finished on <ticket> (exit <exitCode>, <progressLabel>)`, then `: <summary>`.
Consumers read `detail`, whose values are all strings; new keys may be added, none is renamed
(ATR-V0-350..354):

| Key | Values |
| --- | --- |
| `exitCode` | the leader's exit code; `-1` when a signal ended it; `NOT_OBSERVED` |
| `exitSignal` | `SIGINT`, `SIGTERM`, `SIGKILL`, ... from the wait status, or from an exit code 129..159 (128+N); `NONE`; `NOT_OBSERVED` |
| `stopReason` | `completed`, `operator-stop` (SIGHUP, SIGINT, SIGKILL or SIGTERM from outside the dispatcher), `context-limit`, `lease-expired`, `crash` (another signal or a nonzero exit), `idle-timeout`, `wall-limit`, `unknown` (exit not observed) |
| `killReason` | the dispatcher's own stop: `CONTEXT`, `LEASE_EXPIRED`, `IDLE`, `WALL`, `ORPHANED`, or empty |
| `commitCount`, `commits`, `commitsTruncated` | commits reachable from `HEAD` in the launch directory since the `HEAD` resolved just before spawn: a count, comma-separated full ids newest first (at most 32), `true` when more exist; `commitCount: NOT_OBSERVED` outside a Git repository |
| `attemptRelease` | `held` (still held), `handed-off` (released by the dispatcher), `reaped` (lease expired and reaped), `released` (released by the worker), `none` |
| `handoffNote` | `true` when a valid hand-off note of the worker exists |
| `progress`, `session` | the accounting decision that drives cooldown and parking: `true` or `false`; `progress`, `no-progress`, `infrastructure`, `held` |
| `progressLabel` | the label in `message`, derived from `session` and the facts above, such as `stopped by SIGINT (operator-stop); 1 commit, store state unchanged` |
| `runSeconds`, `salvage`, `usage`, `inputTokens`, `outputTokens`, `tokens`, `model`, `tier`, `effort`, `context*`, `carryOver`, `detachedRun`, `workState`, `sessionsSinceStatusChange` | as documented for usage, context warnings, carry-over, detached runs and stalls |

Commits count only in the worker's own launch directory; workers that share `workRoot` see each
other's commits, so give a role a per-ticket `cwd` when the count matters. Commits never count as
progress: `progress` still follows the store-owned work state and the declared progress signal.

# Moving or copying a task store

Keep the store (`.git/taskman` in the git common directory), each dispatch config file and the
config's `stateDir`, which holds every worker's `evidence` directory, on real paths with no
symlink component. A symlinked component is refused, and a symlink left at a moved checkout's old
path does not make the store readable at the new one (TRL-V0-005).

The store records its primary worktree. After moving the checkout, every command refuses
`UNSUPPORTED_FILESYSTEM` or `UNSUPPORTED` and names the recorded path. Bind the store to the new
location explicitly, from the new primary worktree:

```sh
corvint-tasks relocate --from /old/primary/worktree --request-id relocate-1 \
  --dispatch-config /real/path/dispatch.json
```

The command commits one RELOCATE receipt naming both paths and re-pins `head.primaryWorktree`. It
refuses `QUIESCENCE_UNPROVED` while any attempt holds a reservation, so release or reap every
attempt first. With `--dispatch-config` it also refuses while a dispatcher for that config runs or
is starting, refuses a `stateDir` that is not a real path, and refuses any dispatcher start until it
finishes;
without the flag the result warns that the dispatcher check was `NOT_OBSERVED`, so stop every
dispatcher yourself. A dispatch config whose `workRoot` or `stateDir` named the old path must be
edited to the new real path. An interrupted relocation is finished by rerunning the same command
(TRL-V0-001 to TRL-V0-004).

A copy of the store for an off-path drill is bound to its new clone the same way. Copy only while
no `corvint-tasks` writer, dispatcher or live attempt holds the store: a copy taken during writes
refuses `JOURNAL_FORKED` and names that requirement. A relocated copy shares the original queue id
and diverges from it, so keep only one of them writable (TRL-V0-006). Archive restore into a fresh
store at a new path is not available in this build. See
[the relocation contract](specs/corvint-tasks-store-relocation-v0.md).
