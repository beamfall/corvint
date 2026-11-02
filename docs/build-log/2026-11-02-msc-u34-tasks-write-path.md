# 2026-11-02 msc-u34 tasks write path (V1-1148, V1-4229)

Decision: the owner lifted the V1-0910 cache hold on 2026-11-01 ("you can lift it"), relayed by the
orchestrator for lane msc-u34 round 2. For this scope that supersedes the 2026-10-24 rejection
recorded in `docs/build-log/2026-10-24-owner-held-decisions.md`, which had kept decision 0549 F1. A
write-path or in-process structure is admitted; a persisted index needs its own derived-state
argument in the spec, and read commands still persist nothing (invariant 4).

BASE `35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b`; host unraid-lanes (12 CPUs, btrfs, Go 1.27.3).
Lane tkp-u33 is editing `internal/tasks/store`, journal and wire on the same host (V1-4217,
V1-3275, V1-4243: 49 files, including `writer_checkpoint.go`, the segmented journal audit and
`wire/parse.go`/`encode.go` fast paths).

## Before numbers at BASE

`ticket create` through `tasks-scale run`, one cold process per sample, on gen-built stores:

| Store | wall ms | user ms | sys ms | allocated | load (1 min) |
|---|---|---|---|---|---|
| 100 tickets | 218, 224, 222 | 70, 68, 66 | 28, 35, 31 | 26.4-27.0 MB | 2.3-3.3 |
| 500 tickets | 410, 275, 277 | 197, 205, 202 | 182, 42, 59 | 80.6-81.1 MB | 2.3-3.3 |

CPU grows about 0.33 ms and 135 KB per ticket, so a create at 2,000 tickets projects to roughly
700 ms of CPU. The rest of the wall time is off-CPU (durable writes under host I/O load).

`tasks-scale gen` at BASE:

- `--tickets 100`: 36.9 s (load 4.4).
- `--tickets 500`: 256.4 s (load 3.3 to 2.3); 181.8 s in round 1 at load 2.4 to 2.6.
- `--tickets 2000`, bounded at 530 s (the lane's per-command bound): 500 tickets at 187 s, 1,000
  at 519 s (the second 500 took 332 s), and killed at 530 s with 1,014 tickets written. The full
  run is `NOT_OBSERVED`. The 500-ticket chunks grow roughly 187, 332, ... s, so the total projects
  to well over 20 minutes.

## Where a create's CPU goes at 500 tickets

Four creates, 1.40 s of samples:

| Pass | Package | Share |
|---|---|---|
| `ticket.Decode` in total, of which: | `ticket`/`wire` | 38.6% |
| - CLI preload `intent.Load` (`decodeTree`) | `cli` → `intent` | 16.4% |
| - model `transaction.loadTicket` | `transaction` | 17.1% |
| `Record.FileDigest` re-encode in `loadTicket` | `transaction` | 10.7% |
| writer journal audit `AuditForWriter` | `store` → `journal` | 15.7% |
| closed-index rewrite `WriteClosedIndex` (`LoadTreeIndexed` decodes open tickets) | `cli` → `intent` | 10.0% |

Every write runs five passes over every ticket: the CLI preload, the intent tree probe, the
writer's journal audit, the model decode and the closed-index rewrite. A create cost that differs
by under 20% between 500 and 2,000 tickets needs all of them gone or flattened, not only the
model's.
