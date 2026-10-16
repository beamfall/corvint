# 2026-10-16 V1-0328: re-index snapshot bytes and time, no change and one-file change (measured; no code change)

Ticket V1-0328 (no GitHub issue): "re-index is a full rebuild that writes about 63 MB of snapshot
per tree (unverified)". Acceptance: measured snapshot bytes and time per re-index retained; an
improvement lands only with a benchmark. This entry adds the no-change re-index, the `--if-stale`
path and a small fixture to the cold and one-file numbers of
`2026-10-15-index-cost-0328-1380-0231.md`, and records the decision. No code, spec or format
changed.

## Setup

- Base: `origin/main` `08195a48314d9cf0a8bdeeeb92ba8ab932b52c9a`, tree
  `7a3cc75d1b302a71485f1e87796702a116da5659`. Binary built from that commit with
  `GOTOOLCHAIN=local go build -buildvcs=false ./cmd/corvint` (`go1.27.2`), default snapshot format
  (gob; no `CORVINT_SNAPSHOT_FORMAT`, no `CORVINT_INDEX_SHARDS`).
- Host: Apple M2 Max, 12 cores, 64 GiB, darwin/arm64, shared with other lanes and gates. The
  one-minute load average is given with every run; wall time is indicative only. Bytes and CPU
  seconds are the evidence.
- Subjects, each a separate local clone:
  - `repo`: this repository at base, 9,123 tracked files, 8,629 admitted sources, 53,854 symbols.
  - `small`: a fresh one-commit repository holding a copy of `internal/gokernel` (21 files,
    113,681 bytes).
- Per run, in order: `cold` (remove the store `.git/corvint/index` and any `.corvint/index`, then
  `corvint --root SUBJECT index`); `nochange` (`index` again on the same tree); `ifstale`
  (`index --if-stale` on the same tree); `onefile` (append one comment line to one file, commit,
  `index`). Three runs each. Every command ran under `/usr/bin/time -l`; bytes are the receipt's
  `bytes`, store size is `du -sk` of the store before and after, and the snapshot's sha256 was
  taken after each run.
- One earlier `repo` series was discarded: its cold step also removed the tracked `.corvint/`
  directory, so the following commit dropped 793 tracked CEM maps and the snapshot shrank to
  134,675,275 bytes (7,836 sources). That was a harness error, not index behaviour; the series
  below removes only the store and the subject stayed clean apart from the measured commits.

## Results

`repo` (median of three; range in parentheses; CPU is user + system):

| Operation | Snapshot bytes written | Wall s | CPU s | Max RSS | Store after | Load (1-min) |
|---|---|---|---|---|---|---|
| cold `index` | 155,933,247 / 155,933,358 / 155,933,455 | 10.15 (4.52-13.70) | 8.12 (8.10-8.70) | 1.22-1.36 GB | 152,284 KiB, 1 file | 30.9 / 30.9 / 64.0 |
| re-index, no change | same as cold, full rewrite | 10.94 (4.50-15.15) | 8.35 (8.28-8.51) | 1.14-1.33 GB | 152,284 KiB, 1 file (replaced) | 29.8 / 40.0 / 63.1 |
| `index --if-stale`, no change | 0 (`state:"fresh"`) | 1.44 (0.28-1.84) | 0.32 (0.23-0.34) | 0.50 GB | unchanged | 28.5 / 55.3 / 66.0 |
| re-index, one-file change | 155,933,358 / 155,933,455 / 155,933,552 | 7.23 (5.43-13.99) | 8.64 (8.08-8.92) | 1.23-1.36 GB | 304,564 KiB, 2 files | 28.5 / 62.9 / 63.7 |

`small` (21 files):

| Operation | Snapshot bytes written | Wall s | CPU s | Load (1-min) |
|---|---|---|---|---|
| cold `index` | 246,538 / 246,635 / 246,732 | 0.25 (0.19-1.42) | 0.14 (0.14-0.37) | 37.9-38.9 |
| re-index, no change | same as cold, full rewrite | 0.24 (0.21-1.40) | 0.15 (0.14-0.32) | 37.9-38.9 |
| `index --if-stale`, no change | 0 (`state:"fresh"`) | 0.09 (0.08-0.53) | 0.05 (0.05-0.10) | 37.9-38.9 |
| re-index, one-file change | 246,635 / 246,732 / 246,829 | 0.26 (0.23-1.29) | 0.14 (0.14-0.34) | 37.9-38.9 |

