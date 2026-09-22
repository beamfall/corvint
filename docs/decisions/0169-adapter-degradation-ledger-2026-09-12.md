# Decision 0169 — Host adapter degradations are ledgered as content-free self-observation rows

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0161 quieted the Claude Code adapter's expected degradations into `additionalContext`, so
`user-prompt` and `post-tool` degradations reached only the host transcript. `corvint
observations` reads only `.corvint/self-observations.jsonl`, which the adapter never wrote. The
owner directive: quieting adapter notices must not lose the degradation information; it has to
stay recoverable so its causes get fixed.

Options weighed:

1. Read host transcripts (`~/.claude/projects/<slug>/*.jsonl`) from a new verb. Rejected: `AHI-005`
   forbids adapters from reading host transcripts, the transcript format is host-owned and
   undocumented, and the transcript carries prompt text Corvint must not retain.
2. Widen the bounded self-observation ledger with one new row kind. Chosen.

The call:

1. The `codex` and `claude-code` adapters append one best-effort `adapter-degradation` row for any
   degradation returned after the project root is resolved. The row carries only `host`, `event`,
   `adapterCodes` (a sorted set from a closed registry), `window` (the UTC hour), and
   `corvintVersion`. No prompt text, paths, tool input, session identity, or repository content. An
   `corvint-event-rejected:<code>` suffix outside the closed dogfood registries is recorded bare.
2. A row byte-identical to one already retained is not written, so each host, event, code set and
   version is recorded at most once per hour and a busy session cannot flood the ledger. The
   existing 2 KiB row and 128 KiB file bounds, oldest-first rotation, ignore and symlink refusals,
   and secret screening apply unchanged.
3. The append never changes the hook output or exit status and waits no longer than the work
   deadline (50 ms for the Claude Code kill-deadline row, attempted after the watchdog fires).
   Degradations before root resolution and the Codex kill deadline record nothing, because the
   ledger location would have to be guessed.
4. `corvint observations` prints `ADAPTER-DEGRADATION windows=N key=HOST/EVENT/CODE latest=WINDOW`
   lines, read-only. Adapter rows are not `events`, never enter `STANDING`, and never enter
   ranking, learning, evidence, or authority.

Adds `SOL-V0-010`; amends `SOL-V0-007`'s adapter sentence, `AHI-021`, and `AGENTS.md` invariant 4.

Rollback: remove the adapter's `recordAdapterDegradation` calls and the row kind. Retained rows age
out under the cap, and the ledger stays gitignored derived state that nothing depends on.
