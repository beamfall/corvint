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
A mutation payload (`--payload` or `--payload-stdin`) may be any valid JSON for the verb's closed
keys: the CLI canonicalizes whitespace, key order and escape form, and sorts set arrays such as
`labels`, `requirementRefs` and `touchPaths`, before the request digest. Ordered arrays such as
`acceptanceCriteria` and `dependencies` keep the order given; duplicate set elements refuse.
Files written into `.taskman/` (policy, queue) still must be canonical.

To choose a board ID, add optional `"localToken":"BT-002"` to the canonical CREATE payload.
`BT-002`, `F0-5` and `FL-016.matrix` use the existing queue-local token grammar. The resulting
ID is `ticket:AUTHORITY:QUEUE:BT-002`; `ticket show BT-002` and `claim BT-002` use that identity.
Exact and case-fold collisions refuse. Omit localToken for automatic allocation. CREATE accepts
neither `--target` nor `--expected-revision`; those flags address existing records.

`corvint-tasks policy show` reads the effective policy, version and policy content identity SHA.
To update, write compact UTF-8 JSON with sorted keys and exactly one final LF. Set the file's
`policyVersion` to the current version plus one, then pass the current version as
`--expected-policy-version N` with `--file PATH` and a fresh `--request-id`. For Python, use
`json.dumps(policy, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n"`.
Do not overwrite `.taskman/policy.json` directly.


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

To record evidence on an OPEN ticket without changing it, an OWNER (or an OPERATOR whose policy
row explicitly names `ATTACH_EVIDENCE`) attaches 1..16 sorted, unique sha256 digests and a nonblank
reason of at most 512 bytes:

```sh
corvint-tasks ticket attach-evidence --target "$ticket" --expected-revision "$revision" --request-id evidence-a \
  --payload '{"evidence":["<64 lowercase hex>"],"reason":"focused go test log"}'
```

The write bumps the ticket revision only; acceptanceRevision, status, gates and completion are
unchanged, and attached evidence never satisfies a gate or criterion. `ticket show` lists the
entries under `attachedEvidence` with the actor, time and acceptance revision. A digest already
attached at the current acceptance revision refuses `DUPLICATE_ID`; non-OPEN or imported tickets
refuse `TICKET_STATE`; an identical retry replays (TEA-V0-001).

## Payloads and shared worktrees

Payloads use sorted object keys, compact JSON, literal UTF-8 (not `\u00a7` for `§`), and sorted,
deduplicated set arrays such as `touchPaths`. Preserve ordered arrays such as gate argv and
acceptance criteria. For Python, serialize using `ensure_ascii=False, sort_keys=True,
separators=(',', ':')`, then add one LF; sort only fields documented as sets. Each mutation's
`--help` lists its closed payload keys. `ticket create --template` prints a canonical CREATE payload for
this queue (`.items[0].payload`) plus a `fields` table of types, enum values and null-able keys;
fill in `title`, `body` and `acceptanceCriteria`, then submit it with `--payload-stdin`.

A ticket that only some stages must wait for carries optional `executionPrerequisites`, set with
`ticket refine` (for example `{"executionPrerequisites":[{"gateId":null,"obligation":"COMPLETED",
"stages":["integrate"],"ticketId":"ticket:acme:main:AT-02"}]}`) and cleared with `null`. A claim,
`claim-next` or `plan` for a listed stage refuses `PREREQUISITE_UNSATISFIED` naming the
prerequisite; other stages are unaffected, and a read without `--stage` applies every entry.
`ticket blockers` explains the block. A `GATE_PASSED` prerequisite stays `NOT_OBSERVED` (unknown)
on native reads, so its stages stay refused until the entry is removed. Unlike `dependencies`, the
key takes no part in cycles, completion or `requiredGates` (CAL-V0-099). Once any record has
carried the key, older binaries refuse the store even after it is cleared, because the journal keeps
the earlier records; roll back only with a compatible reader or a verified pre-change backup.

