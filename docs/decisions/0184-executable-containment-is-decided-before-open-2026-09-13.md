# Decision 0184 — executable repository containment is decided before the open

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The 2026-09-13 bug-hunt backlog (`docs/agent-memory/ideas.md`) asked whether `within`
(`internal/analyzerexec/runner_posix.go`), which checks repository containment with a lexical
comparison and an ancestor `Lstat` identity walk before `openRelativeNoFollow` opens the
executable, leaves a race that `ACC-V0-002` calls terminal. A scratch probe confirmed the window:
while another goroutine swapped the bound repository directory with an ancestor of the executable
path, `within` returned false and the no-follow open then returned the repository's file within 50
iterations.

The call: containment stays decided before the open, and `ACC-V0-002` says so. The window cannot
be closed with POSIX directory calls. A check of each held directory descriptor during the walk
misses an intermediate directory that is renamed into the repository after its descriptor opens,
and a check after the open can be undone by a second rename. Reaching the window requires renaming
directories around the bound repository root, and the opened file still has to be an invoking-UID,
single-link regular file whose bytes equal the pinned digests. The race cannot change which bytes
are staged and executed. The failure-mode row for rename races now names this exception, so the
spec no longer promises a terminal result the code cannot deliver. No requirement IDs are added and
no code changes.
