# Design: run-evidence

## Decisions

1. **Store layout.** `<git-dir>/corvint/evidence/` holds:
   - `records/<sha256>.json`: the exact bytes of one run document. The file name is the lowercase
     hex SHA-256 of the bytes. A record is written once (temporary file, then rename) and never
     edited.
   - `log.jsonl`: one line per appended record, in write order:
     `{"id":"<sha256>","profile":"...","tree":"<tree>","commit":"<sha>","instances":[...],"writtenAt":"<RFC 3339>"}`.
     Appends take an exclusive lock on `log.lock`.
   The store lives under the git directory, so it is never committed and never in the worktree.
   Source: owner decision R7 (2026-11-09); review T-B2 and NEW-testing-6.

2. **Record ID.** The record ID is the SHA-256 of the record file bytes. It equals the digest
   `corvint-tasks receipt put FILE` stores under `<state>/witness-receipts/<sha256>`, so the same
   file can be handed to Tasks as acceptance evidence without conversion. The Tasks side is in the
   Tasks repository and is not checked in integ 9ec3a29d0.

3. **Record content.** A record is either a `corvint-test-run/0` receipt written by `corvint test`
   or a `test-run-evidence/0` document written by `flows ingest`. The reader normalises both to rows
   of: canonical test ID (CTI-V1-009), outcome, `durationMs` when measured (PTD-V1-001), runner
   instance (TIV-V1-003), tree and commit. A run writes one row per test it ran; a file-granularity
   runner writes one row per file, marked `file`.

4. **Writers.** `corvint test` appends one record after it writes its receipt; `corvint watch`
   appends when it runs tests through the same path; the GitHub Action runs `corvint test`, so it
   appends the same way. `flows ingest --record` appends the converted document; without
   `--record` it prints to stdout as today. No read command writes the store (product invariant 4).

5. **Readers.** `corvint flows status`, `flows map`, `flows gaps`, `flows impact`,
   `corvint test-evidence` and the matching MCP tools read the store by default. `--evidence FILE`
   adds files to what the store gives; `--no-store` reads only the named files. The last result for a
   test is the row from the newest record that contains it; reports show that record's tree and say
   when it differs from HEAD.

6. **Retention and cap.** After an append, the writer keeps, for every test, the newest 20 records
   that contain it, and removes a record no test still needs. Total store size is capped (proposed
   64 MiB, Open 1); when an append would exceed it, the oldest records go first. Only writers prune.

7. **Corruption.** A record whose bytes do not hash to its name, or that does not parse, is skipped;
   readers show the affected tests as `NOT_OBSERVED` with reason `run-evidence-corrupt`. A
   `log.jsonl` that cannot be read is rebuilt by the next writer from `records/`; until then readers
   list records by file time and print `run-evidence-corrupt`.

8. **Write failure.** If the append fails, `corvint test` prints `run-evidence-write-failed` with the
   cause and keeps the exit code of the test run. Whether a failed append should change the exit
   code is Open 2.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Store missing | Readers report no stored runs; tests show `NOT_OBSERVED`. |
| Record hash does not match its name | Skipped, `run-evidence-corrupt`. |
| `log.jsonl` unreadable | Next writer rebuilds it; readers fall back to file order and print `run-evidence-corrupt`. |
| Two writers at once | The lock serialises appends; both records are kept. |
| Store over its cap | Oldest records pruned by the writer; nothing is pruned by a reader. |
| Append fails | `run-evidence-write-failed` printed; test exit code unchanged. |
| Newest result is from another tree | Report shows the result and its tree, marked as not HEAD. |
| Legacy receipt without `testId` | Read at unit granularity (CTI-V1-012). |

## Acceptance evidence

- The PROD-TEST-020 scenario as an end-to-end test: `corvint test` on a fixture, then
  `corvint flows status` with no `--evidence` shows that run's outcomes.
- Unit tests: hash naming, append under lock with two concurrent writers, retention at 21 runs,
  size cap, corrupt record, unreadable log.
- A test that `corvint-tasks receipt put` of a stored record returns the record's file name as its
  digest (run where the Tasks binary is installed; otherwise recorded `NOT_RUN`).
- A read-only test: `flows status` and `test-evidence` leave the store unchanged.

## Rollback

Revert the change's commits. `corvint test` then writes only its receipt, readers use `--evidence`
again, and `<git-dir>/corvint/evidence/` can be deleted by hand with no effect on any committed
state. RVD-V1-001 to RVD-V1-012 stay listed as retired if the change is withdrawn.

## Open

1. The size cap number (64 MiB is proposed).
2. Whether a failed append should change the exit code of `corvint test`.
3. Whether `flows ingest` appends by default or only with `--record` (proposed: only with
   `--record`, so its current stdout behaviour is unchanged).
