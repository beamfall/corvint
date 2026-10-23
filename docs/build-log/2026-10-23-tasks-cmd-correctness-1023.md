# Tasks command correctness: archive export descriptors and interrupted worktree removal

Date: 2026-10-23. Base `7b3a8e55fc909b9692c684e47dc4c6787ac2419a`. Tickets V1-0903 and V1-2368
delivered; V1-2400, V1-0907 and V1-0775 not delivered in this change (owner asked to close out early).

## V1-0903 (CTS-V0-020)

- Reproduction: a 10,000-ticket / 13,000-receipt store built by the store package's
  `historyStoreAt` (the V1-0893 store D shape). 20 sequential and 18 six-way concurrent exports on
  Linux passed at current main, and 26 at the reported base 76f7f2ac, all with the host's
  20,000-descriptor limit. Under `ulimit -n` 256 to 275 every export exited 1 with
  `RESOURCE_EXHAUSTED`; at 276 and above every one passed.
- Cause: `archiveRead` kept one `os.Root` per listed directory for the whole attempt: 256
  `requests/` shards plus 6 others. A process a few descriptors above the threshold fails only
  sometimes. The original run's envelope (written to stderr) was not kept, so tying that exact
  rc=1 to this cause is an inference.
- Fix: at most 16 non-anchor roots held; a released root is reopened under its parent and checked
  against the listed identity. The 10k export now passes at `ulimit -n 64` and its stream is
  byte-identical to the unbounded one.

## V1-2368 (CAL-V0-365..366)

- `WORKTREE_REMOVE` is now a safe takeover phase; the next owner settles the removal
  (`REMOVAL_FINISHED` / `REMOVAL_ROLLED_BACK`) and returns to `FINISHED` or `ADMITTED`.
- Removed worktree records are dropped instead of kept as `removed: true`.

## Not delivered

- V1-2400: needs a new journaled operation (stage enum, receipt kind, model, store, CLI); design
  sketched but not started in code. CTS-V0-021..024 left unused.
- V1-0907: not started.
- V1-0775: already reproduced and recorded as a limit by ATR-V0-241
  (`internal/tasks/store/lease_mapped_write_unix_test.go:27`); the fix trade is left to the owner.