To refine many tickets, use `ticket refine --batch --request-id ID --payload-stdin` with a JSON
array of `{"target":"AT-02","expectedRevision":"3","payload":{...}}` entries, each target once.
Every entry is validated before anything is written; entries then apply in chunks of at most 8
under one writer lock each, released between chunks so claims and heartbeats are not starved. Each
entry is an ordinary refine under request ID `ID/<index>` with its own expected-revision check and
receipt: a stale entry is refused alone. Retry with the same `--request-id` and the `issuedAt` the
result reports (`--issued-at`) to replay completed entries without applying them twice (CAL-V0-106).

Linked worktrees share the primary checkout's `.git/taskman` journal. A fresh clone has no such
journal: current `queue status`, `roadmap`, `ticket show` and `ticket search` can read the
unvalidated-history intent projection and report its limits, but cannot claim or complete work.
`receipt audit` still requires a journal. Do not run `init` over copied populated records;
use the explicit import/authority-cutover route or a worktree of the existing primary checkout.

## Isolated environment pools

Add optional `pools` to policy through `policy update`, for example:

```json
{"pools":[{"id":"test-env","members":["env-0","env-1"],"reservedFor":{"env-1":"review"}}]}
```

A ticket's optional `requiresPool` is acceptance-relevant; create/refine validates the pool name.
Explicitly request it when claiming. Unreserved members accept any stage; a reserved member requires
its declared stage. The stage is an operator declaration, not an authenticated reviewer identity.

```sh
corvint-tasks claim AT-123 --pool test-env --stage implement --holder builder --request-id claim-123
corvint-tasks claim --next --pool test-env --stage review --holder reviewer --request-id review-next
corvint-tasks queue status
corvint-tasks plan preview --pool test-env --stage implement
```

The default `plan preview` (no `--pool`) selects a `requiresPool` ticket only while its pool has a
free eligible member left for it (unreserved members, or those reserved for `--stage` when given).
It defers the rest `RESOURCE_COLLISION` with the pool as blocker, outside `maxActiveAttempts`, so
they do not crowd out lane-free work; the plan's `resourceDeferred` rows give each pool's free,
selected and deferred counts. Unobservable member state reports `NOT_OBSERVED` and defers.
`claim --next` without `--pool` skips `SELECTED` pool tickets; claim them with their pool. Under
`corvint-tasks dispatch`, a pool ticket goes only to a role whose `match.pool` names its pool (its
host should claim with `--pool {pool}`); a pool no role names is deferred in the dispatcher's plan
so it never takes the window from lane-free roles. This
only helps tickets that record `requiresPool`: a ticket that waits for an environment without
declaring it is planned as lane-free and still fills the window (open bugs V1-0754, V1-0758 and
V1-0759). Record `requiresPool` on create or refine for every ticket that needs a member.

A pool may opt into priority-yield admission (CAL-V0-101) with `"priorityAdmission":true`:

```json
{"pools":[{"id":"test-env","members":["env-0","env-1"],"priorityAdmission":true}]}
```

Then an explicit `claim <ticket> --pool test-env` is refused `BLOCKED RESOURCE_COLLISION` when the
higher-priority `OPEN` tickets that record `requiresPool:"test-env"`, have no claim blocker and no
live attempt are at least as many as the pool's free eligible members; the detail ends
`yields to <ticketId>`, naming the first of them in admission order (CAL-V0-108): priority first,
then, at equal priority, tickets whose latest generation handed off to `review` or `integrate`
(earliest handoff first), then plan order. When the ticket yielded to is such a downstream ticket the
detail adds `; <ticketId> awaits <stage> since seq <seq>`. Plan preview (default and `--pool`)
shows such a ticket `DEFERRED RESOURCE_COLLISION` with that ticket ID as blocker, and `claim --next`
never picks it. A competitor whose blockers are unobservable never causes a refusal; `ticket show`
reports `NOT_OBSERVED` claimability instead. Nothing is stored and there is no waitlist (V1-0785).
Remove the key (or set `false`) with `policy update` to disable the yield. Once any policy has
carried the key, older binaries refuse the store even after it is removed, because the journal keeps
the earlier policy records; roll back only with a compatible reader or a verified pre-change backup.
A search for the byte string `"priorityAdmission"` in the policy record and `.git/taskman` (journal
and `evidence/`) tells whether the key was ever written.

