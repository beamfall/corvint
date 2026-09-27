## 2026-09-27 V1-0144 LTPM-V0-012: migrate-traces quarantines a trace stranded by an amend or rebase

A dogfood pass records a trace named for the commit it saw. If that commit is then amended or
rebased away, every later pass refuses with `local trace store contains unreachable revision`, and
before this change only manual edits to `.context-corvint/traces/` recovered the store. V1-0144
reports it from the wave-2 pull requests #83 and #84.

Decision (proposed as `LTPM-V0-012`, implemented experimentally): `corvint migrate-traces`
recognizes a stranded trace. The name is an exact object ID, it is neither a reachable commit nor a
reachable legacy tree, and `git cat-file --batch-check` reports it as a commit object. Dry-run
lists it in `stranded_revisions` and binds it into the plan digest as kind `stranded`. Apply moves
the exact bytes to `.context-corvint/legacy-traces/<rev>.jsonl` through the existing staged,
verified quarantine path, then unlinks the trace. The file is framed and bounded but not decoded,
because nothing is rewritten. The object probe reads IDs on stdin, so a missing object is an answer
and a Git failure or budget exhaustion is an error, never "not a commit". A name that is not a
commit object keeps the unreachable-revision refusal, and a plan with no stranded file keeps its
old digest.

The dogfood refusal text, `docs/DOGFOOD.md` step 5 and the rewritten-history row, and the
`migrate-traces` help now name the command.

Set aside: deleting the file outright, which loses the trace with no rollback; and treating any
unreachable name as stranded, which would also retire a store that names a commit from another
repository or a mistyped ID.

Evidence: `TestMigrationQuarantinesStrandedCommitTrace` (dry-run writes nothing; apply moves exact
bytes at mode 0600 and a replan is empty), `TestMigrationRefusesUnknownOrCollidingStrandedTrace`
(a non-commit keeps the exact refusal, a probe failure refuses, a different quarantine copy is a
collision, an identical copy resumes), and `TestMigrateTracesQuarantinesTraceStrandedByAmend` (a
real `git commit --amend` through the CLI). The owner has not accepted the requirement, so V1-0144
stays open until that decision. Rollback: move the quarantined file back into
`.context-corvint/traces/` and revert the change.
