# Foreground Codex programs

Enable the optional `taskman-codex-supervisor/0` runtime in the native policy and its
`supervision` object before dispatch. The runtime pins executable path/content hashes and mode,
existing BUILDER/REPAIR/REVIEWER/VERIFIER roles, and worker capacity. `supervision` contains
`profile`, `contextRequired: true`, `maxRepairCycles` (0..2), and `program` caps for `turns`,
`wallClockMinutes`, `inputTokens` and `outputTokens` (canonical decimal strings).

A local JSON config contains `profile`, absolute resolved `executable`, `executableSha256`,
explicit `model`, `effort: "low"`, `prompt` (reserved; generated stage prompts govern dispatch),
absolute `workRoot`, numeric `wallSeconds` (1..3600), `coreExecutable`, `coreSha256`, and
`ownIntegrationCheckout` (false unless the operator explicitly designates this clean checkout).
Optional `pool` selects the implementation pool. The policy remains authoritative. Core context
must be READY/fresh at the stage's exact Git tree; stated uncertainty is retained in prompts.

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
`baseCommit + NUL + candidateTree + NUL + intentBranch`. Then run the integrator role with
`--grant GRANT_ID`. The designated checkout must still be clean at that original base; a later tip
requires a fresh candidate/review/gates/grant. No remote push is performed. Native completion and
receipt audit remain the delivery boundary.

`drain` retains WAIT and scope; `cancel` releases scope only after proved shutdown. `retry` is an
explicit operator continuation of a retained handoff. Exact process identity, group membership,
worktree registry and generation fence recovery; uncertain processes/resources remain blocked.
Pool allocations always enter quarantine after a stage and require explicit safe confirmation.
Every stage acquires a health-qualified member under the same native pool protocol; the bounded
stage prompt includes its exact allocation/member/config reference. Credentials are not inferred.

Turns and active wall deadlines are enforced locally. Input/output tokens are observed from
qualified JSONL, with missing/cache dimensions reported NOT_OBSERVED. Token cutoffs prevent later
dispatch; they cannot stop provider consumption mid-turn and may overshoot by one admitted turn
per active lane. Required hard token enforcement refuses. Prompts/context and runtime output are
bounded and retained in the native journal evidence; truncated output never qualifies as success.

Scoped local Codex qualification is recorded in [the build log](build-log/2026-09-29-tasks-codex-supervision.md), including output-limit and unobserved audit boundaries. See
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
`roles` (match by labels/kinds/idGlob/states/statuses/planSelected, or a quarantined pool `lane`;
cap, priority, prompt, idle and wall seconds), `pinned`, `backoff` and `heal`.

```sh
corvint-tasks dispatch --program night --config dispatch.json
corvint-tasks dispatch status --program night --config dispatch.json --events 20
corvint-tasks dispatch unpark --program night --config dispatch.json --key ticket:acme:main:AT-0002
```

Each tick observes the queue, supervises workers (whole-tree kill on wall cap, idle timeout or an
orphaned tree), hands off live attempts of ended workers, reaps expired leases, accounts progress,
and launches the roster. A run that changes no durable ticket state cools the ticket down; after
`parkAfter` such runs it is parked until its state changes or the operator unparks it. Exhausted
retries are reported as `needs-owner`; readmission stays the owner's `ticket reopen`. Every
decision is a plain-language line on stderr and in `events.jsonl`. The `finished` summary is the
worker's final text when the host emits a recognized event stream (OpenCode `run --format json`,
Codex `exec --json`, Claude Code `-p --output-format stream-json --verbose`, or `json` without
`--verbose`), otherwise the output tail. A Claude Code host can look like this, with `corvint-tasks` on the worker `PATH`
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
queue state without the CLI. A host's own permission allow/deny-list or sandbox (Claude Code `--allowedTools`,
Codex `--sandbox`, OpenCode `OPENCODE_CONFIG_CONTENT`) is the host's responsibility, not a
containment guarantee. See
[the accepted contract](specs/corvint-tasks-agent-leases-v0.md#s11--continuous-dispatcher-issue-431).
