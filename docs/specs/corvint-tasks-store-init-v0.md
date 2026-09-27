# Corvint Tasks store initialization V0

Owner: Russell Lewis
Date: 2026-09-25
Intent status: proposed
Delivery status: experimental
Authoritative inputs: decision 0397 (corvint-tasks built in tree), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0323, V1-0310 and V1-0331, and the in-tree sources under
`internal/tasks/store`, `internal/tasks/journal`, `internal/tasks/intent` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks init` refuses an intent store that already holds records, and works in a repository reached through a symlinked ancestor such as macOS `/tmp`.
- Status: proposed/experimental; CTS-V0-001 and CTS-V0-004 are implemented and owner-accepted (2026-09-27), CTS-V0-002 and CTS-V0-003 are proposals only.
- Exists: the coded init refusal, the ancestor resolution, and their store and CLI tests.
- Blocked on: owner acceptance of CTS-V0-002/003 and the recovered task-store contract (V1-0310).
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

The corvint-tasks journal lives in `.git/taskman` and is never committed; the intent store
`.taskman/` is committed. A fresh clone therefore has committed tickets and no journal. Genesis binds
only the queue and policy, so every ticket or release record already present has no journal
afterimage, and every later read refuses the whole store as `INTENT_DIVERGED` (V1-0323). The
operator's only recovery was to delete the new journal by hand.

This spec owns store initialization until the task-store contract is recovered (V1-0310); it does not
restate or replace that contract, and it adds no wire code. Only CTS-V0-001 and CTS-V0-004 are
implemented; the owner accepted both on 2026-09-27, and the rest stays a proposal until accepted.

Non-goals: adopting or importing existing records, a journal-optional read mode, a new wire code, any
change to genesis bytes, journal format, projection checks or the closed code set, and any automatic
repair of an existing frozen store.

## Requirements

- `CTS-V0-001`: `corvint-tasks init` MUST refuse, before it creates the state directory or any
  journal byte, when the intent store's `tickets/` or `releases/` directory holds any entry. The
  refusal MUST carry outcome `BLOCKED` in the store report, `REFUSED` at the CLI, the existing code
  `INTENT_DIVERGED`, and a reason naming the first record found. An absent or empty directory MUST
  NOT refuse. An unreadable directory MUST refuse as `UNSUPPORTED_FILESYSTEM`.
- `CTS-V0-002`: (proposed, not implemented) A read verb (`queue status`, `roadmap`, `ticket show`,
  `ticket search`) run where the journal is absent SHOULD answer from the committed intent store
  alone, labelled journal-absent and unaudited, instead of failing. It MUST NOT create the journal,
  MUST NOT report receipts or audit identities it cannot bind, and every mutation MUST still require
  an initialized journal.
- `CTS-V0-003`: (proposed, not implemented) An explicit `corvint-tasks init --adopt` (or a separate
  `import` verb) SHOULD create a journal whose genesis binds every existing ticket and release record
  exactly as committed, so that later projections match. It MUST validate each record before
  writing, MUST refuse on the first invalid or duplicate record with nothing retained, and MUST NOT
  rewrite committed intent bytes.
- `CTS-V0-004`: Store resolution MUST resolve symbolic links in the directories above the primary
  worktree, such as the macOS `/tmp` and `/var` links into `/private`, and MUST record that resolved
  primary-worktree path, so `init` and every later verb succeed from either spelling and name one
  primary worktree. A symbolic link at the primary worktree or in its `.git` path MUST still refuse
  as `UNSUPPORTED_FILESYSTEM`, and so MUST an ancestor that cannot be resolved.

## Failure modes

| Failure | Behavior |
|---|---|
| Init over committed tickets or releases | Refused with `INTENT_DIVERGED`; no state directory is created (CTS-V0-001). |
| Intent directory unreadable | Refused with `UNSUPPORTED_FILESYSTEM`; nothing is created. |
| Repository reached through a symlinked ancestor | Resolved; the canonical primary worktree is recorded (CTS-V0-004). |
| Primary worktree or `.git` is a symbolic link | Refused with `UNSUPPORTED_FILESYSTEM`, unchanged (CTS-V0-004). |
| Journal already initialized | Unchanged: the existing already-initialized refusal applies first. |
| Store already frozen by an earlier init | Not repaired here; the operator removes `.git/taskman` and waits for CTS-V0-003. |

## Acceptance and rollback

CTS-V0-001 is accepted by the focused store and CLI tests below: a refused init leaves no state
directory, and the same init succeeds once the record is removed. CTS-V0-002 and CTS-V0-003 need
owner acceptance, the recovered task-store contract, and their own tests before any implementation.
Rollback of CTS-V0-001 removes the `existingRecord` guard in `internal/tasks/store/store.go`; no
journal, intent or wire migration is needed, because the guard only refuses before anything is
written.

CTS-V0-004 is accepted by a CLI test that initializes a repository through a symlinked ancestor,
checks that `head.json` records the canonical path, and reads the queue through both spellings.
Rollback removes `canonicalAncestors` in `internal/tasks/intent/worktree.go`; a repository under a
symlinked ancestor is then refused again, and a journal it already wrote keeps the canonical path.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CTS-V0-001 | `internal/tasks/store/store.go` (`Init`, `existingRecord`, `firstEntry`), `internal/tasks/cli/init.go` | TestCTSV0001_InitRefusesOverExistingRecords, TestCTSV0001_InitRefusesOverCommittedTickets |
| CTS-V0-002 | proposed; no implementation | none until accepted |
| CTS-V0-003 | proposed; no implementation | none until accepted |
| CTS-V0-004 | `internal/tasks/intent/worktree.go` (`finish`, `canonicalAncestors`) | TestCTSV0004_InitThroughSymlinkedAncestor |
