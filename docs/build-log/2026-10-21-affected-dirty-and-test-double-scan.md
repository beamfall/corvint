# V1-2073 and V1-1880: two `affected` and `corvint test` spikes

These are spike tickets for Corvint 1.0. Decisions are delegated to the orchestrator (decision
0548). Base is `90ab9f33e` (build 697).

## V1-2073: `plan.dirty` lists committed range paths on a clean worktree

**Finding.**

- AFP-V0-010 ([affected-plan-v0](../specs/affected-plan-v0.md)) defines `plan.dirty` as the union of:
  - the committed range `--base..HEAD`;
  - the worktree dirty set.
- `range.paths` names the committed part, so a consumer can separate the two.
- `UNOWNED_DIRTY_PATH` (`internal/liveverify/affected/select.go`) applies to any path in that union
  that no plugin claims.

The behaviour the ticket reports is therefore the specified contract. Only the name surprised the
reporter.

**Proposed recommendation.** Keep both field names and change the help text.

- `affected` is a frozen Core verb (CCF-V1-001), so renaming `dirty` or the reason code breaks
  consumers.
- The data needed to separate committed paths already exists in `range.paths`.

**Change.** `corvint help affected` now says that "dirty" means "changed since the base". It also
says a clean worktree still lists its committed range paths there, and that `range.paths` names
them. `TestAffectedHelpTopicsAreWired` pins this wording.

## V1-1880: `corvint test` reads the repository twice

**Finding.** `corvint test` builds two independent views of the repository:

- **The affected Go graph.** `observeDirectories` reads files in parallel and parses imports only.
- **The gate selector.** `gateselect.indexRepository` (`internal/gateselect/readers.go`) walks the
  tree serially, reads each file within a bound, parses imports, and then tokenizes literals with
  `scanSource`. Its only consumer is `cmd/corvint/test.go`.

**Measurements.**

- *Whole command.* `corvint test --dry-run` on this repository took 13.17, 12.67 and 10.27 s wall
  time, about 5.7 s user, at host load about 8.
- *Selector scan alone.* A temporary benchmark (not committed) ran over the 4,673 `.go` files at host
  load about 66, which spiked during the run:
  - `indexRepository`: median 1.289 s;
  - the raw walk and read of the same files: median 0.230 s, about 18 % of the scan.

**Proposed recommendation.** Do not share the reads.

- At most about 0.2 s of I/O could be saved.
- The two passes do different work: the selector needs full tokenization, while the graph needs
  imports only.
- Sharing would couple the fail-closed gate selector to the affected graph's admission rules.

**Follow-up.** The serial `scanFile` CPU is the larger cost. Parallelize it with a bounded worker
pool and an ordered merge, as `observeDirectories` already does. Require byte-identical plans before
and after, and measure medians under a quiet host before claiming a saving.
