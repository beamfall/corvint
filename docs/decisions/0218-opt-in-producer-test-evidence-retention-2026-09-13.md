# Decision 0218 — providers retain test evidence on an opt-in `--retain` flag

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0202 made `corvint test-validity --discover` and the MCP `discover` argument read the
newest provider document under `.corvint/test-evidence`, but left producer-side retention open. At
that point nothing wrote there: both providers printed to stdout only. It also left open whether
`corvint-test-validity-mcp` should admit linked worktrees. `MTV-V0-003` required a `.git` directory,
while the CLI `--discover` path accepts any root.

The call:

(a) `corvint-js-test-provider unit|e2e` and `corvint-go-test-provider session` gain an opt-in `--retain`
flag (`LPCV-V0-055`). With it, the provider writes its stdout bytes unchanged. It then writes the
same bytes into `.corvint/test-evidence` of the worktree root through the shared
`internal/testevidence.Retain`:

- The JavaScript root is the nearest ancestor of `--dir` that holds a `.git` entry. The Go root is
  the authority bundle's repository root.
- Go retains only `passed`, `failed` and `stale` event lines, the events discovery projects.
  Retaining running events would push completed runs out of the 32-entry window.
- The file name is `<provider>-<19-digit unix nanoseconds>-<16 hex>.json`. Discovery already selects
  that name as a regular `*.json` entry, so discovery semantics are unchanged.
- The write is atomic: an exclusive dot-named temporary file in the same directory, then fsync and
  rename. The file is `0600`, and created directories are `0700`. Existing directories keep their
  mode.
- A symlinked or non-directory `.corvint` or `test-evidence` is refused. A component that changes
  between the admitting `Lstat` and its open is also refused.
- After each write the provider prunes to its own newest 32 regular files by name. It never removes
  an entry outside its own name pattern, so operator-placed files and the other provider's files
  survive.
- A retention failure goes to stderr and makes the exit nonzero. The stdout document has already
  been emitted.

The one-shot Go authority run does not take the flag, because its transcript is not a document
discovery decodes. Without `--retain` no byte and no exit code changes.

(b) `MTV-V0-003` admits a linked worktree. `.git` may be a regular file of at most 4 KiB, read
through the no-follow receipt reader, whose `gitdir:` line names an existing directory. A relative
path resolves against the root. A missing `.git`, a symlinked `.git`, and a missing gitdir are still
`invalid-root`. The per-call re-identification keeps comparing the same `.git` entry.

Rejected alternatives:

- Retention on by default. It turns stdout-only producers into writers without the caller asking,
  and it changes the files an existing run leaves behind.
- A `--retain PATH` destination. Discovery reads one location, and a free path reopens the
  containment question decision 0202 closed.
- Pruning by modification time across all `*.json` entries. That could delete operator-placed or
  other-provider evidence.

What stays open:

- `NOT_RUN`: a real-provider Electron run with retention.
- `NOT_RUN`: a qualified Go session matrix.
- The VS Code extension does not pass `--retain`.
- Go results stay preview (`GLTP-V0-048`).

`PUB-V0-006` therefore remains not met at release tier.

Consequences: `LPCV-V0-055` is added. `MTV-V0-003` and the `LPCV-V0-053` and `PUB-V0-006`
traceability rows are amended. The reach-gap plan records producer retention.

Rollback: revert the commit. That removes `internal/testevidence`, both `--retain` flags and the
linked-worktree admission. No schema or wire contract changes. Files already retained under the
gitignored `.corvint/test-evidence` stay readable by discovery, or can be deleted.