To avoid a known member for a particular claim or preview, repeat the single-value
`--exclude-member` flag:

```sh
corvint-tasks claim --next --pool test-env --stage review --exclude-member env-0 --holder reviewer --request-id review-other
corvint-tasks plan preview --pool test-env --stage review --exclude-member env-0
```

An exclusion set requires an explicit pool. Reordered duplicates normalize to one
set; missing/empty values, invalid labels and foreign members refuse. At most 256
unique members may be excluded. Excluded reserved and unreserved members provide
no claim or preview capacity and run no health command. Excluding all eligible
capacity produces `RESOURCE_COLLISION` for a claim. These are explicit caller-selected
member labels; they do not discover ticket history, authenticate reviewer independence
or prove separate physical environments. Ordinary resource scope, `requiresPool`
and quarantine remain binding. An identical request replays its original allocation
after release, a successor or a permitted policy change; a changed valid exclusion
set conflicts under the same request ID.

For a review or integrate claim, `--exclude-authors` derives the exclusions from the ticket's
recorded history instead (CAL-V0-098). The bare flag excludes the member of the ticket's most recent
implement generation; `--exclude-authors=all` excludes the member of every recorded implement
generation:

```sh
corvint-tasks claim AT-123 --pool test-env --stage review --exclude-authors --holder reviewer --request-id review-123
corvint-tasks claim --next --pool test-env --stage integrate --exclude-authors=all --holder integrator --request-id integrate-next
corvint-tasks plan preview --pool test-env --stage review --exclude-authors
```

It requires an explicit pool and `--stage review` or `--stage integrate`, and it unions with any
`--exclude-member`. The mode, not the derived member set, is bound into the request, so an identical
request replays and a changed mode conflicts under the same request ID. Walking the ticket's
generations newest first, review and integrate generations are skipped; any other generation
reached must be an implement generation with a recorded pool member. A generation whose member is
`NOT_OBSERVED` (ended before the V1-0788 prior-generation history, or supervised), one with no recorded stage, or an
implement generation without a pool member refuses the claim with `INDEPENDENCE_UNVERIFIED`; it is
never silently unfiltered, and nothing is recovered from receipts. When you also pass at least one
`--exclude-member`, those generations (except a stage-less one that recorded a member) are covered by
your explicit set instead, and the claim result warns that the exclusion is caller-asserted, not
recorded (CAL-V0-107). A ticket with no implement generation has no author to exclude and is not
refused.
When no member remains, the claim refuses `RESOURCE_COLLISION` with a detail naming the excluded
authors. `plan preview` reports the same per ticket, and with this flag adds `detail` and
`excludedAuthors` to each entry. A recorded member label is not an authenticated identity and
proves nothing about who did the work or whether two environments are physically distinct.

Retain the returned `poolAllocation` alongside attempt ID and generation. Replays return the original
receipt-bound allocation, including after a retry has acquired a successor. Release, completion and
reap free the source scope but quarantine the environment. Reads never probe or clean environments.

Optional `memberConfig` supplies immutable regular Git `configRef:{revision,path,blob}` references and
`health`/`cleanup` commands. Each command has `argv`, `cwd`, declared `env` names and
`timeoutSeconds` from 1 to 3600. `cwd` is `"REPOSITORY"` or a pinned external checkout
`{"kind":"PINNED_REPOSITORY","path":"/abs","revision":"<full sha>"}`, which runs only while that
worktree's HEAD is the revision and its tree is clean (otherwise STALE_TREE, DIRTY_WORKTREE or
MISSING_EVIDENCE, nothing runs). Inside `pool sweep` a cleanup still shares the safeReuse deadline. Health failures are skipped and reported in occupancy with reason,
command kind and observation digest. Commands require clean repository inputs outside `.taskman`.
The runner bounds captured output to 64 KiB and retains its digest only. It cleans the owned process
group; detached processes and external databases remain the operator's responsibility.

