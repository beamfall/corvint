# Start an external-agent queue

`corvint-tasks` is a separate binary. The rc.1 core archive contains only `corvint`;
its Tasks companion predates some of the commands below. Check `corvint-tasks help` for
`plan preview`, `claim --next`, `submit`, `gate run` and `complete`. An unstamped or
`0.0.0-tcp01-unverified` version is not qualification evidence. In a checkout, see `docs/INSTALL.md`. In a standalone archive, the binary is under
`bin/`, the templates under `templates/`, and the matching source under `source/`.
Extract the source archive to set `corvint_source` for qualification.

## Fresh native queue

Use the canonical templates at `internal/tasks/cli/testdata/external-agents/` in the
source checkout (or `templates/` in the binary archive). Set
`corvint_source` to a matching Corvint source checkout and run the following in the new
application repository, which must have a `main` branch. Do not copy Corvint's own queue.

```sh
mkdir .taskman
cp "$corvint_source/internal/tasks/cli/testdata/external-agents/queue.json" .taskman/queue.json
cp "$corvint_source/internal/tasks/cli/testdata/external-agents/policy.json" .taskman/policy.json
```

Before `init`, customize `queueId` (`queue:example:main`), `repositoryAuthorityId`
(`repo:example`), `prefix` (`APP`) and `intentBranch`. Keep `fixture:false`,
`canonicalWriter:NATIVE`, and `executionCutover:null`. Preserve compact sorted-key JSON
with exactly one final LF. Leave `tickets/` and `releases/` absent or empty: even a `.keep`
file intentionally prevents genesis over existing records. Create tickets only after init.
Do not overwrite an existing `.taskman` or remove a journal to make initialization succeed.

The policy admits four external attempts with disjoint declared scopes. It requires a
`make verify` command gate; replace that argv with the repository's real gate before init.
Only PATH is forwarded; explicitly add any necessary environment keys to both allowlists.
`requireEnforcedFields:[]` means external-agent budgets are unobserved, not enforced.
`serialFallback:WHOLE_REPOSITORY` serializes tickets whose effects remain incomplete;
never relabel unknown effects QUALIFIED just to admit parallel work. Neither template
authenticates an operator or claims a completed review.

After customizing both files, initialize the queue:

```sh
corvint-tasks init --request-id initialize-queue
```

## Qualify before claims

A native non-fixture queue intentionally refuses claims until the owner records an execution
cutover. Run the actual lease qualification suite from the matching source checkout on the
host where the queue will run, retaining the complete output:

```sh
(cd "$corvint_source" && GOTOOLCHAIN=local go test -json -count=1 -timeout 30m \
  -run '^TestCALV0019_' ./internal/tasks/store) > /tmp/tasks-qualification.jsonl
corvint-tasks cutover --execution --decision enable-external-agents \
  --qualification /tmp/tasks-qualification.jsonl
```

Check the test command's exit status before the cutover. Never synthesize passing events.
The retained qualification remains owner-supplied evidence. A fresh NATIVE queue needs no
separate authority switch. Imported queues instead require `cutover --decision REF` to switch
the canonical writer to NATIVE before this execution cutover.

Copy `ticket-create.json` outside `.taskman/tickets/`, adjust its source queue ID and actual
acceptance criteria, then use `ticket create --request-id ID --payload-stdin < FILE`.
The example deliberately retains `effects.coverage:INCOMPLETE` and requires `verify`.

```sh
corvint-tasks plan preview
corvint-tasks claim --next --holder agent-a --request-id claim-a
```

`plan preview` is a read-only eligibility plan; `ticket blockers` reports static intent checks
and may say UNKNOWN even when admission is possible. `claim --next` makes selection and
reservation atomic. Explicit `claim TICKET --scope src/` can bound a requested scope; its
safety remains the holder's responsibility, and submitted changes outside it refuse.

## Finish the same attempt

Retain the returned attempt ID and generation. Renew before the lease expires. All subsequent
operations carry both values; stale generations refuse even if the old agent keeps running.
Submit the candidate Git tree, run the declared gate, integrate with the repository's own
tooling, and complete only once the exact candidate tree is reachable on the intent branch:

```sh
corvint-tasks submit --attempt "$attempt" --generation "$generation" --request-id submit-a --tree "$tree"
corvint-tasks gate run --attempt "$attempt" --generation "$generation" --request-id gate-a --gate verify --worktree "$worktree"
corvint-tasks complete --attempt "$attempt" --generation "$generation" --request-id complete-a --commit "$commit"
corvint-tasks receipt audit
```

The gate worktree must be clean. Resubmitting invalidates earlier gate results. `complete-manual`
is an operator disposition, not an external-agent completion shortcut. `release --reason` takes
only the `releaseReasonCodes` returned by `help` (for example `GATE_FAILED`); free prose refuses.

## Payloads and shared worktrees

Payloads use sorted object keys, compact JSON, literal UTF-8 (not `\u00a7` for `§`), and sorted,
deduplicated set arrays such as `touchPaths`. Preserve ordered arrays such as gate argv and
acceptance criteria. For Python, serialize using `ensure_ascii=False, sort_keys=True,
separators=(',', ':')`, then add one LF; sort only fields documented as sets. Each mutation's
`--help` lists its closed payload keys.

Linked worktrees share the primary checkout's `.git/taskman` journal. A fresh clone has no such
journal: current `queue status`, `roadmap`, `ticket show` and `ticket search` can read the
unvalidated-history intent projection and report its limits, but cannot claim or complete work.
`receipt audit` still requires a journal. Do not run `init` over copied populated records;
use the explicit import/authority-cutover route or a worktree of the existing primary checkout.
