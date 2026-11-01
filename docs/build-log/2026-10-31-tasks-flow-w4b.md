# Corvint Tasks lane flow, wave 4b — V1-3577 ticket split

Spec: `docs/specs/corvint-tasks-lane-flow-v0.md` (TLF-V0-884, TLF-V0-885). Base
`524bef161f9b57f39172e6e65476f0a6b4c2ffed` (wave 4 lane head).

## Delivered

- V1-3577, `ticket split ID --criteria N[,M...] --expected-revision R --request-id Q` (TLF-V0-884):
  moves the named 1-based acceptance criteria of an OPEN or DRAFT parent at revision R to a new child.
  The child copies the parent's planning fields, names the parent in `supersedes`, and receives the
  parent's attached evidence. The parent keeps the other criteria and gains a body line naming the
  child. Refusals: a parent not OPEN or DRAFT, not at revision R, and an empty, repeated, zero,
  out-of-range or every-criterion list.
- `--dry-run` (TLF-V0-885) prints the child CREATE payload and the parent's resulting criteria and
  writes nothing. The parent completes on its kept criteria, and `receipt audit` stays CONSISTENT.
- Tests: `TestTLFV0884_*` and `TestTLFV0885_*` in `internal/tasks/cli/split_test.go`.

## Decision: the split is a resumable sequence, not one multi-post mutation

The two options were one new mutation producing several post-states, or a sequence of existing
single-post mutations. The split is the sequence: child CREATE (`Q/create`), one ATTACH_EVIDENCE per
parent evidence entry (`Q/evidence/I`), then the parent REFINE (`Q/refine`), all under one issuedAt.

- Every journal validator, recovery path, projection and `receipt audit` assumes one ticket post per
  mutation. A multi-post receipt would change all of them and the wire contract for one command.
- The parent changes last. An interruption leaves the moved criteria on both tickets, never on
  neither, and the parent is still at revision R, so the same command resumes.
- Rerunning with the same `--request-id` and `--issued-at` replays the recorded steps through the
  existing request-ID replay and runs the rest. Once `Q/refine` is recorded, the split writes
  nothing and reports `alreadySplit` with the child.
- The child CREATE passes the duplicate guard with the parent as `notDuplicateOf`, since it shares
  the parent's text by construction.

## Requirement IDs

The brief assigned TLF-V0-888 and TLF-V0-889, but wave 4 already used them for effects proposals
(V1-3576). The spec had already reserved TLF-V0-884 and TLF-V0-885 for the split as "proposed, not
implemented", so this change implements those IDs instead of allocating new ones.

## Failure modes

- A split is not atomic across its steps. Between the CREATE and the REFINE the criteria exist on
  both tickets.
- A concurrent writer that changes the parent between steps makes the REFINE refuse on its
  expected-revision check. The child stays, and the operator either retires it or reruns the split
  with a new request ID against the new revision.
- Request IDs are at most 64 bytes, so `Q` is at most 52 bytes to fit `Q/evidence/NN`.

## Review fixes

The one-round Codex review of `524bef161..950240cbf` reported five P2 findings and one P3, all in
`internal/tasks/cli/split.go`. All six were confirmed and fixed. Each test is in
`internal/tasks/cli/split_test.go`, except the REDO_PENDING test, which is in `split_redo_test.go`
(darwin and linux).

1. P2: the child dropped `requiresPool`, `requiredRoles` and `executionPrerequisites`. The child
   CREATE now carries the roles (deep copy) and the pool. CREATE has no prerequisites field, so a
   parent with prerequisites gets its child created without criteria (DRAFT, not claimable). A
   `Q/child` REFINE then sets the criteria and prerequisites together, which opens it. A MANUAL or
   EXTERNAL parent with prerequisites refuses `TICKET_STATE`, because its child would open at
   CREATE. Test: `TestTLFV0884_SplitKeepsExecutionConstraints`.