Byte counts differ between runs only by the appended comment lines (97-111 bytes each).

## Findings

1. Confirmed and larger than filed: on this repository every plain `index` writes the whole
   snapshot, 155.9 MB, not 63 MB (the 63 MB figure predates the repository's growth; it was
   153.7 MB on 2026-10-15). A one-file change costs the same CPU as a cold build (8.6 against
   8.1 CPU s) and leaves a second full file in the shared store (152,284 to 304,564 KiB by
   `du -sk`, 155.9 to 311.9 MB) until the `IDX-SNAP-V0-007` bound evicts it.
2. A no-change plain `index` also rebuilds and rewrites the full file: same size, a different
   sha256 every time (gob map order is not deterministic, as `IDX-SNAP-V0-022` records). That is
   the specified behaviour: `IDX-SNAP-V0-011` says "Without the flag `index` is unchanged".
3. The no-change skip already exists and is the only automatic path: every hook, adapter and
   refresh caller runs `index --if-stale` (`cmd/corvint/host_adapter.go:668`, `:803`,
   `cmd/corvint/index_refresh_unix.go:51`). It writes 0 bytes and takes 0.32 CPU s against
   8.35 on `repo` (26x less CPU, 0.50 GB RSS against 1.1-1.3 GB).

## Decision

No code change in this lane: no improvement that keeps the encoding and the `index` contract
was qualified here.

- Making plain `index` skip an existing matching snapshot would amend `IDX-SNAP-V0-001`/`-011`
  (plain `index` always rebuilds; the operator's way to force a rewrite) and the frozen `index`
  verb contract, and saves nothing on the automatic path, which already skips. That is an owner
  decision, not a lane change.
- Keeping the previous file for an unchanged tree is not ruled out as unsafe: `IDX-SNAP-V0-022`
  requires equal decoded indexes, not equal gob bytes. But the files themselves differ between
  writes of the same tree, so the lane's "identical index output bytes" proof is not available
  for it, and it is the plain-`index` skip above under another name.
- Writing fewer bytes on a one-file change (reusing unchanged blob facts, not storing bodies, a
  lazily loaded pack) changes the snapshot encoding, which invariant 7 puts behind the
  `deployment-neutral-index-platform-v0.md` format gate. The design is
  `2026-10-06-v1-0328-large-repo-index-design.md`; its per-blob shard variant was falsified on
  latency (decision 0073).

Since no improvement was proposed, there is no BASE-against-fixed benchmark. The measurement
above is the retained evidence for the ticket's first acceptance clause.

Owner decision needed (unchanged from 2026-10-15): close V1-0328 on this measurement, or
schedule the 2026-10-06 format design under its own gate.

## Commands

```sh
GOTOOLCHAIN=local go build -buildvcs=false -o BIN ./cmd/corvint
# per run, for MODE in cold nochange ifstale onefile:
#   cold:    rm -rf SUBJECT/.git/corvint/index SUBJECT/.corvint/index
#   onefile: echo "// measure ..." >> SUBJECT/FILE; git -C SUBJECT commit -qam "measure one-file change"
/usr/bin/time -l BIN --root SUBJECT index            # or: index --if-stale
sysctl -n vm.loadavg; du -sk SUBJECT/.git/corvint/index; shasum -a 256 <receipt path>
```

FILE was `internal/contextindex/snapshot.go` for `repo` and `gokernel/errors.go` for `small`.

## Review

One Codex round (read-only). P2: the first draft ruled out reusing the previous file because gob
bytes differ, but `IDX-SNAP-V0-022` compares decoded indexes; the decision now says no improvement
was qualified rather than that none is safe. P3: the store sizes are `du -sk` KiB, now labelled,
with the MB figures corrected. No P1. Not run: `make gate` and package tests (the change is this
entry only; `corvint affected` selected 0 units).
