# Corvint Tasks closeout debt V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: TCD-V0-001..010 accepted (standing auto-accept 2026-10-11, V1-1615); TCD-V0-151..153 accepted (standing auto-accept 2026-10-11, V1-2193); TCD-V0-154..157 accepted (standing auto-accept 2026-10-11, V1-2216); TCD-V0-158 accepted (standing auto-accept 2026-10-11, V1-1431); TCD-V0-159..164 accepted (standing auto-accept 2026-10-11, V1-2251, V1-2254); TCD-V0-165..166 accepted (standing auto-accept 2026-10-11, V1-2196); TCD-V0-167..168 and the TCD-V0-160/161 amendments accepted (standing auto-accept 2026-10-11, V1-2303); TCD-V0-169..173 accepted (standing auto-accept 2026-10-11, V1-2396, V1-2397, V1-2398)
Delivery status: experimental

Authoritative input: the owner-requested native ticket V1-1615 ("tickets stay OPEN after their work
merges"). An audit on 2026-10-17 found 22 of 37 suspected tickets fully delivered but still OPEN.
Completion is a separate native write that no merge triggers, so a ticket stays OPEN when:
- the closeout waiter dies on a CI failure;
- the merge event is never delivered;
- `complete` is refused after a gate failure;
- the work landed under an umbrella PR or a GitHub issue number instead of the ticket ID.

This profile adds one read verb that lists those tickets as candidates. It never completes them.

V1-2193 adds a second closeout-debt fix. A seal commit's own `full-gate` could not reuse the PASS of
a later commit on `main` that already contains it, so every merged seal re-ran the full gate
(TCD-V0-151..153). This amends the same-tree reuse rule of `CAL-V0-240` without a new CAL ID.

V1-2216 adds a third. The owner's direction is that a change "checked in the cloud" needs no local
gate run. A green GitHub Actions run on a protected intent-branch commit could not satisfy a gate,
so every merged ticket still re-ran its required gates locally (TCD-V0-154..157). V1-1431 closes a
gap in the reuse key: it named the declared environment variables but not their values
(TCD-V0-158). This amends `CAL-V0-241` (5) without a new CAL ID.

V1-2251 and V1-2254 add two reads for the orchestrated ticket sweep. About 40% of the tickets
dispatched to lanes in batches 1021c and 1021d were already delivered or in flight in another PR,
and delivered tickets stayed OPEN after their batch merged. `ticket delivery-screen` screens
candidate tickets before dispatch (TCD-V0-159..162). `ticket batch-delivered` lists what a merged
batch delivered, with the CI evidence a closer feeds to `gate run --ci-run` (TCD-V0-163..164).
GitHub facts come only from an operator's `gh --json` export file.

V1-2196 removes a rerun from failed-gate closeout. A `focused-docs` or `full-gate` failure showed only
`GATE_FAILED`, so the closer reran the whole gate to learn which check failed. The captured output
was already retained as an evidence blob, but no result named it (TCD-V0-165..166).

## Agent digest
- Claim: `corvint-tasks ticket closeout-debt` lists OPEN tickets that merged commits cite, with each one's pending native closeout step, and writes nothing.
- Status: TCD-V0-001..010 accepted (standing auto-accept 2026-10-11, V1-1615); TCD-V0-151..153 accepted (standing auto-accept 2026-10-11, V1-2193); TCD-V0-154..157 accepted (standing auto-accept 2026-10-11, V1-2216); TCD-V0-158 accepted (standing auto-accept 2026-10-11, V1-1431); TCD-V0-159..164 accepted (standing auto-accept 2026-10-11, V1-2251, V1-2254); TCD-V0-165..166 accepted (standing auto-accept 2026-10-11, V1-2196); TCD-V0-167..168 and the TCD-V0-160/161 amendments accepted (standing auto-accept 2026-10-11, V1-2303); TCD-V0-169..173 accepted (standing auto-accept 2026-10-11, V1-2396, V1-2397, V1-2398); experimental; the verb, its help entry and fixture tests exist.
- Exists: a bounded `git log` scan under the store's sanitized Git environment; direct-id, issue-link and umbrella citations; closeout steps derived from the attempt inventory and verified gate evidence; deterministic rows; a typed 0/1/2 exit status; a byte-identical store and journal proof; descendant gate reuse (V1-2193), where a reusable gate's PASS at a protected intent-branch commit covers an attempt whose candidate tree an ancestor commit carries (TCD-V0-151..153); CI-run gate evidence (V1-2216), where `gate run --ci-run FILE` records a policy-admitted green CI run at a protected intent-branch commit that descends from the candidate as the gate's PASS without executing it (TCD-V0-154..157); a reusable gate's reuse key covers its declared environment values (TCD-V0-158); `ticket delivery-screen` and `ticket batch-delivered` reads over `gh --json` exports (TCD-V0-159..164); a `gate run` result that reports the executed gate's state, exit and retained output path, with a bounded, secret-screened tail and failing line when it did not pass (TCD-V0-165..166); `ticket batch-close`, one COMPLETE_MANUAL per delivered OPEN ticket of a merged batch (TCD-V0-169); store-first COMPLETED and HELD verdicts, operator notes and a stdin export in `delivery-screen` (TCD-V0-170..171); multi-target `note set` and `hold` (TCD-V0-172..173).
- Blocked on: nothing for V0. `COMPLETE_REFUSED` is NOT_PRODUCED: a refused `complete` writes no receipt (TCD-V0-006).
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## User and current state

V1-2396, V1-2397 and V1-2398 turn the batch closeout into one command. After batch PR #1525 merged,
the orchestrator closed each delivered ticket with a hand-written `complete-manual` that cited the
merge commit and the green CI run. `ticket batch-close` writes those closes from the same exports as
`batch-delivered` (TCD-V0-169). `ticket delivery-screen` now reads the store's COMPLETED and HELD
state ahead of PR evidence, shows the latest operator note, and takes the PR export on stdin
(TCD-V0-170..171). `ticket note set` and `ticket hold` take several tickets in one call
(TCD-V0-172..173).

The user is an operator or orchestrator who has just merged work, or who is auditing the queue. Today
they find delivered-but-open tickets by hand: they search the merged history for ticket IDs and issue
numbers and then read each ticket's attempt. The simpler baseline is `git log --grep V1-` plus
`ticket show`. That misses issue-number citations and umbrella merges, and it does not say which
closeout step is missing.

## Requirements

- `TCD-V0-001`: `corvint-tasks ticket closeout-debt [--ref REF] [--since REV] [--limit N]` is a read
  verb listed in `ReadVerbs` with a help entry. It takes no positional argument.
  - `--ref` defaults to `refs/remotes/origin/<intentBranch>` when that ref resolves to a commit, and to
    `refs/heads/<intentBranch>` when it does not resolve (a Git failure is refused, `TCD-V0-002`).
    `intentBranch` comes from the queue.
  - `--since REV` excludes the commits reachable from REV.
  - `--limit N` caps the commit scan. N is 1..10000 and defaults to 1000.
  - An unknown flag, a positional argument, a missing value, an out-of-range limit, a revision that starts with
    `-` or holds a control byte, and a revision that does not resolve to a commit are refused with
    `MALFORMED`.
  - The output is always the JSON command-result envelope, so there is no `--json` flag.
- `TCD-V0-002`: The verb reads Git only through `git rev-parse --verify`, `git config --get
  remote.origin.url` and `git log`. Each runs in the primary worktree under the store's sanitized Git
  environment (`gitEnvironment`: no global or system config, no prompt, no optional locks, no lazy
  fetch, no replace objects) with `-c core.hooksPath=/dev/null -c core.fsmonitor=false -c credential.helper=`.
  - `git log` gets `--no-show-signature`, so no signature program runs.
  - `git log` gets `--encoding=UTF-8`, which overrides a configured `i18n.logOutputEncoding`. A UTF-16
    output would hold NUL bytes, which the `-z` record split would take as record ends and lose
    citations.
  - Revisions are passed after `--end-of-options`. Only the resolved commit IDs reach `git log`.
  - None of these commands checks out, reads or writes the index, merges, or runs a hook, filter,
    textconv or merge driver.
  - The `git log` output is read through a 64 MiB bound. A larger output stops the process and is
    refused with `LIMIT_EXCEEDED`.
  - A revision that `git rev-parse --verify --quiet` does not resolve (exit 1) is refused with
    `MALFORMED`. Any other Git failure, such as a corrupt object, a directory that is not a
    repository or Git not starting, is refused with `UNSUPPORTED`, the code the store's Git
    observations use.
  - The default-ref probe of `refs/remotes/origin/<intentBranch>` falls back to `refs/heads/` only when
    that ref does not resolve. A Git failure during the probe is refused with `UNSUPPORTED`.
- `TCD-V0-003`: The scan covers every commit reachable from the ref and not from `--since`, newest
  first, without `--first-parent`, so merge commits and their second-parent constituents both count.
  At most N commits are read. When more exist, `scanTruncated` is true and a warning names the limit.
  `commitsScanned` is the number of commits read.
- `TCD-V0-004`: A commit cites a ticket in one of three ways.
  - Direct ID: a word of the commit message (subject, body or trailer, for example `Refs V1-1234`)
    equals the ticket's local token or its full `ticket:<authority>:<queue>:<local>` ID, compared
    case-folded (`wire.FoldToken`).
    - A word is a maximal run of `[A-Za-z0-9._:-]`, with trailing `.` and `:` removed.
    - `V1-12345` and `CEM-V1-1234` therefore never match `V1-1234`.
  - Issue link: the ticket's title or body names an issue as `issue #N` or `<owner>/<repo>#N`, and
    the commit closes that issue with a GitHub closing keyword (`close`, `closes`, `closed`, `fix`,
    `fixes`, `fixed`, `resolve`, `resolves`, `resolved`) followed by `#N` or `<owner>/<repo>#N`.
    - The `<owner>/<repo>` form counts only when it equals the `remote.origin.url` slug,
      case-folded.
    - With no origin remote, only `issue #N` in the ticket and a bare `#N` in the commit count.
  - Umbrella: when one commit cites more than one ticket, every row it produces has the citation
    `umbrella`. Cited tickets of every status count toward this, so a commit citing one OPEN and one
    COMPLETED ticket gives the OPEN row `umbrella`. Only OPEN rows are then emitted (`TCD-V0-005`).
  - Otherwise a direct ID wins over an issue link. A row carries the linked issue number when an issue
    link contributed, and null otherwise.
- `TCD-V0-005`: Only tickets whose status is OPEN are candidates. An OPEN ticket with a live attempt
  (the brief's "ACTIVE") is included. COMPLETED, ARCHIVED, DRAFT and HELD tickets are never listed,
  however often they are cited. An OPEN ticket that no scanned commit cites is never listed.
- `TCD-V0-006`: Each row names the pending closeout step, read from the native store. The ticket's
  attempt is its live attempt or, when none is live, the attempt that reached its current phase last
  (highest `phaseSinceSeq`, then attempt ID). Steps are a closed set:
  - `NO_ATTEMPT`: no attempt exists.
  - `ATTEMPT_ENDED`: the attempt is FAILED, CANCELLED or COMPLETED while the ticket is still OPEN.
  - `CLAIMED_NOT_SUBMITTED`: the attempt is live with no candidate tree.
  - Otherwise the step is decided over the required gates (policy gates with `required` plus the
    ticket's `requiredGates`). Each gate is checked against its result in the attempt's gate set,
    read from `evidence/<digest>` and verified against the digest. The rules are the ones `complete`
    applies.
    - `GATE_FAILED`, with `gate`, when a current result for that gate is not PASSED.
    - `SUBMITTED_GATES_PENDING`, with `gate`, when the gate has no result, or a stale one (another
      tree, definition or binding).
    - `GATES_PASSED_COMPLETE_PENDING` when every required gate passed.
    - The first failing gate in gate-ID order wins, then the first pending one.
  - `NOT_OBSERVED`: the store has no journal, or a gate result cannot be read or verified. An
    unknown is never reported as a pass.
  - `COMPLETE_REFUSED` is not produced. A refused `complete` writes no receipt or attempt change,
    so the store holds nothing to read it from. A ticket in that state shows the gate or binding step
    that caused the refusal.
- `TCD-V0-007`: The result has one item with profile `taskman-closeout-debt/0`.
  - Top-level keys: `ref`, `refCommit`, `since` (commit or null), `scanLimit`, `commitsScanned`,
    `scanTruncated`, `origin` (slug or null), `candidatesTotal`, `candidatesReturned`,
    `candidatesTruncated`, `candidates`, `human`, and `mutationAuthority: false`.
  - Each candidate row is `{ticketId, local, label: "candidate", commit, subject, citation, issue,
    closeoutStep, gate}`.
  - Rows are sorted by ticket ID, then by commit ID.
  - At most 2000 rows are returned: the first 2000 in that order. `candidatesTotal` counts every
    candidate row and `candidatesTruncated` reports a larger set. The count is kept apart from the
    rows, and no more than 2000 rows are held at any point. A fan-out of many tickets that link one
    issue times many commits that close it therefore costs bounded memory.
  - The subject is the commit's first paragraph, cut on a UTF-8 boundary to at most 512 bytes.
  - The label is always `candidate`. The verb never says that work was delivered.
  - The result is marked untrusted, because subjects are commit text.
- `TCD-V0-008`: The verb is a pure read (product invariant 4). It takes no lock, and it never
  completes, claims or writes the store, the journal, the intent tree or the repository.
- `TCD-V0-009`: The exit status is 0 for an OK result with no candidates, 1 for an OK result with at
  least one candidate, and 2 for any refused or error result, whose codes are typed. This deviates on
  purpose from the CLI's "exit 0 iff OK" rule (`emit`), so a post-merge script can tell debt from
  failure.
- `TCD-V0-010`: `docs/DOGFOOD.md` tells operators to run the verb after merges and to finish each
  candidate through the native closeout path.

- `TCD-V0-151`: A `gate run` of a gate declared `reusable: true` with no qualifying same-tree result
  (`CAL-V0-240`) may reuse a PASSED, unreused result of the same gate, definition digest and
  environment that another attempt's record names at a different tree. This holds only when a
  commit carrying the attempt's candidate tree is an ancestor of, or is, a commit carrying the
  result's tree. Both commits must be read from the protected history: the queue's `intentBranch`
  (`refs/heads/<intentBranch>`) and its configured remote-tracking upstream, at most 100,000
  commits, with at most 16 ancestry checks. The inputs digest is compared at the result's tree. The
  rebound record names the source in `reusedFrom` as in `CAL-V0-241`, the lowest qualifying digest
  is chosen, and the gate does not execute. `complete` accepts that reused result as evidence after
  it rechecks the same ancestry. No new policy field exists: the existing `reusable` flag turns the
  rule on, so a gate that is not reusable is unchanged. The verb never fetches and never writes a
  ref.
- `TCD-V0-152`: The model refuses a reused result at a tree other than the candidate with `BLOCKED
  STALE_TREE` and no write when the store, rechecking under the lock, finds either of these. The
  result's tree is not a commit on the protected intent branch ("is not a commit on the protected
  intent branch"). Or no commit carrying the candidate tree is its ancestor ("is not an ancestor of
  the reused result's tree"). A result whose ancestry was not observed is refused the same way. The
  run then executes the gate as before.
- `TCD-V0-153`: The receipt audit (`GateReuseReceiptAudit`, `CAL-V0-243`) verifies each recorded
  descendant reuse against the protected history of the queue's intent branch in the primary
  checkout. When that branch no longer carries the ancestry, or the history cannot be read, the
  audit fails `JOURNAL_FORKED`. A same-tree reuse is audited as before.
- `TCD-V0-154`: The policy MAY carry an optional top-level `ciEvidence` object
  `{"gates":[{"gateId","repository","requiredChecks"}]}`. Each entry names a declared `COMMAND` gate
  exactly once, a GitHub `owner/name` repository and a sorted, duplicate-free, non-empty set of at
  most 64 check names (1..200 bytes each); at most 16 entries. Any other shape is refused
  `MALFORMED`. Only a gate named here accepts CI evidence, and it need not be `reusable`. An absent
  key admits none, so removing it through a store `policy update` is the rollback. An older binary
  refuses a policy carrying the key, so upgrade every binary that reads the store first.
- `TCD-V0-155`: `gate run --attempt A --generation G --gate ID --ci-run FILE` reads a
  `taskman-ci-run/0` record from FILE: `repository`, a decimal `runId`, the run's `headSha`, its
  `conclusion` (the GitHub Actions run conclusion set), the sorted set of `passedChecks` and
  `completedAt`. It records the gate PASSED without executing anything. The record is posted as
  `evidence/<sha256>`. The gate result's `reusedFrom` names that digest. Its candidate and executed
  trees are the head commit's tree, and its `inputsSha256` is the record's digest. Its
  `environmentSha256` is the digest of an empty environment array, its exit code is the gate's
  expected exit, and its start and end are `completedAt`. Its one evidence entry is labelled
  `ci-run`. The result item reports `reusedFrom` as `{"result": <digest>, "ciRunId": <runId>}`.
  `complete` accepts the result when the head commit is a protected intent-branch commit that
  carries its tree, and when a commit carrying the attempt's candidate tree is that commit or its
  ancestor (the history and bounds of `TCD-V0-151`). The verb never fetches, never queries GitHub
  and never writes a ref. The record's facts are the operator's claim. Only the head commit's
  place in local Git is re-observed.
- `TCD-V0-156`: The model refuses a CI-backed result without writing and without executing the
  gate when: the policy does not admit CI evidence for the gate, or the gate is not `COMMAND`
  (`BLOCKED UNSUPPORTED`); the run is of another repository (`BLOCKED GATE_STALE`); it did not
  conclude `success`, or a required check is not in `passedChecks` (`BLOCKED GATE_FAILED`); the head
  commit is not on the protected intent branch carrying its tree, or no commit carrying the candidate
  tree is its ancestor (`BLOCKED STALE_TREE`). It also refuses (`VALIDATION_FAILED MALFORMED`) a
  record that is not exactly the result `TCD-V0-155` derives from that CI run for the attempt. A
  head commit absent from the local repository is refused `STALE_TREE` before any write ("fetch it
  first").
- `TCD-V0-157`: The receipt audit (`GateReuseReceiptAudit`) verifies each recorded CI-backed result.
  The record must be posted as `evidence/<digest>`. The policy as last posted must admit it
  under the rules of `TCD-V0-156`. The result must equal the record `TCD-V0-155` derives. The head commit must still be
  a protected intent-branch commit carrying the result's tree, with a commit carrying the attempt's
  candidate tree as its ancestor. A rewound branch or an unreadable history fails `JOURNAL_FORKED`.
- `TCD-V0-158`: For a gate declared `reusable: true`, the `environmentSha256` that a run records
  and that reuse compares (`CAL-V0-241` (5), `TCD-V0-151`) also covers the SHA-256 of each set
  declared variable's value. A PASS recorded under another `PATH`, `GOTOOLCHAIN`, `TMPDIR` or other
  declared value is not reused, and the gate executes. A gate not declared reusable keeps the
  names-and-presence digest, so it records nothing derived from its values. This replaces the
  `CAL-V0-241` non-goal "keyed on environment values" in
  `docs/specs/corvint-tasks-agent-leases-v0.md`.

- `TCD-V0-159`: `corvint-tasks ticket delivery-screen <ticketId|local>... [--prs FILE] [--ref REF]
  [--since REV] [--limit N]` screens 1..256 tickets of the queue. It takes no lock, writes nothing
  and never queries a network service. `--ref`, `--since` and `--limit` resolve and bound the commit
  scan exactly as `TCD-V0-001`..`TCD-V0-003` do. `--prs FILE` is a bounded (32 MiB) JSON array that
  `gh pr list --state all --json number,state,title,body,labels` writes. An unknown ticket, a
  missing ticket argument or an undecodable export is refused `MALFORMED`.
- `TCD-V0-160`: A PR title or body line references a ticket as delivered when it names the ticket
  by ID, starts with `Refs`, `Closes`, `Fixes`, `Delivers` or `Delivered` (any case, after leading
  punctuation) or contains `DELIVERED` in capitals, and contains none of `not started`, `not
  delivered`, `skipped`, `residual`, `duplicate`, `blocked` or `already` (any case). Any other line
  naming the ticket only mentions it. A commit message line delivers or mentions under the
  `TCD-V0-167` rule. Delivered on any line wins over mentioned.
- `TCD-V0-161`: The result is one `taskman-delivery-screen/0` item, marked untrusted, with
  `mutationAuthority` false. Per ticket it carries the ticket's status, its verdict, at most 32
  referencing PRs (number, state, reference, batch) and at most 32 referencing ref commits. The
  verdict is `IN_FLIGHT` when an OPEN PR delivers it, else `DELIVERED` when a MERGED PR or a ref
  commit delivers it, else `TRUNCATED` under `TCD-V0-168`, else `CITED` when a ref commit mentions
  it, else `NOT_OBSERVED` when no `--prs` export was given, else `MENTIONED` when a PR mentions it,
  else `CLEAR`. A CLOSED unmerged PR never sets the verdict beyond
  `MENTIONED`. Without an export a ticket is never `CLEAR`, and a warning says so.
- `TCD-V0-162`: `ticket delivery-screen` exits 0 when every ticket is `CLEAR`, 1 when any is not,
  and 2 on a refusal or error.
- `TCD-V0-163`: `corvint-tasks ticket batch-delivered --pr N --prs FILE [--runs FILE]` takes no
  lock, writes nothing, completes nothing and never queries a network service. The `--prs` export
  must hold PR N in state `MERGED` with a `mergeCommit` (otherwise `MALFORMED` when absent and
  `PREREQUISITE_UNSATISFIED` when not merged). The lane PRs are the other exported PRs that the
  batch title or body names as `#N`. Each ticket a lane body, or else the batch body, delivers
  under `TCD-V0-160` is one `delivered` row: the ticket, its status, the lane PR (null for the batch
  body), the batch PR, the merge commit and `greenRuns`. `greenRuns` is the sorted ids of the
  `--runs` export records (`gh run list --json databaseId,headSha,conclusion`) that concluded
  `success` at the merge commit or the batch head, or `NOT_OBSERVED` without that export. The item
  is `taskman-batch-delivered/0`, marked untrusted, with `mutationAuthority` false.
- `TCD-V0-164`: A ticket the batch or a lane only mentions, and that none delivers, is listed under
  `mentioned`, never under `delivered`. A PR the batch does not name contributes nothing.
- `TCD-V0-165`: A `gate run` that executes its command gate and commits the result in that call
  adds `gateResult` to its result item: `gateId`, `state`, `outcomeClass`, `exitCode`, `signal` and
  `output` (`sha256`, `bytes`, and `path` `evidence/<sha256>`). When `state` is not `PASSED`,
  `tail` is the excerpts of the last 20 non-blank output lines, oldest first, and `failingLine` is
  the last of them that names an error, failure, mismatch or violation, else the last of them. Each
  excerpt has terminal escapes and control characters removed and is bounded to 240 bytes. It is
  replaced by the withheld marker when its cleaned or raw line matches the secret screen, or when it
  overlaps a secret-screen match over the raw text of the whole bounded output, so a private-key
  block whose BEGIN line lies before the tail withholds its body; a private-key BEGIN marker with no
  END marker after it withholds every line to the end of the output. One warning then names the
  gate, state, class, exit, failing line and output path. A `PASSED` result has null `tail` and
  `failingLine` and no warning. A reuse, a CI-run record, a replay or a refusal reports
  `gateResult: null`, because no gate ran in that call.
- `TCD-V0-166`: Every executed command gate result retains its full captured output, within the
  16 MiB gate output bound, as the evidence blob `evidence/<sha256>` that the result's `output`
  evidence entry names, whatever the gate definition's `evidence` labels say. `evidence: []` does
  not suppress the capture. Declaring `evidence: ["output"]` is descriptive only, and it changes the
  gate's definition digest, so earlier results of that gate become stale.
- `TCD-V0-167`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2303) A ref
  commit's message line that names a ticket delivers it only when the line contains none of the
  `TCD-V0-160` veto words and either starts with `Refs`, `Closes`, `Fixes`, `Delivers` or
  `Delivered` (any case, after leading punctuation; a `Refs:` trailer qualifies), contains
  `DELIVERED` in capitals, or is the message's first line and is scoped to the ticket: its first
  word, its conventional-commit scope (`type(ID):`) or its final parenthetical (`... (ID).`) names
  the ticket. Any other line naming the ticket only mentions it, so a commit that cites a ticket in
  prose makes it `CITED`, never `DELIVERED`. Non-goal: no judgement of the commit's content.
  Rollback: restore the every-line rule; prose mentions read as delivered again.
- `TCD-V0-168`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2303) When the
  commit scan stops at `--limit` (`scanTruncated` true), a ticket that no OPEN or MERGED PR or
  scanned commit delivers and whose `createdAt` is before the committer time of the oldest scanned
  commit plus one second (committer time has whole seconds) is `TRUNCATED`: history the scan did
  not read may cite it, so it is never reported `CITED`, `NOT_OBSERVED`, `MENTIONED` or `CLEAR`. A
  ticket created after that point keeps its verdict, and the truncation warning names the rule.
  Non-goal: the default limit stays 1000 and is not widened to the ticket's creation. Rollback:
  drop the branch; truncated scans report the window's verdicts again.
- `TCD-V0-169`: `corvint-tasks ticket batch-close --pr N --prs FILE --runs FILE [--repo OWNER/NAME]
  [--dry-run] [--issued-at TS] [--role ROLE]` reads the same exports as `batch-delivered` and
  derives the same `delivered` and `mentioned` sets (`TCD-V0-163..164`). It never queries a network
  service.
  - It cites one green run: a `success` record at the merge commit before one at the batch head, the
    highest `databaseId` first. The run URL is the record's `url` when it is an `https://` URL of at
    most 512 bytes without whitespace. Otherwise it is
    `https://github.com/<repo>/actions/runs/<databaseId>` when `--repo` is given. A missing
    `--runs` is `MALFORMED`. No green run, or no URL, is `PREREQUISITE_UNSATISFIED`, and nothing is
    written.
  - Each delivered ticket is planned under one lock-free snapshot. A COMPLETED ticket is
    `ALREADY_COMPLETED` and skipped. A HELD ticket (`TICKET_STATE`), any other non-OPEN ticket
    (`TICKET_STATE`) and a ticket with a live attempt (`ATTEMPT_LIVE`) are never closed and never
    reaped. Each row names the code and a recovery. A mentioned-only ticket is reported as
    `MENTIONED_ONLY` with a recovery and is never closed.
  - Every other delivered ticket gets one `COMPLETE_MANUAL` through the ordinary §5.2 writer at its
    planned revision. The reason is "Delivered by lane PR #L; merged in batch PR #N at <sha>; CI
    green: <url>", or "Delivered in batch PR #N; merged at <sha>; CI green: <url>" for the batch
    body. The evidence is the sorted SHA-256 digests of the merge commit string and the run URL
    string.
  - The request id is `batch-close-<N>-<local>`, or a 12-byte digest of the ticket id in place of
    the local token when that exceeds 64 bytes. `issuedAt` is `--issued-at`, else the export's
    `mergedAt`, else now, so a rerun over the same export replays.
  - One refused write stays on its row (`FAILED` with its code) and never stops the others.
  - The item is `taskman-batch-close/0`, marked untrusted, with one row per ticket: disposition,
    code, recovery, request id, expected revision, receipt, holds and attempt.
  - The result is OK (exit 0) only when every delivered ticket ends COMPLETED. Otherwise it is
    `REFUSED` (exit 1) with the union of row codes. `--dry-run` writes nothing and plans `CLOSE`
    rows; its result is OK.
- `TCD-V0-170`: `ticket delivery-screen` reads the store before PR evidence. A COMPLETED ticket's
  verdict is `COMPLETED`. A HELD ticket, or one with any hold, is `HELD`. Each row carries `holds`
  (`holdId` and `reason`) and `operatorNote` (the `ticket note show` value of the latest note). The
  human line adds ` hold=<id>` and ` note=<state>` when there is a note. Every other verdict and the
  exit rule of `TCD-V0-162` are unchanged: `COMPLETED` and `HELD` are not `CLEAR`. The verb still
  takes no lock and writes nothing.
- `TCD-V0-171`: `--prs -` reads the PR export from stdin, in `delivery-screen`, `batch-delivered`
  and `batch-close`, under the same size bound as a file (`LIMIT_EXCEEDED` beyond it). An
  undecodable or empty stdin is `MALFORMED`.
- `TCD-V0-172`: `ticket note set` takes several leading ticket ids or local tokens with one
  `--text` or `--text-stdin`. Each distinct ticket gets its own unconditional `NOTE_SET`, with
  request id `<ID>-<local>` (a digest suffix when over 64 bytes) and one shared `issuedAt`. An
  unresolvable target, or a refused write, is refused on its row and never blocks the others.
  `--supersedes`, `--expected-revision` and several tickets on `note clear` are usage errors. The
  item is `taskman-multi-target/0`, with per-target outcome, codes, request id, expected revision,
  receipt and resulting revision. It is OK only when every target applied; otherwise it is
  `REFUSED` with the union of codes. One ticket keeps the original result shape.
- `TCD-V0-173`: `ticket hold` takes `--target` more than once with one `holdId` and reason. Each
  distinct target gets its own `HOLD` at the revision hold reads for it, lock-free, just before its
  write, with the per-target request ids, shared `issuedAt`, row semantics and item that
  `TCD-V0-172` defines. A single `--target` without `--expected-revision` also reads its revision;
  with `--expected-revision` it behaves as before. `--expected-revision` with several targets, and
  several distinct `--target` values on any other mutation verb, are usage errors.

## Failure modes

- A ticket ID mentioned in passing ("follow-up to V1-1234") is listed. This is why the label is
  `candidate`. The operator confirms delivery before completing the ticket.
- Work merged without any ticket ID or linked issue number is not found. The verb reports what the
  history cites. It does not judge content.
- A scan cut at the limit hides older citations. `scanTruncated` and the warning say so. Use a larger
  `--limit` or a `--since` boundary.
- A stale `origin/<intentBranch>` (not fetched) misses recent merges. Fetch first, or pass `--ref`.
- An issue number that a different repository's ticket also links can be listed. Only this queue's
  tickets and this repository's origin slug are considered.

- A force-pushed or rewound intent branch can drop the commit that covered a reuse. The receipt
  audit then fails `JOURNAL_FORKED` (`TCD-V0-153`). Restore the branch to recover.
- An older `corvint-tasks` binary audits a descendant reuse as `JOURNAL_FORKED`, because it requires
  the source tree to equal the attempt's tree. Its `complete` refuses the result as `GATE_STALE`,
  which is safe. Upgrade every binary that audits the store before turning the rule on.
- An unfetched upstream misses a PASS on the remote branch. The run then executes the gate.
- A CI run record is the operator's claim about GitHub. A forged or mistyped record whose head
  commit really descends from the candidate on the protected branch is accepted. The audit
  re-observes only the ancestry, not GitHub. Admit a gate in `ciEvidence` only where the branch
  protection that put the commit on the intent branch already required those checks.
- A CI head commit that has not been fetched is refused `STALE_TREE` with "fetch it first". The
  verb does not fetch.
- A rewound intent branch drops a CI head commit. The audit then fails `JOURNAL_FORKED`
  (`TCD-V0-157`). Restore the branch, or complete those attempts with local gate runs, to recover.
- An older binary decodes a CI-backed result's `reusedFrom` as a same-tree reuse whose source is not
  a gate result. Its audit fails `JOURNAL_FORKED` and its `complete` refuses. Upgrade every binary
  first. The new `taskman-ci-run/0` profile also changes `version`'s live format set
  (`CAL-V0-130`).
- After `TCD-V0-158`, a reusable PASS recorded by an older binary has the names-only digest, so it
  is executed once more instead of being reused. That is safe. A per-session `TMPDIR` or `HOME`
  declared on a reusable gate prevents reuse across sessions. Declare only variables that change
  the result.

- A delivery screen is only as fresh as its export and the fetched ref. A PR opened after the
  export, or a merge not yet fetched, reads as `CLEAR`. Export and fetch immediately before
  dispatch.
- A PR body that writes `Refs <ID>` for a ticket it did not deliver makes the screen and
  `batch-delivered` treat it as delivered. The lane convention is `Refs` lines only for delivered
  tickets and a separate "Not delivered" list without `Refs`.
- A lane PR that a batch body does not name as `#N` is not a lane of that batch.
- A merge-queue run whose head is neither the merge commit nor the batch head is not listed in
  `greenRuns`.
- `batch-close` trusts the export as much as `batch-delivered` does. A wrong `Refs` line closes the
  wrong ticket. Run `--dry-run` first and reopen a wrong close with `ticket reopen`.
- A ticket that changes between the plan snapshot and its write is refused `STALE_TICKET` on its
  row. Rerun: completed tickets are skipped and the rest retry under the same request ids.
- A hold or multi-target write that reads a revision is last-writer-checked, not locked across
  targets. A concurrent write between the read and the write is refused `STALE_TICKET` on that
  row only.

- A failure printed more than 20 non-blank lines before the end of the output is outside `tail`,
  and `failingLine` then names a later line. The full output stays at `output.path`.
- A secret the shared screen does not recognize is quoted in `tail`. The screen and its limits are
  the same as for every other quoted gate or log excerpt.
- The tail is reported only in the executing call's result. `attempt show` and `ticket show` still
  list only result digests; read `evidence/<sha256>` from the store to inspect an older failure.

## Non-goals

- Completing, claiming or submitting anything, or proposing a completion payload.
- HELD and DRAFT tickets, which are not closeout debt.
- Querying GitHub. `closeout-debt` reads only commit text in local Git; the delivery reads take PR
  and run facts only from an operator export file (TCD-V0-159, TCD-V0-163).
- Recording `COMPLETE_REFUSED`. That would need the refusal to be journaled, which is a separate
  change.
- Querying GitHub or any network service for a CI run, or verifying its signature. The record is
  retained, decoded offline and bound to local Git ancestry (invariant 7).
- Dispatch claims or reservations that refuse a second dispatch (V1-2251 acceptance 2), and
  completing tickets from `batch-delivered`, which stays read-only; `batch-close` (TCD-V0-169) is
  the separate writer.
- Reaping an attempt, releasing a hold, or closing a HELD, DRAFT or mentioned-only ticket from
  `batch-close`. Those stay operator decisions.
- Capturing a CI run record automatically, or turning `ciEvidence` on for any store. Both are
  operator actions.
- Changing any store policy for V1-2196. TCD-V0-166 shows the output is already retained, so
  `focused-docs` and `full-gate` need no `evidence` change.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TCD-V0-001 | V1-1615 | `internal/tasks/cli` (verb, help) | `TestTCDV0001_ArgumentsAndRefs` (default ref, `--ref`, `--since`, bad flag, limit range, `-`-leading and unresolvable revision refused); `TestCALV0047_AllCommandHelpIsReadOnly` | none |
| TCD-V0-002 | V1-1615 | `internal/tasks/store` (`ScanCommits`) | `TestTCDV0002_ScanIsBoundedAndPlain` (argv carries no hook, fsmonitor or signature program and no `--first-parent`; revisions follow `--end-of-options`; output bound refused `LIMIT_EXCEEDED`; unresolvable revision refused `MALFORMED`); `TestTCDV0002_ScanForcesUTF8` (`--encoding=UTF-8`; whole messages under `i18n.logOutputEncoding=UTF-16`); `TestTCDV0002_RevisionRefusals` (non-repository refused `UNSUPPORTED`); `TestTCDV0002_GitFailureIsUnsupported` (corrupt ref commit through `--ref`, `--since` and the default origin probe: exit 2 `UNSUPPORTED`; unresolvable ref stays `MALFORMED`) | none |
| TCD-V0-003 | V1-1615 criterion (h) | `internal/tasks/store` (`ScanCommits`) | `TestTCDV0002_ScanIsBoundedAndPlain` and `TestTCDV0005_CandidatesThroughTheCLI` (second-parent constituent found); `TestTCDV0003_ClosedTicketsAndTruncation` (`--limit 1` truncates with a warning) | none |
| TCD-V0-004 | V1-1615 criteria (c), (d) | `internal/tasks/cli` (matching) | `TestTCDV0004_CitationMatching` (short, long, trailer, longer-token and prefixed-token negatives, closing keywords, slug match and mismatch); `TestTCDV0005_CandidatesThroughTheCLI` (umbrella, issue-link); `TestTCDV0003_ClosedTicketsAndTruncation` (a commit citing an OPEN and a COMPLETED ticket is umbrella for the OPEN row) | none |
| TCD-V0-005 | V1-1615 criteria (e), (f) | `internal/tasks/cli` | `TestTCDV0005_CandidatesThroughTheCLI` (uncited OPEN not listed); `TestTCDV0003_ClosedTicketsAndTruncation` (COMPLETED cited ticket not listed) | none |
| TCD-V0-006 | V1-1615 criteria (a), (b) | `internal/tasks/cli` (steps) | `TestTCDV0005_CandidatesThroughTheCLI` (GATE_FAILED with the gate name, NO_ATTEMPT, CLAIMED_NOT_SUBMITTED) | `COMPLETE_REFUSED`: NOT_PRODUCED by construction |
| TCD-V0-007 | V1-1615 | `internal/tasks/cli` (render) | `TestTCDV0005_CandidatesThroughTheCLI` (profile, sorted rows, label, untrusted, mutationAuthority); `TestTCDV0007_FanOutKeepsFirstRowsOnly` (40 tickets x 50 commits fan-out: full total counted, never more than the bound held, first rows kept in order at bounds 0, 1, 7, total-1, total, total+1) | none |
| TCD-V0-008 | V1-1615 criterion (g) | `internal/tasks/cli` | `TestTCDV0005_CandidatesThroughTheCLI` (`.taskman` and `.git/taskman` trees byte-identical before and after) | none |
| TCD-V0-009 | V1-1615 | `internal/tasks/cli` (`Run`) | `TestTCDV0005_CandidatesThroughTheCLI` (exit 1); `TestTCDV0003_ClosedTicketsAndTruncation` (exit 0); `TestTCDV0001_ArgumentsAndRefs` (exit 2 with MALFORMED); `TestTCDV0002_GitFailureIsUnsupported` (exit 2 with UNSUPPORTED) | none |
| TCD-V0-010 | V1-1615 | `docs/DOGFOOD.md` | the one-line instruction | none |
| TCD-V0-151 | V1-2193 | `internal/tasks/store` (`reusableRun`, `descendantSource`, `descendantResults`), `internal/tasks/transaction` (`planGateRun`, `gateVerdict`) | `TestTCDV0151_DescendantPassCoversAMergedSeal` (no execution, `reusedFrom` names the later result, `complete` accepts it) | real seal-closeout timing on the live store: NOT_PRODUCED |
| TCD-V0-152 | V1-2193 | `internal/tasks/transaction` (`planGateRun`), `internal/tasks/store` (`gateReuseFacts`) | `TestTCDV0152_DescendantReuseRefusals` (not on the protected branch, not an ancestor: `BLOCKED STALE_TREE`, no write, then the gate executes) | none |
| TCD-V0-153 | V1-2193 | `internal/tasks/transaction` (`GateReuseReceiptAudit`), `internal/tasks/store` (`descentOracle`) | `TestTCDV0151_DescendantPassCoversAMergedSeal` (receipt audit and binding fold pass; after rewinding the branch the fold fails `JOURNAL_FORKED`) | none |
| TCD-V0-154 | V1-2216 | `internal/tasks/intent` (`readCIEvidence`, `CIEvidenceGate`) | `TestTCDV0154_PolicyCIEvidence` (absent key admits none; declared entry decodes; undeclared or duplicate gate, bad repository, empty, unsorted or duplicate checks and an unknown field are refused) | none |
| TCD-V0-155 | V1-2216 criteria 1, 3 | `internal/tasks/snapshot` (`CIRun`), `internal/tasks/transaction` (`CIGateResult`, `planGateRun`), `internal/tasks/store` (`GateRunCI`, `ciRunFacts`, `commitDescent`), `internal/tasks/cli` (`gate run --ci-run`) | `TestTCDV0155_GreenCIRunClosesMergedTicketsWithoutLocalGates` (three merged tickets, one green CI run at the tip: zero `verify` executions, `reusedFrom` names the record and run id, all three `complete`) | closing real merged tickets on the live store: NOT_PRODUCED |
| TCD-V0-156 | V1-2216 | `internal/tasks/transaction` (`CIRunRefusal`, `ciRefusal`), `internal/tasks/store` (`GateRunCI`) | `TestTCDV0156_CIRunRefusals` (failed run, missing check, other repository, gate not admitted, off the protected branch, not a descendant, absent head commit: refused, no write, no execution); `TestTCDV0155_ModelRefusesAForgedCIResult` (a record claiming the candidate tree for a later head: not completed, no write) | none |
| TCD-V0-157 | V1-2216 criterion 2 | `internal/tasks/transaction` (`GateReuseReceiptAudit.ciReuse`), `internal/tasks/store` (`ciDescentOracle`) | `TestTCDV0155_GreenCIRunClosesMergedTicketsWithoutLocalGates` (receipt audit and binding fold pass; after rewinding the branch behind the CI head the fold fails with "is not a commit carrying tree") | none |
| TCD-V0-158 | V1-1431 | `internal/tasks/store` (`gateEnvironmentKey`) | `TestTCDV0158_ReuseKeyCoversDeclaredEnvironmentValues` (a PASS under another value executes; the same value reuses; dropping the value from the digest fails the test) | none |
| TCD-V0-159 | V1-2251 criterion 1 | `internal/tasks/cli` (`deliveryScreenCommand`, help) | `TestTCDV0159_DeliveryScreenVerdicts` (no ticket and an undecodable export refused `MALFORMED`; `.taskman` and `.git/taskman` byte-identical) | none |
| TCD-V0-160 | V1-2251 criteria 1, 3 | `internal/tasks/cli` (`closeoutIndex.references`) | `TestTCDV0159_DeliveryScreenVerdicts` (Refs line delivers; "skipped" and "already" lines and a "residual" commit line only mention) | none |
| TCD-V0-161 | V1-2251 criterion 1 | `internal/tasks/cli` (`deliveryScreenValue`) | `TestTCDV0159_DeliveryScreenVerdicts` (IN_FLIGHT, DELIVERED by PR and by commit, MENTIONED, CLEAR, NOT_OBSERVED without an export; profile, untrusted, mutationAuthority) | sweep measurement below 10% (V1-2251 criterion 4): NOT_RUN |
| TCD-V0-162 | V1-2251 | `internal/tasks/cli` (`emitDeliveryScreen`) | `TestTCDV0159_DeliveryScreenVerdicts` (exit 0 all CLEAR, 1 otherwise, 2 refused) | none |
| TCD-V0-163 | V1-2254 criterion 1 (listing) | `internal/tasks/cli` (`batchDeliveredCommand`) | `TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns` (lanes from `#N`, rows with lane, batch, merge commit, green runs at merge commit and head only; NOT_OBSERVED without runs; open batch `PREREQUISITE_UNSATISFIED`; absent batch `MALFORMED`) | listing a real merged batch: NOT_RUN |
| TCD-V0-164 | V1-2254 criterion 3 | `internal/tasks/cli` (`batchDeliveredCommand`) | `TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns` (residual ticket under `mentioned`; unnamed PR's ticket absent) | none |
| TCD-V0-165 | V1-2196 criteria 2, 3 | `internal/tasks/store` (`GateRun`, `GateExecution`), `internal/tasks/obligation` (`FailureTail`), `internal/tasks/cli` (`gateResultValue`) | `TestTCDV0165_GateRunReportsFailingTail` (FAILED exit 2, failing `make` line, four-line tail with the secret line withheld and absent from stdout, warning with path; PASSED null tail and no warning; replay null; receipt audit passes); `TestTCDV0165_FailureTail` (order, bound, blank lines, escapes, last actionable line, secret past the excerpt bound withheld); `TestTCDV0165_FailureTailKeyBlockBeforeTail` (a terminated and an unterminated private-key block whose BEGIN line lies before the 20-line tail emit no key material) | a failing live `focused-docs` run: NOT_RUN |
| TCD-V0-166 | V1-2196 criterion 1 | `internal/tasks/store` (`runGate`), `internal/tasks/transaction` (`LeaseGateRun` evidence post) | `TestTCDV0165_GateRunReportsFailingTail` (gate declared `evidence: []`; the full output, secret included, is at `output.path`); live store 2026-10-21: FAILED `focused-docs` record `b0248975` names output blob `c04e7d9e` (30377 bytes) ending in the `line-citations-check` failure | none |
| TCD-V0-167 | V1-2303 criteria 1, 3 | `internal/tasks/cli` (`closeoutIndex.commitReferences`, `subjectTickets`) | `TestTCDV0167_DeliveryScreenCitedAndTruncated` (a prose mention is CITED; a `Refs:` trailer, a leading ID, a `feat(ID):` scope and a final `(ID, ...)` deliver; fails on the pre-change every-line rule); `TestTCDV0159_DeliveryScreenVerdicts` (a vetoed commit line is CITED) | none |
| TCD-V0-168 | V1-2303 criteria 2, 3 | `internal/tasks/cli` (`deliveryScreenCommand` window start), `internal/tasks/store` (`CommitTime`) | `TestTCDV0167_DeliveryScreenCitedAndTruncated` (`--limit 1` makes undelivered tickets older than the window TRUNCATED; a backdated newest commit leaves the ticket CLEAR) | none |
| TCD-V0-169 | V1-2396 | `internal/tasks/cli` (`batchCloseCommand`, `planClose`, `greenRunURL`, help) | `TestTCDV0169_BatchCloseClosesDeliveredOpenTickets` (missing green run and run without URL `PREREQUISITE_UNSATISFIED`, missing `--runs` `MALFORMED`, `--dry-run` all byte-identical; OPEN ticket closed with the lane reason and sorted sha256 of merge sha and run URL, deterministic request id; held `TICKET_STATE`, live attempt `ATTEMPT_LIVE`, mentioned-only reported with recovery and left open; exit 1 `REFUSED`; rerun `ALREADY_COMPLETED` writes nothing; `--repo` composes the URL; exit 0 when all close) | closing a real merged batch on the live store: NOT_RUN |
| TCD-V0-170 | V1-2397 | `internal/tasks/cli` (`deliveryScreenValue`, verdict order) | `TestTCDV0170_DeliveryScreenStoreVerdictsAndNotes` (COMPLETED over an open PR's Refs, HELD with holdId and reason over a merged PR's Refs, CURRENT note text, NONE note; trees byte-identical) | none |
| TCD-V0-171 | V1-2397 | `internal/tasks/cli` (`readGitHubExportArg`) | `TestTCDV0171_DeliveryScreenReadsPRsFromStdin` (stdin and file give IN_FLIGHT; empty stdin refused) | none |
| TCD-V0-172 | V1-2398 | `internal/tasks/cli` (`noteWrite`, `multiTargetSubmit`) | `TestTCDV0172_NoteSetSeveralTickets` (two notes written, unknown target refused on its row, `REFUSED` overall, rerun replays, `--supersedes` and multi-target clear refused, single target keeps its item) | none |
| TCD-V0-173 | V1-2398 | `internal/tasks/cli` (`mutateCommand`, `readTargetRevision`, `multiTargetSubmit`) | `TestTCDV0173_HoldSeveralTicketsReadsRevisions` (holds at read revisions 1 and 2, COMPLETED target refused `TICKET_STATE` without blocking; single hold without `--expected-revision`; `--expected-revision` with several targets and several targets on release-hold refused) | none |

## Rollout and rollback

The verb is additive and read-only. There is no store format, wire codec or policy change. To roll
back, revert the change. Nothing persisted depends on the verb.

TCD-V0-165..166 add one result field and change no store format, wire codec, receipt or policy. To
roll back, revert the change; the output blobs stay retained as before.

TCD-V0-151..153 add no store format, wire codec or policy field. They take effect only for gates
the operator marks `reusable: true`. To roll back, set the gate back to `reusable: false` through a
store `policy update`. Then, if a descendant reuse was already recorded, revert the change only
after those attempts have completed, because an older binary's audit refuses the recorded reuse.

TCD-V0-154..157 add the optional policy key `ciEvidence` and the evidence profile `taskman-ci-run/0`.
To roll back, remove `ciEvidence` through a store `policy update`; no gate then accepts CI
evidence. Revert the change only after every attempt with a CI-backed result has completed,
because an older binary's audit refuses those results. TCD-V0-158 adds no format. Reverting it
restores the names-only key, and results recorded with values are then executed once more instead
of being reused.

TCD-V0-159..164 add two read verbs and two output-only profiles, with no store format, wire codec
or policy change. To roll back, revert the change.

TCD-V0-169..173 add one writer verb, two output-only profiles (`taskman-batch-close/0`,
`taskman-multi-target/0`) and row fields, with no store format, wire codec, receipt or policy
change. Every close is an ordinary `COMPLETE_MANUAL` receipt and every hold or note an ordinary
mutation. To roll back, revert the change; reopen any wrong close with `ticket reopen`.
