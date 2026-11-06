# pk-affected self-review repair

The self-review found three incomplete behaviors: completion headers were read
without affecting retention (V1-4472 / TSP-V0-7305), the stale-binary notice sat
outside the dry-run summary budget (V1-3424 / STR-V0-7313), and a mixed direct /
unbounded by-path selection discarded the real unbounded cause (AFP-V0-990 /
TSP-V0-7311).

Retention decision: probe the newest 64 mtime candidates plus the unsharded
`test-run.json` when outside that prefix, then keep the newest 64 by bounded
completion time, falling back to mtime. The maximum is 65 header probes of
64 KiB and 64 canonical receipt reads. This admits the latest unsharded run
under equal mtimes without unbounded content scanning; completion recency for
older shards outside the pool is explicitly unclaimed. The optional shard capture in the existing receipt-name parser identifies
the unsharded candidate, preserving the receipt-reader site inventory. The first
whole-package run rejected a duplicate receipt-name literal in this reader; the
inventory assertion was retained. The existing 65-shard
probe-bound assertion remains unchanged. A new equal-mtime regression uses
unequal completion times and requires the unsharded run to displace the oldest
completion. Rollback restores mtime-only retention, losing that qualification.

The summary reserves the unchanged stderr notice's byte count before rendering
stdout. The command-entry regression includes an actual stale build stamp and
a wide repository plan, covering the notice omitted by direct renderer tests.
The by-path projection retains the first actual per-path unbounded witness,
including representative dirty path and cause, rather than reconstructing it
from a directly reached union selection. The mixed-path regression compares
the whole compact witness to the individual selection. No wire shape, stderr
text, exit code, golden or existing test assertion was changed.

Verification results, failed fixture evaluations, external sandbox limitations
and final binding identities are retained in the lane handback and external
`pk-affected-evidence/self-review-repair/` directory. Pre-change captures are
reused. Independent review and repository-wide checks belong to the host
orchestrator; this repair does not claim their execution or native ticket closure.
