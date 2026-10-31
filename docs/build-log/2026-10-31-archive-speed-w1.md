# Lane archive-speed-w1 — V1-3459 closed-ticket index for OPEN-path reads

Base `bb52a888317f28539c04c885d3771bad59495ac3`, branch `claude/archive-speed-w1-1031`, Darwin arm64,
go1.27.3 local toolchain. Contract: `docs/specs/corvint-tasks-closed-ticket-index-v0.md`
(`TCX-V0-980` to `TCX-V0-985`, from the lane block 980-999; prefix TCX because TCD is closeout debt).

## Problem

Every snapshot read hashed and decoded every ticket file. On the live store (about 3,480 tickets,
2,456 closed) `ticket search --status OPEN` cost about 3x the same read on an open-only store, and
each further 1,000 archived tickets added about 30%.

## Design

- A derived file `<git common dir>/taskman.closed-index` beside the state directory holds, per
  closed ticket file, its path, size, modification time, SHA-256 and a summary (status, priority,
  order, labels, dependencies and the other planning fields). A header carries the format, queue ID,
  policy digest, capture time and the body's SHA-256.
- The snapshot probe trusts an entry only when size and mtime match and the mtime is more than 2 s
  before capture (Git's racy-index rule); the file is then neither read nor decoded, and its digest
  is the entry's, so the intent tree digest and every receipt are unchanged.
- The inventory holds those tickets as summaries. Structure, cycle, dependency and ordering checks
  run on summaries; `Get` decodes the record once, re-hashing it against the snapshot digest. A
  mismatch or an ID/status disagreement fails the read with `SNAPSHOT_MOVED`.
- Only `ticket search`, `roadmap`, `queue status`, `ticket show` and `ticket blockers` opt in. An
  index of another queue or policy, or any unreadable, corrupt or checksum-failing index, is ignored
  and the read is a full scan.
- Reads never write the index. After a command that is not a pure read, the CLI rewrites it only
  when the journal head's stat changed, i.e. a mutation was committed; a refused command leaves the
  filesystem untouched (`CALV0215`).
- A reader's journal audit accepts an indexed tree file as sharing the probed digest when its
  recorded size matches; a writer's audit always reads the bytes.

Alternatives rejected: a persistent decoded cache (authority risk, larger), moving closed tickets
to another directory (changes the projection and wire contract), and parallel decode (cost still
grows with the closed set).

## Measurements

Medians in ms, journal-absent copies of the live store, host load about 27, so the absolute numbers
are noisy. "open" holds only OPEN/HELD tickets, "full" is the live store, "big" is full plus 1,000
synthetic ARCHIVED tickets.

| Command | base open | base full | base big | new open | new full | new big |
| --- | --- | --- | --- | --- | --- | --- |
| `ticket search --status OPEN --facets` | 93 | 302 | 402 | 94 | 113 | 131 |
| `roadmap` | 142 | 396 | 457 | 88 | 123 | 140 |
| `queue status` | 84 | 292 | 382 | 84 | 116 | 106 |
| `ticket show` (OPEN) | 78 | 261 | 345 | 79 | 99.5 | 103 |

A second run of the new build alone, 31 samples per cell:

| Command | full/open | big/full |
| --- | --- | --- |
| `--facets` | 1.21 | 1.068 |
| `--count` | 1.22 | 1.072 |
| `roadmap` | 1.33 | 1.067 |
| `queue status` | 1.21 | 1.088 |
| `ticket show` | 1.21 | 1.068 |

A rerun at host load about 4 (rebuilt final binary, 31 samples) gave full/open 1.20, 1.22, 1.33,
1.22, 1.21 and big/full 1.074, 1.078, 1.070, 1.078, 1.075, in the same command order.

Acceptance 2 (1,000 archived tickets change read time by under 10%) holds for every command.
Acceptance 1 (within 1.3x of open-only) holds for all but `roadmap` at 1.33: its output lists
closed tickets, so it sorts and pages over them; that remainder stays open on V1-3459. The remaining per-ticket cost is
the directory listing and one lstat per file.

## Evidence

- Parity: the base and new binaries gave byte-identical output for every opted-in read on the live
  store copies. `TestTCXV0_IndexedReadParity` compares indexed and full reads, and `receipt audit`,
  on journaled and journal-absent stores; it masks only `queue status`'s wall-clock `observedAt`.
- Focused tests: whole `internal/tasks/intent`, `internal/tasks/ticket` and
  `internal/tasks/journal`; the `TCXV0` tests and a broad `-run` group of `internal/tasks/cli`
  (search, roadmap, queue status, show, blockers, audit, receipts, snapshot, complete, archive).
  `corvint affected` selected a wide set; the rest is left to batch CI.

## Bugs found during the work

- Trusting entries before checking the index's queue and policy made a policy change look like a
  moved snapshot; the probe now re-captures with no index when the index does not govern the tree.
- The audit computed the file size before its writer fallback, reporting an empty intent file.
- Refreshing after every non-read command wrote the index after refused commands; it is now gated on
  a journal head change.

## Limits

- A same-size rewrite that keeps the same mtime outside the racy window is not detected (the Git
  limit); Corvint Tasks writers always change the mtime.
- Benchmarks used journal-absent copies; journaled reads add the audit, which shares the same
  digests.
- The refresh-after-commit path is tested by moving the journal head directly: the fixture's
  journaled store refuses real CLI mutations.

## Rollback

Delete `<git common dir>/taskman.closed-index` to fall back to full scans, or revert the
implementing commit; the file is then ignored.