```sh
corvint-tasks health --member env-0 --request-id health-0
corvint-tasks pool cleanup --member env-0 --allocation ALLOCATION_SHA256 --request-id cleanup-0
corvint-tasks pool recover --member env-0 --allocation ALLOCATION_SHA256 --reason 'runner exited unexpectedly' --request-id recover-0
corvint-tasks pool confirm-safe --member env-0 --allocation ALLOCATION_SHA256 --evidence local-reset-record --reason 'external owner revoked and environment reset' --request-id safe-0
```

Standalone health leaves quarantine even on success. Cleanup exit zero also leaves quarantine;
configured cleanup must pass before confirmation. `confirm-safe` is your attributable assertion
that the old external owner has been revoked/reset and reuse is safe. It requires the exact current
allocation. Pending health/cleanup replay never reexecutes a command. `recover` refuses an observed
live runner and quarantines an orphan; it does not prove physical termination. Do not confirm safety
until independent cleanup is complete. No timeout automatically frees a member.

Pools support up to 64 pools and 256 queue-unique members within the existing policy byte bound.
Names alone cannot detect two configurations pointing at the same physical database. Old records
remain readable, but older binaries do not understand the new state: stop claims and use a recorded
safe migration before downgrading.

## Hand off clean work

Claim with `--stage implement`, `review` or `integrate` when work will be handed off. An unstaged
claim cannot qualify. `HANDOFF` is available to all three stages; `REVIEW_RETURNED` is available
only to review. These reasons preserve the charged retry count when the writer verifies clean
prospective accounting. They do not complete the ticket or authorize integration or publication.

For work in the queue repository, submit the actual candidate tree first, then release:

```sh
corvint-tasks release --attempt "$attempt" --generation "$generation" \
  --request-id handoff-a --reason HANDOFF
```

This requires phase BUILT or CHECKING, a candidate with scopeCheck WITHIN, and no pending effects.
For work outside the queue repository, retain the actual artifact separately and reference it
without submitting a tree:

```sh
corvint-tasks release --attempt "$attempt" --generation "$generation" \
  --request-id external-handoff-a --reason HANDOFF --evidence local:review-record-1
```

The no-tree path requires RUNNING, no candidate or gate results, scopeCheck UNKNOWN and no pending
effects. `--evidence` uses the existing Identifier grammar: 1..128 UTF-8 bytes, no hostile code
points or TAB/LF/CR. The reference is recorded on the terminal attempt as `handoffEvidence` and
bound to replay. The tool never fetches, executes or verifies its contents. Do not submit an
unchanged base tree merely to unlock accounting. Evidence on candidate-bearing handoffs or
ordinary cancellations refuses. Repeating the identical request replays; changing its reference
under the same request ID conflicts.

Both paths require a live unexpired matching generation, unchanged acceptance revision and policy,
and writer-produced prospective accounting with no recorded failed or unknown gate result. A
later passing gate or resubmission cannot clear an earlier failure. Legacy missing accounting and
supervised attempts cannot receive this exemption. A `policy update` changes the binding and makes
in-flight handoffs refuse STALE_POLICY; do not update policy expecting it to repair those attempts.
There is no automatic refund or stale-policy bypass.

### Name the next stage

A clean `HANDOFF` or `REVIEW_RETURNED` release may record which stage should run next, with an
optional reason from a closed set:

```sh
corvint-tasks release --attempt "$attempt" --generation "$generation" \
  --request-id handoff-b --reason HANDOFF --handoff-to review --handoff-reason STAGE_COMPLETE
```

