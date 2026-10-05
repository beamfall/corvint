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

## Payloads and shared worktrees

Payloads use sorted object keys, compact JSON, literal UTF-8 (not `\u00a7` for `§`), and sorted,
deduplicated set arrays such as `touchPaths`. Preserve ordered arrays such as gate argv and
acceptance criteria. For Python, serialize using `ensure_ascii=False, sort_keys=True,
separators=(',', ':')`, then add one LF; sort only fields documented as sets. Each mutation's
`--help` lists its closed payload keys. `ticket create --template` prints a canonical CREATE payload for
this queue (`.items[0].payload`) plus a `fields` table of types, enum values and null-able keys;
fill in `title`, `body` and `acceptanceCriteria`, then submit it with `--payload-stdin`.

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

Retain the returned `poolAllocation` alongside attempt ID and generation. Replays return the original
receipt-bound allocation, including after a retry has acquired a successor. Release, completion and
reap free the source scope but quarantine the environment. Reads never probe or clean environments.

Optional `memberConfig` supplies immutable regular Git `configRef:{revision,path,blob}` references and
`health`/`cleanup` commands. Each command has `argv`, `cwd:"REPOSITORY"`, declared `env` names and
`timeoutSeconds` from 1 to 300. Health failures are skipped and reported in occupancy with reason,
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
