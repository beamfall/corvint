# Decision 0250 — source-view native port gaps: implement, amend, or leave NOT_PRODUCED

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The native Go port of the experimental source views (`54735d98`, `cmd/corvint/source_handoff.go`)
left seven clauses of `docs/specs/experimental-source-views-v0.md` unimplemented, listed in its
Native implementation status. The reference for each was the deleted Python consumer
(`benchmarks/selfuse-batch/source_views.py` at `54735d98^`). The call for each clause:

| Clause | Call | What changed |
|---|---|---|
| `ESV-V0-001` HEAD re-verification | implement | `executeSourceView` runs `rev-parse HEAD` again after extraction and refuses `stale-commit` if it moved. |
| `ESV-V0-001` packet request/coverage/subject | implement, requirement text made explicit | `selectSourceHandle` refuses `packet-shape` when `request` or `coverage` is not an object or `subject` is absent, as the Python `select_handle` did. |
| Interface: packet opened without following symlinks | implement | The packet is read through the existing `openBoundedRegularFile` (`O_NOFOLLOW`, `O_NONBLOCK`, descriptor `SameFile` check), capped at 65,537 bytes. Symlink and FIFO refusals already held through `Lstat`, so their tests pin behavior rather than fail first; the closed window is the swap between `Lstat` and the path re-read. |
| Interface: 4,096-byte argument bound | implement | `parseSourceViewOptions` refuses any option name or value over 4,096 bytes with `malformed-input`. Scoped to `adapter source-view`, whose interface states the bound. |
| `ESV-V0-002` prefix grammar | implement | Requirement mode scans the prefix for unclosed frontmatter, an open fence, any HTML comment delimiter and line-leading HTML, and the clause for fence/comment tokens, tabs, line-leading HTML and empty clause text, as `plain_prefix` and `requirement_span` did. |
| `ESV-V0-003` TERM grace on Git children | implement with the existing runner, amend the figures | `sourceGit` runs through `procgroup.Run` in an owned process group: cancellation sends TERM to the group, KILLs it if it has not quiesced within the runner's 100 ms grace, and reaps descendants before returning, with a 1.5-second shutdown bound. The spec's one-second TERM grace plus half-second drain grace were the Python supervisor's figures; changing the shared runner's grace for one experimental consumer was rejected. |
| `ESV-V0-003` raw fallback | implement | The fallback argv carries the same `--no-pager` sanitized options as every Git read, and the fallback object returns the sanitized `environment`, as Python did. |
| Interface: root is the Git toplevel (amendment, 2026-09-13) | implement | `executeSourceView` first runs `rev-parse --show-toplevel` and refuses `repository-root` unless it resolves to the given root, as Python `read_source` did. |
| `ESV-V0-001` object format and OID width (amendment, 2026-09-13) | implement | `executeSourceView` refuses `object-identity` before building the fallback argv unless the commit is lower-case 40- or 64-hex, then runs `rev-parse --show-object-format`, refuses `object-format` for anything but `sha1`/`sha256`, and refuses `object-identity` unless the commit, packet revision and handle blob all have that format's width; the success `source` carries `object_format`, as Python `execute`/`read_source` did. |
| `ESV-V0-001` blob size and object digest (amendment, 2026-09-13) | implement the digest, requirement text made explicit; drop the size pre-read | After `cat-file blob`, `executeSourceView` recomputes the object ID over `blob <size>NUL` plus the bytes under the repository format and refuses `stale-blob` on mismatch, as Python `read_source` did. Python's separate `cat-file -s` pre-read is not ported: the read is already capped at 1,048,577 bytes and refused `blob-budget` above the limit, and the recomputed object ID covers the size. |
| Interface: 12 Git commands (amendment, 2026-09-13) | amend: bound by construction, pinned by a test | Python counted commands at run time. `executeSourceView` issues a fixed sequence of seven Git commands (toplevel, object format, HEAD, tree, `ls-tree`, `cat-file`, HEAD) with no loop or retry, so no input can reach a counter; `TestSourceViewStaysWithinGitCommandBudget` counts the commands through a substituted Git and fails above 12. |
| `ESV-V0-005` native source digest | leave NOT_PRODUCED | The requirement freezes digests of the two deleted Python supervisor files and does not say which native files a digest covers, so none is invented. |
| `ESV-V0-007` plugin script identity | amend | The native wrapper takes no plugin and executes no hook script; it renders `additionalContext` itself. There is no script identity to bind, so the clause is removed rather than satisfied by digesting an unrelated file. The shared 10-second capture deadline was already the handoff section's stated contract, so no separate hook-timeout bound is added. |

Tests are in `cmd/corvint/source_handoff_test.go` and `cmd/corvint/source_handoff_unix_test.go`.
The HEAD and cancellation tests substitute the Git executable through an in-process context key,
which no command-line path sets. Delivery status stays experimental; nothing is promoted.

The four amendment rows dated 2026-09-13 close the checks first found while porting and filed then:
root-is-toplevel, object-format OID width, blob size and object digest, and the 12-command count.