`--handoff-to` takes `implement`, `review` or `integrate`. `--handoff-reason` takes
`CHANGES_REQUESTED` (back to implement, never from implement), `STAGE_INCOMPLETE` (the same stage)
or `STAGE_COMPLETE` (a different stage), and needs `--handoff-to`. `REVIEW_RETURNED` may target
only `implement`, which is also its default. Other combinations, and either flag on any other
release or verb, refuse MALFORMED. The target is recorded on the terminal attempt as `handoffTo`
and `handoffReason` and moves into the next generation's `priorGenerations[]` entry. It does not
change retry accounting.

`ticket show`, `plan preview` and the dispatcher (`{nextStage}` launch placeholder and `launched`
event) expose the derived `nextStage`: the latest generation's target, `null` (`NONE` in the
dispatcher) when none is observed, and `STALE` after the acceptance revision changed. It is
advisory; claims for another stage are not refused. Older binaries refuse attempt records that
contain the new keys, so do not downgrade a store after recording a target.

Operator notes (experimental, `corvint-tasks-operator-notes-v0.md`): `ticket note history <ticket>
[--limit 1..50] [--cursor C]` reads superseded and cleared notes newest first as a pure read, with
an opaque cursor anchored to the head the first page read. A dispatcher role prompt may include
`{operatorNote}`; it renders nothing for a never-noted ticket and otherwise a launch-time copy of
the current note, labelled advisory. The claim result's `operatorNote` remains the authoritative
note for the admitted attempt. The placeholder is refused in host argv, env and activity paths, and a role using it needs a host
that passes `{prompt}` as one whole argv element, has no other placeholder in argv, names no shell or interpreter (`sh`, `bash`,
`env`, `python`, `node` and similar) and takes no code-string option such as `-c`, `+c`, `-e`,
`--eval` or `--command`; use a wrapper executable when a shell is needed.

When every external review gate the policy declares or the ticket references is a CURRENT PASS and
nothing else blocks an OPEN ticket, `ticket show`, `ticket blockers` and the `plan preview` entry
report `nextAction: complete-manual` with the review heads as `suggestedEvidence` (ERG-V0-011). The
offer is read-only: nothing completes the ticket automatically, and an operator who accepts it
passes those digests as the `complete-manual` evidence. STALE, UNKNOWN, RETURN or resubmitted
reviews, a live attempt, any unknown, or a planner blocker such as a queue pause give no offer.
`ticket show` and `ticket blockers` then keep their existing `nextAction` (for example `admit` or
`wait-attempt`) without `suggestedEvidence`; a `plan preview` entry carries neither member.
Executable gate results are not part of the offer; they remain NOT_OBSERVED in these reads, and
the owner decided on 2026-10-05 that they do not withhold it.

The reviewed experimental issue 482 writer adds a narrow clean-release exception: only
policyVersion and reservedFor entries for other members in the exact allocated pool may differ.
No-pool attempts permit version changes only. It must fully audit every intervening policy
afterimage against the original claim policy; a relevant change stays stale after restoration.
Original policy/config/capability hashes, acceptance, allocation, lease and clean-work conditions
remain bound. Missing or unproved history refuses STALE_POLICY; malformed history fails closed.
This exception does not permit old-generation completion, live stale-holder reap, retry refunds
or physical pool reuse. Issue 482 was integrated at public commit `094700bfbc7b637bd2d6405cd82ab5508ac4aa20`
and natively completed at receipt 2173. Qualification remains scoped to disposable fixtures;
actor authentication, runtime qualification, history and liveness remain NOT_OBSERVED.

### Detect no-progress loops

A policy may opt into loop detection (CAL-V0-102..103) with a top-level `loopDetection` object:

```json
{"loopDetection":{"maxAlternatingReturns":"2","maxNoProgressGenerations":"2"}}
```

