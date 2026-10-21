# Lane ctx-1021: context/impact name a slow snapshot miss; exact generic-import prefilter

Date: 2026-10-21. Ticket: V1-1640 (plus triage of the context/index/impact sweep). Base: origin/main
c9ca8beff. Requirements `IDX-SNAP-V0-102` and `IDX-SNAP-V0-103` are accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## Problem

V1-1640: `corvint context` and path `impact` without a snapshot for the current tree rebuild the
whole index in memory on every call (the ticket measured 22.8 s and 813 MB per call on this
repository). Nothing on the read path said so, and `docs/DOGFOOD.md` did not tell a fresh checkout
to run `corvint index` once.

## Decisions

- `IDX-SNAP-V0-102`: a miss that pays an in-memory build of at least one second writes one stderr
  line naming the verb, the build time and the exact `corvint --root ROOT index` argv. Stdout, the
  exit code and the snapshot store are unchanged. Read commands still never write a snapshot
  (product invariant 4); the notice tells the operator to run the one writer.
- `IDX-SNAP-V0-103`: the generic-import regex now runs only on bodies where a `from`/`import`
  keyword reaches a quote through the regex's own opening shape (`mayHoldGenericImport`). The old
  gate admitted any body holding a quote and either keyword. The new gate is exact on the opening, so
  it cannot drop a match. Extracted imports, and therefore index content, are unchanged.
  `script/analyzer-schema.sh bump` raised the analyzer schema and repinned the use-case receipts.
- The notice covers `context`, `impact` and `feature`. Other snapshot readers (`query`, MCP context,
  calibrate, prove) are left as a residual.

## Evidence

- `TestSnapshotMissNoticeNamesTheBuild` (cmd/corvint): a build under the threshold is silent; at
  threshold 0, context and impact each print exactly one notice, context stdout is unchanged and no
  store file is written; after `index` a hit is silent. Fails with the notice call removed.
- `TestGenericImportPrefilterLosesNoMatch` and `FuzzGenericImportPrefilter` (internal/contextindex):
  a rejected body never has a regex match. A 60 s fuzz run: 1.65M execs, no failure. Removing
  `\p{Z}` from the space class fails the table.
- Parity over 11,870 tracked files of this repository: identical extracted imports, base vs new.
  The gate now skips 7,099 bodies (was 3,893); generic regex time over the corpus fell from 2.27 s to
  1.16 s. `corvint index` user CPU: base 6.6-7.0 s, new 5.79-6.01 s at host load 40-80. Snapshot gob
  bytes are not deterministic between two runs of one binary, so identity was proven by
  import-extraction parity rather than by comparing snapshot files.

## Triage of the other sweep tickets (no code)

- V1-1285 already delivered: at 74fc3378d (tree 000141a0) a task naming STR-V0-008/013 returns a
  packet at the HEAD tree with `docs/specs/session-test-runner-v0.md` included ("defines
  STR-V0-008"). Snapshots are keyed by exact tree, so a stale one is never served.
- Reproduced, not fixed this lane: V1-1766 (`/testdata/` and `/internal/` anchored lone components
  and short fragments like `s/p` place a new testdata `policy.json` under 11 readers), V1-0923 (a new
  build-log file is still `UNOWNED_DIRTY_PATH`), V1-0922 (`interop/cem01-go/cem.go` is still
  `UNINDEXED_SOURCE_PATH`) and V1-1010 (a ticket ID in the task does not anchor its ticket).

## NOT_RUN

`make gate`, as lane policy says. Context evaluation gates were not run because ranking is unchanged.

## Rollback

Revert the lane commit. The notice and prefilter are independent of each other. Reverting the
prefilter needs the analyzer schema back at its prior value, which the revert restores.
