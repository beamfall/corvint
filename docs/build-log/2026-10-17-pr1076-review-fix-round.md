# PR #1076 fix round: defect register, value screen and worktree findings

Follow-up to [the V1-1588/V1-1593/V1-1602 entry](2026-10-17-v1-1588-tasks-worktree-defects-secrets.md).
CI found one unclassified wire profile, and an independent Codex review found two P1 and three P2
defects. Each fix has a regression test, and each test was confirmed to fail against the reviewed
head `9e42f9e7d`. The requirement text was widened under the same standing auto-accept
(2026-10-11) as the original lane. No requirement IDs were added.

- **CI: `TestCALV0131_LiveFormatsCoverEveryDecodedProfile`.** `taskman-defect/0` is a persisted
  local file that Tasks reads and appends, so it joins `LiveFormats`, with a CAL-V0-131 row that
  refuses a later-version line. The row exposed a latent bug: the line decoder called `Profile`
  on the `profile` string rather than on the object, so it never checked the profile. The decoder
  now uses `ProfileVersion` before the closed-key check and `CheckProfile` after it. A foreign
  profile is `MALFORMED`; a later version is `UNSUPPORTED_VERSION` (`TDR-V0-004`).
- **P1: base64 deny line bypass (`KHN-V0-029`).** `c2VjcmV0MTIzNDU2Nzg5MA==` parsed as KEY=VALUE
  with VALUE `=`, which was dropped as too short. Now, when a KEY=VALUE value would be shorter than
  8 bytes, the whole line is compared instead. `FOO=bar` stays ignored, because it is under 8
  bytes whole.
- **P1: repeated `defects add` (`TDR-V0-010`).** The append ran inside `withInventoryStore`,
  which retries its callback when Tasks state moves, so one command could append two defects. The
  callback now only resolves the register path and prefix, and the single append runs after the
  callback returns.
- **P2: symlinked parent (`TDR-V0-009`).** Readers already refuse any symlinked component. Writers
  now refuse `UNSUPPORTED_FILESYSTEM` when the existing part of the register directory resolves
  through a symlink, both before and after `MkdirAll`, so nothing is created at a symlink target.
  The check after `MkdirAll` narrows the race but does not close it: a parent swapped for a
  symlink between that check and the open is not covered, and a pinned-descriptor writer is left
  for later.
- **P2: append past the read bound (`TDR-V0-005`).** An append whose result would exceed 8 MiB
  (`MaxRegisterBytes`, the same bound readers enforce) refuses `LIMIT_EXCEEDED` and writes
  nothing.
- **P2: concurrent ensure (`TWT-V0-003`).** A loser that finds the path empty, or already holding
  a `.git` entry, re-observes the branch's worktree every 20 ms for up to 30 s, then returns the
  winner. Any other existing content still refuses `RESOURCE_COLLISION` at once. 30 s was chosen
  because a 2 s settle failed under a host load near 60, while the winner's checkout was still
  running. The deterministic test injects the winner between lookup and path check; a four-racer
  test passed 30 of 30 runs.

Evidence: focused `internal/tasks/{defects,mutation,store,cli}` tests and both CI traps
(`TestCALV0131` in `./internal/tasks/cli`, `TestImportDirection` in `./internal/tasks`). Linux
and multi-process qualification remain `NOT_RUN`.