Both counts are required and 1..256. While the key is present, each re-claim records the ended
generation's disposition, candidate tree and gate and review counts as `loopEvidence` in its
`priorGenerations[]` entry. When more consecutive generations than `maxNoProgressGenerations`
ended in a clean `HANDOFF` with no new candidate tree, no gate result and no external review, or
more implement-`HANDOFF`/review-`REVIEW_RETURNED` pairs than `maxAlternatingReturns` alternate,
the ticket is held `LOOP_DETECTED`: `ticket show`, `ticket blockers`, `plan preview`, `claim`,
`claim --next` and `dispatch status` (`loopDetected`) report it with the counted generations and
next action `reopen`; a held `plan preview` entry also carries
`loop {signal, acceptanceRevision, generations, limit}`. The dispatcher skips the ticket and emits
one `needs-owner` event (`kind: blocked`) per episode, retrying it while the event log is
unwritable and skipping it when the log already holds it. Generations recorded without evidence (before the opt-in, legacy
or supervised) are UNKNOWN and never count. The ticket status is not changed.

The hold clears when the acceptance revision changes, for example through the owner's
`corvint-tasks ticket reopen --role OWNER`, which acknowledges the loop and readmits the ticket with
a fresh attempt. Remove the key with `policy update` to stop detection. Older binaries refuse a
store whose policy history ever carried `loopDetection` or whose attempts carry `loopEvidence`;
search the policy record and `.git/taskman` for those byte strings before a downgrade.

### Explicit operator-attested untouched pool release

```sh
corvint-tasks release --attempt ID --generation G --request-id ID \
  --lane-untouched --evidence LOCAL_REF
```

This optional CAL-V0-067 profile records an OWNER/OPERATOR statement with four fixed
acknowledgements: no physical lane access occurred, no lane command was issued, no physical
capability/resource was issued or remains retained, and the operator accepts responsibility
for the statement and safe reuse. Logical source reservations are separate. The required
reference is an inert Identifier (1..128 UTF-8 bytes); it is never fetched or executed.
Recorded identity is not authentication. Physical non-use and revocation remain NOT_OBSERVED;
private Dispatcher records are not scanned. Known or uncertain external use prevents an honest
attestation.

Only a fresh direct no-health pooled claim with a prospective origin witness can qualify.
The current unexpired external-agent RUNNING generation must retain its exact original
allocation, holder, stage and admission sequences, unchanged acceptance and exact current
policy/config. Legacy, prepared/health, renewed, retry-inherited or used generations refuse.
Candidate, gate, review, manifest, failed/unknown retry accounting, pending effects, runner,
worker or supervision identity also refuses. Inventory-bound native Program association,
including ADMITTED before ATTACH with zero leader PID, refuses regardless of phase.
The ordinary compatible-policy handoff exception above never authorizes this profile.

An eligible release atomically records the original allocation and closed attestation,
cancels and logically fences the generation, releases reservations and removes only its
exact occupancy without configured cleanup. It does not claim PROVED physical quiescence.
The response on both fresh execution and replay comes from the original release receipt,
even after a successor or policy change. Missing or damaged original payload refuses;
changed request fields under the same request ID conflict.

Optional HANDOFF or REVIEW_RETURNED reasons must independently satisfy their existing
accounting and stage rules; the flag grants no retry refund or completion authority.
Without the flag, release/handoff, expiry, reap and completion retain quarantine and
configured cleanup/safe-confirm requirements. Stop future opt-in use for rollback, retain
compatible readers and metadata, and preserve exact replay. Old readers may refuse new
metadata: never strip it, silently downgrade, or overwrite a successor.

Scoped source review and native/archive/crash fixtures passed. The native fixture uses
synthetic cutover input as parser/admission test setup, not CAL019 or deployment qualification.
Publication-artifact faults cover receipt, attempt, pool, request, reservation and head;
stage-file/descriptor internals are NOT_INJECTED. Final issue integration and native completion
remain separate from these local proofs.

| Refusal | Meaning |
|---|---|
| FENCED | Generation differs or the lease expired. |
| STALE_TICKET | Acceptance differs from the claim. |
| STALE_POLICY | Policy/config binding differs from the claim. |
| TICKET_STATE | Runtime or stage is ineligible; REVIEW_RETURNED requires review. |
| MISSING_EVIDENCE | Prospective accounting or the selected tree/no-tree conditions are missing. |
| MALFORMED | Evidence or request shape is invalid, including evidence on ordinary cancellation without the explicit untouched profile. |
| REQUEST_ID_CONFLICT | The same request ID was reused with different content. |

