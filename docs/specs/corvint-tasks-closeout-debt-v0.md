# Corvint Tasks closeout debt V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: TCD-V0-001..010 accepted (standing auto-accept 2026-10-11, V1-1615)
Delivery status: experimental

Authoritative input: the owner-requested native ticket V1-1615 ("tickets stay OPEN after their work
merges"). An audit on 2026-10-17 found 22 of 37 suspected tickets fully delivered but still OPEN.
Completion is a separate native write that no merge triggers, so a ticket stays OPEN when:
- the closeout waiter dies on a CI failure;
- the merge event is never delivered;
- `complete` is refused after a gate failure;
- the work landed under an umbrella PR or a GitHub issue number instead of the ticket ID.

This profile adds one read verb that lists those tickets as candidates. It never completes them.

## Agent digest
- Claim: `corvint-tasks ticket closeout-debt` lists OPEN tickets that merged commits cite, with each one's pending native closeout step, and writes nothing.
- Status: TCD-V0-001..010 accepted (standing auto-accept 2026-10-11, V1-1615); experimental; the verb, its help entry and fixture tests exist.
- Exists: a bounded `git log` scan under the store's sanitized Git environment; direct-id, issue-link and umbrella citations; closeout steps derived from the attempt inventory and verified gate evidence; deterministic rows; a typed 0/1/2 exit status; a byte-identical store and journal proof.
- Blocked on: nothing for V0. `COMPLETE_REFUSED` is NOT_PRODUCED: a refused `complete` writes no receipt (TCD-V0-006).
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## User and current state

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

## Non-goals

- Completing, claiming or submitting anything, or proposing a completion payload.
- HELD and DRAFT tickets, which are not closeout debt.
- Querying GitHub. Issue and PR state is not read, and only commit text in local Git is used.
- Recording `COMPLETE_REFUSED`. That would need the refusal to be journaled, which is a separate
  change.

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

## Rollout and rollback

The verb is additive and read-only. There is no store format, wire codec or policy change. To roll
back, revert the change. Nothing persisted depends on the verb.
