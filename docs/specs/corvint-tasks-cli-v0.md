# Corvint Tasks CLI companion V0

Owner: Russell Lewis
Date: 2026-10-30
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-3236, V1-3237, V1-3223)
Delivery status: experimental

Authoritative inputs: native tickets V1-3236 (the
[agent lease contract](corvint-tasks-agent-leases-v0.md) Requirements section is at the
256-obligation OCM limit, so new Tasks CLI requirements need a companion spec with its own
prefix), V1-3237 (duplicate checks for friction tickets) and V1-3223 (remote lane stores are
stale snapshots with no journal). This spec is that companion: new `corvint-tasks` CLI
requirements that would otherwise extend `CAL-V0` take the `TCL-V0` prefix here. `CAP-V0-060`
(the unknown-verb hint for `corvint-tasks`, V1-3212, in
[capability-catalog-v0.md](capability-catalog-v0.md)) is the first Tasks CLI requirement placed
outside `CAL-V0` for the same reason; it stays in the capability catalog spec and is
cross-referenced here, not moved. None of these requirements changes a stored record, receipt,
mutation payload codec or wire code.

## Agent digest
- Claim: Tasks ranks friction against open tickets with BM25, guards duplicate creates, and serves lane hosts without a journal from a verified snapshot.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-3236, V1-3237, V1-3223); experimental; TCL-V0-001..014 implemented with focused tests and a frozen duplicate eval.
- Exists: `internal/tasks/similar`, `internal/contextindex/lexrank.go`, the `ticket similar|digest|recur` and `queue snapshot` verbs, and the create guard in `internal/tasks/cli`.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and current state

Human-owned input: the 2026-10-30 lane round. Five lane reports listed 27 friction items, 13 of
which duplicated open tickets; each was found by hand with several `ticket search --text`
substring queries, and remote lanes (WSL, Unraid) could not search at all because their
`.taskman/` is the last committed snapshot and they have no journal. `ticket search --text` is a
case-folded substring filter: it misses reworded duplicates and ranks nothing. The context
compiler already ranks evidence with a deterministic, evidence-carrying BM25 over a camel- and
snake-aware tokenizer (`internal/contextindex/taskcontext.go` `lexicalHits`); this spec reuses
that tokenizer and that scoring function for tickets instead of adding a second retrieval method.

## Requirements

- `TCL-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237)
  `corvint-tasks ticket similar (--text TEXT | --payload-stdin) [--status S]... [--label L]...
  [--limit N] [--digest FILE]` MUST rank tickets against the query by BM25 (k1 1.2, b 0.3, the
  context compiler's idf `ln(1 + (N - n + 0.5)/(n + 0.5))`) over five fields of each candidate:
  ticket ID (gain 4, so text naming a ticket points at it), title (gain 2), body plus acceptance
  criteria (length-normalised), labels and effect touch-path terms (each tf 1, no length
  normalisation). Query terms are the context compiler's task terms
  (`contextindex.LexicalTaskTerms`): the query title's terms with gain 2, then at most the 24
  rarest (highest idf) other body terms with gain 1, so a long body does not rank long tickets by
  vocabulary overlap. `--text` is a title alone. Candidates default to status `OPEN` and limit 5
  (1..50). `--payload-stdin` reads a CREATE payload and queries its title as the title and its
  body, acceptance criteria, labels and touch paths as the body. The command MUST be a read: it
  writes neither the state nor the intent tree.
- `TCL-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237) Each
  candidate item MUST carry `ticketId`, `title`, `status`, `score`, `similarity`,
  `recurrences` and `evidence`: the matched query terms in descending idf order, each with its
  field, idf and term frequency, and a one-line reason in the context compiler's form
  (`N distinct query terms; rarest T (idf X); bm25 Y`). `similarity` is the matched share of
  the query's idf mass, in [0,1]. Order MUST be score descending, then ticket ID ascending, so
  equal inputs give byte-identical output. A candidate with no matched term MUST NOT be listed.
- `TCL-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237) The
  ranking MUST use `contextindex` exported functions for tokenisation and the BM25 term score,
  so the Tasks ranking and the context compiler cannot drift. Only `internal/tasks/similar/terms.go`
  may import `internal/contextindex`, as a per-file exception to decision 0397's import boundary.
- `TCL-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237) A
  `ticket create` whose request ID is not yet recorded and whose payload carries the label
  `agent-memory` (the friction-filing path) MUST rank `OPEN` tickets against its payload before
  submitting; other creates are not guarded. When any candidate reaches the duplicate threshold (similarity at
  at least 0.60 and at least three matched query terms) and is not named by a `--not-duplicate-of ID`
  flag, the create MUST be refused `DUPLICATE_ID` (retry class "request must change first")
  with nothing written, naming each such candidate with its title and evidence and both
  remedies: `ticket recur ID --context …` for a recurrence, or `--not-duplicate-of ID` to file it
  anyway. `--not-duplicate-of` is repeatable and MUST NOT enter the payload or the request
  digest; a candidate it names does not displace an unnamed one from the ranked window. A replay
  of an already recorded request ID MUST skip the guard.
