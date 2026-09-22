# Decision 0285 — CEM publication path and crash boundary

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to settle the six
authority-store and CEM-publication hypotheses (wave16 orchestration task, 2026-09-13).

The CEM pilot's bounded-read and durable-publication clause already forbids blocking reads,
repository escape, partial publication, and unsynced final renames, but it did not settle three
adjacent cases. A regular input could become a FIFO between metadata inspection and open; an
explicit worktree output could replace `.git/config`; and a crash after the new map landed could
leave a recognized prior-map backup that a later interrupted transaction would make ambiguous.

The call:

- on the supported FIFO-capable targets, Darwin and Linux, the bounded final open is nonblocking
  and no-follow, followed by the existing descriptor identity, regular-file, exact-size, and
  byte-bound checks. Windows filesystem inputs do not expose FIFOs; other GOOS targets fail to
  build this reader rather than silently use a blocking fallback;
- every worktree-rooted CEM output refuses a case-folded `.git` segment. A publication root that is
  itself the independently verified Git directory may still write its fixed internal artifacts;
- `cite`, `mark`, and `report` outputs refuse the input map's `.lock` and `.bak-` siblings under
  case folding, just as the prepare cache already does;
- the prior-map set-aside and rollback rename/removal are directory-synced. A successful pair syncs
  both new final entries while retaining the durable prior-map backup, then removes and syncs that
  backup. Once `prepare` validates a present final map under its held lock, it removes and syncs
  exactly one recognized stale backup. Zero is a no-op; several remain ambiguous and fail
  `map-unavailable` for manual disposition.

An empty lock file left after a crash carries no age or ownership assertion. Corvint does not delete
it based on time: a later updater may remove it only after acquiring its advisory lock and proving
the path still names that held inode. Fixed-duration lock waits remain their contract's explicit
hang or operation bounds; decision 0082 does not turn every finite deadline into a defect.

Rejected: opening first and checking later without nonblocking flags, because FIFO open can prevent
the check; allowing Git metadata because the output stays inside the worktree, because containment
does not authorize repository administration; choosing among multiple backups; and stale-lock age
heuristics, because none proves the prior holder is dead or the path still names its inode.

Consequence: `CEM-PILOT-013` is amended. No CEM wire, stable error code, or requirement ID changes.
The new regressions drive the real bounded reader, pair publisher, and workflow entry points.

Rollback: revert the commit. That restores the four confirmed failure modes and is not a safe
operational fallback; callers must then avoid FIFOs, explicit `.git`/sidecar outputs, and interrupted
pair publication.
