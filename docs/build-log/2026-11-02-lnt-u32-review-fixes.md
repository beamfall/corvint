# Lane lnt-u32: independent review fixes for the lane-host tools

Date: 2026-11-02. Lane `lnt-u32`, base `35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b`. Follows
`docs/build-log/2026-11-01-lnt-u32.md`.

## Problem

An independent Codex review of the lane change returned three P1 and three P2 findings against
`script/lane-scratch.sh`, `script/ssh-heredoc-check.sh`, `script/lane-base-check.sh` and
`script/lane-host-check.sh`. Each was checked against the code and reproduced; none was refuted.

## Decisions

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | P1 | `lane-scratch.sh`: two worktrees with one lane name could both see no marker (or both adopt an orchestrator-staged marker) and both receive the directory | fixed: the marker is read, checked and written under a `.lane.lock` directory (mkdir is atomic; bounded 5 s wait, a stale lock is reported, never removed) and published by rename (ASD-V0-765) |
| 2 | P1 | `ssh-heredoc-check.sh`: an allowed stdin heredoc exited 0 at once, leaving later commands unchecked | fixed: the terminator is parsed (quotes, `<<-` and backslashes handled), the body is skipped at the next newline and the scan resumes; a here-string (`<<<`) takes no body (ASD-V0-769) |
| 3 | P1 | `lane-base-check.sh`: a missing or unreadable reflog skipped the creation-base check, so a branch cut from a HEAD containing BASE plus other work passed | fixed: an empty reflog read is refused as `no-reflog`; creation-base evidence is required (ASD-V0-772) |
| 4 | P2 | `ssh-heredoc-check.sh`: `X=1 ssh h 'cat <<EOF'` took the assignment as the command name | fixed: leading `NAME=value` words and the prefixes `env`, `exec`, `command`, `nohup`, `time` keep the command-name position open (ASD-V0-769) |
| 5 | P2 | `lane-host-check.sh`: an empty `peers` array aborted under macOS `/bin/bash` 3.2 with `set -u` | fixed: guarded expansions for `peers` and `only` (ASD-V0-767). The existing unguarded-array guard in `script/lane-scripts_test.sh` had flagged both, so that test failed on the lane head before this fix |
| 6 | P2 | `lane-scratch.sh`: a failed `chmod 0700` was ignored | fixed: refused as `chmod-failed` (ASD-V0-765) |

## Evidence

- `script/lane-host-tools_test.sh` gains a fixture per finding: chmod stubbed to fail; eight racing
  rounds each for fresh creation and orchestrator-staged adoption, each acquiring exactly once; a
  quoted heredoc after a stdin heredoc body and after a here-string; a `<<-` body holding quotes
  and `<<`; assignment- and `env`-prefixed ssh; a no-peer run under `/bin/bash`; a branch created
  with `core.logAllRefUpdates=false`. PASS on unraid-lanes. With the pre-fix `lane-scratch.sh` the
  chmod fixture and every race round fail.
- `script/lane-scripts_test.sh` (50 groups): PASS on unraid-lanes.
- `/bin/bash` 3.2.57 on the Mac: `lane-host-check.sh --only ssh` without `--peer` exits 0; the
  compound, assignment-prefixed and `<<-` heredoc cases give the expected exit codes.
- `corvint affected --base 35475f860` tier 0 and 1 shell units (`lane-host-tools`,
  `check-error-code-ownership`, `check-install-lifecycle`, `check-license-headers`): PASS.

## NOT_RUN

- The 83 tier-2 Go packages `corvint affected` selected (scope UNKNOWN, 76 `LANGUAGE_FRONTIER`
  unknowns, `self-binary-stale`): the fix touches only shell scripts and docs; batch CI covers them.
- `integrations/host-adapters.test.mjs`: unchanged by this fix; its live run is in the 2026-11-01
  entry.

## Rollback

Revert this commit; the scripts return to the 2026-11-01 lane behavior.

## Outcome

All six findings fixed; dispositions of the lane tickets are unchanged from the 2026-11-01 entry.