- `TCL-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237)
  `corvint-tasks ticket digest [--label L]... [--status S]... [--since YYYY-MM-DD] [--format json|text]`
  MUST list the matching tickets (default label `agent-memory`, status `OPEN`; `--since` keeps
  those updated on or after the date, so a brief can carry the recent window when the full set is
  too long to paste) in ticket ID order, each with
  `ticketId`, `title` and `recurrences`. `--format text` MUST write plain lines instead of an
  envelope: a header `# corvint-ticket-digest/0 tree <sha256> tickets <count>` and then one
  `ID<TAB>title` line per ticket (a ` [recurrences N]` suffix when N > 0), titles with control
  characters replaced by spaces, so the text can be pasted into a lane brief. Served from a
  snapshot (TCL-V0-011), a `# ` comment line after the header MUST name its provenance; the
  parser skips `# ` lines after the header.
- `TCL-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237)
  `ticket similar --digest FILE` MUST rank against the digest's lines only (title field), read no
  repository, intent tree or journal, and work from any directory. It MUST refuse `MALFORMED` a
  file without the digest header. Its result warnings MUST name the digest's tree digest, so a
  lane can report a recurrence by ticket ID without a journal.
- `TCL-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237)
  `corvint-tasks ticket recur ID --context TEXT --request-id R [--issued-at T] [--role ROLE]`
  MUST submit one REFINE of the ticket's `body` that appends the line
  `Recurrence YYYY-MM-DD: TEXT` (the UTC date of `--issued-at`, or of now), against the ticket's
  current revision. It MUST refuse `TICKET_STATE` for a `COMPLETED` or `ARCHIVED` ticket, naming
  `ticket reopen`, and `MALFORMED` an empty or multi-line context. A recorded request ID replays
  only when it names this ticket and the body already holds this context's recurrence line;
  otherwise it is refused `REQUEST_ID_CONFLICT`. A recurrence is never a new ticket.
- `TCL-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237) A
  ticket's `recurrences` count, in `ticket similar` and `ticket digest`, MUST be the number of its
  body lines that begin `Recurrence YYYY-MM-DD: `.
- `TCL-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237) A
  frozen labelled eval MUST be kept in `internal/tasks/similar/testdata`: the 2026-10-30 lane
  friction items with their duplicate tickets (or none), plus historical pairs mined from ticket
  text that names an earlier duplicate (the store has no `supersedes` links; a pair whose text
  says the tickets are related but not duplicated is labelled new), each ranked against a frozen
  corpus of the store's ticket records as of the item's filing. The test MUST hold recall@3 and
  recall@5 (the default `ticket similar` view) at 100% for labelled duplicates and report the duplicate threshold's precision. A labelled item
  no lexical ranking can place in the top 3 MAY be retained as a known miss with a
  `knownMissRank`: the test then requires a labelled duplicate at or above that rank, fails if
  it rises into the top 3 (so it is counted again), and reports the miss count. Ranking one
  query against the full store (3,257 tickets) MUST take under 300 ms on a developer host,
  building the index included; on a shared hosted CI runner (`CI=true`) the test bounds it at
  2 s, an order-of-magnitude regression guard.
- `TCL-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3223)
  `corvint-tasks queue snapshot [--out FILE]` MUST write one JSON object whose `profile` is
  `taskman-intent-snapshot/0`: the intent tree's files (path, sha256 and content), its tree
  digest, the source host name and primary worktree, the journal head (`lastSeq`, receipt digest)
  when a journal exists or `null`, and `producedAt`. It MUST read under the inventory-read
  protocol (an absent journal, never an incomplete one) and write nothing but `--out` (or stdout).
  A reader MUST refuse a newer `taskman-intent-snapshot/N` as `UNSUPPORTED_VERSION` (CAL-V0-131).
- `TCL-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3223) When the
  environment variable `CORVINT_TASKS_SNAPSHOT` names a snapshot file, the inventory reads
  `ticket search`, `ticket show`, `ticket blockers` (TCL-V0-869), `ticket export`, `ticket similar`,
  `ticket digest`, `roadmap` and `queue status` MUST be served from it without resolving a repository or journal, with every
  journal fact `NOT_OBSERVED` as for an absent journal (`ticket list` reads attempt history, so it
  is refused as in TCL-V0-012; `ticket search --status S` lists the same tickets): every
  file's sha256 and the tree digest are verified first (a mismatch is `MALFORMED`), and each
  result carries a warning naming the source host, primary worktree, `producedAt` and journal
  sequence. A ticket missing from the snapshot MUST be reported as not in that snapshot, naming
  the owning store.
- `TCL-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3223) Under
  `CORVINT_TASKS_SNAPSHOT` every other verb (every mutation, lease, gate, dispatch and journal
  verb) MUST be refused `UNSUPPORTED` before any repository read or write, naming the owning
  store's host and primary worktree and the command to run there. `help` and `version` stay
  available.
- `TCL-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3237, V1-3223)
  Each new verb and flag MUST ship `--help` (usage, required inputs, an example), its command-tree
  entry and its `docs/CAPABILITIES.md` row (CAP-V0-040..050).