Before handing off, retire the processes you own and retain any separately observed cleanup
results. Release fences the generation, removes its reservation, and quarantines an allocated
pool. Quarantine does not prove physical cleanup or safe reuse; use the existing cleanup and
safe-confirm flow before reuse. A consistent `receipt audit` proves journal consistency, not
physical cleanup, external work quality or independent review. Old readers may refuse the new
optional metadata; preserve the entire store and use a compatible reader/writer for rollback.

## Configure charged retries

`retries.admissionsPerRevision` is a required canonical Count in **0..16**. Its historical name
means charged retries after the initial admission: value 3 retains the initial attempt plus three
charged retries, 0 permits only the initial attempt, and 4 permits four charged retries. Fixture
and example policies retain 3. Explicit/next claim, plan preview (including pool/stage), and safe
OWNER recovery use the current policy value. Clean verified handoffs preserve debt even at the
limit; cancellation, failure and expiry remain charged. A raised budget does not erase debt.

Changing a real queue policy remains an explicit operator action. Older writers capped at 3 can
refuse values above 3; stop admissions and keep compatible binaries and complete journals for
rollback. Never strip recorded metadata or rewrite history to downgrade. An exhausted ticket needs
the existing safe OWNER `ticket reopen` flow for fresh acceptance; help and handoffs do not grant it.

## Discover command inputs

Every command and command family supports exact `--help` and `-h`, including `plan preview`,
`submit`, lease `release`, and release-artifact subcommands. Help returns OK with usage and flags
without a queue, stdin reads, locks or writes. Lease release help includes accepted reason codes
and handoff preconditions. Omitted commands explain that execution remains NOT_RUN. Existing
mutation `operation` and `payloadKeys` help is preserved. A flag value spelled `--help` remains a
value; unknown command paths still refuse.

## Read beside concurrent writers

A read that probes a writer between its receipt link-in and its head rename waits it out for up
to two seconds with backoff before it reports `NOT_RUN`/`REDO_PENDING`, and a snapshot that moved
during the read is re-read within the same budget before `NOT_RUN`/`SNAPSHOT_MOVED` (CTS-V0-006).
Both outcomes remain `NOT_RUN`, not `ERROR`: nothing was decided, the store was not changed, and the
warning names the wait and says the read is retryable. A dispatcher that still sees one should
retry the read rather than treat it as a failed command; `REDO_PENDING` that outlives the budget
means a writer crashed inside the window and the next mutating command redoes its receipt.

## Retry by the `retryable` member

Every non-`OK` result that carries a code also carries `retryable` (CAL-V0-078); `OK` and uncoded
results do not. Branch on that boolean instead of matching codes. It is true only when every code
is one of these three, and then the same command with the same `--request-id` can succeed after a
bounded backoff:

- `LOCK_TIMEOUT`: another writer held the store lock or a preparation admission past the wait;
  nothing was locked or written. This is the `ERROR` a renew or heartbeat reports under heavy
  concurrency while the lease is still FRESH; retry it before `expiresAt`. `release` and
  `attempt heartbeat` take `--lock-wait SECONDS` (whole seconds, 1..300) to wait longer than the
  default 30 seconds for that command only (CAL-V0-111). The wait is not part of the request, so
  resubmitting a timed-out `HANDOFF` release with the same request ID commits once or replays the
  committed receipt, still without a retry charge (CAL-V0-112). A plain release is never turned
  into a `HANDOFF` (CAL-V0-113); keep the reason and evidence when you retry.
- `SNAPSHOT_MOVED`: the store, head, intent tree or worktree moved during the read or before commit.
  A release or attestation candidate whose head no longer matches, and a criterion capture that
  wraps a refused read, repeat until the caller's input changes.
- `REDO_PENDING`: a writer is between receipt link-in and head rename. One that outlives the
  budget crashed there, and the next mutating command redoes its receipt.

