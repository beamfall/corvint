# 2026-10-24: claim base bound to the ref files (V1-2531, PSR-V0-224)

## Decision

A contended claim's `HEAD^{commit}` observation is bound to the claim root's HEAD and ref files
instead of to the store snapshot (PSR-V0-224, amending PSR-V0-168's V1-2138 claim deferral). The
lock-held observer confirms it by resolving HEAD from the files, with no Git command, and serves
it while they still name the commit Git reported. Every other observation keeps its PSR-V0-168
binding. Implementation: `internal/tasks/store/claim_head.go`, `claimProbe.fresh` and
`claimProbe.head` in `claim_probe.go`.

## Evidence (macOS arm64, 12 CPUs, UNQUALIFIED under load)

Profile first. An instrumented `TestCALV0263_LeaseGateLoad` run (stage and decline capture added
to the harness's samples: `Stages`, `Declines`) showed every contended claim's writer-route
attempt declining with `claim Git observation deferred`, 1 to 13 times per claim. The HEAD memo
was bound to the store snapshot, and heartbeats and renews advanced the snapshot between the
probe and the lock. Each decline still paid a full lease observe under the preparation gate
(lease.observe 640 to 1,925 ms summed per claim). The claim then fell to the complete route and
its SNAPSHOT_MOVED rounds. The uncontended claim hold (`TestCALV0118_WriterHoldProfile`, 40,000
receipts) was already one gate entry of about 63 ms, so the contended hold was declines, not work.

`TestCALV0263_LeaseGateLoad` (40,000 receipts, 15 writers, 120 s, 10 s think):

| verb | before (load 15) wall p50/p99, hold p50/p99 ms | after run 1 (load 9 to 48) | after run 2 (load 24 to 40) |
| --- | --- | --- | --- |
| claim | 45,827/59,195, 757/1,445 | 1,106/1,723, 91/142 | 1,015/2,060, 121/155 |
| renew | 78/2,380, 64/139 | 172/729, 100/171 | 151/549, 108/188 |
| heartbeat | 82/651, 63/134 | 148/1,447, 101/150 | 145/803, 110/225 |
| widen | 11,687/22,552, 363/434 | 16,934/17,222, 656/752 | 16,987/20,558, 627/763 |
| reap | 11,006/21,510, 306/658 | 16,155/18,437, 511/530 | 16,532/16,670, 634/657 |

After the change every claim committed on its first writer-route gate entry with no decline.
Single-writer profile claim preparation hold: 62.8 ms before (load 6 to 9), 68.5 ms after (load
61 to 66), unchanged within noise. The host load differs between runs, so heartbeat p99 is not
comparable here; the WSL2 quiet-host rerun decides the acceptance. These numbers are not
comparable to the WSL2 numbers on V1-2205 or V1-2203.

## Review findings (Codex, PR #1672)

- Mixed ref reads: HEAD and the target ref were read at different instants, so a checkout that
  moved HEAD to another branch and reset the old branch in between paired the old HEAD with the
  reset commit. Each resolution now re-reads HEAD and every symref after the target and refuses
  on any byte difference. `TestPSRV0224_HeadRefusesChainMovedBetweenReads` fails without it.
- Unbound cached layout: the probe's layout was never rebound to the root, so a linked worktree
  moved away and replaced at the same root still resolved through the old admin directory. The
  layout now carries the root's route (`.git` identity or bytes, `gitdir`, `commondir`), and
  every resolution refuses when it changed. `TestPSRV0224_CachedLayoutRefusesReplacedWorktree`
  fails without it.

## Found, not fixed

The remaining gate holders under this load are `widen` and `reap`: 500 to 760 ms per hold, of
which lease.observe is 360 to 490 ms and lease.bind 100 to 155 ms, taken under the gate on their
resumed route. They now set the heartbeat and renew tail.

Before this change, a commit integrated in the claim root while nothing wrote the journal was
served from the stale snapshot-bound HEAD memo. `TestPSRV0224_ClaimRebindsMovedHeadWithoutJournalAdvance`
fails on the old binding and passes now.

## Rollback

Make `headQuery` in `internal/tasks/store/claim_head.go` report false. The claim then uses the
PSR-V0-168 snapshot binding, and `TestV12138_ClaimDerivationOutsideWriterLock` reverts to
expecting two declines and the complete route.