- `TCL-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3236) A new
  `corvint-tasks` CLI requirement that would extend `CAL-V0` MUST take the next `TCL-V0` number in
  this spec; requirements already numbered elsewhere (for example `CAP-V0-060`) stay where they
  are and are cross-referenced.
- `TCL-V0-860`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3714) A
  `ticket` mutation verb given a positional argument or an unknown flag MUST refuse `MALFORMED`
  with that code in the top-level `codes`; a positional argument's refusal names
  `--target TICKET` and `--expected-revision N|current`. `--expected-revision current` on a
  single-target mutation other than CREATE reads the target's revision from the intent store
  and submits it, so the store's revision check still refuses `STALE_TICKET` when the ticket
  changes between that read and the write; with several `--target` values it is refused as a
  number is. A retry of a recorded request with the same `--request-id` and `--issued-at`
  submits the revision the recorded request was digested with, not the advanced live revision,
  so it replays the recorded result instead of being refused `REQUEST_ID_CONFLICT`.
- `TCL-V0-861`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3714) The
  usage of every single-target mutation other than CREATE and HOLD MUST mark `--target` and
  `--expected-revision` as required, so the derived help example names both and, with its
  placeholders filled, runs.
- `TCL-V0-862`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3714)
  `ticket list` and `ticket search` `--fields` MUST accept every key a `ticket show` item
  carries, `record.` keys included: when a requested key is not in the list item, each page
  item is the `ticket show` item of that ticket, projected to the requested keys. Without such a
  key the items stay the CAL-V0-173 list items (no `record` member).
- `TCL-V0-863`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3714)
  `ticket search --query` MUST refuse `MALFORMED` naming `--text T` as the text filter.
- `TCL-V0-864`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3714)
  `roadmap --status S[,S...]` MUST keep only rows whose status is listed, with the `ticket list`
  status grammar (a closed status set, each named once), applied from the ticket summary so a
  closed ticket is never decoded; paging, facets and counts apply to the kept rows.
- `TCL-V0-865`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3534) A
  `ticket show` item MUST carry `intentChecksFailed`, the codes of its blockers in blocker order,
  each once (empty when `intentChecks` is `PASSED`), so every failing intent check is named
  beside the verdict. Intent checks are admission checks: a COMPLETED or ARCHIVED ticket fails
  them by its status alone, and its `TICKET_STATE` blocker detail MUST say so, that the
  completion itself is not in question and that `reopen` is only for rework.
- `TCL-V0-866`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3597) `claim`
  MUST refuse `--whole-repository` at parse time, `MALFORMED` before any store read, naming
  `widen --attempt A --generation G --whole-repository` as the verb that takes it and saying that
  a claim without `--scope` of a ticket that declares no scope already claims
  `WHOLE_REPOSITORY`. The claim usage never lists the flag.
- `TCL-V0-867`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3715) Where
  the journal at `<git-common-dir>/taskman` exists, `ticket list`, `ticket search` and their
  facets MUST judge each ticket with the journal's reservation set, as `ticket show` does, so
  attempt liveness is observed and no `no journal available to this reader` unknown or
  CAL-V0-173 store-wide warning appears. When that reservation set cannot be read, the
  store-wide ATTEMPT_LIVE warning MUST name the read failure rather than a missing journal.
  `roadmap` rows MUST be judged with the same reservation set as its facets, so a row and the
  facet it is counted in agree, and a `--count` read of `ticket list`, `ticket search` or
  `roadmap` MUST carry the same store-wide ATTEMPT_LIVE warning when that set cannot be read.
- `TCL-V0-868`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3794) A ticket
  view MUST list a non-`QUALIFIED` effects coverage under policy `serialFallback: BLOCK` as a
  `COVERAGE_UNKNOWN` entry in a separate `advisories` array, never in `blockers`, because no queue
  policy makes coverage a claim prerequisite: claim admission and `plan preview` already skip it
  and reserve `WHOLE_REPOSITORY` (CAL-V0-014, TCD-V0-174). `intentChecks`, `eligibility` and the
  `roadmap` row are therefore derived from the other checks alone, so such a ticket reads `PASSED`
  and `UNKNOWN` rather than `FAILED` and `BLOCKED`. The `advisories` member is present only when
  non-empty. An OPEN ticket with no blocker and that advisory keeps `nextAction: "set-effects"`
  and `alternativeActions: ["claim --scope-from-diff"]` (TCD-V0-182), so the `byNextAction` facet
  is unchanged. Under `serialFallback: WHOLE_REPOSITORY` the view carries no coverage entry at all.
  A policy that makes coverage a claim prerequisite would render it as a blocker again; none
  exists in this contract.
- `TCL-V0-869`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3173)
  `corvint-tasks queue snapshot --worktree DIR [--out FILE]` MUST read the intent tree at
  `DIR/.taskman` itself, without resolving a repository or reading any journal, so a linked or
  pruned worktree's store can be validated instead of the primary checkout's: every record is
  decoded and checked as a store load is (a malformed record is refused, nothing is written), the
  snapshot's `primaryWorktree` is the absolute `DIR` and its `journal` is `null`. Served with
  `CORVINT_TASKS_SNAPSHOT` (TCL-V0-011), `ticket show`, `ticket blockers` (also served),
  `queue status` and `roadmap` then report that tree's blockers and dependency closure, such as
  `DEPENDENCY_MISSING` for a dependency the tree no longer holds. `receipt audit` audits journal
  receipts, which a worktree tree does not own, so it stays refused under the snapshot and is run
  against the owning journal. A `DIR` without a `.taskman` directory MUST be refused
  `UNINITIALIZED`. The `DIR` tree is digested again after the snapshot is built, as an inventory
  read is: a changed tree is read afresh, and after four changed reads the command is refused
  `SNAPSHOT_MOVED` with nothing written.
- `TCL-V0-870`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3552) A
  mutation the store refused (any `taskman-outcome/0` outcome other than `COMPLETED`) MUST put at
  least one closed §11 code in the command result's top-level `codes`, and a `--batch` entry MUST
  do the same in its own `codes`. The outcome's recorded codes are used when it has any. An
  outcome recorded without codes gets one derived code: `REVISION_CONFLICT` → `STALE_TICKET`,
  `VALIDATION_FAILED` and `UNAUTHORIZED` → `MALFORMED` (as `wire.CodeOf` and the lease writer
  already report an uncoded refusal), `BLOCKED` → `TICKET_STATE`, `UNSUPPORTED` →
  `UNSUPPORTED`, `CAPACITY_EXHAUSTED` → `RESOURCE_EXHAUSTED`, `UNCERTAIN_EFFECT` →
  `UNCERTAIN_EFFECT`, `REQUEST_ID_CONFLICT` → `REQUEST_ID_CONFLICT`, `STORAGE_FAILED` →
  `UNSUPPORTED_FILESYSTEM`. The item's `outcome` field still names the store outcome. The
  derivation happens only in the CLI envelope: receipts and recorded outcome codes are unchanged.
- `TCL-V0-990`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3517) The
  duplicate threshold of `TCL-V0-004` MUST also require at least one matched query term in the
  candidate's title or ID field (a title anchor). A candidate whose matches all come from its
  body, labels or paths, such as a long ticket that holds the query's vocabulary in unrelated
  prose or one shared rare term among common words, MUST NOT reach a duplicate verdict, however
  much of the query's idf mass it covers; `ticket similar` still ranks and lists it. An absolute
  idf floor is not used, because idf scales with the store's size and a small store has no term
  above it. The frozen evaluation of `TCL-V0-009` MUST keep its recall, refusal count and
  precision.
- `TCL-V0-991`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3766) A
  single mutation replayed by its recorded `--request-id` MUST report `replayed: true`, an empty
  `receipt` (it wrote none, which existing callers and tests read as the no-write marker), and an
  added `originalReceipt` member naming (`<seq as 12 digits>.json`) the receipt that committed
  the original, so a caller that confirms a write by its receipt can find it on replay, whether
  the store decides the replay on the complete audit or on the writer checkpoint. A fresh
  commit's envelope is unchanged and has no `originalReceipt`. The mutation help (`replay`
  member of `--help --verbose`) MUST document this envelope. `--batch` entries are out of scope.
- `TCL-V0-859`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3736) An
  unknown first word MUST put the closed code `MALFORMED` in the result's `codes`, and its hint
  MUST also name the first verb words within edit distance 2 of it, so `tikcet` suggests
  `ticket`. A read refusal for a ticket or escalation that does not exist in the queue
  (`ticket show`, `ticket note show`, `ticket note history`, `critical-path`,
  `ticket escalation show`, `ticket escalation history`) MUST carry `MALFORMED`, as the review and
  obligation reads already do. No new code is added.
- `TCL-V0-820`: (experimental under the standing lane auto-accept (2026-11-01); V1-3559)
  `receipt audit --through SEQ` MUST walk receipts 1..SEQ only, with the same per-receipt checks
  as the complete walk (the CAL-V0-265 prefix walk, bounded at SEQ), and its item MUST add
  `throughSeq` naming that bound; `headSeq` and `lastReceiptSha256` then name receipt SEQ.
  Receipts after SEQ are not validated, so a store whose later receipts are inconsistent still
  reports `structuralConsistency: CONSISTENT` for the bound, and `projectionAgreement` stays
  `NOT_OBSERVED` because the current projection is not compared with the state at SEQ. The
  bounded read decodes `head.json` but MUST NOT require the head receipt to verify, so a corrupt
  receipt after SEQ, the head receipt included, never refuses the bound. Elided liveness receipts
  (CAL-V0-300) within the bound MUST be restored as the complete walk restores them; only a walk
  that reaches the head applies the check that each elided receipt is superseded. A SEQ of
  0, above the head, or not a decimal sequence MUST be refused `MALFORMED` before any walk;
  `--through` with `--witness-receipts` MUST be refused `MALFORMED`; a store with a retired
  prefix or staged names is refused as the prefix walk refuses it. Without `--through` the
  command is the unchanged complete audit.
- `TCL-V0-821`: (experimental under the standing lane auto-accept (2026-11-01); V1-0896; refines
  CAL-V0-124) `policy update --dry-run --expected-policy-version N --file PATH` MUST decode the
  file as `policy.json` and check that its `policyVersion` is N+1, then return one item with
  `dryRun: true`, `newPolicySha256` and the CAL-V0-124 `handoffFences` preview, and write
  nothing: the journal, receipts, head, checkpoints, the intent `policy.json` and the derived
  closed-ticket index stay byte-identical, even when another writer advances the head during the
  preview. `--request-id` and `--role` are accepted and unused. A file that does not decode
  or carries another version is refused with the decoder's code or `MALFORMED`. Over a pending
  receipt `handoffFences` is `null` with a `NOT_OBSERVED` warning, as for a committed update. The
  preview is advisory: the later write still takes its own version check under the writer lock.
- `TCL-V0-822`: (experimental under the standing lane auto-accept (2026-11-01); V1-3925; refines
  TCL-V0-860) A single-target `ticket hold` given no `--expected-revision` reads the live revision
  as before, and when it is given `--issued-at` and the request index records its `--request-id`,
  it MUST submit the earlier revision whose rebuilt envelope digest equals the recorded digest,
  so a retry of an applied hold replays its recorded result instead of refusing
  `REQUEST_ID_CONFLICT`. A changed payload under the same request ID still conflicts.
- `TCL-V0-735`: (experimental under the standing lane auto-accept (2026-11-01); V1-3958; refines
  CAL-V0-250 and ATR-V0-275) A mutation given no `--issued-at` that the store refuses
  `REQUEST_ID_CONFLICT` MUST be resubmitted once at the recorded issuedAt that the CAL-V0-250 /
  ATR-V0-275 reconstruction proves (the rebuilt envelope digest equals the recorded digest), so a
  plain retry with the same request ID and payload replays the recorded result (`replayed: true`,
  the original ticket) and warns with the recovered issuedAt. Every mutate verb shares this path.
  A retry whose other bytes differ still refuses `REQUEST_ID_CONFLICT` with the CAL-V0-250
  explanation, and nothing is written.
- `TCL-V0-736`: (experimental under the standing lane auto-accept (2026-11-01); V1-4148)
  `release readiness` MUST take its readiness inputs, member counts, milestone drift and member
  lifecycles from one store read, so a write that lands during the read retries the whole
  assessment instead of mixing an earlier intent load with a later lifecycle read. Only when that
  read fails does it fall back to a plain intent load, counting every member lifecycle `UNKNOWN`
  with a `lifecycle NOT_OBSERVED` warning as before.
- `TCL-V0-737`: (experimental under the standing lane auto-accept (2026-11-01); V1-3606)
  The shared flag parser MUST accept declared boolean flags: a declared boolean is a bare
  `--name` in any argument position, a value on it (`--name=v`) is refused `MALFORMED` with
  "takes no value", and an undeclared flag is still refused. `ticket delivery-screen` MUST parse
  `--lane-refs` and `--json` through it rather than stripping them by hand, and the flags its usage
  names MUST equal the booleans and value flags its parser declares.
- `TCL-V0-738`: (experimental under the standing lane auto-accept (2026-11-01); V1-3485; ships
  TCX-V0-988) `corvint-tasks doctor --rebuild-closed-index`, given alone, MUST validate the store
  with the inventory read (a journaled store, or a journal-free copy whose tracked intent tree
  loads) and then write the closed-ticket index with the writer the CLI runs after a committed
  mutation, reporting `path`, `journal` (`PRESENT` or `ABSENT`) and `indexSha256`. The index file
  is its only write. A repository with no store, or a tree that does not load, MUST refuse with
  the read's own code and write nothing; a snapshot bundle refuses `UNSUPPORTED`; the flag with
  any other argument refuses `MALFORMED`.
- `TCL-V0-739`: (experimental under the standing lane auto-accept (2026-11-01); V1-3272; extends
  TCL-V0-004) The record's existing typed `supersedes` link (written by a create or refine payload
  and by `ticket split`) is the duplicate-of link. `ticket search --supersedes TICKET` MUST match
  exactly the tickets whose `supersedes` names TICKET, given as a full or local ID in the queue,
  in conjunction with the other filters. The `TCL-V0-004` guard MUST treat a payload whose
  `supersedes` resolves to a candidate as naming that candidate, so the create records the
  duplicate as the typed link; a link to any other ticket names nothing, and the refusal names the
  link as a remedy. No record, payload or wire format changes.
- `TCL-V0-741`: (experimental under the standing lane auto-accept (2026-11-01); V1-3353, V1-3817)
  `ticket show <ticket> --format text` (or `--format=text`) MUST print the one ticket as plain
  text: a header line marking the content untrusted, the ticket ID and title, status, priority,
  kind and revision, milestone when set, labels, touch paths, coverage, requirement refs when
  set, then the body and the acceptance criteria numbered from 1, each verbatim with no JSON
  escaping and with control characters other than newline and tab dropped. `--format json`, the
  default, is the unchanged envelope. A refusal stays the JSON envelope, and `--format text` with
  `--summary` or `--fields`, or a format other than `json` or `text`, refuses `MALFORMED`. A lane
  ticket packet is this output.
- `TCL-V0-742`: (experimental under the standing lane auto-accept (2026-11-01); V1-3931)
  `queue parity --against FILE` MUST compare the store this host reads (its worktree store, or
  the `CORVINT_TASKS_SNAPSHOT` bundle) with the authoritative `queue snapshot` FILE by intent file
  sha256 and return one item naming `inSync`, the host's tree digest and source, the authority's
  path, host, primary worktree, tree digest, `producedAt`, age and journal sequence, and the ticket
  IDs `missing` (in the authority only), `stale` (different content) and `extra` (on the host
  only), plus differing non-ticket files, each with a whole count and a list capped at 200. A host
  whose tree digest differs MUST be refused `STALE_TICKET` (the closed wire code set, SPEC section
  11, has no store-level code) with a warning naming both trees, the counts and the
  `CORVINT_TASKS_SNAPSHOT` remedy; an unreadable or tampered FILE refuses as `queue snapshot`
  reads do, and a missing or empty `--against` refuses `MALFORMED`. It is a read verb and runs
  under `CORVINT_TASKS_SNAPSHOT`. Every snapshot read's warning MUST also name the snapshot's age
  and tree digest. `script/agent-preflight.sh` MUST print a `tasks-store` row from this verb
  against `CORVINT_TASKS_AUTHORITY` or the newest staged `queue-snapshot.json`, and never fail on
  it. Non-goals: syncing or repairing the host store, which stays the orchestrator's export.
- `TCL-V0-743`: (experimental under the standing lane auto-accept (2026-11-01); V1-3696, V1-3622)
  A `ticket create` whose request ID is not yet recorded and whose payload declares
  `effects.touchPaths` MUST check each path at `origin/HEAD`, else `origin/main`, in this clone
  without fetching (a directory key ending in `/` is present when anything lies under it). When
  any path is absent there, the create MUST be refused `MALFORMED` with nothing written, naming
  the base, its commit and each absent path, unless `--creates-paths` declares that the ticket
  creates them or the body records where the code lives with a line `Source ref: BRANCH@SHA`
  (SHA a full 40-hex commit). A recorded commit present in this clone MUST hold every absent path
  or the create is refused naming those still absent; one not in this clone is accepted with a
  `NOT_OBSERVED` warning. An accepted create whose paths are absent from the base carries a
  warning naming the source ref. A clone with neither ref, or a git observation that fails,
  MUST NOT refuse: it warns that subject presence is `NOT_OBSERVED`. `--creates-paths` MUST NOT
  enter the payload or the request digest and is refused by every other verb. The body line is the
  record: `script/lane-dispatch-check.sh` names it on an `ABSENT` finding (ASD-V0-645), and the
  dispatch-time dependency decision is to order such a ticket after its branch lands rather than
  dispatch it against a base that lacks the code. Non-goals: symbol-level presence (tickets declare
  paths, not symbols) and a typed source-ref record field.
- `TCL-V0-744`: (experimental under the standing lane auto-accept (2026-11-01); V1-3856)
  `ticket orphan-lanes [--ref REF] [--batch GLOB] [--limit N]` MUST, without a lock or a write,
  list every commit on a local branch or detached worktree head that the screened ref (as
  `ticket closeout-debt` resolves it) does not contain (the lane heads of `TLF-V0-887`), that no
  local branch matching GLOB (default `claude/batch-*`, matched on the name under `refs/heads/`)
  contains beyond the ref, and that delivers (`TCD-V0-167`) an `OPEN` ticket, as one
  `taskman-orphan-lanes/0` item with `ref`, `refCommit`, `batchGlob`, `batchRefs` (ref, commit,
  `scanTruncated`), `laneHeads` (ref, commit, `orphanCount`, `scanTruncated`), `orphanCount` and
  `orphans` (ref, commit, `ticketId`, `local`, subject; sorted by ticket, ref, commit; at most 500
  listed). Each head is read at most `--limit` commits beyond the ref; a cut scan sets its
  `scanTruncated` and adds a warning that the list is incomplete. No branch matching GLOB MUST add a
  warning that in-flight batches are `NOT_OBSERVED`. A ticket ID argument, a bad `--limit` or an
  invalid glob refuses `MALFORMED`. `script/lane-dispatch-check.sh --orphans FILE` (ASD-V0-645)
  reads this result and reports `ORPHAN` for each assigned ticket it lists. Non-goals: reading
  another host's branches; the orchestrator pushes batch branches to each lane host clone before
  dispatch (`docs/LANE-ORCHESTRATION.md`).

- `TCL-V0-815`: (experimental under the standing lane auto-accept (2026-11-01); V1-4184; refines
  CAL-V0-131) `tools/tasks-profiles-check` (`make tasks-profiles-check`, a `DOC_GATE_TARGETS`
  entry, so CI doc gates and `make batch-precheck` run it) MUST exit 1, naming the profile and its
  file, when a `taskman-NAME/N` literal in the non-test sources under `internal/tasks` is not in
  exactly one of `cli.LiveFormats`, `outputOnlyProfiles` and `adoptOnlyProfiles`. It reads
  `LiveFormats` from source, resolving an identifier or package selector to the literal its
  constant or variable is declared with, and leaves an unresolvable entry and the stale-entry check
  to `TestCALV0131_LiveFormatsCoverEveryDecodedProfile`. It runs in about a second, so the
  `46e9ee1fd` shape (an output-only `taskman-closed-index-rebuild/0` constant no list registers)
  fails before the long `internal/tasks/cli` package runs.

## Non-goals

- Embeddings, learned similarity or any network service (invariant 7).
- Merging or closing duplicates automatically: the guard refuses and names the remedies; the
  writer chooses.
- A counter field in the ticket record: recurrences are body lines, so no codec or receipt
  changes.
- Writing to a snapshot or syncing a snapshot back: snapshot hosts stay read-only.

## Failure modes

- A reworded duplicate scores under the threshold: the frozen eval holds recall@3 at 100% and
  `ticket similar` still lists it; the guard is a check, not the only search.
- A distinct ticket trips the guard: `--not-duplicate-of ID` files it, and the refusal shows the
  evidence that matched so the writer can judge.
- The ranking drifts from the context compiler: both call `contextindex.LexicalTaskTerms` and
  `contextindex.LexicalTermScore`; a test pins the score against the compiler's formula.
- A replayed create is refused by a guard that now sees its own ticket: a recorded request ID
  skips the guard.
- A crash left a redo pending: the guard's read refuses `REDO_PENDING`, so the guard is skipped
  and the create proceeds to the store, which recovers the redo first (retained limit: that one
  create is unguarded).
- A stale snapshot is read as current: every result names its `producedAt` and journal sequence.
- A tampered snapshot: file and tree digests are verified before decoding.
- A write on a snapshot host: refused before any read, naming the owning store.
- A bounded audit is read as a whole-store verdict: the item names `throughSeq`, and its
  `headSeq` is SEQ, not the store head (TCL-V0-820).
- A dry run is read as a committed update: its item says `dryRun: true` and carries no outcome, and
  the store bytes are unchanged (TCL-V0-821).
- A profile only a computed `LiveFormats` entry names is not resolved statically: the runtime
  CAL-V0-131 test still covers it (TCL-V0-815).

## Acceptance evidence

- `internal/tasks/similar/similar_test.go`: `TestTCLV0002_RankIsDeterministicWithEvidence`,
  `TestTCLV0008_RecurrenceLines`, `TestTCLV0006_DigestRoundTrip`;
  `internal/tasks/similar/anchor_test.go`: `TestTCLV0990_OneRareTermIsNotDuplicate`;
  `internal/tasks/similar/eval_test.go`: `TestTCLV0009_FrozenDuplicateEval` (recall@3 and @5, known
  misses, precision, latency).
- `internal/contextindex/lexrank_test.go`: `TestTCLV0003_ScoreMatchesContextCompiler` (the
  exported tokeniser, `LexicalCounter` and term scores against the compiler's own).
- `internal/tasks/cli/similar_test.go`: `TestTCLV0001_SimilarRanksWithEvidence`,
  `TestTCLV0004_CreateGuard`, `TestTCLV0005_DigestAndOfflineMatch`,
  `TestTCLV0007_RecurAppendsOneLine`.
- `internal/tasks/cli/replay_receipt_test.go`: `TestTCLV0991_ReplayNamesTheOriginalReceipt`.
- `internal/tasks/store/writer_checkpoint_test.go`: `TestCALV0190_WriterRouteServesReplaysAndRefusals`
  (writer-checkpoint replays name the same `originalReceipt` as the complete route; TCL-V0-991).
- `internal/tasks/cli/snapshot_bundle_test.go`: `TestTCLV0010_QueueSnapshot`,
  `TestTCLV0011_SnapshotReads`, `TestTCLV0012_SnapshotRefusesWrites`.
- `internal/tasks/boundary_test.go`: the `similar/terms.go` exception and its controls
  (TCL-V0-003).
- `tools/capabilities-check` and the command-tree snapshot (TCL-V0-013).
- `internal/tasks/cli/cli_papercuts_test.go`: `TestTCLV0860_PositionalAndCurrentRevision`,
  `TestTCLV0861_MutationExamplesRun`, `TestTCLV0862_ListFieldsAcceptShowKeys`,
  `TestTCLV0863_SearchQueryNamesText`, `TestTCLV0864_RoadmapStatusFilter`,
  `TestTCLV0865_ShowNamesFailingIntentChecks`, `TestTCLV0866_ClaimRefusesWholeRepository`,
  `TestTCLV0867_JournalPresentReadsWarnNoAbsence`, `TestTCLV0868_CoverageIsAdvisoryUnderEveryPolicy`,
  `TestTCLV0869_SnapshotValidatesAWorktreeStore`; `review_fixes_test.go`:
  `TestTCLV0860_CurrentRevisionRetryReplays`, `TestTCLV0867_RoadmapRowsMatchFacets`,
  `TestTCLV0867_CountWarnsWhenReservationsUnreadable`; `snapshot_worktree_internal_test.go`:
  `TestTCLV0869_WorktreeSnapshotRefusesAMovingTree`.
- `internal/tasks/cli/refusal_codes_internal_test.go`: `TestTCLV0870_EveryRefusalOutcomeHasACode`;
  `internal/tasks/cli/refusal_codes_test.go`:
  `TestTCLV0870_StaleUpdateCarriesStaleTicket`; `internal/tasks/cli/attach_evidence_witness_test.go`:
  `TestTEAV0001_TwoProcessAttachEvidenceCAS` asserts `STALE_TICKET` in the loser's codes
  (TCL-V0-870).
- `internal/tasks/cli/refusal_codes_test.go`: `TestTCLV0859_UnknownVerbAndMissingTicketCarryCodes`;
  `conformance/cli-errors-v0`: `TestCLIErrorContracts_ACXV0859` (TCL-V0-859).
- `internal/tasks/cli/receipt_audit_through_test.go`:
  `TestTCLV0820_BoundedAuditIgnoresLaterInconsistentReceipt`,
  `TestTCLV0820_BoundedAuditIgnoresCorruptHeadReceipt`; `internal/tasks/cli/receipt_compact_test.go`:
  `TestTCLV0820_BoundedAuditRestoresElidedReceipts` (TCL-V0-820).
- `internal/tasks/cli/policy_dry_run_test.go`:
  `TestTCLV0821_PolicyDryRunPreviewsFencesAndWritesNothing`,
  `TestTCLV0821_PolicyDryRunHeadAdvanceWritesNoClosedIndex` (TCL-V0-821).
- `internal/tasks/cli/review_fixes_test.go`:
  `TestTCLV0822_DefaultRevisionHoldRetryReplays` (TCL-V0-822).
- `internal/tasks/cli/create_retry_issued_at_test.go`:
  `TestCALV0250_DefaultedIssuedAtRetryNamesTheDifference`,
  `TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers` (TCL-V0-735).
- `internal/tasks/cli/release_one_read_internal_test.go`:
  `TestTCLV0736_ReadinessReadsTheStoreOnce`; the wsl-lanes byte and syscall measurement in
  `docs/build-log/2026-11-01-tcl-w31.md` (TCL-V0-736).
- `internal/tasks/cli/flags_bool_internal_test.go`:
  `TestTCLV0737_FlagsParseDeclaredBooleans`, `TestTCLV0737_DeliveryScreenUsageMatchesParser`
  (TCL-V0-737).
- `internal/tasks/cli/doctor_closed_index_test.go`:
  `TestTCLV0738_RebuildClosedIndexMatchesJournaled`; the wsl-lanes live rebuild in
  `docs/build-log/2026-11-01-tcl-w31.md` (TCL-V0-738).
- `internal/tasks/cli/supersedes_link_test.go`:
  `TestTCLV0739_DuplicateRecordedAsSupersedesLink`; `TestTCLV0004_CreateGuard` (TCL-V0-739).
- `internal/tasks/cli/ticket_show_text_test.go`:
  `TestTCLV0741_ShowFormatTextRendersBodyAndAcceptance` (TCL-V0-741).
- `internal/tasks/cli/queue_parity_test.go`:
  `TestTCLV0742_QueueParityNamesMissingAndStaleTickets`; `script/agent-preflight_test.sh` case 6
  (`tasks-store` row); the wsl-lanes live snapshot read in `docs/build-log/2026-11-01-tcl-w31.md`
  (TCL-V0-742).
- `internal/tasks/cli/create_source_ref_test.go`:
  `TestTCLV0743_CreateRecordsSourceRefForAbsentPaths`; `make lane-scripts-test` group `ASD-V0-645`
  (an `ABSENT` finding naming a recorded source ref) (TCL-V0-743).
- `internal/tasks/cli/orphan_lanes_test.go`:
  `TestTCLV0744_OrphanLanesListsUnbatchedLaneDeliveries`; `make lane-scripts-test` group
  `ASD-V0-645` (`ORPHAN` and a bad `--orphans` file); the wsl-lanes live run in
  `docs/build-log/2026-11-01-tcl-w31.md` (TCL-V0-744).
- `tools/tasks-profiles-check/main_test.go`: `TestTCLV0815_UnregisteredProfileFailsUntilRegistered`,
  `TestTCLV0815_ProfileInTwoListsFails` (TCL-V0-815).

## Rollback

Revert the change. The new verbs, the guard and the snapshot environment variable disappear;
recurrence lines already written stay as ordinary body text. No stored state, receipt or wire
contract depends on these requirements.
