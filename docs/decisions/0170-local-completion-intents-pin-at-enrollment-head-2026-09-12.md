# Decision 0170 — local-completion intents pin at the enrollment-time HEAD, not the plan base

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/ideas.md` (2026-09-12) recorded that `corvint dogfood begin` resolves each intent
at `snap.target`, the enrollment-time `HEAD`
(`internal/localcompletion/lifecycle.go:114-115@2124574f`), while `LCP-V0-002` and the comment
above that lookup said the intent must resolve at the plan base. No existing test distinguished the
two: every fixture enrolls with `HEAD` equal to the base.

The call follows what the rest of the package depends on:

1. Intents resolve at the enrollment-time `HEAD` commit. Each `IntentPointer` records that commit
   OID and the blob OID found there. `begin` still refuses an intent absent from that commit with
   `intent-path-not-found` before any local write.
2. `finish` counts intents absent from the plan base as bootstrap unknowns
   (`internal/localcompletion/finish.go:154-162@2124574f`) and passes that count as the CEM
   `--max-unknown` ceiling. This matches `OCM-V0-009`, which admits a same-change proposed intent
   absent at the CEM base. Under a base-only lookup `begin` would refuse every such intent, so the
   bootstrap count would always be zero and the documented bootstrap path could never be enrolled.
3. Invariant 1 holds: the pointer names an immutable commit OID and blob OID, never a worktree
   path or a symbolic ref. Invariant 2 holds: an intent that commit lacks is refused, never recorded
   with an empty hash. A same-change intent stays proposed intent: `OCM-V0-009` keeps its matching
   CEM hunks `unknown`, and this decision grants it no historical-evidence status.

Alternative rejected: change the lookup to the plan base, matching the old spec wording. It would
make pointers name historical intent only, but it refuses the same-change bootstrap intents that
`OCM-V0-009` and `finish` accept, leaving `finish.go`'s bootstrap counting unreachable.

Consequences: `LCP-V0-002` wording names the enrollment-time `HEAD` and the bootstrap relationship.
The misleading lookup comment in `internal/localcompletion/lifecycle.go` is corrected, with no
behavior change. `TestEnrollmentPinsSameChangeIntentAtHead` enrolls an intent committed after the
base, checks the pointer's commit and blob OIDs, and fails under the base-only reading. The
ideas.md entry is removed.

Rollback: revert this decision's commit. The spec and comment return to naming the base, the test is
removed, and the ideas.md entry returns.
