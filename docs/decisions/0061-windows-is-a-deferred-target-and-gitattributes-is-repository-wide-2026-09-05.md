# Decision 0061 — Windows is a deferred build target and `.gitattributes` is repository-wide `-text`

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-5-release-platform.md` after independent review of its cited evidence.

## Context

`GPK-V0-018` requires a windows/amd64 smoke that verifies `--version` and a real read-only query, and
records in the same clause that the Go query profile refuses there. No Windows host has run `go test`
for the shipped closure; containment is `UNSUPPORTED` on Windows. Separately the repository carries
golden bytes and byte-exact release evidence with no protection against `core.autocrlf`.

## Decision

1. A new `GPK-V0-057` states that a target is shipped only after its native conformance,
   filesystem-safety, process-cleanup and read-nonmutation matrix has run on that operating system;
   windows/amd64 stays in the archive matrix as reproducibility evidence and must not be published
   until the Windows profile passes. Exit criteria: query qualification, a native `go test` of the
   shipped closure, Windows-skipped POSIX-mode and oracle assertions, and a process-cleanup matrix
   disclosed as `PARTIAL` while containment is unsupported. `GPK-V0-018`'s text is unchanged.
2. The root `.gitattributes` is `* -text` with no exemptions. A narrower testdata-only rule loses:
   it is a list that must grow with every golden directory, `text=auto` needs explicit exemptions for
   the two intentional CRLF and mixed fixtures, and worktree byte-exactness feeds the snapshot status
   digest, the archive gate's dirty-tree refusal and `dogfood-check` repository-wide.
3. Backlog, not release work: the Windows `FILE_SHARE_DELETE` snapshot rename and eviction hazard,
   and the post-reap process-group quiescence probe (a sound fix must bind the probe to the leader's
   pid and start-time identity). Implement now: the transient umask flip in `internal/procgroup`
   becomes a one-time setting owned by each executable; the Claude Code hook adapter kills the
   process group only when its wait did not reap the child.

## Consequences

The 0.4.0 release publishes four archives, not five, and the notes say Windows is unsupported. Git
normalises nothing, so a CRLF-committing contributor is not corrected automatically; a
`git ls-files --eol` drift check is a test backlog item.

## Rollback

Drop `GPK-V0-057` and its requirements row; the archive matrix is untouched either way. Delete
`.gitattributes`; it changes no committed blob, only checkout behaviour.
