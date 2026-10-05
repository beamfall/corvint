## 2026-10-04 V1-0780: command results state their retryability

Human-owned intent: ticket V1-0780, the owner's follow-up to issue 494. Under about 8,900 receipts
and 13 concurrent sessions, lease renew and heartbeat returned `ERROR`/`LOCK_TIMEOUT` while the
lease was FRESH and the client runner killed healthy runs. The client now pattern-matches a private
list (`LOCK_TIMEOUT`, `SNAPSHOT_MOVED`, `STALE`, `REDO_PENDING`, `STORAGE_FAILED`, `HEAD_MOVED`,
`JOURNAL_SATURATED`). The owner asked Tasks to document the retryable codes and mark each result.

Requirement: `CAL-V0-078` and amendment A19 in `docs/specs/corvint-tasks-agent-leases-v0.md`
("V1-0780 retryable result amendment"). `CAL-V0-074..077` are taken by the concurrent V1-0755,
V1-0756 and V1-0781 branches, so this change uses the first ID none of them claims.

### Decision

- One table, `wire.RetryOf` (`internal/tasks/wire/retry.go`), classifies all 71 §11 codes from
  their in-tree producers, not their names. Retryable: `LOCK_TIMEOUT`, `SNAPSHOT_MOVED`,
  `REDO_PENDING`; every producer reports them before anything is decided or written. Everything
  else is false with a recorded reason: fencing codes always; mixed, capacity or uncertain codes
  (`LIMIT_EXCEEDED`, `JOURNAL_SATURATED`, `UNSUPPORTED_FILESYSTEM`) by the fail-closed rule; 19
  reserved codes because they have no producer.
- The `taskman-command-result/0` envelope gains an absent-only optional boolean `retryable` on a
  non-`OK` result with codes, true only when every code is retryable. `OK` and uncoded envelopes
  keep their bytes, decoders accept legacy coded bytes, so no profile bump. Precedent: A18's
  absent-only `handoffEvidence`.
- `Result.NotRetryable` lets a command report false despite retryable codes. `attempt run` sets it
  once its child has run, because an unrecorded outcome (`ERROR`/`LOCK_TIMEOUT`, exit 127) would
  otherwise invite a retry that runs the child again.
- `STALE`, `STORAGE_FAILED` and `HEAD_MOVED` from the client list are not §11 codes and cannot be
  classified; `JOURNAL_SATURATED` is deliberately not retryable.

### Limits and open questions

- A consumer built before this change that decodes the closed envelope exactly refuses coded
  non-`OK` envelopes carrying the member. No in-tree closed reader consumes such envelopes.
- `SNAPSHOT_MOVED` keeps three input-mismatch producers (release candidate head, attestation
  candidate, criterion capture wrapping a refused read) that repeat until the caller's input
  changes; recoding them is an owner decision.
- The transient `LIMIT_EXCEEDED` cases (64 live preparation slots, active-attempt cap) stay not
  retryable until split into their own code.

Evidence: `TestCALV0078_*` in `internal/tasks/wire` and `internal/tasks/cli`; focused package runs
recorded in the change evidence.
