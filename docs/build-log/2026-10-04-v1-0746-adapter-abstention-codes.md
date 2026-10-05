## 2026-10-04 V1-0746: named codes for silent host-adapter abstentions

Human-owned intent: the owner asked to start V1-0746. The V1-0712 inventory found host adapters
that drop input without a degradation code:

- the Codex adapter returns `{}` for a hook event it does not handle;
- the Claude Code adapter turns an out-of-project `post-tool` target into an empty change set;
- the OpenCode plugin drops rejected paths and stops adding paths past its 256-path cap.

The fail-open policy (`AHI-044`, V1-0711) is unchanged: every output stays non-blocking.

### Change

- **Codex unhandled event.** The output stays `{}`. When the payload `cwd` is absolute and inside
  a Git repository, the adapter now appends one `SOL-V0-010` row: event `unrecognised`, code
  `unsupported-hook-event`. The host's event name is never recorded.
- **Claude Code out-of-project post-tool target.** The output stays `{}` (`AHI-019`). The adapter
  now records `post-tool-path-not-project-relative`, without the path.
  - Both cases go through `recordAdapterReason`, which was split out of
    `recordAdapterDegradation` so a code can be recorded without a degraded hook output. The
    append keeps that function's deadline and deduplication bounds.
  - `internal/observations` admits the new event label and both codes.
- **OpenCode.** The plugin now names two codes at `console.info` (`AHI-022`):
  - `post-tool-path-not-project-relative` when a completed tool call reports a non-empty path
    that `normalizeRepositoryPath` rejects;
  - `changed-paths-truncated` when a new path is dropped at the 256-path cap, at most once per
    session (`post-tool`) and once per `file-change` batch.
- Specs amended in place: `SOL-V0-010` (with its failure-modes row), `AHI-019` and `AHI-022`.

### Evidence

- `TestAdapterSilentAbstentionsAreLedgered`, covering an unknown Codex event and an out-of-project
  Write target. It checks:
  - the output is `{}`;
  - there is exactly one row per case, with no event name or path in it;
  - a relative `cwd` and a non-repository `cwd` record nothing.
- `AHI-022 V1-0746 OpenCode names the path cap and an out-of-project path at info level`. Two
  concurrent 200-target `patch` calls fill both the session set and one `file-change` batch past
  256, and the test checks:
  - each truncation is named once;
  - there are no warnings;
  - the batch and the stop input carry exactly 256 paths;
  - an outside target is named once.

### Limits

- OpenCode is an in-process JavaScript plugin with no `SOL-V0-010` writer. Its two codes reach the
  OpenCode server log, not the self-observation ledger. A ledger row for OpenCode would need a new
  Core input and is NOT_PRODUCED here.
- `boundedPaths` still drops entries past its own 256-item slice of a single call without a code.
  Only paths it admitted but that the session set or batch cannot hold are reported.
- Real hosts remain NOT_OBSERVED.