2. P2: after an interruption, the resumed parent REFINE could drop criteria that another writer had
   since removed from the child. The parent REFINE now runs through the new `store.MutateGuarded`
   with a `TicketGuard`. The guard reads the child under the same writer lock, after redo, and
   refuses `STALE_TICKET` unless the child carries the moved criteria at the acceptance revision the
   split gave it and is not ARCHIVED. A replayed request skips the guard. Test:
   `TestTLFV0884_ResumeRefusesARefinedChild`.
3. P2: a receipt linked but not yet in head.json left `REDO_PENDING`, so the initial inventory read
   refused before any writer could recover. The new `store.Settle` runs the writer guards and redo
   under the lock. The split calls it when its first read returns `REDO_PENDING`, then reads again,
   and reports the redo warning. Test: `TestTLFV0884_ResumeSettlesAPendingReceipt` holds one
   receipt pending using the existing `holdPending` injection.
4. P2: any recorded `Q/refine` was reported as `alreadySplit`. It now counts only when all of these
   hold, and otherwise refuses `REQUEST_ID_CONFLICT`:
   - the recorded refine changed this parent, to revision R+1;
   - `Q/create` is recorded;
   - its child supersedes the parent;
   - the child's body starts with this split's note, which names R and the criteria.
   Test: `TestTLFV0884_RecordedRefineMustBeThisSplit`, covering a plain body refine under `Q/refine`
   and a rerun with other criteria.
5. P2: a parent body near 64 KiB passed the dry run, then failed after the child was created. The
   parent's final REFINE payload is now built and canonicalized before any write, using the longest
   possible child ID in the parent's queue as the stand-in. A note that does not fit refuses
   `LIMIT_EXCEEDED`, in the dry run too. The note's length is bounded by the criteria list, the
   revision and the ID. Test: `TestTLFV0884_ParentBodyAtTheLimitRefusesFirst`.
6. P3: deduplicating by digest dropped later entries and their provenance. Each digest is still
   copied once, because the child has one acceptance revision and a repeat would refuse
   DUPLICATE_ID. An entry whose digests were all copied already now appends its segment
   (`[acceptanceRevision A, ACTOR] REASON`) to the reason of the copy that carries its first digest.
   Within the 512-byte reason bound, each segment gets an equal share. Test:
   `TestTLFV0884_EvidenceProvenanceSurvivesRepeatedDigests`.

## Friction

1. The orchestrator-assigned IDs TLF-V0-888 and TLF-V0-889 collided with wave 4's own definitions on
   the base it named. The ID-block assignment did not read the lane head it was cut from.
2. The transaction model has no multi-ticket mutation, so any command that must change two tickets
   together is a non-atomic sequence with a resume rule of its own.
3. The 64-byte request-ID limit bounds the user-chosen prefix of any derived step ID.
4. Correction to wave 4 friction 6: `listFlags` in `internal/tasks/cli/similar.go` does take boolean
   flags; only `flags()` in `cli.go` lacks them.
5. `usage(...)` refusals carry no error code, unlike `wire.Errorf` refusals, so a caller cannot tell
   a missing flag from other ERROR outcomes by code.
6. `ticket search` refuses with no filter, so a test that needs every ticket must pick an arbitrary
   filter such as `--kind FEATURE`.
7. heavy.sh offloads `internal/tasks/cli` to wsl-lanes even though the package holds darwin-only
   files, which that host neither builds nor runs.
8. The store had no exported way to settle `REDO_PENDING` before a read-driven writer. This change
   adds `store.Settle`.
9. The writer had no cross-ticket precondition, so a command that must check one ticket while
   writing another could not do it atomically. This change adds `store.TicketGuard` and
   `store.MutateGuarded`.
10. Receipt post-records can be compacted into segments, so the CLI cannot reliably read a recorded
    step's post-state from its receipt. Validation (fix 4) uses the request index and the live
    inventory instead.
11. CREATE cannot carry `executionPrerequisites`, so creating a constrained ticket takes two
    receipts.
12. A `requiresPool` naming a pool absent from policy is refused MALFORMED, which needed a
    pool-bearing policy fixture for fix 1.