Every other code is false, including `FENCED`, `BOOT_FENCED` and `SUPERVISOR_LOST` (the attempt
really lost; start a new one), `LIMIT_EXCEEDED`, `JOURNAL_SATURATED` and `UNSUPPORTED_FILESYSTEM`.
`attempt run` and `gate run` report false once their program has started, whatever the code,
because a retry would run it again. A supervised `run --role` reports false for any failure after its stage
dispatch has committed, because a repeat no longer selects that attempt and would leave its stage,
gates or `READY` step unfinished. Integration is the exception: its stage returns the attempt to
`READY_FOR_INTEGRATION`, and a `GRANT` never leaves it, so a repeat finishes it. A `--count`
batch reports false when any failed lane is not retryable. `health` and `pool cleanup` report false once their
preparation or cleanup receipt commits, and `pool sweep` once a fresh sweep commits its owner,
because a retry then replays that step without running the program or recording what it saw. `STALE`, `STORAGE_FAILED` and `HEAD_MOVED` are not result codes. The spec's
"V1-0780 retryable result amendment" lists every code with its reason.

## Observe holders and retry debt

Send `corvint-tasks attempt heartbeat --attempt ID --generation G --request-id FRESH_ID`
periodically while holding a work lease. Each fresh request records lastHeartbeatAt; replaying
one does not refresh it. Claims initialize the signal and readmission resets it. The fixed
observation TTL is 600 seconds; heartbeat does not renew the work lease. `attempt show` and
`queue status` report FRESH_HOLDER or STALE_HOLDER for a recorded signal with a live work lease,
LEASE_EXPIRED or TERMINAL for those states, CLOCK_BEFORE_HEARTBEAT for a backwards observation,
and NOT_OBSERVED for a legacy record without the signal. Stale means a missing recent signal;
it does not establish process death, physical quiescence or permission to release a holder.

`ticket show`, full `plan preview` and `queue status.retries` expose current acceptance-revision
charged debt, the current policy limit, remaining retry capacity and admission exhaustion.
Remaining zero still permits an initial claim or an eligible clean handoff. Reason buckets are
EXPIRED, RELEASED, FAILED and UNKNOWN and sum to charged debt; legacy debt remains UNKNOWN and
reasonHistory INCOMPLETE. Only charged readmission increments a bucket. New acceptance resets
debt, clean handoff preserves it and policy updates change the bound without erasing history.

## Upgrade the binary with live attempts

Live attempts do not need a drain to replace corvint-tasks build N with build N+1 when both builds
report the same `formats` from `corvint-tasks version` (CAL-V0-130..134, proposed). Attempts carry
no build identity: heartbeat, renew and release are fenced by generation, phase and expiry only.

1. Build N+1 somewhere other than the installed path. Compare `formats` from both `version`
   outputs. If the sets differ, drain to zero live attempts first, as before.
2. Stop a foreground dispatcher with SIGTERM. Its close leaves workers running and recorded in
   the ledger.
3. Install N+1 by writing it beside the installed path and renaming it over that path. Do not copy
   over the running file in place: detached attempt runners and supervised program owners keep
   running build N from their own executable image until they finish.
4. Restart the dispatcher. It adopts the recorded workers; their exit codes are not observed. Under
   the user service, skip step 2: the changed executable holds new launches until
   `service install --replace`, which preserves known workers (SERVICE500-002). The maintained
   tests exercise the foreground path only.
5. Run `receipt audit`. Live attempts keep heartbeating, renewing and releasing with their original
   attempt ID and generation.

A dispatcher ledger written by another dispatch-state version, or carrying a member this build
does not know (for example after rolling back to N), refuses `UNSUPPORTED_VERSION` and the
dispatcher does not open; restore the ledger that build wrote, or drain. A store `VERSION` another
build wrote refuses every lease verb with `UNSUPPORTED_VERSION`; reads never migrate. Supervised
Codex, Claude Code and OpenCode programs pin the host runtime in policy `runtimes`, not this
binary; upgrading that runtime still needs its own drain and policy update.
