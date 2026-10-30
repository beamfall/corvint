# Decision 0202 — test-validity discovers retained evidence with digest-bound freshness

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`PUB-V0-006` requires live test support to track supported unit and E2E execution automatically.
After decisions 0147 and 0149, `corvint test-validity --receipt FILE` and the
`corvint-test-validity-mcp` tool project a JavaScript receipt or completed Go preview event through the
shared builder `internal/testvaliditydoc`, but only when the caller names the file. The reach-gap plan
(`docs/plans/pub-v0-006-reach-gap-2026-09-12.md`) left automatic discovery as an owner decision.
A search on 2026-09-13 found no retained location: `cmd/corvint-js-test-provider` and
`cmd/corvint-go-test-provider` write their document to stdout only, and the VS Code extension keeps
events in memory.

The call:

(a) One retained-evidence location, `.corvint/test-evidence` under the symlink-resolved worktree root.
Operators or a future producer place single-document `*.json` provider outputs there. It is local
derived state like `.corvint/index/`, is gitignored, and is never authority.

(b) Discovery is opt-in: `corvint [--root R] test-validity --discover` and the MCP argument
`discover: true`. Both call `testvaliditydoc.Discover` (`LPCV-V0-053`, `MTV-V0-009`). With no flag,
the no-input `UNSUPPORTED` contract (`LPCV-V0-049`, `MTV-V0-004`) and every existing vector stay
byte-identical. Discovery is bounded: it lists one directory with at most 256 entries, makes at most
16 read attempts, follows no symlink, and does not recurse. It reuses the `LPCV-V0-051` safe reader
and decoder, writes nothing, and runs nothing.

(c) Freshness is bound to the worktree at call time, not to file age (`LPCV-V0-054`):

- A changed or removed bound test or config file is `STALE`.
- An identity that cannot be recomputed from the retained document is `UNKNOWN`. That covers a
  bound path outside the worktree, an unreadable file, no bound file digests, a package or app-build
  digest, and every Go event, because the event does not retain its watched file set.
- Only fully matched digests are `CURRENT`.

The execution axis keeps the observed outcome, so a passed test with unmatched identity reads
`PASSED` with a `STALE` or `UNKNOWN` freshness axis. It is never a current pass, and no axis rewrites
another (`LPCV-V0-047`).

(d) Discovered Go events keep `tier:"preview"` and `promotable:false`. `GLTP-V0-048` is unchanged.

Rejected alternatives:

- Automatic discovery on every no-argument call. It changes the pinned no-input bytes, and it makes
  a read verb's answer depend on untracked local files without the caller asking.
- A recursive search of the worktree for provider documents. It is unbounded and can pick up
  fixtures as evidence.
- Having the providers write retained files in this change. That turns stdout-only producers into
  writers, which is a separate authority and containment call.

What stays open:

- `NOT_RUN`: no producer retains evidence into the location automatically, so end-to-end automatic
  tracking of a real unit or E2E run is not demonstrated.
- `NOT_RUN`: the real-provider Electron run.
- Preview only: Go results.
- Not recomputed, so they stay `UNKNOWN` by construction: package and app-build identities.

`PUB-V0-006` therefore remains not met at release tier.

Consequences: `LPCV-V0-053..054` and `MTV-V0-009` are added. `LPCV-V0-051`, `MTV-V0-002`,
`MTV-V0-003`, `MTV-V0-004` and the profile non-goals are amended. The `PUB-V0-006` traceability row
and the reach-gap plan now record discovery as delivered at experimental tier.

Rollback: revert the commit. That removes `testvaliditydoc.Discover`, the `--discover` flag, the
`discover` MCP argument and its two vectors, and the new requirement text. No persisted state,
schema version or client configuration needs migration, and files an operator left under
`.corvint/test-evidence` are inert.
