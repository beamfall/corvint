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
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-3236, V1-3237, V1-3223); experimental; TCL-V0-001..014 implemented with focused tests and a frozen duplicate eval; TCL-V0-870 (refusal codes in the envelope) implemented with focused tests.
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
  `ticket search`, `ticket show`, `ticket export`, `ticket similar`, `ticket digest`, `roadmap`
  and `queue status` MUST be served from it without resolving a repository or journal, with every
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

## Acceptance evidence

- `internal/tasks/similar/similar_test.go`: `TestTCLV0002_RankIsDeterministicWithEvidence`,
  `TestTCLV0008_RecurrenceLines`, `TestTCLV0006_DigestRoundTrip`;
  `internal/tasks/similar/eval_test.go`: `TestTCLV0009_FrozenDuplicateEval` (recall@3 and @5, known
  misses, precision, latency).
- `internal/contextindex/lexrank_test.go`: `TestTCLV0003_ScoreMatchesContextCompiler` (the
  exported tokeniser, `LexicalCounter` and term scores against the compiler's own).
- `internal/tasks/cli/similar_test.go`: `TestTCLV0001_SimilarRanksWithEvidence`,
  `TestTCLV0004_CreateGuard`, `TestTCLV0005_DigestAndOfflineMatch`,
  `TestTCLV0007_RecurAppendsOneLine`.
- `internal/tasks/cli/snapshot_bundle_test.go`: `TestTCLV0010_QueueSnapshot`,
  `TestTCLV0011_SnapshotReads`, `TestTCLV0012_SnapshotRefusesWrites`.
- `internal/tasks/boundary_test.go`: the `similar/terms.go` exception and its controls
  (TCL-V0-003).
- `tools/capabilities-check` and the command-tree snapshot (TCL-V0-013).
- `internal/tasks/cli/refusal_codes_internal_test.go`: `TestTCLV0870_EveryRefusalOutcomeHasACode`;
  `internal/tasks/cli/refusal_codes_test.go`:
  `TestTCLV0870_StaleUpdateCarriesStaleTicket`; `internal/tasks/cli/attach_evidence_witness_test.go`:
  `TestTEAV0001_TwoProcessAttachEvidenceCAS` asserts `STALE_TICKET` in the loser's codes
  (TCL-V0-870).

## Rollback

Revert the change. The new verbs, the guard and the snapshot environment variable disappear;
recurrence lines already written stay as ordinary body text. No stored state, receipt or wire
contract depends on these requirements.
