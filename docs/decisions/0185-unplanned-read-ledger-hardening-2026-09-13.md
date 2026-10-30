# Decision 0185 — the unplanned-read ledger takes stored spellings, the host cwd, and the observation ledger's refusals

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The 2026-09-13 bug hunt left five hypotheses about `internal/unplannedread` and the analyzer CLIs in
`docs/agent-memory/ideas.md`, and `docs/agent-memory/bugs.md` held a confirmed case-alias defect.
Each ledger item was confirmed by a failing test before this change. The calls, all within
`docs/specs/unplanned-read-events-v0.md` (no requirement IDs added):

(a) Stored spelling (`URE-V0-001`). A read of `PLANNED.GO` on a case-insensitive volume scored as an
unplanned read of a planned `planned.go`. The fix is a private `storedSpelling` step in
`internal/unplannedread` after `projectpath.Relative`, not a change to `Relative`: its only other
caller, `projectRelativePath` in `cmd/corvint/host_adapter.go`, has its own contract, and a
directory listing per component is a cost that caller does not need. Each existing component takes
the exact directory entry, else a case-folded entry that `os.SameFile` confirms. A component no
entry names (for example a Unicode normalization alias) abstains, an under-count rather than a
guessed spelling (invariant 2). An absent suffix keeps its spelling.

(b) Host working directory (`URE-V0-002`). Claude Code's hook `cwd` follows the session's `cd`
(code.claude.com/docs/en/hooks), so the post-tool hook resolves a relative target from an absolute
payload `cwd`. That `cwd` is taken after the command ran, so a Bash command holding a directory
change in any segment, before or after the read, now abstains.

(c) Ignore gate (`URE-V0-003`/`004`). `Enable` and every append refuse unless one ignore file covers
the marker, the ledger and its temporaries, by the same conservative rule `SOL-V0-001` applies to
`.corvint/self-observations.jsonl`. The rule is shared as `observations.CorvintEntriesIgnored`. This
keeps `AGENTS.md` invariant 4's "private derived state": a clone that commits a marker without the
ignore entries writes nothing. Residual, accepted: a repository that commits both the ignore
entries and a force-added marker still enables a bounded, ignored, private ledger. Asking git
whether the marker is tracked was rejected, because it would spawn a process on every hook call.

(d) Symlinked ledger reads (`URE-V0-004`/`005`/`009`). `Read` and packet lookup open the ledger only
when `.corvint` is a real directory and the ledger a regular file that matches the opened
descriptor. `Read` returns an error and packet lookup abstains.

(e) Degraded prompts (`URE-V0-008`). A degraded `user-prompt` delivered no packet, yet later reads
scored against the session's previous packet. The adapter now records a refused packet row through
`RefusePacket` on every degraded prompt path whose session identity is valid, so those reads
abstain until the next delivered prompt.

Rollback: revert the change. The ledger is experimental, derived, and deletable.
