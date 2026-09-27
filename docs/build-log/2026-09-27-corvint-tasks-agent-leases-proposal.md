## 2026-09-27 Corvint Tasks agent leases: proposed route for corvint-tasks to take over Beamfall's runner

The owner asked for corvint-tasks to reach a level where it can replace Beamfall's
`script/roadmap.sh`, then asked which execution model is the better long-term one. This entry
records the proposal, `docs/specs/corvint-tasks-agent-leases-v0.md`; nothing in it is accepted.

Findings.

- `roadmap.sh` (14,532 lines of Bash plus Python helpers) is driven by the agents themselves: `next`,
  a `mkdir` lease from `claim`, the repository gate engine, `merge` when safe, `checkoff`, and `reap`
  for crashed holders. Claude and Codex run the identical loop, and it is the only sanctioned writer
  of Beamfall's roadmap shards. This comes from a read-only survey of the Beamfall repository, not
  from running the runner.
- TCP-00 binds execution to a supervisor that forks a `lane-leader` for each attempt, with a
  `.boot`/`.ack` handshake and process-group liveness (§6.2 to §6.4). The TCP-00 slices that would
  build it (TCP-02 remainder, TCP-03 to TCP-06 and TCP-09) are not started or incomplete in tree.
  The in-tree writer requires an empty reservation set and refuses every non-fixture queue.

Decision proposed. Keep TCP-00's attempt, generation, reservation and receipt records, and replace
only the spawn and liveness layer with an `external-agent` runtime whose liveness is a lease the
agent renews. The generation fences a holder that lost its lease. A supervisor can later be added
as an optional spawner for fanout, over the same records, rather than as a precondition. The
reasons given to the owner: it matches how the queues' agents work today, it fits invariants 6 and
7 (integrate with existing agents; no permanent daemon), and it keeps host-version spawn adapters
off the takeover path while host lifecycle qualification (V1-0016) is open. These are reasoning,
not measurement.

Set aside: building the full TCP-00 supervisor first (weeks of slices before any switch, and a
component the queues' agents do not need), and a lease path separate from TCP-00's records that is
later retired (two state machines to keep consistent).

The proposal has seven slices: a non-fixture writer (V1-0398), cutover of imported records to
native, leases, plan preview, gates and completion, a linear first import (the follow-up named in
the CTS-V0-003 entry), and a qualification suite with the execution cutover record. It amends
TCP-00 (A8 to A11). Switching Beamfall's agents stays a separate owner-run step in that repository.
